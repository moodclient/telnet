package telnet

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// TelOptLibrary is an interface used to abstract Terminal from PrinterOutput
// for the benefit of anyone who may be using TelnetScanner without Terminal.
//
// Any method that accepts this type will likely want to use *Terminal
type TelOptLibrary interface {
	CommandString(c Command) string
}

// TerminalData is an interface output by Terminal and TelnetScanner to represent
// a single unit of output from telnet
type TerminalData interface {
	String() string
	EscapedString(terminal TelOptLibrary) string
}

// TextData is a type representing printable text that has been received from telnet
type TextData struct {
	Text string
}

var _ TerminalData = TextData{}

func (o TextData) String() string {
	return o.Text
}

func (o TextData) EscapedString(terminal TelOptLibrary) string {
	return o.Text
}

// CommandData is a type representing a single IAC command received from telnet
type CommandData struct {
	Command Command
}

var _ TerminalData = CommandData{}

func (o CommandData) String() string { return "" }
func (o CommandData) EscapedString(terminal TelOptLibrary) string {
	return terminal.CommandString(o.Command)
}

// PromptData is a type representing a hint received from telnet about where the user
// prompt should be placed in the output stream.
type PromptData struct {
	Type PromptCommands
}

var _ TerminalData = PromptData{}

func (o PromptData) String() string {
	return ""
}

func (o PromptData) EscapedString(terminal TelOptLibrary) string {
	switch o.Type {
	case PromptCommandGA:
		return "IAC GA"
	case PromptCommandEOR:
		return "IAC EOR"
	default:
		return "<???>"
	}
}

// SequenceData is a type representing a single escape sequence or control code received
// from telnet.
type SequenceData struct {
	Sequence ansi.Sequence
}

var _ TerminalData = SequenceData{}

func (o SequenceData) String() string {
	stringer, isString := o.Sequence.(fmt.Stringer)
	if isString {
		return stringer.String()
	}

	return ""
}

func controlCodeText(code ansi.ControlCode) string {
	switch code {
	case ansi.NUL:
		return "\\0"
	case ansi.SOH:
		return "<SOH>"
	case ansi.STX:
		return "<STX>"
	case ansi.ETX:
		return "<ETX>"
	case ansi.EOT:
		return "<EOT>"
	case ansi.ENQ:
		return "<ENQ>"
	case ansi.ACK:
		return "<ACK>"
	case ansi.BEL:
		return "\\a"
	case ansi.BS:
		return "\\b"
	case ansi.HT:
		return "\t"
	case ansi.LF:
		return "\n"
	case ansi.VT:
		return "\\v"
	case ansi.FF:
		return "\\f"
	case ansi.CR:
		return "\r"
	case ansi.SO:
		return "<SO>"
	case ansi.SI:
		return "<SI>"
	case ansi.DLE:
		return "<DLE>"
	case ansi.DC1:
		return "<DC1>"
	case ansi.DC2:
		return "<DC2>"
	case ansi.DC3:
		return "<DC3>"
	case ansi.DC4:
		return "<DC4>"
	case ansi.NAK:
		return "<NAK>"
	case ansi.SYN:
		return "<SYN>"
	case ansi.ETB:
		return "<ETB>"
	case ansi.CAN:
		return "<CAN>"
	case ansi.EM:
		return "<EM>"
	case ansi.SUB:
		return "<SUB>"
	case ansi.ESC:
		return "\\e"
	case ansi.FS:
		return "<FS>"
	case ansi.GS:
		return "<GS>"
	case ansi.RS:
		return "<RS>"
	case ansi.US:
		return "<US>"
	case ansi.PAD:
		return "<PAD>"
	case ansi.HOP:
		return "<HOP>"
	case ansi.BPH:
		return "<BPH>"
	case ansi.NBH:
		return "<NBH>"
	case ansi.IND:
		return "<IND>"
	case ansi.NEL:
		return "<NEL>"
	case ansi.SSA:
		return "<SSA>"
	case ansi.ESA:
		return "<ESA>"
	case ansi.HTS:
		return "<HTS>"
	case ansi.HTJ:
		return "<HTJ>"
	case ansi.VTS:
		return "<VTS>"
	case ansi.PLD:
		return "<PLD>"
	case ansi.PLU:
		return "<PLU>"
	case ansi.RI:
		return "<RI>"
	case ansi.SS2:
		return "<SS2>"
	case ansi.SS3:
		return "<SS3>"
	case ansi.DCS:
		return "<DCS>"
	case ansi.PU1:
		return "<PU1>"
	case ansi.PU2:
		return "<PU2>"
	case ansi.STS:
		return "<STS>"
	case ansi.CCH:
		return "<CCH>"
	case ansi.MW:
		return "<MW>"
	case ansi.SPA:
		return "<SPA>"
	case ansi.EPA:
		return "<EPA>"
	case ansi.SOS:
		return "<SOS>"
	case ansi.SGCI:
		return "<SGCI>"
	case ansi.SCI:
		return "<SCI>"
	case ansi.CSI:
		return "<CSI>"
	case ansi.ST:
		return "<ST>"
	case ansi.OSC:
		return "<OSC>"
	case ansi.PM:
		return "<PM>"
	case ansi.APC:
		return "<APC>"
	}

	return "<???>"
}

func (o SequenceData) EscapedString(terminal TelOptLibrary) string {
	switch s := o.Sequence.(type) {
	case ansi.ControlCode:
		return controlCodeText(s)
	case ansi.OscSequence:
		return strings.ReplaceAll(strings.ReplaceAll(s.String(), "\x1b", "\\e"), string(rune(ansi.BEL)), "\\a")
	default:
		stringer, isStringer := s.(fmt.Stringer)
		if isStringer {
			return strings.ReplaceAll(stringer.String(), "\x1b", "\\e")
		}
	}

	return "<???>"
}
