package telnet

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type TerminalDataParser struct {
	parsedBytes  []byte
	parser       *ansi.Parser
	parserState  byte
	builder      strings.Builder
	terminalData *queue[TerminalData]
	bytes        *queue[byte]
}

func NewTerminalDataParser() *TerminalDataParser {
	parser := &TerminalDataParser{
		terminalData: newQueue[TerminalData](50),
		bytes:        newQueue[byte](1000),
	}
	parser.parser = ansi.NewParser(nil)
	return parser
}

func NextOutput[T string | []byte](p *TerminalDataParser, data T) TerminalData {
	if len(data) > 0 {
		for byteIndex := 0; byteIndex < len(data); byteIndex++ {
			p.bytes.Queue(data[byteIndex])
		}

	}

	if p.terminalData.Len() > 0 {
		return p.terminalData.Dequeue()
	}

	for p.bytes.Len() > 0 {
		var parsed []byte
		var width, consumed int
		parsed, width, consumed, p.parserState = ansi.DecodeSequence(p.bytes.Buffer(), p.parserState, p.parser)

		p.bytes.DropElements(consumed)

		if width == 0 {
			p.parsedBytes = append(p.parsedBytes, parsed...)
		} else {
			p.builder.Write(parsed)
			continue
		}

		if p.parserState != ansi.NormalState {
			return p.terminalData.Dequeue()
		}

		if p.builder.Len() > 0 {
			p.terminalData.Queue(TextData(p.builder.String()))
			p.builder.Reset()
		}

		cmd := p.parser.Cmd().Command()
		if cmd != 0 && ansi.HasCsiPrefix(p.parsedBytes) {
			p.terminalData.Queue(CsiData{ansi.CsiSequence{Cmd: p.parser.Cmd(), Params: append([]ansi.Parameter{}, p.parser.Params()...)}})
		} else if cmd != 0 && ansi.HasOscPrefix(p.parsedBytes) {
			p.terminalData.Queue(OscData{ansi.OscSequence{Cmd: cmd, Data: append([]byte{}, p.parser.Data()...)}})
		} else if cmd != 0 && ansi.HasDcsPrefix(p.parsedBytes) {
			p.terminalData.Queue(DcsData{ansi.DcsSequence{Cmd: p.parser.Cmd(), Params: append([]ansi.Parameter{}, p.parser.Params()...), Data: append([]byte{}, p.parser.Data()...)}})
		} else if cmd != 0 && ansi.HasEscPrefix(p.parsedBytes) {
			p.terminalData.Queue(EscData{ansi.EscSequence(p.parser.Cmd())})
		} else if ansi.HasSosPrefix(p.parsedBytes) {
			p.terminalData.Queue(SosData{ansi.SosSequence{Data: append([]byte{}, p.parser.Data()...)}})
		} else if ansi.HasPmPrefix(p.parsedBytes) {
			p.terminalData.Queue(PmData{ansi.PmSequence{Data: append([]byte{}, p.parser.Data()...)}})
		} else if ansi.HasApcPrefix(p.parsedBytes) {
			p.terminalData.Queue(ApcData{ansi.ApcSequence{Data: append([]byte{}, p.parser.Data()...)}})
		} else {
			for parsedIndex := 0; parsedIndex < len(p.parsedBytes); parsedIndex++ {
				p.terminalData.Queue(ControlCodeData(p.parsedBytes[parsedIndex]))
			}
		}

		p.parsedBytes = p.parsedBytes[:0]
	}

	return p.terminalData.Dequeue()
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
