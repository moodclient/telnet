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

	incomingTextHooks      *EventPublisher[IncomingTextData]
	incomingCommandHooks   *EventPublisher[Command]
	outboundTextHooks      *EventPublisher[string]
	outboundCommandHooks   *EventPublisher[Command]
	encounteredErrorHooks  *EventPublisher[error]
	telOptStateChangeHooks *EventPublisher[TelOptStateChangeData]
	telOptEventHooks       *EventPublisher[TelOptEventData]

	remoteSuppressGA bool
	remoteEcho       bool
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

		incomingTextHooks:      NewPublisher(config.EventHooks.IncomingText),
		incomingCommandHooks:   NewPublisher(config.EventHooks.IncomingCommand),
		outboundTextHooks:      NewPublisher(config.EventHooks.OutboundText),
		outboundCommandHooks:   NewPublisher(config.EventHooks.OutboundCommand),
		encounteredErrorHooks:  NewPublisher(config.EventHooks.EncounteredError),
		telOptStateChangeHooks: NewPublisher(config.EventHooks.TelOptStateChange),
		telOptEventHooks:       NewPublisher(config.EventHooks.TelOptEvent),
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

func (t *Terminal) IsCharacterMode() bool {
	return t.remoteEcho && t.remoteSuppressGA
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

	// SUPPRESS-GO-AHEAD 3
	if side == TelOptSideRemote && option.Code() == 3 {
		if option.RemoteState() == TelOptActive {
			t.remoteSuppressGA = true
		} else if option.RemoteState() == TelOptInactive {
			t.remoteSuppressGA = false
		}
	}

	// ECHO 1
	if side == TelOptSideRemote && option.Code() == 1 {
		if option.RemoteState() == TelOptActive {
			t.remoteEcho = true
		} else if option.RemoteState() == TelOptInactive {
			t.remoteEcho = true
		}
	}

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
	t.incomingTextHooks.Register(EventHook[IncomingTextData](incomingText))
}

func (t *Terminal) RegisterIncomingCommandHook(incomingCommand CommandEvent) {
	t.incomingCommandHooks.Register(EventHook[Command](incomingCommand))
}

func (t *Terminal) RegisterOutboundTextHook(outboundText OutboundTextEvent) {
	t.outboundTextHooks.Register(EventHook[string](outboundText))
}

func (t *Terminal) RegisterOutboundCommandHook(outboundCommand CommandEvent) {
	t.outboundCommandHooks.Register(EventHook[Command](outboundCommand))
}

func (t *Terminal) RegisterEncounteredErrorHook(encounteredError ErrorEvent) {
	t.encounteredErrorHooks.Register(EventHook[error](encounteredError))
}

func (t *Terminal) RegisterTelOptEvent(telOptEvent TelOptEvent) {
	t.telOptEventHooks.Register(EventHook[TelOptEventData](telOptEvent))
}

func (t *Terminal) RegisterTelOptStateChangeEvent(telOptStateChange TelOptStateChangeEvent) {
	t.telOptStateChangeHooks.Register(EventHook[TelOptStateChangeData](telOptStateChange))
}
