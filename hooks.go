package telnet

import "sync"

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

// ErrorHandler is an event hook type that receives errors
type ErrorHandler func(t *Terminal, err error)

// TerminalDataHandler is an event hook type that receives text, control codes, escape sequences, and commands from the printer
type TerminalDataHandler func(t *Terminal, output TerminalData)

// TelOptEventHandler is an event hook type that receives arbitrary events raised by telopts
// with Terminal.RaiseTelOptEvent
type TelOptEventHandler func(t *Terminal, event TelOptEvent)

// EventHooks is used to pass in a set of pre-registered event hooks to a Terminal
// when calling NewTerminal.  See TerminalConfig for more info.
type EventHooks struct {
	EncounteredError []ErrorHandler
	PrinterOutput    []TerminalDataHandler
	OutboundData     []TerminalDataHandler

	TelOptEvent []TelOptEventHandler
}
