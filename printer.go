package telnet

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"time"
)

// TelnetPrinter is a Terminal subsidiary that parses text sent by the remote peer.
// This object is largely not used by consumers. It has a few methods that are consumed
// by telopts, but received text is largely handled through the Terminal itself.
type TelnetPrinter struct {
	scanner        *bufio.Scanner
	err            error
	readyBytes     bytes.Buffer
	command        Command
	awaitingScan   bool
	scanResult     chan bool
	complete       chan error
	eventPump      *terminalEventPump
	charset        *Charset
	promptCommands atomicPromptCommands
}

func newTelnetPrinter(charset *Charset, inputStream io.Reader, eventPump *terminalEventPump) *TelnetPrinter {
	scan := bufio.NewScanner(inputStream)
	scan.Split(ScanTelnet)

	printer := &TelnetPrinter{
		charset:    charset,
		scanner:    scan,
		scanResult: make(chan bool),
		complete:   make(chan error, 1),
		eventPump:  eventPump,
	}
	printer.promptCommands.Init()

	return printer
}

func (p *TelnetPrinter) printerLoop(ctx context.Context) {
	awaitingMore := 0

	// telnetPrinter.scan() will return one of the following:
	// * A complete line of text ending with \n indicating EOL (\r\n is converted to \n)
	// * An IAC command (printer.Command())
	// * An incomplete line of text that does not have a newline but was followed by an IAC command,
	//   indicating that the text is a complete package
	// * An incomplete line of text that was sitting for at least 100ms without further bytes incoming
	//
	// If an incomplete line of text is received, then we will print it immediately, but write over it when we
	// either receive an IAC command or a newline
	for ctx.Err() == nil && p.scan(ctx) {
		if p.err != nil {
			// Don't worry about temporary errors
			var netErr net.Error
			if errors.As(p.err, &netErr) {
				if netErr.Timeout() {
					continue
				}
			}

			break
		} else if ctx.Err() != nil {
			break
		}

		printBytes := p.readyBytes.Bytes()
		if len(printBytes) > 0 {
			var lineEnding LineEnding

			if printBytes[len(printBytes)-1] == '\n' {
				lineEnding = LineEndingCRLF
			}

			if len(printBytes) > awaitingMore {
				p.eventPump.EncounteredText(p.decode(printBytes, lineEnding), lineEnding, awaitingMore > 0)
			}

			awaitingMore = 0
			if lineEnding == LineEndingNone {
				awaitingMore = len(printBytes)
			} else {
				p.readyBytes.Reset()
			}
		}

		if p.command.OpCode == 0 || p.command.OpCode == NOP {
			continue
		}

		promptCommands := p.promptCommands.Get()
		if (p.command.OpCode == GA && promptCommands&PromptCommandGA != 0) ||
			(p.command.OpCode == EOR && promptCommands&PromptCommandEOR != 0) {

			var lineEnding LineEnding
			if p.command.OpCode == GA {
				lineEnding = LineEndingGA
			} else {
				lineEnding = LineEndingEOR
			}

			p.eventPump.EncounteredText(p.decode(printBytes, lineEnding), lineEnding, awaitingMore > 0)
			p.readyBytes.Reset()
			awaitingMore = 0
			continue
		}

		if p.command.OpCode == GA || p.command.OpCode == EOR {
			// We received a suppressed prompt code
			continue
		}

		// Command
		p.eventPump.EncounteredCommand(p.command)
	}

	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
		p.complete <- ctx.Err()
	} else if p.err != nil && !errors.Is(p.err, net.ErrClosed) {
		p.complete <- p.err
	}

	p.complete <- nil
}

// asyncScan will call the underlying scanner, but if there are bytes waiting to return to the caller,
// it will be called with a 100ms timeout. This is because some MUDs like to return prompts without an ER
// or GA or newline or anything, and we have absolutely no way of knowing that what we received is a prompt.
//
// We won't treat the resulting text as a prompt or complete line, but we will display it immediately. The
// idea is that if we receive a chunk of text without a newline we write it to the client screen but
// don't consume it from the scanner.  The next iteration, we either get more text (including the text we
// received previously) and CR and rewrite it, or we get a command (which "bakes in" our changes).
func (p *TelnetPrinter) asyncScan(ctx context.Context) bool {
	alreadyAwaitingScan := p.awaitingScan

	if !alreadyAwaitingScan {
		p.awaitingScan = true
		go func() {
			p.scanResult <- p.scanner.Scan()
		}()
	}

	if alreadyAwaitingScan || p.readyBytes.Len() == 0 {
		select {
		case result := <-p.scanResult:
			p.awaitingScan = false
			return result
		case <-ctx.Done():
			return true
		}
	}

	select {
	case result := <-p.scanResult:
		p.awaitingScan = false
		return result
	case <-ctx.Done():
		return true
	case <-time.After(100 * time.Millisecond):
		return true
	}
}

