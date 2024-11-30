package telnet

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

type keyboardTransport struct {
	text    string
	command Command
}

type TelnetKeyboard struct {
	charset        *Charset
	outputStream   io.Writer
	input          chan keyboardTransport
	complete       chan bool
	eventPump      *terminalEventPump
	lock           *keyboardLock
	promptCommands PromptCommands
}

func newTelnetKeyboard(charset *Charset, output io.Writer, eventPump *terminalEventPump) (*TelnetKeyboard, error) {
	keyboard := &TelnetKeyboard{
		charset:        charset,
		outputStream:   output,
		input:          make(chan keyboardTransport, 10),
		complete:       make(chan bool, 1),
		eventPump:      eventPump,
		lock:           newKeyboardLock(),
		promptCommands: PromptCommandGA,
	}

	return keyboard, nil
}

func (k *TelnetKeyboard) SetLock(lockName string, duration time.Duration) {
	k.lock.SetLock(lockName, duration)
}

func (k *TelnetKeyboard) ClearLock(lockName string) {
	k.lock.ClearLock(lockName)
}

func (k *TelnetKeyboard) HasActiveLock(lockName string) bool {
	return k.lock.HasActiveLock(lockName)
}

func (k *TelnetKeyboard) writeOutput(b []byte) error {
	for {
		_, err := k.outputStream.Write(b)

		// Retry when error is temporary
		var netError net.Error
		if errors.As(err, &netError) {
			if netError.Temporary() {
				continue
			}
		}

		return err
	}
}

func (k *TelnetKeyboard) writeCommand(c Command) error {
	// Don't send prompt commands that are being suppressed
	if c.OpCode == GA && k.promptCommands&PromptCommandGA == 0 {
		return nil
	} else if c.OpCode == EOR && k.promptCommands&PromptCommandEOR == 0 {
		return nil
	}

	k.eventPump.SentCommand(c)

	size := 2
	if c.OpCode != GA && c.OpCode != NOP && c.OpCode != EOR {
		size++
	}

	if c.OpCode == SB {
		size += len(c.Subnegotiation)
		size += 2
	}

	b := make([]byte, 0, size)
	b = append(b, IAC, c.OpCode)

	if size > 2 {
		b = append(b, byte(c.Option))
	}

	if size > 3 {
		b = append(b, c.Subnegotiation...)
		b = append(b, IAC, SE)
	}

	return k.writeOutput(b)
}

func (k *TelnetKeyboard) writeText(text string) error {
	k.eventPump.SentText(text)

	b, err := k.charset.Encode(text)
	if err != nil {
		return err
	}

	return k.writeOutput(b)
}

func (k *TelnetKeyboard) handleError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, io.EOF) {
		return true
	}

	k.encounteredError(err)
	return false
}

func (k *TelnetKeyboard) keyboardLoop(ctx context.Context) {
	go func() {
		<-ctx.Done()
		close(k.input)
	}()
	queuedText := make([]string, 0, 10)

keyboardLoop:
	for {
		select {
		case <-ctx.Done():
			break keyboardLoop
		case input := <-k.input:
			if input.command.OpCode != 0 {
				err := k.writeCommand(input.command)
				if k.handleError(err) {
					break keyboardLoop
				}

				continue
			}

			if len(queuedText) > 0 || k.lock.IsLocked() {
				// We may have unlocked but the unlock handler hasn't actually
				// run yet- we don't want this random bit of text to write out of
				// order, so place it at the end of the queue if one exists
				queuedText = append(queuedText, input.text)
				continue
			}

			err := k.writeText(input.text)
			if k.handleError(err) {
				break keyboardLoop
			}

		case <-k.lock.C:
			// Write all queued text
			for _, text := range queuedText {
				err := k.writeText(text)
				if k.handleError(err) {
					break keyboardLoop
				}
			}

			queuedText = queuedText[:0]
		}
	}

	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
		k.encounteredError(ctx.Err())
	}

	k.complete <- true
}

func (k *TelnetKeyboard) encounteredError(err error) {
	k.eventPump.EncounteredError(err)
}

func (k *TelnetKeyboard) WriteCommand(c Command) {
	k.input <- keyboardTransport{
		command: c,
	}
}

func (k *TelnetKeyboard) WriteString(str string) {
	if len(str) == 0 {
		return
	}

	k.input <- keyboardTransport{
		text: str,
	}
}

func (k *TelnetKeyboard) WaitForExit() {
	<-k.complete
	k.complete <- true
}

func (k *TelnetKeyboard) SetPromptCommand(flag PromptCommands) {
	k.promptCommands |= flag
}

func (k *TelnetKeyboard) ClearPromptCommand(flag PromptCommands) {
	k.promptCommands &= ^flag
}
