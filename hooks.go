package telnet

import "sync"

// LineEnding indicates what specific event terminated a line of text received
// from the remote.
type LineEnding int

const (
	// LineEndingNone indicates that a line of text has not been terminated- this
	// line will likely be overwritten by a future line of text, and may not be a valid
	// prompt.
	LineEndingNone LineEnding = iota
	// LineEndingCRLF indicates that a line of text was terminated by a line break. This
	// line will not be overwritten by a future line of text.
	LineEndingCRLF
	// LineEndingGA indicates that a line of text was terminated by an IAC GA command.
	// This line will likely be overwritten by a future line of text, but is probably
	// a valid prompt.
	LineEndingGA
	// LineEndingEOR indicates that a line of text was terminated by an IAC EOR command.
	// This line will likely be overwritten by a future line of text, but is probably
	// a valid prompt.
	LineEndingEOR
)

func (l LineEnding) String() string {
	switch l {
	case LineEndingCRLF:
		return "CRLF"
	case LineEndingEOR:
		return "EOR"
	case LineEndingGA:
		return "GA"
	default:
		return "None"
	}
}

// IncomingTextData is the parameter for an IncomingTextEvent. It provides a line
// of text received from the printer and some additional metadata.
type IncomingTextData struct {
	// Text is the text received from the printer
	Text string
	// LineEnding indicates how the text was terminated. Because the printer prefers
	// to give only full lines of text, there was usually some event that resulted
	// in this line being packaged and sent.  LineEndingNone will only be provided
	// if a partially-complete line was sitting for 100ms without completion. This
	// may indicate that the remote has stuttered, but some MUDs also provide prompts
	// without any IAC GA/IAC EOR to demarcate them.
	LineEnding LineEnding
	// OverwritePrevious means that this line of text is a superset of the previously-sent
	// line of text.  This may happen when a remote  stutter has resolved and the remote
	// sent us the rest of a line that was previously sent with LineEndingNone, or it
	// could just indicate that the remote is moving on after previously sending us a prompt.
	OverwritePrevious bool
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

// TelOptStateChangeData is the parameter for a TelOptStateChangeEvent. It provides information
// about a TelOpt becoming active or inactive
type TelOptStateChangeData struct {
	// Option indicates the specific telopt whose state has changed
	Option TelnetOption
	// Side indicates whether the state change occurred on the local or remote side
	Side TelOptSide
	// OldState is the state that previously was active for the specified side
	OldState TelOptState
}

// TelOptEventData is the parameter for a TelOptEvent. It provides information about
// an arbitrary event raised by a telopt
type TelOptEventData struct {
	// Option indicates the option who raised the event
	Option TelnetOption
	// EventType is an arbitrary code- this code is not global, it's specific to the
	// telopt that raised the event
	EventType int
	// EventPayload is an object that may be attached to this event by the telopt. The
	// contents of this payload are specific to the event being raised.
	EventPayload any
}

// EventHook is a type for function pointers that are registered to receive events
type EventHook[T any] func(terminal *Terminal, data T)

// EventPublisher is a type used to register and fire arbitrary events
type EventPublisher[U any] struct {
	lock sync.Mutex

	registeredHooks []EventHook[U]
}

// NewPublisher creates a new EventPublisher for a particular EventHook. A slice of
// hooks can be passed in- in which case the hooks will be registered to receive events
// from the publisher.  Otherwise, nil can be passed in.
func NewPublisher[U any, T ~func(terminal *Terminal, data U)](hooks []T) *EventPublisher[U] {
	var convertedHooks []EventHook[U]

	for _, hook := range hooks {
		convertedHooks = append(convertedHooks, EventHook[U](hook))
	}

	return &EventPublisher[U]{
		registeredHooks: convertedHooks,
	}
}

// Register registers a single EventHook to receive events from this publisher.
func (e *EventPublisher[U]) Register(hook EventHook[U]) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.registeredHooks = append(e.registeredHooks, hook)
}

// Fire calls the event for all EventHook instances registered to this publisher with
// the provided parameters
func (e *EventPublisher[U]) Fire(terminal *Terminal, eventData U) {
	e.lock.Lock()
	defer e.lock.Unlock()

	for _, hook := range e.registeredHooks {
		hook(terminal, eventData)
	}
}

// ErrorEvent is an event hook type that receives errors
type ErrorEvent func(t *Terminal, err error)

// PrinterOutputEvent is an event hook type that receives text, control codes, escape sequences, and commands from the printer
type PrinterOutputEvent func(t *Terminal, output PrinterOutput)

// CommandEvent is an event hook type that receives Command objects
type CommandEvent func(t *Terminal, c Command)

// StringEvent is an event hook type that receives strings
type StringEvent func(t *Terminal, text string)

// TelOptStateChangeEvent is an event hook type that receives state changes for telopts.
// See TelOptStateChangeData for more info
type TelOptStateChangeEvent func(t *Terminal, data TelOptStateChangeData)

// TelOptEvent is an event hook type that receives arbitrary events raised by telopts
// with Terminal.RaiseTelOptEvent
type TelOptEvent func(t *Terminal, data TelOptEventData)

// EventHooks is used to pass in a set of pre-registered event hooks to a Terminal
// when calling NewTerminal.  See TerminalConfig for more info.
type EventHooks struct {
	EncounteredError []ErrorEvent
	PrinterOutput    []PrinterOutputEvent

	OutboundText    []StringEvent
	OutboundCommand []CommandEvent

	TelOptStateChange []TelOptStateChangeEvent
	TelOptEvent       []TelOptEvent
}
