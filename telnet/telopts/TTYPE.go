package telopts

import (
	"encoding/ascii85"
	"errors"
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const ttype telnet.TelOptCode = 24
const ttypeKeyboardLock string = "lock.ttype"

const (
	ttypeIS byte = iota
	ttypeSEND
)

type TTYPE struct {
	BaseTelOpt

	localTerminalCursor int
	localTerminals      []string

	remoteReady     bool
	remoteTerminals []string
}

var TTYPEInstance = &TTYPE{}
var _ telnet.TelnetOption = TTYPEInstance

func (o *TTYPE) Code() telnet.TelOptCode {
	return ttype
}

func (o *TTYPE) String() string {
	return "TTYPE"
}

func (o *TTYPE) writeRequestSend() {
	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         ttype,
		Subnegotiation: []byte{ttypeSEND},
	})
}

func (o *TTYPE) writeTerminal(terminal string) {
	terminalBytes := make([]byte, len(terminal)+1)
	terminalBytes[0] = ttypeIS
	_ = ascii85.Encode(terminalBytes[1:], []byte(terminal))

	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         ttype,
		Subnegotiation: terminalBytes,
	})
}

func (o *TTYPE) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptInactive {
		o.localTerminalCursor = 0
	}

	return nil
}

func (o *TTYPE) TransitionRemoteState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptInactive {
		o.remoteTerminals = nil
		o.remoteReady = false
	} else if newState == telnet.TelOptActive {
		// If we didn't request to use TTYPE but the client did, start blocking outbound until we harvest
		// all the info we want
		if !o.Terminal().Keyboard().HasActiveLock(ttypeKeyboardLock) {
			o.Terminal().Keyboard().SetLock(ttypeKeyboardLock, LocalBlockTimeout)
		}

		o.writeRequestSend()
	} else if newState == telnet.TelOptRequested {
		// Start blocking outbound when we request to use TTYPE until we harvest all the info we want
		o.Terminal().Keyboard().SetLock(ttypeKeyboardLock, LocalBlockTimeout)
	}

	return nil
}

func (o *TTYPE) Subnegotiate(subnegotiation []byte) error {
	if len(subnegotiation) < 1 {
		return errors.New("ttype: received empty subnegotiation")
	}

	// Remote is sending us an IS subnegotation giving us a terminal
	if subnegotiation[0] == ttypeIS {
		if o.RemoteState() != telnet.TelOptActive {
			return nil
		}

		var newTerminal string
		if len(subnegotiation) > 1 {
			terminalBytes := make([]byte, len(subnegotiation)-1)
			_, _, err := ascii85.Decode(terminalBytes, subnegotiation[1:], true)
			if err != nil {
				return fmt.Errorf("ttype: failed to decode terminal name: %w", err)
			}

			newTerminal = string(terminalBytes)
		}

		if len(o.remoteTerminals) == 0 || o.remoteTerminals[len(o.remoteTerminals)-1] != newTerminal {
			// New terminal, so let's ask for another
			o.remoteTerminals = append(o.remoteTerminals, newTerminal)
			o.writeRequestSend()
		} else {
			o.Terminal().Keyboard().ClearLock(ttypeKeyboardLock)
			o.remoteReady = true
		}

		return nil
	}

	// Remote is sending us a SEND request to give them a terminal
	if subnegotiation[0] == ttypeSEND {
		if o.LocalState() != telnet.TelOptActive {
			return nil
		}

		if len(o.localTerminals) == 0 {
			o.writeTerminal("UNKNOWN")
			return nil
		}

		if o.localTerminalCursor >= len(o.localTerminals) {
			// Resend the last item until they shut up
			o.writeTerminal(o.localTerminals[len(o.localTerminals)-1])
			return nil
		}

		// Send the current terminal and then increment
		o.writeTerminal(o.localTerminals[o.localTerminalCursor])
		o.localTerminalCursor++

		return nil
	}

	return fmt.Errorf("ttype: unknown subnegotiation: %+v", subnegotiation)
}

func (o *TTYPE) SetLocalTerminals(terminals []string) {
	o.localTerminals = terminals
}

func (o *TTYPE) GetRemoteTerminals() []string {
	return o.remoteTerminals
}

func (o *TTYPE) IsRemoteReady() bool {
	return o.remoteReady
}
