package telnet

import (
	"context"
)

type eventType byte

const (
	eventUnknown eventType = iota
	eventError
	eventPrinterOutput
	eventOutboundData
)

type eventsTransport struct {
	eventType eventType
	err       error
	output    TerminalData
}

type terminalEventPump struct {
	events   chan eventsTransport
	complete chan bool
}

func newEventPump() *terminalEventPump {
	return &terminalEventPump{
		events:   make(chan eventsTransport, 100),
		complete: make(chan bool, 1),
	}
}

func (p *terminalEventPump) processEvent(terminal *Terminal, event eventsTransport) {
	switch event.eventType {
	case eventError:
		terminal.encounteredError(event.err)
	case eventPrinterOutput:
		terminal.encounteredPrinterOutput(event.output)
	case eventOutboundData:
		terminal.encounteredOutboundData(event.output)
	default:
		panic("invalid event")
	}
}

func (p *terminalEventPump) loopCleanup(terminal *Terminal) {
	close(p.events)

	for ev := range p.events {
		p.processEvent(terminal, ev)
	}

	p.complete <- true
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

func (p *terminalEventPump) WaitForExit() {
	<-p.complete
	p.complete <- true
}

func (p *terminalEventPump) EncounteredError(err error) {
	p.events <- eventsTransport{
		eventType: eventError,
		err:       err,
	}
}

func (p *terminalEventPump) EncounteredPrinterOutput(output TerminalData) {
	p.events <- eventsTransport{
		eventType: eventPrinterOutput,
		output:    output,
	}
}

func (p *terminalEventPump) EncounteredOutboundData(output TerminalData) {
	p.events <- eventsTransport{
		eventType: eventOutboundData,
		output:    output,
	}
}
