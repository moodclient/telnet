package telnet

import (
	"context"
)

type eventType byte

const (
	eventUnknown eventType = iota
	eventError
	eventPrinterOutput
	eventOutboundCommand
	eventOutboundText
)

type eventsTransport struct {
	eventType eventType
	err       error
	command   Command
	text      string
	output    PrinterOutput
}

type terminalEventPump struct {
	events chan eventsTransport
}

func newEventPump() *terminalEventPump {
	return &terminalEventPump{
		events: make(chan eventsTransport, 10),
	}
}

func (p *terminalEventPump) processEvent(terminal *Terminal, event eventsTransport) {
	switch event.eventType {
	case eventError:
		terminal.encounteredError(event.err)
	case eventPrinterOutput:
		terminal.encounteredPrinterOutput(event.output)
	case eventOutboundText:
		terminal.sentText(event.text)
	case eventOutboundCommand:
		terminal.sentCommand(event.command)
	default:
		panic("invalid event")
	}
}

func (p *terminalEventPump) loopCleanup(terminal *Terminal) {
	close(p.events)

	for ev := range p.events {
		p.processEvent(terminal, ev)
	}
}

func (p *terminalEventPump) TerminalLoop(ctx context.Context, terminal *Terminal) {
	defer p.loopCleanup(terminal)

	for {
		select {
		case ev := <-p.events:
			p.processEvent(terminal, ev)
		case <-ctx.Done():
			return
		}
	}
}

func (p *terminalEventPump) EncounteredError(err error) {
	p.events <- eventsTransport{
		eventType: eventError,
		err:       err,
	}
}

func (p *terminalEventPump) EncounteredPrinterOutput(output PrinterOutput) {
	p.events <- eventsTransport{
		eventType: eventPrinterOutput,
		output:    output,
	}
}

func (p *terminalEventPump) SentCommand(command Command) {
	p.events <- eventsTransport{
		eventType: eventOutboundCommand,
		command:   command,
	}
}

func (p *terminalEventPump) SentText(text string) {
	p.events <- eventsTransport{
		eventType: eventOutboundText,
		text:      text,
	}
}
