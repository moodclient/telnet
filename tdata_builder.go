package telnet

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type TerminalDataParser struct {
	bytes        []byte
	parsedBytes  []byte
	parser       *ansi.Parser
	parserState  byte
	builder      strings.Builder
	terminalData []TerminalData
	queueStart   int
	queueEnd     int
}

func NewTerminalDataParser() *TerminalDataParser {
	parser := &TerminalDataParser{}
	parser.parser = ansi.NewParser(nil)
	return parser
}

func (p *TerminalDataParser) queueData(data TerminalData) {
	if p.queueEnd < len(p.terminalData) {
		p.terminalData[p.queueEnd] = data
		p.queueEnd++
		return
	}

	len := p.queueLen()
	if p.queueStart < 10 || p.queueStart < len/4 {
		p.terminalData = append(p.terminalData, data)
		p.queueEnd++
		return
	}

	copy(p.terminalData[0:len], p.terminalData[p.queueStart:p.queueEnd])
	p.queueStart = 0
	p.queueEnd = len
	p.queueData(data)
}

func (p *TerminalDataParser) dequeueData() TerminalData {
	if p.queueEnd == p.queueStart {
		return nil
	}

	out := p.terminalData[p.queueStart]
	p.queueStart++
	return out
}

func (p *TerminalDataParser) queueLen() int {
	return p.queueEnd - p.queueStart
}

func NextOutput[T string | []byte](p *TerminalDataParser, data T) TerminalData {
	if len(data) > 0 {
		p.bytes = append(p.bytes, data...)
	}

	index := 0

	defer func() {
		if len(p.bytes) > 0 {
			remainingLen := len(p.bytes) - index

			if remainingLen > 0 {
				copy(p.bytes[:remainingLen], p.bytes[index:len(p.bytes)])
			}

			p.bytes = p.bytes[:remainingLen]
		}
	}()

	if p.queueLen() > 0 {
		return p.dequeueData()
	}

	for index < len(p.bytes) {
		var parsed []byte
		var width, consumed int
		parsed, width, consumed, p.parserState = ansi.DecodeSequence(p.bytes[index:], p.parserState, p.parser)

		index += consumed
		if width == 0 {
			p.parsedBytes = append(p.parsedBytes, parsed...)
		} else {
			p.builder.Write(parsed)
			continue
		}

		if p.parserState != ansi.NormalState {
			return p.dequeueData()
		}

		if p.builder.Len() > 0 {
			p.queueData(TextData(p.builder.String()))
			p.builder.Reset()
		}

		cmd := p.parser.Cmd().Command()
		if cmd != 0 && ansi.HasCsiPrefix(p.parsedBytes) {
			p.queueData(CsiData{ansi.CsiSequence{Cmd: p.parser.Cmd(), Params: append([]ansi.Parameter{}, p.parser.Params()...)}})
		} else if cmd != 0 && ansi.HasOscPrefix(p.parsedBytes) {
			p.queueData(OscData{ansi.OscSequence{Cmd: cmd, Data: append([]byte{}, p.parser.Data()...)}})
		} else if cmd != 0 && ansi.HasDcsPrefix(p.parsedBytes) {
			p.queueData(DcsData{ansi.DcsSequence{Cmd: p.parser.Cmd(), Params: append([]ansi.Parameter{}, p.parser.Params()...), Data: append([]byte{}, p.parser.Data()...)}})
		} else if cmd != 0 && ansi.HasEscPrefix(p.parsedBytes) {
			p.queueData(EscData{ansi.EscSequence(p.parser.Cmd())})
		} else if ansi.HasSosPrefix(p.parsedBytes) {
			p.queueData(SosData{ansi.SosSequence{Data: append([]byte{}, p.parser.Data()...)}})
		} else if ansi.HasPmPrefix(p.parsedBytes) {
			p.queueData(PmData{ansi.PmSequence{Data: append([]byte{}, p.parser.Data()...)}})
		} else if ansi.HasApcPrefix(p.parsedBytes) {
			p.queueData(ApcData{ansi.ApcSequence{Data: append([]byte{}, p.parser.Data()...)}})
		} else {
			for parsedIndex := 0; parsedIndex < len(p.parsedBytes); parsedIndex++ {
				p.queueData(ControlCodeData(p.parsedBytes[parsedIndex]))
			}
		}

		p.parsedBytes = p.parsedBytes[:0]
	}

	return p.dequeueData()
}

func (p *TerminalDataParser) Flush() TerminalData {
	if p.builder.Len() > 0 {
		out := p.builder.String()
		p.builder.Reset()
		return TextData(out)
	}

	return nil
}

func (p *TerminalDataParser) FireAll(terminal *Terminal, data string, publisher *EventPublisher[TerminalData]) {
	outData := NextOutput(p, data)

	for outData != nil {
		publisher.Fire(terminal, outData)
		outData = NextOutput(p, "")
	}

	final := p.Flush()
	if final != nil {
		publisher.Fire(terminal, final)
	}
}

func (p *TerminalDataParser) FireSingle(terminal *Terminal, data string, hook TerminalDataHandler) {
	outData := NextOutput(p, data)

	for outData != nil {
		hook(terminal, outData)
		outData = NextOutput(p, "")
	}

	final := p.Flush()
	if final != nil {
		hook(terminal, final)
	}
}
