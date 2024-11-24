package telnet

import (
	"errors"
	"fmt"
)

type TelOptCode byte
type TelOptFactory func(terminal *Terminal) TelnetOption

var ErrOptionCollision = errors.New("telopt: option collision")
var ErrOptionUnknown = errors.New("telopt: unknown option")

var telOptLibrary map[TelOptCode]TelOptFactory = make(map[TelOptCode]TelOptFactory)

func RegisterOption[OptionStruct any, T TypedTelnetOption[OptionStruct]](factory TelOptFactory) error {
	var zero OptionStruct
	telnetOpt := T(&zero)

	code := telnetOpt.Code()
	name := telnetOpt.String()

	oldName, hasOldOption := telOptLibrary[code]
	if hasOldOption {
		return fmt.Errorf("%w: could not register option %s because code %d is occupied by option %s", ErrOptionCollision, name, code, oldName)
	}

	telOptLibrary[code] = factory
	return nil
}

type TelOptCache struct {
	options  map[TelOptCode]TelnetOption
	terminal *Terminal
}

func newTelOptCache(terminal *Terminal) *TelOptCache {
	return &TelOptCache{
		options:  make(map[TelOptCode]TelnetOption),
		terminal: terminal,
	}
}

func (c *TelOptCache) get(code TelOptCode) (TelnetOption, error) {
	option, hasOption := c.options[code]
	if hasOption {
		return option, nil
	}

	factory, hasInLibrary := telOptLibrary[code]
	if !hasInLibrary {
		return nil, fmt.Errorf("%w: could not find option %d", ErrOptionUnknown, code)
	}

	option = factory(c.terminal)
	c.options[code] = option
	return option, nil
}

func GetTelOpt[OptionStruct any, T TypedTelnetOption[OptionStruct]](cache *TelOptCache) (T, error) {
	var zero OptionStruct
	var err error
	code := T(&zero).Code()

	option, err := cache.get(code)

	if err != nil {
		return nil, err
	}

	typed, ok := option.(T)
	if !ok {
		return nil, fmt.Errorf("factory for TelOpt %s did not return type %T- it returned type %T", zero, zero, option)
	}

	return typed, err
}

type TypedTelnetOption[OptionStruct any] interface {
	*OptionStruct
	TelnetOption
}

type TelnetOption interface {
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
}

type TelOptPreferences struct {
	AllowRemote   []TelOptCode
	RequestRemote []TelOptCode
	AllowLocal    []TelOptCode
	RequestLocal  []TelOptCode
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

type telOptStack struct {
	cache *TelOptCache

	allowRemoteSet map[TelOptCode]struct{}
	allowLocalSet  map[TelOptCode]struct{}

	requestRemote []TelOptCode
	requestLocal  []TelOptCode

	awaitedRequests int
}

func newTelOptStack(library *TelOptCache, preferences TelOptPreferences) *telOptStack {
	allowRemote := make(map[TelOptCode]struct{})
	for _, telOpt := range preferences.AllowRemote {
		allowRemote[telOpt] = struct{}{}
	}
	for _, telOpt := range preferences.RequestRemote {
		allowRemote[telOpt] = struct{}{}
	}

	allowLocal := make(map[TelOptCode]struct{})
	for _, telOpt := range preferences.AllowLocal {
		allowLocal[telOpt] = struct{}{}
	}
	for _, telOpt := range preferences.RequestLocal {
		allowLocal[telOpt] = struct{}{}
	}

	return &telOptStack{
		cache: library,

		allowRemoteSet: allowRemote,
		allowLocalSet:  allowLocal,

		requestRemote: preferences.RequestRemote,
		requestLocal:  preferences.RequestLocal,
	}
}

func (s *telOptStack) rejectNegotiationRequest(terminal *Terminal, c Command) {
	if c.IsNegotiationRequest() {
		terminal.Keyboard().WriteCommand(c.Reject())
	}
}

func (s *telOptStack) processSubnegotiation(terminal *Terminal, c Command) error {
	option, err := s.cache.get(c.Option)

	if errors.Is(err, ErrOptionUnknown) {
		// Getting subnegotiations for stuff we haven't agreed to
		return nil
	} else if err != nil {
		return err
	}

	if option.LocalState() != TelOptActive && option.RemoteState() != TelOptActive {
		// Getting subnegotiations for stuff we haven't agreed to
		return nil
	}

	return option.Subnegotiate(c.Subnegotiation)
}

func (s *telOptStack) WriteRequests(terminal *Terminal) error {
	for _, request := range s.requestLocal {
		terminal.Keyboard().WriteCommand(Command{
			OpCode: WILL,
			Option: request,
		})
		option, err := s.cache.get(request)
		if err != nil {
			return err
		}
		oldState := option.LocalState()

		if oldState == TelOptActive {
			continue
		} else if oldState != TelOptRequested {
			s.awaitedRequests++
		}

		err = option.TransitionLocalState(TelOptRequested)
		if err != nil {
			return err
		}
	}

	for _, request := range s.requestRemote {
		terminal.Keyboard().WriteCommand(Command{
			OpCode: DO,
			Option: request,
		})
		option, err := s.cache.get(request)
		if err != nil {
			return err
		}

		oldState := option.RemoteState()

		if oldState == TelOptActive {
			continue
		} else if oldState != TelOptRequested {
			s.awaitedRequests++
		}

		err = option.TransitionRemoteState(TelOptRequested)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *telOptStack) ProcessCommand(terminal *Terminal, c Command) error {
	if c.OpCode == SB {
		return s.processSubnegotiation(terminal, c)
	}

	// It's not a negotiation command
	if c.OpCode != DO && c.OpCode != DONT && c.OpCode != WILL && c.OpCode != WONT {
		return nil
	}

	// Is this an option we know about?
	option, err := s.cache.get(c.Option)
	if errors.Is(err, ErrOptionUnknown) {
		// Unregistered telopt
		s.rejectNegotiationRequest(terminal, c)

		return nil
	}

	oldState := option.RemoteState()
	transitionFunc := option.TransitionRemoteState
	allowList := s.allowRemoteSet
	if c.IsRequestForLocal() {
		oldState = option.LocalState()
		transitionFunc = option.TransitionLocalState
		allowList = s.allowLocalSet
	}

	// They are requesting WONT/DONT
	if !c.IsNegotiationRequest() && oldState == TelOptInactive {
		// already turned off
		return nil
	} else if !c.IsNegotiationRequest() {
		if oldState == TelOptRequested {
			s.awaitedRequests--
		}

		// need to turn it off
		return transitionFunc(TelOptInactive)
	}

	// They are requesting DO/WILL
	if oldState == TelOptActive {
		// Already turned on
		return nil
	}

	_, isAllowed := allowList[c.Option]
	if !isAllowed {
		// Disallowed telopt
		s.rejectNegotiationRequest(terminal, c)

		return nil
	}

	if oldState == TelOptInactive {
		// Need to send an accept command
		terminal.Keyboard().WriteCommand(c.Accept())
	} else if oldState == TelOptRequested {
		s.awaitedRequests--
	}

	return transitionFunc(TelOptActive)
}
