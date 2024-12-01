package telnet

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Telnet opcodes
const (
	// EOR - End Of Record. The real meaning is implementation-specific, but these
	// days IAC EOR is primarily used as an alternative to IAC GA that can indicate
	// where a prompt is without all the historical baggage of GA
	EOR byte = 239
	// SE - Subnegotiation End. IAC SE is used to mark the end of a subnegotiation command
	SE byte = 240
	// NOP - No-Op. IAC NOP doesn't indicate anything at all, and this library ignores it.
	NOP byte = 241
	// GA - Go Ahead. IAC GA is often used to indicate the end of a prompt line, so
	// that clients know where to place a cursor. However, it was originally used for
	// half-duplex terminals to indicate that the user could start typing and there is
	// a lot of weird baggage around "kludge line mode", so it is usually preferable
	// not to use this if the remote supports the EOR telopt.
	GA byte = 249
	// SB - Subnegotiation Begin. IAC SB is used to indicate the beginning of a subnegotiation
	// command. These are telopt-specific commands that have telopt-specific meanings.
	SB byte = 250
	// WILL - IAC WILL is used to indicate that this terminal intends to activate a telopt
	WILL byte = 251
	// WONT - IAC WONT is used to indicate that this terminal refuses to activate a telopt
	WONT byte = 252
	// DO - IAC DO is used to request that the remote terminal activates a telopt
	DO byte = 253
	// DONT - IAC DONT is used to demand that the remote terminal do not activate a telopt
	DONT byte = 254
	// IAC - This opcode indicates the beginning of a new command
	IAC byte = 255
)

var commandCodes = map[byte]string{
	EOR:  "EOR",
	SE:   "SE",
	NOP:  "NOP",
	GA:   "GA",
	SB:   "SB",
	WILL: "WILL",
	WONT: "WONT",
	DO:   "DO",
	DONT: "DONT",
	IAC:  "IAC",
}

// Command is a struct that indicates some sort of IAC command either received from
// or sent to
type Command struct {
	OpCode         byte
	Option         TelOptCode
	Subnegotiation []byte
}

func (c Command) IsActivateNegotiation() bool {
	return c.OpCode == DO || c.OpCode == WILL
}

func (c Command) IsLocalNegotiation() bool {
	return c.OpCode == DO || c.OpCode == DONT
}

func (c Command) Reject() Command {
	var newOpCode byte
	switch c.OpCode {
	case DO:
		newOpCode = WONT
	case WILL:
		newOpCode = DONT
	default:
		return Command{OpCode: NOP}
	}

	return Command{OpCode: newOpCode, Option: c.Option}
}

func (c Command) Accept() Command {
	var newOpCode byte
	switch c.OpCode {
	case DO:
		newOpCode = WILL
	case WILL:
		newOpCode = DO
	default:
		return Command{OpCode: NOP}
	}

	return Command{OpCode: newOpCode, Option: c.Option}
}

func parseCommand(data []byte) (Command, error) {
	if data[0] != IAC {
		return Command{}, fmt.Errorf("command did not begin with IAC: %q", commandStream(data))
	}

	if len(data) < 2 {
		return Command{}, errors.New("command was just a standalone IAC with no opcode")
	}

	_, validOpcode := commandCodes[data[1]]
	if !validOpcode {
		return Command{}, fmt.Errorf("command did not have valid opcode: %q", commandStream(data))
	}

	if data[1] == NOP || data[1] == GA || data[1] == EOR {
		return Command{
			OpCode: data[1],
		}, nil
	}

	if len(data) < 3 {
		return Command{}, fmt.Errorf("command did not contain parameters: %q", commandStream(data))
	}

	if data[1] != SB {
		return Command{
			OpCode: data[1],
			Option: TelOptCode(data[2]),
		}, nil
	}

	if len(data) < 5 || data[len(data)-2] != IAC || data[len(data)-1] != SE {
		return Command{}, fmt.Errorf("subnegotiation command did not end with IAC SE: %q", commandStream(data))
	}

	// doubled 255s in the subnegotiation data need to be pared down to a single 255 just like in the main
	// text stream. We can do that by just compacting the data into the final slice
	subnegotiationData := data[3 : len(data)-2]
	finalBuffer := make([]byte, len(subnegotiationData))
	bufferIndex, dataIndex := 0, 0

	for ; dataIndex < len(subnegotiationData); bufferIndex++ {
		finalBuffer[bufferIndex] = subnegotiationData[dataIndex]
		dataIndex++
		if subnegotiationData[bufferIndex] == IAC && dataIndex < len(subnegotiationData) && subnegotiationData[dataIndex] == IAC {
			dataIndex++
		}
	}

	return Command{
		OpCode:         data[1],
		Option:         TelOptCode(data[2]),
		Subnegotiation: finalBuffer[:bufferIndex],
	}, nil
}

func commandStream(b []byte) string {
	var sb strings.Builder

	for i := 0; i < len(b); i++ {
		if i > 0 {
			sb.WriteRune(' ')
		}

		code, hasCode := commandCodes[b[i]]
		if !hasCode {
			sb.WriteString(strconv.Itoa(int(b[i])))
		} else {
			sb.WriteString(code)
		}
	}

	return sb.String()
}
