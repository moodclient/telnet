package telnet

import (
	"fmt"
	"strconv"
	"strings"
)

type TelOptUsage byte

// There's no situation where we'd want to request usage of a telopt but not allow the remote to
// propose it, so the TelOptRequestRemote/Local exposed to consumers includes both flags

const (
	TelOptAllowRemote TelOptUsage = 1 << iota
	telOptOnlyRequestRemote
	TelOptAllowLocal
	telOptOnlyRequestLocal
)

const (
	TelOptRequestRemote TelOptUsage = TelOptAllowRemote | telOptOnlyRequestRemote
	TelOptRequestLocal  TelOptUsage = TelOptAllowLocal | telOptOnlyRequestLocal
)

type TelOptCode byte

type TypedTelnetOption[OptionStruct any] interface {
	*OptionStruct
	TelnetOption
}

type TelnetOption interface {
	Initialize(terminal *Terminal)
	Terminal() *Terminal

	// LocalState returns the current state of this option locally- receiving a DO command will activate
	// it and a DONT command will deactivate it
	LocalState() TelOptState
	// RemoteState returns the current state of this option in the remote- receiving a WILL command
	// will activate it and a WONT command will deactivate it
	RemoteState() TelOptState

	// Code returns the code this option should be registered under. This method is expected to run succesfully
	// with an uninitialized option
	Code() TelOptCode
	// String should return the short name used to refer to this option. This method is expected to run
	// successfully with an uninitialized option
	String() string
	// Usage indicates the way in which this TelOpt is permitted to be used.
	Usage() TelOptUsage

	// TransitionLocalState is called when the terminal attempts to change this option to a new state
	// locally.  This is not called when the option is initialized to Inactive at the start of a new
	// terminal, and it will not be called if the terminal tries to repeatedly transition this option
	// to the same state.
	TransitionLocalState(newState TelOptState) error
	// TransitionRemoteState is calledw hen the terminal attempts to change this option to a new state
	// for the remote.  This is not called when the option is initialized to Inactive at the start of
	// a new terminal, and it will nto be called if the terminal tries to repeatedly transition this
	// option to the same state
	TransitionRemoteState(newState TelOptState) error

	// Subnegotiate is called when a subnegotiation request arrives from the remote party. This will only
	// be called when the option is active on one side of the connection
	Subnegotiate(subnegotiation []byte) error
	// SubnegotiationString creates a legible string for a subnegotiation request
	SubnegotiationString(subnegotiation []byte) (string, error)
	// EventString creates legible strings for eventdata contents
	EventString(eventData TelOptEventData) (eventName string, payload string, err error)
}

type TelOptState byte

const (
	// TelOptUnknown is the zero value for the telopt state value.  This is generally interchangeable with
	// TelOptInactive
	TelOptUnknown TelOptState = iota
	// TelOptInactive indicates that the option is not currently active
	TelOptInactive
	// TelOptRequested indicates that this client has sent a request to activate the telopt to the other party
	// but has not yet heard back
	TelOptRequested
	// TelOptActive indicates that the
	TelOptActive
)

func (s TelOptState) String() string {
	switch s {
	case TelOptInactive:
		return "Inactive"
	case TelOptRequested:
		return "Requested"
	case TelOptActive:
		return "Active"
	default:
		return "Unknown"
	}
}

type telOptStack struct {
	options map[TelOptCode]TelnetOption
}

func newTelOptStack(terminal *Terminal, options []TelnetOption) (*telOptStack, error) {

	optionMap := make(map[TelOptCode]TelnetOption)

	for _, option := range options {
		oldOption, hasOldOption := optionMap[option.Code()]
		if hasOldOption {
			return nil, fmt.Errorf("telopt collision: TelOpt %d is already registered to an option of type %T. it cannot be registered to an option of type %T", option.Code(), oldOption, option)
		}

		option.Initialize(terminal)
		optionMap[option.Code()] = option
	}

	return &telOptStack{
		options: optionMap,
	}, nil
}

func (s *telOptStack) rejectNegotiationRequest(terminal *Terminal, c Command) {
	if c.IsNegotiationRequest() {
		terminal.Keyboard().WriteCommand(c.Reject())
	}
}

func (s *telOptStack) processSubnegotiation(c Command) error {
	option, hasOption := s.options[c.Option]
	if !hasOption {
		// Getting subnegotiations for stuff we haven't agreed to
		return nil
	}

	if option.LocalState() != TelOptActive && option.RemoteState() != TelOptActive {
		// Getting subnegotiations for stuff we haven't agreed to
		return nil
	}

	return option.Subnegotiate(c.Subnegotiation)
}

