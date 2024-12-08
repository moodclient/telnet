package telnet

import (
	"fmt"
)

// TelOptUsage indicates how a particular TelnetOption is supposed to be used by the
// terminal.  Whether it is permitted to be activated locally or on the remote, and
// whether we should request activation locally or on the remote when the Terminal launches.
type TelOptUsage byte

// There's no situation where we'd want to request usage of a telopt but not allow the remote to
// propose it, so the TelOptRequestRemote/Local exposed to consumers includes both flags

const (
	// TelOptAllowRemote - if the remote requests to activate this telopt on their side,
	// we will permit it
	TelOptAllowRemote TelOptUsage = 1 << iota
	telOptOnlyRequestRemote
	// TelOptAllowLocal - if the remote requests that we activate this telopt on our side,
	// we will comply
	TelOptAllowLocal
	telOptOnlyRequestLocal
)

const (
	// TelOptRequestRemote - we will request that the remote activate this telopt during
	// Terminal startup
	TelOptRequestRemote TelOptUsage = TelOptAllowRemote | telOptOnlyRequestRemote
	// TelOptRequestLocal - we will request that the remote allow us to activate this
	// telopt on our side during Terminal startup
	TelOptRequestLocal TelOptUsage = TelOptAllowLocal | telOptOnlyRequestLocal
)

// TelOptCode - each telopt has a unique identification number between 0 and 255
type TelOptCode byte

// TelnetOption is an object representing a single telopt within the currently-running
// terminal.  Each terminal has its own telopt object for each telopt it supports.
type TelnetOption interface {
	// Code returns the code this option should be registered under. This method is expected to run succesfully
	// before Initialize is called.
	Code() TelOptCode
	// String should return the short name used to refer to this option. This method is expected to run
	// successfully before Initialize is called.
	String() string
	// Usage indicates the way in which this TelOpt is permitted to be used. This method
	// is expected to run successfully before Initialize is called.
	Usage() TelOptUsage

	// Initialize sets the terminal used by this telopt and performs any other necessary
	// business before other methods may be called.
	Initialize(terminal *Terminal)
	// Terminal returns the current terminal. This method must successfully return nil
	// before Initialize is called.
	Terminal() *Terminal

	// LocalState returns the current state of this option locally- receiving a DO command will activate
	// it and a DONT command will deactivate it.
	LocalState() TelOptState
	// RemoteState returns the current state of this option in the remote- receiving a WILL command
	// will activate it and a WONT command will deactivate it
	RemoteState() TelOptState

	// TransitionLocalState is called when the terminal attempts to change this option to a new state
	// locally.  This is not called when the option is initialized to Inactive at the start of a new
	// terminal, and it will not be called if the terminal tries to repeatedly transition this option
	// to the same state.
	//
	// This method returns a simple callback method.  If that callback is not nil, then it will
	// be executed as soon as the command associated with this state change is written to the
	// keyboard stream (or immediately if no command is necessary).  This is vital for cases when
	// a telopt changes the semantics of outbound communications, since that semantic change needs
	// to take place immediately after we send the command indicating that we will be changing things.
	TransitionLocalState(newState TelOptState) (func() error, error)
	// TransitionRemoteState is calledw hen the terminal attempts to change this option to a new state
	// for the remote.  This is not called when the option is initialized to Inactive at the start of
	// a new terminal, and it will nto be called if the terminal tries to repeatedly transition this
	// option to the same state
	//
	// This method returns a simple callback method.  If that callback is not nil, then it will
	// be executed as soon as the command associated with this state change is written to the
	// keyboard stream (or immediately if no command is necessary).  This is vital for cases when
	// a telopt changes the semantics of outbound communications, since that semantic change needs
	// to take place immediately after we send the command indicating that we will be changing things.
	TransitionRemoteState(newState TelOptState) (func() error, error)

	// Subnegotiate is called when a subnegotiation request arrives from the remote party. This will only
	// be called when the option is active on at least one side of the connection
	Subnegotiate(subnegotiation []byte) error
	// SubnegotiationString creates a legible string for a subnegotiation request
	SubnegotiationString(subnegotiation []byte) (string, error)
}

// TelOptState indicates whether the telopt is currently active, inactive, or other
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
	// TelOptActive indicates that the option is currently active
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

// TelOptSide is used to distinguish the two "sides" of a telopt.  Telopts can be active
// on either the local side, the remote side, both, or neither.  As a result,
// the current state of a telopt needs to be requested for a particular side of the connection.
type TelOptSide byte

const (
	TelOptSideUnknown TelOptSide = iota
	TelOptSideLocal
	TelOptSideRemote
)

func (s TelOptSide) String() string {
	switch s {
	case TelOptSideLocal:
		return "Local"
	case TelOptSideRemote:
		return "Remote"
	default:
		return "Unknown"
	}
}

// TelOptEvent is an interface used for all TelOptEvents issued by anyone, both TelOptStateChangeEvent,
// which is issued by this terminal, and other events issued by telopts themselves
type TelOptEvent interface {
	// String produces human-readable text describing the event that occurred
	String() string
	// Option is the specific telopt that experienced an event
	Option() TelnetOption
}

// TelOptStateChangeEvent is a TelOptEvent that indicates that a single telopt has changed state
// on one side of the connection
type TelOptStateChangeEvent struct {
	TelnetOption TelnetOption
	Side         TelOptSide
	OldState     TelOptState
	NewState     TelOptState
}

func (e TelOptStateChangeEvent) Option() TelnetOption {
	return e.TelnetOption
}

func (e TelOptStateChangeEvent) String() string {
	return fmt.Sprintf("%s: %s state changed from %s to %s", e.Option(), e.Side, e.OldState, e.NewState)
}

// TypedTelnetOption - this is used as a bit of a hack for GetTelOpt. It allows
// the generic semantic for that method to work
type TypedTelnetOption[OptionStruct any] interface {
	*OptionStruct
	TelnetOption
}

// GetTelOpt retrieves a live telopt from a terminal. It is used like this:
//
//	telnet.GetTelOpt[telopts.ECHO](terminal)
//
// The above will return a value of type *telopts.ECHO, or nil if ECHO is not a registered
// telopt.  If there is a telopt of a different type registered under ECHO's code, then the method
// will return an error.
//
// This can be used to update the local state of a telopt, or respond to TelOptEvents by querying
// the newly-updated remote state of a telopt.
func GetTelOpt[OptionStruct any, T TypedTelnetOption[OptionStruct]](terminal *Terminal) (T, error) {
	var zero OptionStruct
	var err error
	code := T(&zero).Code()

	option, hasOption := terminal.options[code]

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
