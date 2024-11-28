package telopts

import (
	"errors"
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
	"strings"
	"sync"
)

const ttype telnet.TelOptCode = 24
const ttypeKeyboardLock string = "lock.ttype"

const (
	ttypeIS byte = iota
	ttypeSEND
)

func TTYPE(usage telnet.TelOptUsage, localTerminals []string) telnet.TelnetOption {
	return &TTYPEOption{
		BaseTelOpt: NewBaseTelOpt(usage),

		localTerminals: localTerminals,
	}
}

type TTYPEOption struct {
	BaseTelOpt

	terminalLock sync.Mutex

	localTerminalCursor int
	localTerminals      []string

	remoteTerminals []string
}

func (o *TTYPEOption) Code() telnet.TelOptCode {
	return ttype
}

func (o *TTYPEOption) String() string {
	return "TTYPE"
}

func (o *TTYPEOption) writeRequestSend() {
	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         ttype,
		Subnegotiation: []byte{ttypeSEND},
	})
}

func (o *TTYPEOption) writeTerminal(terminal string) {
	terminalBytes := make([]byte, 0, len(terminal)+1)
	terminalBytes = append(terminalBytes, ttypeIS)
	terminalBytes = append(terminalBytes, []byte(terminal)...)

	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         ttype,
		Subnegotiation: terminalBytes,
	})
}

func (o *TTYPEOption) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	o.terminalLock.Lock()
	defer o.terminalLock.Unlock()

	if newState == telnet.TelOptInactive {
		o.localTerminalCursor = 0
	}

	return nil
}

func (o *TTYPEOption) TransitionRemoteState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return err
	}

	o.terminalLock.Lock()
	defer o.terminalLock.Unlock()

	if newState == telnet.TelOptInactive {
		o.remoteTerminals = nil
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

func (o *TTYPEOption) SubnegotiationString(subnegotiation []byte) (string, error) {
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

func (o *TTYPEOption) Subnegotiate(subnegotiation []byte) error {
	if len(subnegotiation) < 1 {
		return errors.New("ttype: received empty subnegotiation")
	}

	o.terminalLock.Lock()
	defer o.terminalLock.Unlock()

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

func (o *TTYPEOption) SetLocalTerminals(terminals []string) {
	o.terminalLock.Lock()
	defer o.terminalLock.Unlock()

	o.localTerminals = terminals
}

func (o *TTYPEOption) GetRemoteTerminals() []string {
	o.terminalLock.Lock()
	defer o.terminalLock.Unlock()

	return o.remoteTerminals
}
