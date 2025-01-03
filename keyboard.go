package telnet

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

type keyboardTransport struct {
	unparsedString string
	data           TerminalData
	postSend       func() error
}

// TelnetKeyboard is a Terminal subsidiary that is in charge of sending outbound data
// to the remote peer.
type TelnetKeyboard struct {
	terminal       *Terminal
	charset        *Charset
	baseStream     io.Writer
	outputStream   io.Writer
	input          chan keyboardTransport
	complete       chan bool
	eventPump      *terminalEventPump
	lock           *keyboardLock
	promptCommands atomicPromptCommands
	decoder        *keyboardDecoder
}

func newTelnetKeyboard(charset *Charset, output io.Writer, eventPump *terminalEventPump, middlewares ...Middleware) (*TelnetKeyboard, error) {
	keyboard := &TelnetKeyboard{
		charset:      charset,
		baseStream:   output,
		outputStream: output,
		input:        make(chan keyboardTransport, 100),
		complete:     make(chan bool, 1),
		eventPump:    eventPump,
		lock:         newKeyboardLock(),
		decoder:      newKeyboardDecoder(middlewares...),
	}
	keyboard.promptCommands.Init()

	return keyboard, nil
}

// SetLock will buffer all text output without sending until the provided lockName
// is cleared with ClearLock, or until the provided duration expires. This method
// is primarily used by telopts to handle changes in communication semantics.  According
// to the Telnet RFC, communication semantics should change the moment a side sends
// a command that requests that they change.  Since it is not known at that time whether
// the remote can receive these semantics, it is recommended that writes are buffered
// until the remote responds to the request.
func (k *TelnetKeyboard) SetLock(lockName string, duration time.Duration) {
	k.lock.SetLock(lockName, duration)
}

// ClearLock will clear a named lock in order to end buffering (assuming there are no
// other active locks) and immediately write buffered text.
func (k *TelnetKeyboard) ClearLock(lockName string) {
	k.lock.ClearLock(lockName)
}

// HasActiveLock will indicate whether a named lock is currently active on the keyboard
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
	promptCommands := k.promptCommands.Get()
	if c.OpCode == GA && promptCommands&PromptCommandGA == 0 {
		return nil
	} else if c.OpCode == EOR && promptCommands&PromptCommandEOR == 0 {
		return nil
	}

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

func (k *TelnetKeyboard) writeText(data TerminalData) error {
	b, err := k.charset.Encode(data.String())
	if err != nil {
		return err
	}

	return k.writeOutput(b)
}

func (k *TelnetKeyboard) write(transport keyboardTransport) bool {
	var err error

	if transport.data != nil {
		k.decoder.Decode(k.terminal, transport.data)
	} else if transport.unparsedString != "" {
		k.decoder.DecodeString(k.terminal, transport.unparsedString)
	}

	decoded := k.decoder.Decoded()
	for _, data := range decoded {
		switch d := data.(type) {
		case CommandData:
			err = k.writeCommand(d.Command)
		case PromptData:
			prompts := k.promptCommands.Get()

			if prompts&PromptCommandEOR != 0 {
				transport.data = PromptData(PromptCommandEOR)
				err = k.writeCommand(Command{
					OpCode: EOR,
				})
			} else if prompts&PromptCommandGA != 0 {
				transport.data = PromptData(PromptCommandGA)
				err = k.writeCommand(Command{
					OpCode: GA,
				})
			} else {
				continue
			}
		default:
			err = k.writeText(d)
		}

		if err != nil {
			return k.handleError(err)
		}

		k.eventPump.EncounteredOutboundData(data)
	}

	if transport.postSend != nil {
		err = transport.postSend()
	}

	return k.handleError(err)
}

func (k *TelnetKeyboard) handleError(err error) bool {
	if err == nil {
		return true
	}

	if errors.Is(err, io.EOF) {
		return false
	}

	k.encounteredError(err)
	return true
}

