package telnet

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"

	"golang.org/x/text/transform"
)

// TelnetScanner is used internally by TelnetPrinter to read sequences from a Reader and output
// units of received output.  It is exported due to the object being potentially useful outside
// the context of this library's Terminal object. If you intend to use Terminal, there is no
// need to use or think about this type.
//
// TelnetScanner's Scan method works like an io.Scanner, except that it accepts a context.Context.
// If the ctx is cancelled or timed out, Scan will return false with with the appropriate error.
// Otherwise, it will return true until it reaches the input stream's EOF. Like io.Scanner, Scan
// is a blocking call.
//
// After Scan returns, even if it returns false, Err and Output may have useful return values.
// Output returns a PrinterOutput object, or nil. PrinterOutput may be one of the PrinterOutput
// implementations defined in this package (TextOutput, PromptOutput, SequenceOutput, etc.).
//
// PrinterOutput's String method will always return the correct text to print to a VT100 compatible
// terminal, and EscapedString will always return the correct text to print to a default log in which
// you'd like to see escape sequences, commands, and control characters.
//
// Otherwise, you can inspect the PrinterOutput objects by using a type switch.
//
// As with Scanner, one should deal with the Output() return value, if any, before dealing with
// the Err() return value.
type TelnetScanner struct {
	scanner    *bufio.Scanner
	scanResult chan bool

	charset       *Charset
	parser        *TerminalDataParser
	atEOF         bool
	bytesToDecode []byte

	err        error
	nextOutput TerminalData
	outCommand Command
}

// NewTelnetScanner creates a new TelnetScanner from a Charset (used to decode bytes from
// the stream) and an input stream
func NewTelnetScanner(charset *Charset, inputStream io.Reader) *TelnetScanner {
	scan := bufio.NewScanner(inputStream)

	scanner := &TelnetScanner{
		scanner:       scan,
		scanResult:    make(chan bool, 1),
		charset:       charset,
		parser:        NewTerminalDataParser(),
		bytesToDecode: make([]byte, 0, 100),
	}

	scan.Split(scanner.ScanTelnet)
	return scanner
}

// Err returns the error, if any, raised by the most recent call to Scan
func (s *TelnetScanner) Err() error {
	return s.err
}

// Output returns the PrinterOutput, if any, assembled by the most recent call to Scan
func (s *TelnetScanner) Output() TerminalData {
	return s.nextOutput
}

func (s *TelnetScanner) pushError(err error) {
	if err != nil && s.err == nil {
		s.err = err
	}
}

func (s *TelnetScanner) pushCommand() {
	if s.nextOutput != nil {
		return
	}

	if s.outCommand.OpCode == GA {
		s.nextOutput = PromptData{Type: PromptCommandGA}
	} else if s.outCommand.OpCode == EOR {
		s.nextOutput = PromptData{Type: PromptCommandEOR}
	} else if s.outCommand.OpCode != 0 {
		s.nextOutput = CommandData{Command: s.outCommand}
	}

	s.outCommand = Command{}
}

func (s *TelnetScanner) processDanglingBytes() TerminalData {
	tmpBytesSlice := s.bytesToDecode
	var fallback bool
	var decodedBytes [1000]byte

	defer func() {
		if len(s.bytesToDecode) > 0 && len(tmpBytesSlice) < len(s.bytesToDecode) {
			if len(tmpBytesSlice) > 0 {
				copy(s.bytesToDecode[:len(tmpBytesSlice)], tmpBytesSlice)
			}

			s.bytesToDecode = s.bytesToDecode[:len(tmpBytesSlice)]
		}
	}()

	output := NextOutput(s.parser, "")
	if output != nil {
		return output
	}

	for len(tmpBytesSlice) > 0 {
		consumed, buffered, fellback, err := s.charset.Decode(decodedBytes[:], tmpBytesSlice, fallback)

		fallback = fallback || fellback

		if consumed > 0 {
			tmpBytesSlice = tmpBytesSlice[consumed:]
		}

		if buffered > 0 {
			output := NextOutput(s.parser, decodedBytes[0:buffered])
			if output != nil {
				return output
			}
		}

		if errors.Is(err, transform.ErrShortSrc) {
			if s.atEOF {
				tmpBytesSlice = tmpBytesSlice[:0]
			}

			return nil
		} else if err != nil {
			s.err = err
			return nil
		}
	}

	return s.parser.Flush()
}

