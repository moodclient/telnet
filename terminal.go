package telnet

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// Terminal is a wrapper around a connection to enable telnet communications
// over the connection.  Telnet's base protocol doesn't distinguish between
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
// many event hooks that can be registered for.  The PrinterOutput hook
// produces structured data that is read in from the peer, and output is
// provided to the printer directly by calling terminal.Keyboard().Send*
//
// Telnet has a mechanism for sending and receiving Command objects. Most
// of these are related to telopt negotiation, which the Terminal handles
// on your behalf based on the telopt preferences provided at creation time.
// In order to receive commands from the other side, it's best to register
// for TelOptEvent hooks, which provide the results of received commands in
// a more legible format.  Generally, the user should not write commands unless
// they really, really know what they're doing. If you want to signal a prompt
// to the remote with IAC GA, use terminal.Keyboard().SendPromptHint().
//
// The user should bear in mind that the terminal runs three (substantive)
// goroutines: one for the printer, one for the keyboard, and one
// for the terminal.  The terminal loop receives data from both of the other
// loops and forwards it to registered hooks, which means that
// blocking calls in hook methods that last long enough will block functioning
// of the terminal altogether. It is the responsibility of the consumer to
// move long-running calls to their own concurrency scheme where necessary.
type Terminal struct {
	reader             io.Reader
	writer             io.Writer
	side               TerminalSide
	charset            *Charset
	keyboard           *TelnetKeyboard
	printer            *TelnetPrinter
	options            map[TelOptCode]TelnetOption
	outboundDataText   strings.Builder
	outboundDataParser *TerminalDataParser

	printerOutputHooks    *EventPublisher[TerminalData]
	outboundDataHooks     *EventPublisher[TerminalData]
	encounteredErrorHooks *EventPublisher[error]
	telOptEventHooks      *EventPublisher[TelOptEvent]

	remoteSuppressGA bool
	remoteEcho       bool
}

// NewTerminal initializes a new terminal object from a net.Conn and begins reading from
// the printer and writing to the keyboard. Telopt negotiation begins with the remote
// immediately when this method is called.
//
// The terminal will continue until either the passed context is cancelled, or until
// the connection is closed.
//
// All functioning of this terminal is determined by the properties passed in the TerminalConfig
// object.  See that type for more information.
func NewTerminal(ctx context.Context, conn net.Conn, config TerminalConfig) (*Terminal, error) {
	return NewTerminalFromPipes(ctx, conn, conn, config)
}

// NewTerminalFromPipes initializes a new terminal from a Reader and Writer instead of a net.Conn.
// This is useful for testing, or when data is arriving via more circuitous means than a simple
// connection.  This Terminal will continue until BOTH the reader and writer are closed (or the context
// is cancelled).  Only closing one will cause the connection to stall but the terminal will remain
// active, so that should never be done.
func NewTerminalFromPipes(ctx context.Context, reader io.Reader, writer io.Writer, config TerminalConfig) (*Terminal, error) {
	charset, err := NewCharset(config.DefaultCharsetName, config.FallbackCharsetName, config.CharsetUsage)
	if err != nil {
		return nil, err
	}

	pump := newEventPump()

	keyboard, err := newTelnetKeyboard(charset, writer, pump)
	if err != nil {
		return nil, err
	}

	printer := newTelnetPrinter(charset, reader, pump)
	terminal := &Terminal{
		reader:   reader,
		writer:   writer,
		side:     config.Side,
		charset:  charset,
		keyboard: keyboard,
		printer:  printer,
		options:  make(map[TelOptCode]TelnetOption),

		printerOutputHooks:    NewPublisher(config.EventHooks.PrinterOutput),
		outboundDataHooks:     NewPublisher(config.EventHooks.OutboundData),
		encounteredErrorHooks: NewPublisher(config.EventHooks.EncounteredError),
		telOptEventHooks:      NewPublisher(config.EventHooks.TelOptEvent),
	}
	terminal.outboundDataParser = NewTerminalDataParser()
	err = terminal.initTelopts(config.TelOpts)
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
		go printer.printerLoop(connCtx, terminal)

		// We use WaitForExit purely to ensure that we don't cancel the terminal loop
		// context until the keyboard and printer are closed- the consumer will actually
		// care about the error when they call it but we don't
		_ = printer.waitForExit()

		// If the printer closed because the conn died, the keyboard might not notice- cancel explicitly
		connCancel()
		keyboard.waitForExit()
	}()

	// Kick off telopt negotiation by writing commands for our requested telopts
	err = terminal.writeTelOptRequests()
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

func (t *Terminal) encounteredError(err error) {
	t.encounteredErrorHooks.Fire(t, err)
}

