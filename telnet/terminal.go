package telnet

import (
	"context"
	"fmt"
	"net"
)

type PromptCommands byte

const (
	PromptCommandGA PromptCommands = 1 << iota
	PromptCommandEOR
)

type Terminal struct {
	conn        net.Conn
	side        TerminalSide
	charset     *Charset
	keyboard    *TelnetKeyboard
	printer     *TelnetPrinter
	telOptStack *telOptStack
}

func NewTerminal(ctx context.Context, conn net.Conn, config TerminalConfig) (*Terminal, error) {
	charset, err := NewCharset(config.DefaultCharsetName, config.CharsetUsage)
	if err != nil {
		return nil, err
	}

	pump := newEventPump()

	keyboard, err := newTelnetKeyboard(charset, conn, pump)
	if err != nil {
		return nil, err
	}

	printer := newTelnetPrinter(charset, conn, pump)
	terminal := &Terminal{
		conn:     conn,
		side:     config.Side,
		charset:  charset,
		keyboard: keyboard,
		printer:  printer,
	}

	terminal.telOptStack = newTelOptStack(terminal, config.TelOpts)

	// Run the keyboard, printer, and terminal loop until the connection is closed
	// or the consumer kills the context
	go func() {
		connCtx, connCancel := context.WithCancel(ctx)
		defer connCancel()

		terminalCtx, terminalCancel := context.WithCancel(context.Background())
		defer terminalCancel()

		// Stop the terminal loop whenever this method returns
		go pump.TerminalLoop(terminalCtx, terminal)

		// These goroutines will stop whenever the connection dies or whenever the
		// original context passed in by the consumer is cancelled
		go keyboard.keyboardLoop(connCtx)
		go printer.printerLoop(connCtx)

		// We use WaitForExit purely to ensure that we don't cancel the terminal loop
		// context until the keyboard and printer are closed- the consumer will actually
		// care about the error when they call it but we don't
		_ = printer.WaitForExit()

		// If the printer closed because the conn died, the keyboard might not notice- cancel explicitly
		connCancel()
		keyboard.WaitForExit()
	}()

	err = terminal.telOptStack.WriteRequests(terminal)
	if err != nil {
		return nil, err
	}

	return terminal, nil
}

func (t *Terminal) Side() TerminalSide {
	return t.side
}

func (t *Terminal) Charset() *Charset {
	return t.charset
}

func (t *Terminal) Keyboard() *TelnetKeyboard {
	return t.keyboard
}

func (t *Terminal) Printer() *TelnetPrinter {
	return t.printer
}

func (t *Terminal) encounteredCommand(c Command) {
	fmt.Println("COMMAND:", t.telOptStack.CommandString(c))
	err := t.telOptStack.ProcessCommand(t, c)
	if err != nil {
		t.encounteredError(err)
	}
}

func (t *Terminal) encounteredError(err error) {
	fmt.Println(err)
}

func (t *Terminal) encounteredPrompt(prompt string, overwrite bool) {
	if overwrite {
		fmt.Print(string('\r'))
	}
	fmt.Print(prompt)
}

func (t *Terminal) encounteredText(text string, overwrite bool, complete bool) {
	if overwrite {
		// Rewrite line
		fmt.Print(string('\r'))
	}

	fmt.Print(text)
}

func (t *Terminal) sentText(text string) {
	fmt.Println("SENT:", text)
}

func (t *Terminal) sentCommand(c Command) {
	fmt.Println("OUTBOUND:", t.telOptStack.CommandString(c))
}

func (t *Terminal) WaitForExit() error {
	t.keyboard.WaitForExit()

	return t.printer.WaitForExit()
}
