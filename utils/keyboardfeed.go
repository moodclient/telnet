package utils

import (
	"strings"
	"sync"

	"github.com/charmbracelet/x/input"
	"github.com/moodclient/telnet"
	"github.com/moodclient/telnet/telopts"
)

type KeyboardFeed struct {
	terminal *telnet.Terminal
	driver   *input.Reader

	inputLock sync.Mutex

	justAppendedCR bool
	bufferedInput  strings.Builder

	echoPublisher *telnet.EventPublisher[string]
}

type EchoEvent func(terminal *telnet.Terminal, text string)

func NewKeyboardFeed(terminal *telnet.Terminal, inputDriver *input.Reader, echoHandlers []EchoEvent) (*KeyboardFeed, error) {
	feed := &KeyboardFeed{
		terminal:      terminal,
		echoPublisher: telnet.NewPublisher(echoHandlers),
		driver:        inputDriver,
	}
	terminal.RegisterTelOptEventHook(feed.telOptEvents)

	return feed, nil
}

func (f *KeyboardFeed) RegisterEchoHandler(hook EchoEvent) {
	f.echoPublisher.Register(telnet.EventHook[string](hook))
}

func (f *KeyboardFeed) appendInput(input rune) {
	f.inputLock.Lock()
	defer f.inputLock.Unlock()

	// \r needs to become \r\n in line mode
	if input == '\r' {
		f.justAppendedCR = true
		f.bufferedInput.WriteRune(input)
	} else if input == '\n' && f.justAppendedCR {
		// The input sent us \r\n but we already sent the \r off with a \n so just eat this
		f.justAppendedCR = false
		return
	} else if input == '\n' {
		f.bufferedInput.WriteString("\r\n")
	} else {
		// We have some text other than \n so clear the carriage return flag
		f.justAppendedCR = false
		f.bufferedInput.WriteRune(input)
	}
}

func (f *KeyboardFeed) flushInput() string {
	f.inputLock.Lock()
	defer f.inputLock.Unlock()

	str := f.bufferedInput.String()
	f.bufferedInput.Reset()

	return str
}

func (f *KeyboardFeed) handleText(text rune) {
	// When we switch over to character mode, it can be in a different goroutine, so
	// in order to prevent characters from going out of order when we flush our outstanding
	// buffer, we'll always append and then flush
	f.appendInput(text)

	if f.terminal.IsCharacterMode() || text == '\n' || text == '\r' {
		f.terminal.Keyboard().WriteString(f.flushInput())
		return
	}
}

func (f *KeyboardFeed) FeedLoop() error {

	for {
		events, err := f.driver.ReadEvents()
		if err != nil {
			return err
		}

		for _, event := range events {
			switch e := event.(type) {
			case input.WindowSizeEvent:
				naws, err := telnet.GetTelOpt[telopts.NAWS](f.terminal)
				if err != nil {
					// Ignore problems with naws
					continue
				}

				naws.SetLocalSize(e.Width, e.Height)
			case input.KeyPressEvent:
				if e.Code != 0 {
					f.handleText(e.Code)
				}
			}
		}
	}
}

func (f *KeyboardFeed) telOptEvents(terminal *telnet.Terminal, event telnet.TelOptEvent) {
	switch typed := event.(type) {
	case telnet.TelOptStateChangeEvent:
		if typed.Side != telnet.TelOptSideRemote {
			return
		}

		// If we switch over to character mode as a result of a remote change, flush
		// all characters
		if terminal.IsCharacterMode() {
			terminal.Keyboard().WriteString(f.flushInput())
		}
	}
}
