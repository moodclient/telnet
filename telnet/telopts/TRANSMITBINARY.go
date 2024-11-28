package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const transmitbinaryKeyboardLock string = "lock.binary"
const transmitbinary telnet.TelOptCode = 0

func TRANSMITBINARY(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &TRANSMITBINARYOption{
		NewBaseTelOpt(usage),
	}
}

type TRANSMITBINARYOption struct {
	BaseTelOpt
}

var _ telnet.TelnetOption = &TRANSMITBINARYOption{}

func (o *TRANSMITBINARYOption) Code() telnet.TelOptCode {
	return transmitbinary
}

func (o *TRANSMITBINARYOption) String() string {
	return "TRANSMIT-BINARY"
}

func (o *TRANSMITBINARYOption) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptRequested {
		o.Terminal().Keyboard().SetLock(transmitbinaryKeyboardLock, telnet.DefaultKeyboardLock)
		return nil
	}

	o.Terminal().Keyboard().ClearLock(transmitbinaryKeyboardLock)
	o.Terminal().Charset().BinaryEncode = newState == telnet.TelOptActive

	return nil
}

func (o *TRANSMITBINARYOption) TransitionRemoteState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptActive {
		o.Terminal().Charset().BinaryDecode = true
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Charset().BinaryDecode = false
	}

	return nil
}

func (o *TRANSMITBINARYOption) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("transmit-binary: unknown subnegotiation: %+v", subnegotiation)
}

func (o *TRANSMITBINARYOption) SubnegotiationString(subnegotiation []byte) (string, error) {
	return "", fmt.Errorf("transmit-binary: unknown subnegotiation: %+v", subnegotiation)
}
