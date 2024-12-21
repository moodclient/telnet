package telnet

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
)

// TelnetPrinter is a Terminal subsidiary that parses text sent by the remote peer.
// This object is largely not used by consumers. It has a few methods that are consumed
// by telopts, but received text is largely handled through the Terminal itself.
type TelnetPrinter struct {
	scanner        *TelnetScanner
	complete       chan error
	eventPump      *terminalEventPump
	promptCommands atomicPromptCommands
}

func newTelnetPrinter(charset *Charset, inputStream io.Reader, eventPump *terminalEventPump) *TelnetPrinter {
	scanner := NewTelnetScanner(charset, inputStream)

	printer := &TelnetPrinter{
		scanner:   scanner,
		complete:  make(chan error, 1),
		eventPump: eventPump,
	}
	printer.promptCommands.Init()

	return printer
}

func (p *TelnetPrinter) isSuppressedPromptCommand(t PromptCommands) bool {
	promptCommands := p.promptCommands.Get()
	return (t == PromptCommandGA && promptCommands&PromptCommandGA == 0) ||
		(t == PromptCommandEOR && promptCommands&PromptCommandEOR == 0)
}

func (p *TelnetPrinter) printerLoop(ctx context.Context, terminal *Terminal) {
	for ctx.Err() == nil && p.scanner.Scan(ctx) {
		if p.scanner.Err() != nil {
			// Don't worry about temporary errors
			var netErr net.Error
			if errors.As(p.scanner.Err(), &netErr) {
				if netErr.Timeout() {
					continue
				}
			}

			p.eventPump.EncounteredError(p.scanner.Err())
		} else if ctx.Err() != nil {
			break
		}

		output := p.scanner.Output()

		if output == nil {
			continue
		}

		switch o := output.(type) {
		case PromptData:
			if p.isSuppressedPromptCommand(PromptCommands(o)) {
				continue
			}
		case CommandData:
			if o.Command.OpCode == 0 || o.Command.OpCode == NOP {
				continue
			}

			terminal.processTelOptCommand(o.Command)
		}

		p.eventPump.EncounteredPrinterOutput(p.scanner.Output())
	}

	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
		p.complete <- ctx.Err()
	} else if p.scanner.Err() != nil && !errors.Is(p.scanner.Err(), net.ErrClosed) {
		p.complete <- p.scanner.Err()
	}

	p.complete <- nil
}

// waitForExit will block until the printer is disposed of
func (p *TelnetPrinter) waitForExit() error {
	err := <-p.complete
	p.complete <- err
	return err
}

// SetPromptCommand will activate a particular prompt command and permit
// it to be received by the printer.  Prompt commands are IAC GA/IAC EOR, commands
// that indicate to the consumer where to place a prompt
func (p *TelnetPrinter) SetPromptCommand(flag PromptCommands) {
	p.promptCommands.SetPromptCommand(flag)
}

// ClearPromptCommand will deactivate a particular prompt command and cause it
// to be ignored by the printer. Prompt commands are IAC GA/IAC EOR, commands
// that indicate to the consumer where to place a prompt
func (p *TelnetPrinter) ClearPromptCommand(flag PromptCommands) {
	p.promptCommands.ClearPromptCommand(flag)
}

func (p *TelnetPrinter) WrapReader(wrap func(reader io.Reader) (io.Reader, error)) error {
	wrapped, err := wrap(p.scanner.baseStream)
	if err != nil {
		return err
	}

	p.scanner.inputStream = wrapped
	scan := bufio.NewScanner(wrapped)
	scan.Split(p.scanner.ScanTelnet)
	p.scanner.scanner = scan

	return nil
}
