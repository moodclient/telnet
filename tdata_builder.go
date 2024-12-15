package telnet

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func buildDispatcher(builder *strings.Builder, data *TerminalData) ansi.ParserDispatcher {
	return func(sequence ansi.Sequence) {
		switch seq := sequence.(type) {
		case ansi.Rune:
			builder.WriteRune(rune(seq))
		case ansi.Grapheme:
			builder.WriteString(seq.Cluster)
		default:
			*data = SequenceData{Sequence: seq.Clone()}
		}
	}
}

func ParseTerminalData[T string | []byte](data T, output func(data TerminalData)) {
	parser := ansi.GetParser()
	defer ansi.PutParser(parser)

	var builder strings.Builder
	var terminalData TerminalData

	parser.SetDispatcher(buildDispatcher(&builder, &terminalData))

	for byteIndex := 0; byteIndex < len(data); byteIndex++ {
		parser.Advance(data[byteIndex])

		if terminalData != nil {
			if builder.Len() > 0 {
				output(TextData{Text: builder.String()})
				builder.Reset()
			}
			output(terminalData)
			terminalData = nil
		}
	}

	if builder.Len() > 0 {
		output(TextData{Text: builder.String()})
	}
}

type TerminalDataParser struct {
	bytes        []byte
	parser       *ansi.Parser
	builder      strings.Builder
	terminalData TerminalData
}

func NewTerminalDataParser() *TerminalDataParser {
	parser := &TerminalDataParser{}
	parser.parser = ansi.NewParser(buildDispatcher(&parser.builder, &parser.terminalData))
	return parser
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

	if p.terminalData != nil {
		out := p.terminalData
		p.terminalData = nil
		return out
	}

	for ; index < len(p.bytes); index++ {
		p.parser.Advance(p.bytes[index])

		if p.terminalData != nil {
			index++

			if p.builder.Len() > 0 {
				out := p.builder.String()
				p.builder.Reset()
				return TextData{Text: out}
			}

			out := p.terminalData
			p.terminalData = nil
			return out
		}
	}

	return nil
}

func (p *TerminalDataParser) Flush() TerminalData {
	if p.builder.Len() > 0 {
		out := p.builder.String()
		p.builder.Reset()
		return TextData{Text: out}
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
