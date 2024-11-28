package telnet

import (
	"context"
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

	incomingTextHooks      *eventPublisher[IncomingTextData]
	incomingCommandHooks   *eventPublisher[Command]
	outboundTextHooks      *eventPublisher[string]
	outboundCommandHooks   *eventPublisher[Command]
	encounteredErrorHooks  *eventPublisher[error]
	telOptStateChangeHooks *eventPublisher[TelOptStateChangeData]
	telOptEventHooks       *eventPublisher[TelOptEventData]
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

		incomingTextHooks:      newPublisher(config.EventHooks.IncomingText),
		incomingCommandHooks:   newPublisher(config.EventHooks.IncomingCommand),
		outboundTextHooks:      newPublisher(config.EventHooks.OutboundText),
		outboundCommandHooks:   newPublisher(config.EventHooks.OutboundCommand),
		encounteredErrorHooks:  newPublisher(config.EventHooks.EncounteredError),
		telOptStateChangeHooks: newPublisher(config.EventHooks.TelOptStateChange),
		telOptEventHooks:       newPublisher(config.EventHooks.TelOptEvent),
	}

	terminal.telOptStack, err = newTelOptStack(terminal, config.TelOpts)
	if err != nil {
		return nil, err
	}

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
	t.incomingCommandHooks.Fire(t, c)

	err := t.telOptStack.ProcessCommand(t, c)
	if err != nil {
		t.encounteredError(err)
	}
}

func (t *Terminal) encounteredError(err error) {
	t.encounteredErrorHooks.Fire(t, err)
}

func (t *Terminal) encounteredText(text string, lineEnding LineEnding, overwrite bool) {
	t.incomingTextHooks.Fire(t, IncomingTextData{
		Text:              text,
		LineEnding:        lineEnding,
		OverwritePrevious: overwrite,
	})
}

func (t *Terminal) sentText(text string) {
	t.outboundTextHooks.Fire(t, text)
}

func (t *Terminal) sentCommand(c Command) {
	t.outboundCommandHooks.Fire(t, c)
}

func (t *Terminal) teloptStateChange(option TelnetOption, side TelOptSide, oldState TelOptState) {
	t.telOptStateChangeHooks.Fire(t, TelOptStateChangeData{
		Option:   option,
		Side:     side,
		OldState: oldState,
	})
}

func (t *Terminal) RaiseTelOptEvent(data TelOptEventData) {
	t.telOptEventHooks.Fire(t, data)
}

func (t *Terminal) CommandString(c Command) string {
	return t.telOptStack.CommandString(c)
}

func (t *Terminal) WaitForExit() error {
	t.keyboard.WaitForExit()

	return t.printer.WaitForExit()
}

func (t *Terminal) RegisterIncomingTextHook(incomingText IncomingTextEvent) {
	t.incomingTextHooks.Register(eventHook[IncomingTextData](incomingText))
}

func (t *Terminal) RegisterIncomingCommandHook(incomingCommand CommandEvent) {
	t.incomingCommandHooks.Register(eventHook[Command](incomingCommand))
}

func (t *Terminal) RegisterOutboundTextHook(outboundText OutboundTextEvent) {
	t.outboundTextHooks.Register(eventHook[string](outboundText))
}

func (t *Terminal) RegisterOutboundCommandHook(outboundCommand CommandEvent) {
	t.outboundCommandHooks.Register(eventHook[Command](outboundCommand))
}

func (t *Terminal) RegisterEncounteredErrorHook(encounteredError ErrorEvent) {
	t.encounteredErrorHooks.Register(eventHook[error](encounteredError))
}

func (t *Terminal) RegisterTelOptEvent(telOptEvent TelOptEvent) {
	t.telOptEventHooks.Register(eventHook[TelOptEventData](telOptEvent))
}

func (t *Terminal) RegisterTelOptStateChangeEvent(telOptStateChange TelOptStateChangeEvent) {
	t.telOptStateChangeHooks.Register(eventHook[TelOptStateChangeData](telOptStateChange))
}