func (t *Terminal) encounteredPrinterOutput(output TerminalData) {
	t.printerOutputHooks.Fire(t, output)
}

func (t *Terminal) encounteredOutboundData(output TerminalData) {
	t.outboundDataHooks.Fire(t, output)
}

// RaiseTelOptEvent is called by telopt implementations, and the Terminal, to inject an event
// into the terminal event stream. Telopts can use this method to fire arbitrary events
// that can be interpreted by the consumer. This terminal will use this method to inject
// TelOptStateChangeEvent when negotiations cause a telopt to change its state.  This is good
// for event-delivery telopts such as GCMP, but it can also be used for things like NAWS to alert
// the consumer that basic data has been collected from the remote.
func (t *Terminal) RaiseTelOptEvent(event TelOptEvent) {
	switch typed := event.(type) {
	case TelOptStateChangeEvent:
		// SUPPRESS-GO-AHEAD 3
		if typed.Side == TelOptSideRemote && typed.TelnetOption.Code() == 3 {
			if typed.NewState == TelOptActive {
				t.remoteSuppressGA = true
			} else if typed.NewState == TelOptInactive {
				t.remoteSuppressGA = false
			}
		}

		// ECHO 1
		if typed.Side == TelOptSideRemote && typed.TelnetOption.Code() == 1 {
			if typed.NewState == TelOptActive {
				t.remoteEcho = true
			} else if typed.NewState == TelOptInactive {
				t.remoteEcho = true
			}
		}
	}

	t.telOptEventHooks.Fire(t, event)
}

// CommandString converts a Command object into a legible stream. This can be useful
// when logging a received command object
func (t *Terminal) CommandString(c Command) string {
	var sb strings.Builder
	sb.WriteString("IAC ")

	opCode, hasOpCode := commandCodes[c.OpCode]
	if !hasOpCode {
		opCode = strconv.Itoa(int(c.OpCode))
	}

	sb.WriteString(opCode)

	if c.OpCode == GA || c.OpCode == NOP || c.OpCode == EOR {
		return sb.String()
	}

	sb.WriteByte(' ')

	option, hasOption := t.options[c.Option]

	if !hasOption {
		sb.WriteString("? Unknown Option ")
		sb.WriteString(strconv.Itoa(int(c.Option)))
		sb.WriteString("?")
	} else {
		sb.WriteString(option.String())
	}

	if c.OpCode != SB {
		return sb.String()
	}

	sb.WriteByte(' ')

	if !hasOption {
		sb.WriteString(fmt.Sprintf("%+v", c.Subnegotiation))
	} else {
		str, err := option.SubnegotiationString(c.Subnegotiation)

		if err != nil {
			sb.WriteString(fmt.Sprintf("%+v", c.Subnegotiation))
		} else {
			sb.WriteString(str)
		}
	}

	sb.WriteString(" IAC SE")
	return sb.String()
}

// WaitForExit will block until the terminal has ceased operation, either due to
// the context passed to NewTerminal being cancelled, or due to the underlying data streams closing.
func (t *Terminal) WaitForExit() error {
	t.keyboard.waitForExit()

	err := t.printer.waitForExit()
	return err
}

// RegisterPrinterOutputHook will register an event to be called when data is received
// from the printer.
func (t *Terminal) RegisterPrinterOutputHook(printerOutput TerminalDataHandler) {
	t.printerOutputHooks.Register(EventHook[TerminalData](printerOutput))
}

// RegisterOutboundDataHook will register an event to be called when something
// has been sent from the keyboard. This is primarily useful for debug logging.
func (t *Terminal) RegisterOutboundDataHook(outboundText TerminalDataHandler) {
	t.outboundDataHooks.Register(EventHook[TerminalData](outboundText))
}

// RegisterEncounteredErrorHook will register an event to be called when an error
// was encountered by the terminal or one of its subsidiaries. Not all errors will
// be sent via this hook: just errors that are not returned to the user immediately.
//
// If a method call to Terminal or one of its subsidiaries immediately returns an error
// to the user, it will not be delivered via this hook. If an error ends terminal
// processing immediately, it will not be delivered via this hook, it will be delivered
// via WaitForExit.
func (t *Terminal) RegisterEncounteredErrorHook(encounteredError ErrorHandler) {
	t.encounteredErrorHooks.Register(EventHook[error](encounteredError))
}

// RegisterTelOptEventHook will register an event to be called when a telopt delivers
// an event via RaiseTelOptEvent.
func (t *Terminal) RegisterTelOptEventHook(telOptEvent TelOptEventHandler) {
	t.telOptEventHooks.Register(EventHook[TelOptEvent](telOptEvent))
}