func (p *TelnetPrinter) scan(ctx context.Context) bool {
	p.err = nil
	p.command = Command{}

	for {
		keepGoing := p.asyncScan(ctx)
		if p.awaitingScan {
			// We're still waiting on scan, meaning we timed out- return what characters we have
			// to the caller
			return true
		}

		err := p.scanner.Err()
		if err != nil || !keepGoing {
			p.err = err
			return keepGoing
		}

		scanBytes := p.scanner.Bytes()
		if len(scanBytes) == 2 && scanBytes[0] == IAC && scanBytes[1] == IAC {
			p.readyBytes.WriteByte(255)
			continue
		}

		if len(scanBytes) == 1 {
			if scanBytes[0] == '\r' || scanBytes[0] == 0 {
				continue
			}

			if scanBytes[0] == '\n' {
				p.readyBytes.WriteByte('\n')
				return true
			}
		}

		if scanBytes[0] == IAC {
			p.command, p.err = parseCommand(scanBytes)

			return true
		}

		p.readyBytes.Write(scanBytes)
	}
}

// waitForExit will block until the printer is disposed of
func (p *TelnetPrinter) waitForExit() error {
	err := <-p.complete
	p.complete <- err
	return err
}

func (p *TelnetPrinter) decode(textBytes []byte, ending LineEnding) string {
	text, err := p.charset.Decode(textBytes)
	if err != nil {
		p.eventPump.EncounteredError(err)
	}

	if ending == LineEndingNone {
		text = strings.TrimSuffix(text, "\ufffd")
	}

	return text
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

func findNextSpecialChar(data []byte, onlyIAC bool) (int, byte) {
	for i := 0; i < len(data); i++ {
		if onlyIAC && data[i] != IAC {
			continue
		}

		if data[i] == '\r' || data[i] == '\n' || data[i] == 0 || data[i] == IAC {
			return i, data[i]
		}
	}

	return -1, 0
}

func scanTelnetWithoutEOF(data []byte) (advance int, token []byte, err error) {
	// A special char is IAC, 0, \r, or \n
	specialCharIndex, specialChar := findNextSpecialChar(data, false)

	if specialCharIndex > 0 {
		// Release all data until we get to a special char
		return specialCharIndex, data[:specialCharIndex], nil
	} else if specialCharIndex < 0 {
		// No special char, dump everything
		return len(data), data, nil
	}

	// Release on their own: 'IAC IAC', or any other special characters
	if specialChar == IAC && len(data) >= 2 && data[1] == IAC {
		return 2, data[:2], nil
	} else if specialChar != IAC {
		return 1, data[:1], nil
	}

	// if it's just IAC by itself, wait for more data
	if len(data) < 2 {
		return 0, nil, nil
	}

	// IAC GA, IAC EOR, and IAC NOP release on their own
	// SE should never appear here but if it does we should recover by consuming the data
	if data[1] == GA || data[1] == NOP || data[1] == SE || data[1] == EOR {
		return 2, data[:2], nil
	}

	// All other codes require at least 3 characters
	if len(data) < 3 {
		return 0, nil, nil
	}

	if data[1] != SB {
		// Everything else except subnegotiations comes in three code sets
		return 3, data[:3], nil
	}

	nextIndex := 0

	for {
		nextSpecialCharIndex, _ := findNextSpecialChar(data[nextIndex+1:], true)

		// No more IACs, subnegotiation end is not in buffer yet
		if nextSpecialCharIndex < 0 {
			return 0, nil, nil
		}

		nextIndex += nextSpecialCharIndex + 1
		if len(data) <= nextIndex+1 {
			// IAC is last character, but we need more
			return 0, nil, nil
		}

		if data[nextIndex+1] == SE {
			// Found subnegotiation end
			return nextIndex + 2, data[:nextIndex+2], nil
		}

		if data[nextIndex+1] == IAC {
			// Double 255's should be skipped over
			nextIndex++
		}
	}
}

// ScanTelnet is a method used as the split method for io.Scanner. It will receive
// chunks of text or commands as individual tokens.
func ScanTelnet(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 {
		return 0, nil, nil
	}

	advance, token, err = scanTelnetWithoutEOF(data)
	if err != nil {
		return
	}

	if advance > 0 {
		return
	}

	if atEOF {
		return len(data), data, nil
	}

	return
}
