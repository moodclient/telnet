package telopts

import (
	"github.com/moodclient/telnet"
)

const transmitbinaryKeyboardLock string = "lock.binary"
const transmitbinary telnet.TelOptCode = 0

func RegisterTRANSMITBINARY(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &TRANSMITBINARY{
		NewBaseTelOpt(transmitbinary, "TRANSMIT-BINARY", usage),
	}
}

type TRANSMITBINARY struct {
	BaseTelOpt
}

func (o *TRANSMITBINARY) TransitionLocalState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return postSend, err
	}

	if newState == telnet.TelOptRequested {
		o.Terminal().Keyboard().SetLock(transmitbinaryKeyboardLock, telnet.DefaultKeyboardLock)
		return postSend, nil
	}

	return func() error {
		o.Terminal().Charset().SetBinaryEncode(newState == telnet.TelOptActive)
		o.Terminal().Keyboard().ClearLock(transmitbinaryKeyboardLock)
		return nil
	}, nil
}

func (o *TRANSMITBINARY) TransitionRemoteState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return postSend, err
	}

	if newState == telnet.TelOptActive {
		o.Terminal().Charset().SetBinaryDecode(true)
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Charset().SetBinaryDecode(false)
	}

	return postSend, nil
}
