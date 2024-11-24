package telnet

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	SE   byte = 240
	NOP  byte = 241
	GA   byte = 249
	SB   byte = 250
	WILL byte = 251
	WONT byte = 252
	DO   byte = 253
	DONT byte = 254
	IAC  byte = 255
)

var commandCodes = map[byte]string{
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

type Command struct {
	OpCode         byte
	Option         TelOptCode
	Subnegotiation []byte
}

func (c Command) String() string {
	var sb strings.Builder
	sb.WriteString("IAC ")

	opCode, hasOpCode := commandCodes[c.OpCode]
	if !hasOpCode {
		opCode = strconv.Itoa(int(c.OpCode))
	}

	sb.WriteString(opCode)

	if c.OpCode == GA || c.OpCode == NOP {
		return sb.String()
	}

	sb.WriteByte(' ')
	sb.WriteString(strconv.Itoa(int(c.Option)))

	if c.OpCode != SB {
		return sb.String()
	}

	for _, b := range c.Subnegotiation {
		sb.WriteByte(' ')
		sb.WriteString(strconv.Itoa(int(b)))
	}

	sb.WriteString(" IAC SE")
	return sb.String()
}

func (c Command) IsNegotiationRequest() bool {
	return c.OpCode == DO || c.OpCode == WILL
}

func (c Command) IsRequestForLocal() bool {
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

	if data[1] == NOP || data[1] == GA {
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
