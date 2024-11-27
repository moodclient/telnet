package telopts

import (
	"errors"
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
	"strings"
)

const CodeTTYPE telnet.TelOptCode = 24
const ttypeKeyboardLock string = "lock.ttype"

const (
	ttypeIS byte = iota
	ttypeSEND
)

func TTYPERegistration(localTerminals []string) telnet.TelOptFactory {
	return func(terminal *telnet.Terminal) telnet.TelnetOption {
		return &TTYPE{
			BaseTelOpt: NewBaseTelOpt(terminal),

			localTerminals: localTerminals,
		}
	}
}

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
	return CodeTTYPE
}

func (o *TTYPE) String() string {
	return "TTYPE"
}

func (o *TTYPE) writeRequestSend() {
	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         CodeTTYPE,
		Subnegotiation: []byte{ttypeSEND},
	})
}

func (o *TTYPE) writeTerminal(terminal string) {
	terminalBytes := make([]byte, 0, len(terminal)+1)
	terminalBytes = append(terminalBytes, ttypeIS)
	terminalBytes = append(terminalBytes, []byte(terminal)...)

	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         CodeTTYPE,
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
			o.Terminal().Keyboard().SetLock(ttypeKeyboardLock, telnet.DefaultKeyboardLock)
		}

		o.writeRequestSend()
	} else if newState == telnet.TelOptRequested {
		// Start blocking outbound when we request to use TTYPE until we harvest all the info we want
		o.Terminal().Keyboard().SetLock(ttypeKeyboardLock, telnet.DefaultKeyboardLock)
	}

	return nil
}

func (o *TTYPE) SubnegotiationString(subnegotiation []byte) (string, error) {
	if len(subnegotiation) < 1 {
		return "", errors.New("ttype: received empty subnegotiation")
	}

	if subnegotiation[0] == ttypeIS {
		var sb strings.Builder
		sb.WriteString("IS ")
		sb.WriteString(string(subnegotiation[1:]))
		return sb.String(), nil
	}

	if subnegotiation[0] == ttypeSEND {
		return "SEND", nil
	}

	return "", fmt.Errorf("ttype: unknown subnegotiation: %+v", subnegotiation)
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
			newTerminal = string(subnegotiation[1:])
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