func (s *telOptStack) WriteRequests(terminal *Terminal) error {
	for _, option := range s.options {
		usage := option.Usage()
		if usage&telOptOnlyRequestLocal != 0 {
			terminal.Keyboard().WriteCommand(Command{
				OpCode: WILL,
				Option: option.Code(),
			})

			oldState := option.LocalState()

			if oldState == TelOptInactive {
				err := option.TransitionLocalState(TelOptRequested)
				if err != nil {
					return err
				}

				option.Terminal().teloptStateChange(option, TelOptSideLocal, oldState)
			}
		}

		if usage&telOptOnlyRequestRemote != 0 {
			terminal.Keyboard().WriteCommand(Command{
				OpCode: DO,
				Option: option.Code(),
			})

			oldState := option.RemoteState()

			if oldState == TelOptInactive {
				err := option.TransitionRemoteState(TelOptRequested)
				if err != nil {
					return err
				}

				option.Terminal().teloptStateChange(option, TelOptSideRemote, oldState)
			}
		}
	}

	return nil
}

func (s *telOptStack) ProcessCommand(terminal *Terminal, c Command) error {
	if c.OpCode == SB {
		return s.processSubnegotiation(c)
	}

	// It's not a negotiation command
	if c.OpCode != DO && c.OpCode != DONT && c.OpCode != WILL && c.OpCode != WONT {
		return nil
	}

	// Is this an option we know about?
	option, hasOption := s.options[c.Option]
	if !hasOption {
		// Unregistered telopt
		s.rejectNegotiationRequest(terminal, c)

		return nil
	}

	oldState := option.RemoteState()
	side := TelOptSideRemote
	transitionFunc := option.TransitionRemoteState
	allowFlag := TelOptAllowRemote
	if c.IsRequestForLocal() {
		oldState = option.LocalState()
		side = TelOptSideLocal
		transitionFunc = option.TransitionLocalState
		allowFlag = TelOptAllowLocal
	}

	// They are requesting WONT/DONT
	if !c.IsNegotiationRequest() && oldState == TelOptInactive {
		// already turned off
		return nil
	} else if !c.IsNegotiationRequest() {
		// need to turn it off
		err := transitionFunc(TelOptInactive)
		if err != nil {
			return err
		}

		option.Terminal().teloptStateChange(option, side, oldState)

		return nil
	}

	// They are requesting DO/WILL
	if oldState == TelOptActive {
		// Already turned on
		return nil
	}

	if option.Usage()&allowFlag == 0 {
		// Disallowed telopt
		s.rejectNegotiationRequest(terminal, c)

		return nil
	}

	if oldState == TelOptInactive {
		// Need to send an accept command
		terminal.Keyboard().WriteCommand(c.Accept())
	}

	err := transitionFunc(TelOptActive)
	if err != nil {
		return err
	}

	terminal.teloptStateChange(option, side, oldState)
	return nil
}

func (s *telOptStack) CommandString(c Command) string {
	var sb strings.Builder
	sb.WriteString("IAC ")

	opCode, hasOpCode := commandCodes[c.OpCode]
	if !hasOpCode {
		opCode = strconv.Itoa(int(c.OpCode))
	}

	sb.WriteString(opCode)

	if c.OpCode == GA || c.OpCode == NOP || c.OpCode == EOR {
		return sb.String()
	}

	sb.WriteByte(' ')

	option, hasOption := s.options[c.Option]

	if !hasOption {
		sb.WriteString("? Unknown Option ")
		sb.WriteString(strconv.Itoa(int(c.Option)))
		sb.WriteString("?")
	} else {
		sb.WriteString(option.String())
	}

	if c.OpCode != SB {
		return sb.String()
	}

	sb.WriteByte(' ')

	if !hasOption {
		sb.WriteString(fmt.Sprintf("%+v", c.Subnegotiation))
	} else {
		str, err := option.SubnegotiationString(c.Subnegotiation)

		if err != nil {
			sb.WriteString(fmt.Sprintf("%+v", c.Subnegotiation))
		} else {
			sb.WriteString(str)
		}
	}

	sb.WriteString(" IAC SE")
	return sb.String()
}

func GetTelOpt[OptionStruct any, T TypedTelnetOption[OptionStruct]](terminal *Terminal) (T, error) {
	var zero OptionStruct
	var err error
	code := T(&zero).Code()

	option, hasOption := terminal.telOptStack.options[code]

	if !hasOption {
		return nil, nil
	}

	typed, ok := option.(T)
	if !ok {
		name := T(&zero).String()
		return nil, fmt.Errorf("TelOpt %s did not return type %T- it returned type %T", name, zero, option)
	}

	return typed, err
}