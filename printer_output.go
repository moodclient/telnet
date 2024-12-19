package telnet

import (
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
type TextData string

var _ TerminalData = TextData("")

func (o TextData) String() string {
	return string(o)
}

func (o TextData) EscapedString(terminal TelOptLibrary) string {
	return string(o)
}

// CommandData is a type representing a single IAC command received from telnet
type CommandData struct {
	Command
}

var _ TerminalData = CommandData{}

func (o CommandData) String() string { return "" }
func (o CommandData) EscapedString(terminal TelOptLibrary) string {
	return terminal.CommandString(o.Command)
}

// PromptData is a type representing a hint received from telnet about where the user
// prompt should be placed in the output stream.
type PromptData PromptCommands

var _ TerminalData = PromptData(PromptCommandGA)

func (o PromptData) String() string {
	return ""
}

func (o PromptData) EscapedString(terminal TelOptLibrary) string {
	switch PromptCommands(o) {
	case PromptCommandGA:
		return "IAC GA"
	case PromptCommandEOR:
		return "IAC EOR"
	default:
		return "<???>"
	}
}

type CsiData struct {
	ansi.CsiSequence
}

func (o CsiData) EscapedString(terminal TelOptLibrary) string {
	return strings.ReplaceAll(strings.ReplaceAll(o.String(), "\x1b", "\\e"), "\x9b", "<CSI>")
}

type OscData struct {
	ansi.OscSequence
}

func (o OscData) EscapedString(terminal TelOptLibrary) string {
	return strings.ReplaceAll(strings.ReplaceAll(o.String(), "\x1b", "\\e"), "\x9d", "<OSC>")
}

type EscData struct {
	ansi.EscSequence
}

func (o EscData) EscapedString(terminal TelOptLibrary) string {
	return strings.ReplaceAll(o.String(), "\x1b", "\\e")
}

type DcsData struct {
	ansi.DcsSequence
}

func (o DcsData) EscapedString(terminal TelOptLibrary) string {
	return strings.ReplaceAll(strings.ReplaceAll(o.String(), "\x1b", "\\e"), "\x90", "<DCS>")
}

type SosData struct {
	ansi.SosSequence
}

func (o SosData) EscapedString(terminal TelOptLibrary) string {
	return strings.ReplaceAll(strings.ReplaceAll(o.String(), "\x1b", "\\e"), "\x98", "<SOS>")
}

type PmData struct {
	ansi.PmSequence
}

func (o PmData) EscapedString(terminal TelOptLibrary) string {
	return strings.ReplaceAll(strings.ReplaceAll(o.String(), "\x1b", "\\e"), "\x9e", "<PM>")
}

type ApcData struct {
	ansi.ApcSequence
}

func (o ApcData) EscapedString(terminal TelOptLibrary) string {
	return strings.ReplaceAll(strings.ReplaceAll(o.String(), "\x1b", "\\e"), "\x9f", "<APC>")
}

type ControlCodeData ansi.ControlCode

func (o ControlCodeData) String() string {
	return ansi.ControlCode(o).String()
}

func (o ControlCodeData) EscapedString(terminal TelOptLibrary) string {
	switch o {
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
