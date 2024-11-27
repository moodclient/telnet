package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const transmitbinaryKeyboardLock string = "lock.binary"
const CodeTRANSMITBINARY telnet.TelOptCode = 0

func TRANSMITBINARYRegistration() telnet.TelOptFactory {
	return func(terminal *telnet.Terminal) telnet.TelnetOption {
		return &TRANSMITBINARY{
			NewBaseTelOpt(terminal),
		}
	}
}

type TRANSMITBINARY struct {
	BaseTelOpt
}

var _ telnet.TelnetOption = &TRANSMITBINARY{}

func (o *TRANSMITBINARY) Code() telnet.TelOptCode {
	return CodeTRANSMITBINARY
}

func (o *TRANSMITBINARY) String() string {
	return "TRANSMIT-BINARY"
}

func (o *TRANSMITBINARY) TransitionLocalState(newState telnet.TelOptState) error {
	if newState == telnet.TelOptRequested {
		o.Terminal().Keyboard().SetLock(transmitbinaryKeyboardLock, telnet.DefaultKeyboardLock)
		return nil
	}

	o.Terminal().Keyboard().ClearLock(transmitbinaryKeyboardLock)
	o.Terminal().Charset().BinaryEncode = newState == telnet.TelOptActive

	return nil
}

func (o *TRANSMITBINARY) TransitionRemoteState(newState telnet.TelOptState) error {
	if newState == telnet.TelOptActive {
		o.Terminal().Charset().BinaryDecode = true
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Charset().BinaryDecode = false
	}

	return nil
}

func (o *TRANSMITBINARY) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("transmit-binary: unknown subnegotiation: %+v", subnegotiation)
}

func (o *TRANSMITBINARY) SubnegotiationString(subnegotiation []byte) (string, error) {
	return "", fmt.Errorf("transmit-binary: unknown subnegotiation: %+v", subnegotiation)
}
