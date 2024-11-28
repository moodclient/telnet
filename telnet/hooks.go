package telnet

import "encoding/json"

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
	Option    TelnetOption
	EventType int
	EventData json.RawMessage
}

type IncomingTextEvent func(terminal *Terminal, data IncomingTextData)

type IncomingCommandEvent func(terminal *Terminal, command Command)

type OutboundTextEvent func(terminal *Terminal, text string)

type OutboundCommandEvent func(terminal *Terminal, command Command)

type EncounteredErrorEvent func(terminal *Terminal, err error)

type TelOptStateChangeEvent func(terminal *Terminal, data TelOptStateChangeData)

type TelOptEvent func(terminal *Terminal, data TelOptEventData)

type EventHooks struct {
	EncounteredError EncounteredErrorEvent

	IncomingText    IncomingTextEvent
	IncomingCommand IncomingCommandEvent

	OutboundText    OutboundTextEvent
	OutboundCommand OutboundCommandEvent

	TelOptStateChange TelOptStateChangeEvent
	TelOptEvent       TelOptEvent
}
