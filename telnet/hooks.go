package telnet

import "sync"

type LineEnding int

const (
	LineEndingNone LineEnding = iota
	LineEndingCRLF
	LineEndingGA
	LineEndingEOR
)

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

type eventHook[T any] func(terminal *Terminal, data T)

type eventPublisher[U any] struct {
	lock sync.Mutex

	registeredHooks []eventHook[U]
}

func newPublisher[U any, T ~func(terminal *Terminal, data U)](hooks []T) *eventPublisher[U] {
	var convertedHooks []eventHook[U]

	for _, hook := range hooks {
		convertedHooks = append(convertedHooks, eventHook[U](hook))
	}

	return &eventPublisher[U]{
		registeredHooks: convertedHooks,
	}
}

func (e *eventPublisher[U]) Register(hook eventHook[U]) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.registeredHooks = append(e.registeredHooks, hook)
}

func (e *eventPublisher[U]) Fire(terminal *Terminal, eventData U) {
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
