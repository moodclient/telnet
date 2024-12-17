package utils

import (
	"bufio"
	"io"

	"github.com/moodclient/telnet"
	"github.com/moodclient/telnet/telopts"
)

type KeyboardFeed struct {
	terminal *telnet.Terminal
	input    io.Reader
	parser   *telnet.TerminalDataParser

	lineFeed *LineFeed
}

func NewKeyboardFeed(terminal *telnet.Terminal, input io.Reader, lineFeed *LineFeed) (*KeyboardFeed, error) {
	feed := &KeyboardFeed{
		terminal: terminal,
		input:    input,
		lineFeed: lineFeed,
		parser:   telnet.NewTerminalDataParser(),
	}

	terminal.RegisterTelOptEventHook(feed.telOptEvents)

	return feed, nil
}

func (f *KeyboardFeed) FeedLoop() error {
	scanner := bufio.NewScanner(f.input)
	scanner.Split(bufio.ScanRunes)

	for scanner.Scan() {
		f.parser.FireSingle(f.terminal, scanner.Text(), f.lineFeed.LineIn)

		if scanner.Err() != nil {
			return scanner.Err()
		}
	}

	return scanner.Err()
}

func (f *KeyboardFeed) telOptEvents(terminal *telnet.Terminal, event telnet.TelOptEvent) {
	switch typed := event.(type) {
	case telnet.TelOptStateChangeEvent:
		if typed.Side != telnet.TelOptSideRemote {
			return
		}

		_, isEcho := typed.TelnetOption.(*telopts.ECHO)
		if isEcho && typed.NewState == telnet.TelOptActive {
			f.lineFeed.SetSuppressLocalEcho(true)
		} else if isEcho && typed.NewState == telnet.TelOptInactive {
			f.lineFeed.SetSuppressLocalEcho(false)
		}

		f.lineFeed.SetCharacterMode(terminal.IsCharacterMode())
	}
}
