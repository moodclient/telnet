package telnet

import (
	"context"
	"net"
)

// Terminal is a wrapper around a net.Conn to enable telnet communications
// over the net.Conn.  Telnet's base protocol doesn't distinguish between
// client and server, so there is only one terminal type for both sides of
// the connection.  A few telopts have different behavior for the client
// and server side though, so the terminal is aware of which side it is
// supposed to be.
//
// Telnet functions as a "full duplex" protocol, meaning that it does not
// operate in a request-response type of semantic that users may be
// familiar with. Instead, it's best to envision a telnet connection as
// two asynchronous datastreams- a printer reader that produces
// text from the remote peer, and a keyboard writer that sends text to
// the remote peer.
//
// Text from the printer is sent to the consumer of the Terminal via the
// many event hooks that can be registered for.  The IncomingText hook
// provides individual lines of text.  Output is provided to the printer
// directly by calling terminal.Keyboard().Send*
//
// Telnet has a mechanism for sending and receiving Command objects. Most
// of these are related to telopt negotiation, which the Terminal handles
// on your behalf based on the telopt preferences provided at creation time.
// In order to receive commands from the other side, it's best to register
// for the TelOptStateChange and TelOptEvent hooks, which provide the
// results of received commands in a more legible format.  Generally,
// the user should not write commands unless they really, really know
// what they're doing. If you want to signal a prompt to the remote with
// IAC GA, use terminal.Keyboard().SendPromptHint().
//
// The user should bear in mind that the terminal runs three (substantive)
// goroutines: one for the printer, one for the keyboard, and one
// for the terminal.  The terminal loop interacts with both of the other
// loops directly. It also directly calls registered hooks, which means that
// blocking calls made in hook methods will block functioning of the
// terminal altogether. It is the responsibility of the consumer to
// move long-running calls to their own concurrency scheme where necessary.
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

// NewTerminal initializes a new terminal object and begins reading from
// the printer and writing to the keyboard. Telopt negotiation begins with the remote
// immediately when this method is called.
//
// The terminal will continue until either the passed context is cancelled, or until
// the connection is closed.
//
// All functioning of this terminal is determined by the properties passed in the TerminalConfig
// object.  See that type for more information.
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
		_ = printer.waitForExit()

		// If the printer closed because the conn died, the keyboard might not notice- cancel explicitly
		connCancel()
		keyboard.waitForExit()
	}()

	// Kick off telopt negotiation by writing commands for our requested telopts
	err = terminal.telOptStack.WriteRequests(terminal)
	if err != nil {
		return nil, err
	}

	return terminal, nil
}

// Side returns a TerminalSide object indicating whether the
// terminal represents a client or server
func (t *Terminal) Side() TerminalSide {
	return t.side
}

// Charset returns the relevant Charset object for the terminal, which stores what
// charset the terminal uses for encoding & decoding by default, what charset has
// been negotiated for use with TRANSMIT-BINARY, etc.
func (t *Terminal) Charset() *Charset {
	return t.charset
}

// Keyboard returns the object that is used for sending outbound communications
func (t *Terminal) Keyboard() *TelnetKeyboard {
	return t.keyboard
}

// Printer returns the object that is used for receiving inbound communciations
func (t *Terminal) Printer() *TelnetPrinter {
	return t.printer
}

// IsCharacterMode will return true if both the ECHO and SUPPRESS-GO-AHEAD options are
// enabled.  Technically this is supposed to be the case when NEITHER or BOTH are enabled,
// as traditionally, "kludge line mode", the line-at-a-time operation you might be familiar
// with, is supposed to occur when either ECHO or SUPPRESS-GO-AHEAD, but not both, are
// enabled.  However, MUDs traditionally operate in a line-at-a-time manner and do not
// usually request SUPPRESS-GO-AHEAD (instead using IAC GA to indicate the location of
// a prompt to clients), resulting in a relatively common expectation that
// kludge line mode is active when neither telopt is active.
//
// As a result, in order to allow the broadest support for the most clients possible,
// it's recommended that you activate both SUPPRESS-GO-AHEAD and EOR when you want to
// support line-at-a-time mode and activate both SUPPRESS-GO-AHEAD and ECHO when
// when you want to support character mode. If line-at-a-time is desired and EOR
// is not available, then leaving SUPPRESS-GO-AHEAD and ECHO both inactive and proceeding
// with line-at-a-time will generally work.
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

