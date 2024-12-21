package telopts

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/moodclient/telnet"
)

const linemode telnet.TelOptCode = 34

type LineModeFlags int

const (
	LineModeEDIT LineModeFlags = 1 << iota
	LineModeTRAPSIG
	LineModeACK
	LineModeSOFTTAB
	LineModeLITECHO
)

const supportedModes = LineModeEDIT | LineModeTRAPSIG

const (
	linemodeMODE byte = iota + 1
	linemodeFORWARDMASK
	linemodeSLC
)

func (f LineModeFlags) String() string {
	var sb strings.Builder
	hasSeenValue := false

	sb.WriteRune('[')
	if f&LineModeEDIT != 0 {
		hasSeenValue = true
		sb.WriteString("EDIT")
	}

	if f&LineModeTRAPSIG != 0 {
		if hasSeenValue {
			sb.WriteString(" ")
		}
		hasSeenValue = true
		sb.WriteString("TRAPSIG")
	}

	if f&LineModeSOFTTAB != 0 {
		if hasSeenValue {
			sb.WriteString(" ")
		}
		hasSeenValue = true
		sb.WriteString("SOFTTAB")
	}

	if f&LineModeLITECHO != 0 {
		if hasSeenValue {
			sb.WriteString(" ")
		}
		hasSeenValue = true
		sb.WriteString("LITECHO")
	}

	if f&LineModeACK != 0 {
		if hasSeenValue {
			sb.WriteString(" ")
		}
		sb.WriteString("ACK")
	}
	sb.WriteRune(']')

	return sb.String()
}

type LINEMODEChangeEvent struct {
	BaseTelOptEvent
	NewMode LineModeFlags
}

func (e LINEMODEChangeEvent) String() string {
	return "LINEMODE Mode changed: " + e.NewMode.String()
}

func RegisterLINEMODE(usage telnet.TelOptUsage, mode LineModeFlags) telnet.TelnetOption {
	linemode := &LINEMODE{
		BaseTelOpt: NewBaseTelOpt(linemode, "LINEMODE", usage),
	}
	linemode.mode.Store(int64(mode))
	return linemode
}

// LINEMODE allows linemode to be negotiated- this is used by some BBS's but we
// are not going to support most features provided by the telopt.  We'll just support
// MODE EDIT and that's it.  RFC LINEMODE also has a system
// for defining characters to trigger telnet functions, and FORWARDMASK, which allows
// the remote to demand we instantly send them our line-in-progress. We will
// accept the functions but never use them, and we will reject all attempts to
// establish FORWARDMASK.  We will also reject attempts at MODE SOFT_TAB and
// MODE LIT_ECHO.  We will accept MODE TRAPSIG, as that is required by the
// RFC, but we won't do anything about it since we don't allow the client
// to send any of the TRAPSIG signals on demand anyway.
type LINEMODE struct {
	BaseTelOpt

	mode atomic.Int64
}

func (m *LINEMODE) writeModeCommand(mode LineModeFlags) {
	command := telnet.Command{
		OpCode:         telnet.SB,
		Option:         linemode,
		Subnegotiation: []byte{linemodeMODE, byte(mode)},
	}
	m.Terminal().Keyboard().WriteCommand(command, nil)
}

func (m *LINEMODE) TransitionRemoteState(newState telnet.TelOptState) (func() error, error) {
	if newState == telnet.TelOptActive {
		// We need to send the MODE request immediately after the client confirms their
		// state
		m.writeModeCommand(m.Mode())
	}

	return m.BaseTelOpt.TransitionRemoteState(newState)
}

func (m *LINEMODE) updateMode(mode LineModeFlags) {
	m.mode.Store(int64(mode))
	m.Terminal().RaiseTelOptEvent(LINEMODEChangeEvent{
		BaseTelOptEvent: BaseTelOptEvent{m},
		NewMode:         mode,
	})
}

