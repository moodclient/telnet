package telopts

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/cannibalvox/moodclient/telnet"
)

const ttype telnet.TelOptCode = 24
const ttypeKeyboardLock string = "lock.ttype"

const (
	ttypeIS byte = iota
	ttypeSEND
)

const (
	TTYPEEventRemoteTerminals int = iota
)

func RegisterTTYPE(usage telnet.TelOptUsage, localTerminals []string) telnet.TelnetOption {
	return &TTYPE{
		BaseTelOpt: NewBaseTelOpt(usage),

		localTerminals: localTerminals,
	}
}

type TTYPE struct {
	BaseTelOpt

	localTerminalLock  sync.Mutex
	remoteTerminalLock sync.Mutex

	localTerminalCursor int
	localTerminals      []string

	remoteTerminals []string
}

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
	terminalBytes := make([]byte, 0, len(terminal)+1)
	terminalBytes = append(terminalBytes, ttypeIS)
	terminalBytes = append(terminalBytes, []byte(terminal)...)

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
		o.remoteTerminalLock.Lock()
		defer o.remoteTerminalLock.Unlock()

		o.remoteTerminals = nil

		return nil
	} else if newState == telnet.TelOptActive {
		// If we didn't request to use TTYPE but the client did, start blocking outbound until we harvest
		// all the info we want
		if !o.Terminal().Keyboard().HasActiveLock(ttypeKeyboardLock) {
			o.Terminal().Keyboard().SetLock(ttypeKeyboardLock, telnet.DefaultKeyboardLock)
		}

		o.localTerminalLock.Lock()
		defer o.localTerminalLock.Unlock()

		o.writeRequestSend()

		return nil
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

func (o *TTYPE) addTerminal(subnegotiation []byte) bool {
	o.remoteTerminalLock.Lock()
	defer o.remoteTerminalLock.Unlock()

	var newTerminal string
	if len(subnegotiation) > 1 {
		newTerminal = string(subnegotiation[1:])
	}

	if len(o.remoteTerminals) == 0 || o.remoteTerminals[len(o.remoteTerminals)-1] != newTerminal {
		// New terminal, so let's ask for another
		o.remoteTerminals = append(o.remoteTerminals, newTerminal)
		o.writeRequestSend()
		return false
	}

	o.Terminal().Keyboard().ClearLock(ttypeKeyboardLock)
	return true
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

		complete := o.addTerminal(subnegotiation)
		if complete {
			o.Terminal().RaiseTelOptEvent(telnet.TelOptEventData{
				Option:    o,
				EventType: TTYPEEventRemoteTerminals,
			})
		}

		return nil
	}

	// Remote is sending us a SEND request to give them a terminal
	if subnegotiation[0] == ttypeSEND {
		if o.LocalState() != telnet.TelOptActive {
			return nil
		}

		o.localTerminalLock.Lock()
		defer o.localTerminalLock.Unlock()

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
	o.localTerminalLock.Lock()
	defer o.localTerminalLock.Unlock()

	o.localTerminals = terminals
}

func (o *TTYPE) GetRemoteTerminals() []string {
	o.remoteTerminalLock.Lock()
	defer o.remoteTerminalLock.Unlock()

	return o.remoteTerminals
}

func (o *TTYPE) EventString(eventData telnet.TelOptEventData) (eventName string, payload string, err error) {
	if eventData.EventType == TTYPEEventRemoteTerminals {
		return "Update Terminals", "", nil
	}

	return "", "", fmt.Errorf("ttype: unknown event: %+v", eventData)
}
