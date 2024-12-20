package utils

import (
	"bufio"
	"io"
	"os"
	"time"

	"github.com/moodclient/telnet"
	"github.com/moodclient/telnet/telopts"
)

type KeyboardFeed struct {
	terminal *telnet.Terminal
	input    io.Reader
	parser   *telnet.TerminalDataParser

	characterMode *CharacterModeTracker
	lineFeed      *LineFeed
}

func NewKeyboardFeed(terminal *telnet.Terminal, input io.Reader, lineFeed *LineFeed, characterMode *CharacterModeTracker) (*KeyboardFeed, error) {
	feed := &KeyboardFeed{
		terminal:      terminal,
		input:         input,
		lineFeed:      lineFeed,
		characterMode: characterMode,
		parser:        telnet.NewTerminalDataParser(),
	}

	terminal.RegisterTelOptEventHook(feed.telOptEvents)

	return feed, nil
}

func (f *KeyboardFeed) FeedLoop() error {
	scanner := bufio.NewScanner(f.input)
	scanner.Split(bufio.ScanRunes)

	nulTimeout := time.NewTimer(100 * time.Millisecond)
	nulTimeout.Stop()

	scannerSet := make(chan bool)
	scannerReset := make(chan bool)
	go func() {
		for scanner.Scan() {
			scannerSet <- true
			<-scannerReset
		}

		scannerSet <- false
	}()

loop:
	for {
		select {
		case c := <-scannerSet:
			if !c {
				break loop
			}

			text := scanner.Text()

			if text == "\x7f" {
				text = "\x08"
			}

			if text == "\x03" {
				os.Exit(0)
			}

			f.parser.FireSingle(f.terminal, text, f.lineFeed.LineIn)
			nulTimeout.Reset(100 * time.Millisecond)

			if scanner.Err() != nil {
				return scanner.Err()
			}

			scannerReset <- true

		case <-nulTimeout.C:
			f.parser.FireSingle(f.terminal, "\x00", f.lineFeed.LineIn)
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
	}

	f.lineFeed.SetCharacterMode(f.characterMode.IsCharacterMode())
}
