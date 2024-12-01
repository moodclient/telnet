package utils

import (
	"bufio"
	"io"
	"strings"
	"sync"

	"github.com/moodclient/telnet"
)

type KeyboardFeed struct {
	terminal *telnet.Terminal
	input    io.Reader

	inputLock sync.Mutex

	justAppendedCR bool
	bufferedInput  strings.Builder

	echoPublisher *telnet.EventPublisher[string]
}

type EchoEvent func(terminal *telnet.Terminal, text string)

func NewKeyboardFeed(terminal *telnet.Terminal, input io.Reader, echoHandlers []EchoEvent) (*KeyboardFeed, error) {
	feed := &KeyboardFeed{
		terminal:      terminal,
		input:         input,
		echoPublisher: telnet.NewPublisher(echoHandlers),
	}
	terminal.RegisterTelOptStateChangeEventHook(feed.telOptStateChange)

	return feed, nil
}

func (f *KeyboardFeed) RegisterEchoHandler(hook EchoEvent) {
	f.echoPublisher.Register(telnet.EventHook[string](hook))
}

func (f *KeyboardFeed) appendInput(input string) {
	f.inputLock.Lock()
	defer f.inputLock.Unlock()

	// \r needs to become \r\n in all cases
	if strings.HasSuffix(input, "\r") {
		f.justAppendedCR = true
		input = "\r\n"
	} else if input == "\n" && f.justAppendedCR {
		// The input sent us \r\n but we already sent the \r off with a \n so just eat this
		f.justAppendedCR = false
		return
	}

	// We have some text other than \n so clear the carriage return flag
	f.justAppendedCR = false

	// Replace \n by itself with \r\n
	if strings.HasSuffix(input, "\n") && !strings.HasSuffix(input, "\r\n") {
		input = input[:len(input)-1] + "\r\n"
	}

	f.echoPublisher.Fire(f.terminal, input)
	f.bufferedInput.WriteString(input)
}

func (f *KeyboardFeed) flushInput() string {
	f.inputLock.Lock()
	defer f.inputLock.Unlock()

	str := f.bufferedInput.String()
	f.bufferedInput.Reset()

	return str
}

func (f *KeyboardFeed) handleText(text string) {
	if len(text) == 0 {
		return
	}

	// When we switch over to character mode, it can be in a different goroutine, so
	// in order to prevent characters from going out of order when we flush our outstanding
	// buffer, we'll always append and then flush
	f.appendInput(text)

	if f.terminal.IsCharacterMode() {
		f.terminal.Keyboard().WriteString(f.flushInput())
		return
	}

	// Depending on what precisely our input reader is, we could receive different line
	// indicators.  Raw mode terminals give us \r, cooked mode terminals give us \r\n or
	// \n.  A process manually piping stuff in will probably give us \n.
	//
	// What we'll do is just always flush if the token is a linebreak symbol- appendinput
	// will do the thinking and add \n to \r itself and then eat any \n that might come after.
	// It will also replace \n with \r\n
	if text == "\n" || text == "\r" {
		f.terminal.Keyboard().WriteString(f.flushInput())
	}
}

func (f *KeyboardFeed) FeedLoop() error {
	scanner := bufio.NewScanner(f.input)
	scanner.Split(bufio.ScanRunes)

	for scanner.Scan() {
		f.handleText(scanner.Text())

		if scanner.Err() != nil {
			return scanner.Err()
		}
	}

	return scanner.Err()
}

func (f *KeyboardFeed) telOptStateChange(terminal *telnet.Terminal, data telnet.TelOptStateChangeData) {
	if data.Side != telnet.TelOptSideRemote {
		return
	}

	// If we switch over to character mode as a result of a remote change, flush
	// all characters
	if terminal.IsCharacterMode() {
		terminal.Keyboard().WriteString(f.flushInput())
	}
}