// RaiseTelOptEvent is called by telopt implementations to inject an event
// into the terminal event stream. Telopts can use this method to fire arbitrary events
// that can be interpreted by the consumer.  This is good for event-delivery telopts
// such as GCMP, but it can also be used for things like NAWS to alert the consumer
// that basic data has been collected from the remote.
func (t *Terminal) RaiseTelOptEvent(data TelOptEventData) {
	t.telOptEventHooks.Fire(t, data)
}

// CommandString converts a Command object into a legible stream. This can be useful
// when logging a received command object
func (t *Terminal) CommandString(c Command) string {
	return t.telOptStack.CommandString(c)
}

// WaitForExit will block until the terminal has ceased operation, either due to
// the context passed to NewTerminal being cancelled, or due to the underlying network
// connection closing.
func (t *Terminal) WaitForExit() error {
	t.keyboard.waitForExit()

	err := t.printer.waitForExit()
	return err
}

// RegisterIncomingTextHook will register an event to be called when a line of text
// has been received from the printer.
func (t *Terminal) RegisterIncomingTextHook(incomingText IncomingTextEvent) {
	t.incomingTextHooks.Register(EventHook[IncomingTextData](incomingText))
}

// RegisterIncomingCommandHook will register an event to be called when an IAC
// command has been received from the printer.  This is useful for debug logging,
// but in most cases, the consumer will want to use RegisterTelOptEventHook and/or
// RegisterTelOptStateChangeEventHook in order to receive the outcome of a received
// command.
func (t *Terminal) RegisterIncomingCommandHook(incomingCommand CommandEvent) {
	t.incomingCommandHooks.Register(EventHook[Command](incomingCommand))
}

// RegisterOutboundTextHook will register an event to be called when a line of text
// has been sent from the keyboard. This is primarily useful for debug logging.
func (t *Terminal) RegisterOutboundTextHook(outboundText StringEvent) {
	t.outboundTextHooks.Register(EventHook[string](outboundText))
}

// RegisterOutboundCommandHook will register an event to be called when a command
// has been sent from the keyboard. This is primarily useful for debug logging.
func (t *Terminal) RegisterOutboundCommandHook(outboundCommand CommandEvent) {
	t.outboundCommandHooks.Register(EventHook[Command](outboundCommand))
}

// RegisterEncounteredErrorHook will register an event to be called when an error
// was encountered by the terminal or one of its subsidiaries. Not all errors will
// be sent via this hook: just errors that are not returned to the user immediately.
//
// If a method call to Terminal or one of its subsidiaries immediately returns an error
// to the user, it will not be delivered via this hook. If an error ends terminal
// processing immediately, it will not be delivered via this hook, it will be delivered
// via WaitForExit.
func (t *Terminal) RegisterEncounteredErrorHook(encounteredError ErrorEvent) {
	t.encounteredErrorHooks.Register(EventHook[error](encounteredError))
}

// RegisterTelOptEventHook will reigster an event to be called when a telopt delivers
// an event via RaiseTelOptEvent.
func (t *Terminal) RegisterTelOptEventHook(telOptEvent TelOptEvent) {
	t.telOptEventHooks.Register(EventHook[TelOptEventData](telOptEvent))
}

// RegisterTelOptStateChangeEventHook will register an event to be called when a telopt's
// state changes. The possible states are located in TelOptState. All TelOpts registered
// in NewTerminal begin in the TelOptInactive state. If a telopt has been registered to
// request functioning, there will be an event call changing the state to TelOptRequested.
// This event will only be called when the state actually changes- an external request
// to move the telopt to a state it's already in will not trigger this event.
//
// Bear in mind that telopts have two states: the local state, indicating whether the telopt
// is active on our side of the connection, and the remote state, indicating whether
// the telopt is active on the peer's side of the connection.  Telopts can be active
// on only one side of the connection, both, or neither.  Different telopts have different
// expected behaviors.
func (t *Terminal) RegisterTelOptStateChangeEventHook(telOptStateChange TelOptStateChangeEvent) {
	t.telOptStateChangeHooks.Register(EventHook[TelOptStateChangeData](telOptStateChange))
}