func (k *TelnetKeyboard) keyboardLoop(ctx context.Context) {
	queuedWrites := make([]keyboardTransport, 0, 50)

keyboardLoop:
	for {
		select {
		case <-ctx.Done():
			break keyboardLoop
		case input := <-k.input:
			_, isCommand := input.data.(CommandData)
			if isCommand {
				if !k.write(input) {
					break keyboardLoop
				}

				continue
			}

			if len(queuedWrites) > 0 || k.lock.IsLocked() {
				// We may have unlocked but the unlock handler hasn't actually
				// run yet- we don't want this random bit of text to write out of
				// order, so place it at the end of the queue if one exists
				queuedWrites = append(queuedWrites, input)
				continue
			}

			if !k.write(input) {
				break keyboardLoop
			}

		case <-k.lock.C:
			// Make sure the lock hasn't unlocked & relocked in the time we've been away
			if !k.lock.IsLocked() {
				// Write all queued text
				for _, singleWrite := range queuedWrites {
					if !k.write(singleWrite) {
						break keyboardLoop
					}
				}

				queuedWrites = queuedWrites[:0]
			}
		}
	}

	// Try to flush any remaining text
	anyWriteFailed := false
	if len(queuedWrites) > 0 && !k.lock.IsLocked() {
		for _, singleWrite := range queuedWrites {
			if !k.write(singleWrite) {
				anyWriteFailed = true
				break
			}
		}
	}

	for !anyWriteFailed {
		select {
		case input := <-k.input:
			_, isCommand := input.data.(CommandData)

			if !k.lock.IsLocked() || isCommand {
				if !k.write(input) {
					anyWriteFailed = true
					continue
				}
			}
		default:
			// If we get to the end of the channel, we're done
			anyWriteFailed = true
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

// WriteCommand will queue a command to be sent to the remote. A post-send event can be provided,
// which is useful for cases where the provided command will signal to the remote that the
// communication semantic is changing in some way. If the postSend method is not nil, it will
// be executed immediately after writing the command to the output stream, and can be used
// to change the communication semantic for future writes.
func (k *TelnetKeyboard) WriteCommand(c Command, postSend func() error) {
	k.input <- keyboardTransport{
		data:     CommandData{c},
		postSend: postSend,
	}
}

func (k *TelnetKeyboard) LineOut(t *Terminal, data TerminalData) {
	k.input <- keyboardTransport{data: data}
}

// WriteString will queue some text to be sent to the remote
func (k *TelnetKeyboard) WriteString(str string) {
	if len(str) == 0 {
		return
	}

	k.input <- keyboardTransport{
		unparsedString: str,
	}
}

// waitForExit will block until the keyboard has been disposed of
func (k *TelnetKeyboard) waitForExit() {
	<-k.complete
	k.complete <- true
}

// SetPromptCommand will activate a particular prompt command and permit
// it to be sent by the keyboard.  Prompt commands are IAC GA/IAC EOR, commands
// that indicate to the remote where to place a prompt
func (k *TelnetKeyboard) SetPromptCommand(flag PromptCommands) {
	k.promptCommands.SetPromptCommand(flag)
}

// ClearPromptCommand will deactivate a particular prompt command and prevent it
// from being sent by the keyboard. Prompt commands are IAC GA/IAC EOR, commands
// that indicate to the remote where to place a prompt
func (k *TelnetKeyboard) ClearPromptCommand(flag PromptCommands) {
	k.promptCommands.ClearPromptCommand(flag)
}

// SendPromptHint will send a IAC GA or IAC EOR if possible, indicating
// to the remote to place a prompt after the most-recently-sent text.
//
// This command will send an EOR if that telopt is active.  Otherwise,
// it will send a GA if it isn't being suppressed. If it is not valid to
// send either prompt hint, this method will do nothing.
//
// If one wants to send an IAC GA or IAC EOR command, this method should be used
// rather than WriteCommand. Commands sent via WriteCommand will not be buffered
// when the keyboard is under a lock, so prompt hints sent via WriteCommand will arrive
// before the prompt text when a keyboard lock is active.
func (k *TelnetKeyboard) SendPromptHint() {
	k.input <- keyboardTransport{
		data: PromptData(0),
	}
}

func (k *TelnetKeyboard) WrapWriter(wrap func(io.Writer) (io.Writer, error)) error {
	wrapped, err := wrap(k.baseStream)
	if err != nil {
		return err
	}

	k.outputStream = wrapped
	return nil
}