// Scan will block until either the provided context is done, or a complete block of data is
// received from the input stream. "Complete" is subjective, but the TelnetScanner will not output
// partial ANSI sequences or partial glyphs of text.
//
// Scan returns true if the caller should continue to call Scan to receive additional data. After
// calling Scan, Err and Output should be called to check for useful data.
func (s *TelnetScanner) Scan(ctx context.Context) bool {
	s.err = nil
	s.nextOutput = nil

	// We usually build up a text buffer and then return it when we find something other
	// than text. As a result, when we come back, we need to return whatever we found that
	// wasn't text, if anything
	s.pushCommand()
	if s.nextOutput != nil || s.err != nil {
		return true
	}

	s.nextOutput = s.processDanglingBytes()
	if s.nextOutput != nil || s.err != nil {
		return true
	}

	var err error
	for ctx.Err() == nil && s.cancellableScan(ctx) {
		s.atEOF = false
		s.err = s.scanner.Err()

		bytes := s.scanner.Bytes()
		if len(bytes) == 0 {
			continue
		}

		if len(bytes) > 1 && bytes[0] == IAC {
			s.outCommand, err = parseCommand(bytes)
			s.pushError(err)
			s.bytesToDecode = s.bytesToDecode[:0]

			s.pushCommand()
			return true
		}

		s.bytesToDecode = append(s.bytesToDecode, bytes...)
		s.nextOutput = s.processDanglingBytes()

		if s.nextOutput != nil || s.err != nil {
			return true
		}
	}

	s.atEOF = true
	s.err = s.scanner.Err()
	return len(s.bytesToDecode) > 0
}

func (s *TelnetScanner) cancellableScan(ctx context.Context) bool {
	go func() {
		s.scanResult <- s.scanner.Scan()
	}()

	select {
	case result := <-s.scanResult:
		return result
	case <-ctx.Done():
		return false
	}
}

func (s *TelnetScanner) scanTelnetWithoutEOF(data []byte) (advance int, err error) {
	specialCharIndex := bytes.Index(data, []byte{IAC})

	if specialCharIndex > 0 {
		// Release all data until we get to an IAC
		return specialCharIndex, nil
	} else if specialCharIndex < 0 {
		// No special char, dump everything
		return len(data), nil
	}

	// Release 'IAC IAC' on its own, it's actually escaped text
	if len(data) >= 2 && data[1] == IAC {
		return 2, nil
	}

	// if it's just IAC by itself, wait for more data
	if len(data) <= 1 {
		return 0, nil
	}

	// IAC GA, IAC EOR, and IAC NOP release on their own
	// SE should never appear here but if it does we should recover by consuming the data
	if data[1] == GA || data[1] == NOP || data[1] == SE || data[1] == EOR ||
		data[1] == AYT {
		return 2, nil
	}

	// All other codes require at least 3 characters
	if len(data) < 3 {
		return 0, nil
	}

	if data[1] == WILL || data[1] == WONT || data[1] == DO || data[1] == DONT {
		// Negotiation commands in three code sets
		return 3, nil
	}

	if data[1] != SB {
		// We received some kind of exotic code that we don't actually handle.
		return 2, nil
	}

	nextIndex := 0

	for {
		nextSpecialCharIndex := bytes.Index(data[nextIndex+1:], []byte{IAC})

		// No more IACs, subnegotiation end is not in buffer yet
		if nextSpecialCharIndex < 0 {
			return 0, nil
		}

		nextIndex += nextSpecialCharIndex + 1
		if len(data) <= nextIndex+1 {
			// IAC is last character, but we need more
			return 0, nil
		}

		if data[nextIndex+1] == SE {
			// Found subnegotiation end
			return nextIndex + 2, nil
		}

		if data[nextIndex+1] == IAC {
			// Double 255's should be skipped over
			nextIndex++
		}
	}
}

// ScanTelnet is a method used as the split method for io.Scanner. It will receive
// chunks of text or commands as individual tokens.
func (s *TelnetScanner) ScanTelnet(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 {
		return 0, nil, nil
	}

	advance, err = s.scanTelnetWithoutEOF(data)

	if err != nil || (advance == 0 && !atEOF) {
		return advance, data[:advance], err
	}

	if advance == 0 && atEOF {
		return len(data), data, nil
	}

	if advance == 2 && data[0] == IAC && data[1] == IAC {
		return 2, data[1:2], nil
	}

	return advance, data[:advance], nil
}
