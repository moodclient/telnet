package telnet

import "sync"

type LineEnding int

const (
	LineEndingNone LineEnding = iota
	LineEndingCRLF
	LineEndingGA
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

type IncomingTextData struct {
	Text              string
	LineEnding        LineEnding
	OverwritePrevious bool
}

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

type TelOptStateChangeData struct {
	Option   TelnetOption
	Side     TelOptSide
	OldState TelOptState
}

type TelOptEventData struct {
	Option       TelnetOption
	EventType    int
	EventPayload any
}

type EventHook[T any] func(terminal *Terminal, data T)

type EventPublisher[U any] struct {
	lock sync.Mutex

	registeredHooks []EventHook[U]
}

func NewPublisher[U any, T ~func(terminal *Terminal, data U)](hooks []T) *EventPublisher[U] {
	var convertedHooks []EventHook[U]

	for _, hook := range hooks {
		convertedHooks = append(convertedHooks, EventHook[U](hook))
	}

	return &EventPublisher[U]{
		registeredHooks: convertedHooks,
	}
}

func (e *EventPublisher[U]) Register(hook EventHook[U]) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.registeredHooks = append(e.registeredHooks, hook)
}

func (e *EventPublisher[U]) Fire(terminal *Terminal, eventData U) {
	e.lock.Lock()
	defer e.lock.Unlock()

	for _, hook := range e.registeredHooks {
		hook(terminal, eventData)
	}
}

type ErrorEvent func(t *Terminal, err error)
type IncomingTextEvent func(t *Terminal, data IncomingTextData)
type CommandEvent func(t *Terminal, c Command)
type OutboundTextEvent func(t *Terminal, text string)
type TelOptStateChangeEvent func(t *Terminal, data TelOptStateChangeData)
type TelOptEvent func(t *Terminal, data TelOptEventData)

type EventHooks struct {
	EncounteredError []ErrorEvent

	IncomingText    []IncomingTextEvent
	IncomingCommand []CommandEvent

	OutboundText    []OutboundTextEvent
	OutboundCommand []CommandEvent

	TelOptStateChange []TelOptStateChangeEvent
	TelOptEvent       []TelOptEvent
}
