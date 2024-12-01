package telopts

import (
	"fmt"

	"github.com/moodclient/telnet/telnet"
)

const transmitbinaryKeyboardLock string = "lock.binary"
const transmitbinary telnet.TelOptCode = 0

func RegisterTRANSMITBINARY(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &TRANSMITBINARY{
		NewBaseTelOpt(usage),
	}
}

type TRANSMITBINARY struct {
	BaseTelOpt
}

func (o *TRANSMITBINARY) Code() telnet.TelOptCode {
	return transmitbinary
}

func (o *TRANSMITBINARY) String() string {
	return "TRANSMIT-BINARY"
}

func (o *TRANSMITBINARY) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptRequested {
		o.Terminal().Keyboard().SetLock(transmitbinaryKeyboardLock, telnet.DefaultKeyboardLock)
		return nil
	}

	o.Terminal().Keyboard().ClearLock(transmitbinaryKeyboardLock)
	o.Terminal().Charset().SetBinaryEncode(newState == telnet.TelOptActive)

	return nil
}

func (o *TRANSMITBINARY) TransitionRemoteState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptActive {
		o.Terminal().Charset().SetBinaryDecode(true)
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Charset().SetBinaryDecode(false)
	}

	return nil
}

func (o *TRANSMITBINARY) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("transmit-binary: unknown subnegotiation: %+v", subnegotiation)
}

func (o *TRANSMITBINARY) SubnegotiationString(subnegotiation []byte) (string, error) {
	return "", fmt.Errorf("transmit-binary: unknown subnegotiation: %+v", subnegotiation)
}

func (o *TRANSMITBINARY) EventString(eventData telnet.TelOptEventData) (eventName string, payload string, err error) {
	return "", "", fmt.Errorf("transmit-binary: unknown event: %+v", eventData)
}
