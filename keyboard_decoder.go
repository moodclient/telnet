package telnet

type keyboardDecoder struct {
	middlewareStack *MiddlewareStack
	parser          *TerminalDataParser

	decoded []TerminalData
}

func newKeyboardDecoder(middlewares ...Middleware) *keyboardDecoder {
	decoder := &keyboardDecoder{
		parser: NewTerminalDataParser(),
	}
	decoder.middlewareStack = NewMiddlewareStack(decoder.lineOut, middlewares...)
	return decoder
}

func (d *keyboardDecoder) Decode(t *Terminal, data TerminalData) {
	d.decoded = d.decoded[:0]

	d.middlewareStack.LineIn(t, data)
}

func (d *keyboardDecoder) DecodeString(t *Terminal, text string) {
	data := NextOutput(d.parser, text)
	for data != nil {
		d.middlewareStack.LineIn(t, data)

		data = NextOutput(d.parser, []byte{})
	}

	data = d.parser.Flush()
	if data != nil {
		d.middlewareStack.LineIn(t, data)
	}

	// Force any remaining bytes out since this was a self-contained line of text
	data = NextOutput(d.parser, []byte{0})
	if data != nil {
		d.middlewareStack.LineIn(t, data)
	}
}

func (d *keyboardDecoder) lineOut(t *Terminal, data TerminalData) {
	d.decoded = append(d.decoded, data)
}

func (d *keyboardDecoder) Decoded() []TerminalData {
	return d.decoded
}