func (m *LINEMODE) subnegotiateMODE(subnegotiation []byte) error {
	requestedMask := LineModeFlags(subnegotiation[1])
	currentMode := m.Mode()
	isClient := m.LocalState() == telnet.TelOptActive

	withoutACK := requestedMask & ^LineModeACK

	if withoutACK == currentMode {
		// Nothing has changed
		return nil
	}

	if requestedMask&LineModeACK != 0 && isClient {
		// Ignore acks
		return nil
	}

	if isClient {
		// Do we support what the server sent?
		supported := requestedMask & supportedModes
		if supported == requestedMask {
			// Ack this
			m.writeModeCommand(requestedMask | LineModeACK)
			m.updateMode(requestedMask)
			return nil
		}

		// Tell the server we can't
		m.writeModeCommand(supported)

		if supported != currentMode {
			m.updateMode(supported)
		}

		return nil
	}

	// Don't allow the client to turn off EDIT or TRAPSIG if we requested it
	required := currentMode & (LineModeEDIT | LineModeTRAPSIG)
	correctedMask := withoutACK | required

	// Don't allow the client to turn on new flags
	correctedMask &= currentMode

	if correctedMask != currentMode {
		m.updateMode(correctedMask)

		if requestedMask&LineModeACK == 0 && correctedMask != requestedMask {
			// The client asked for a mask we couldn't do but didn't ACK so
			// we can update our request
			m.writeModeCommand(correctedMask)
		}
	}

	return nil
}

func (m *LINEMODE) Subnegotiate(subnegotiation []byte) error {
	if len(subnegotiation) == 0 {
		return fmt.Errorf("linemode: received empty subnegotiation")
	}

	if subnegotiation[0] == linemodeSLC {
		// Don't do anything with SLC
		return nil
	}

	if len(subnegotiation) < 2 {
		return fmt.Errorf("linemode: unexpected subnegotiation: %+v", subnegotiation)
	}

	if subnegotiation[0] == linemodeMODE {
		return m.subnegotiateMODE(subnegotiation)
	}

	if (subnegotiation[0] == telnet.DONT || subnegotiation[0] == telnet.WONT) &&
		subnegotiation[1] == linemodeFORWARDMASK {
		// They're refusing to use forwardmask for some reason, and we
		// didn't want it anyway
		return nil
	}

	// Don't let the remote use FORWARDMASK
	if subnegotiation[0] == telnet.DO && subnegotiation[1] == linemodeFORWARDMASK {
		m.Terminal().Keyboard().WriteCommand(telnet.Command{
			OpCode:         telnet.SB,
			Option:         linemode,
			Subnegotiation: []byte{telnet.WONT, linemodeFORWARDMASK},
		}, nil)
		return nil
	}

	if subnegotiation[0] == telnet.WILL && subnegotiation[1] == linemodeFORWARDMASK {
		m.Terminal().Keyboard().WriteCommand(telnet.Command{
			OpCode:         telnet.SB,
			Option:         linemode,
			Subnegotiation: []byte{telnet.DONT, linemodeFORWARDMASK},
		}, nil)
		return nil
	}

	return m.BaseTelOpt.Subnegotiate(subnegotiation)
}

func (m *LINEMODE) SubnegotiationString(subnegotiation []byte) (string, error) {
	if len(subnegotiation) == 0 {
		return "", nil
	}

	var sb strings.Builder

	if subnegotiation[0] == linemodeSLC {
		sb.WriteString("SLC ")
		sb.WriteString(fmt.Sprintf("%+v", subnegotiation[1:]))
		return sb.String(), nil
	}

	if subnegotiation[0] == linemodeMODE {
		sb.WriteString("MODE ")
		if len(subnegotiation) > 1 {
			sb.WriteString(LineModeFlags(subnegotiation[1]).String())
		}
		return sb.String(), nil
	}

	if subnegotiation[0] == telnet.DO {
		sb.WriteString("DO ")
	} else if subnegotiation[0] == telnet.WILL {
		sb.WriteString("WILL ")
	} else if subnegotiation[0] == telnet.DONT {
		sb.WriteString("DONT ")
	} else if subnegotiation[0] == telnet.WONT {
		sb.WriteString("WONT ")
	} else {
		return m.BaseTelOpt.SubnegotiationString(subnegotiation)
	}

	if len(subnegotiation) > 1 && subnegotiation[1] == linemodeFORWARDMASK {
		sb.WriteString("FORWARDMASK")
	}

	return sb.String(), nil
}

func (m *LINEMODE) Mode() LineModeFlags {
	return LineModeFlags(m.mode.Load())
}

func (m *LINEMODE) SetMode(mode LineModeFlags) {
	mode &= supportedModes

	if mode != m.Mode() {
		m.updateMode(mode)
	}
}
