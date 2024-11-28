package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const eorKeyboardLock string = "lock.eor"
const eor telnet.TelOptCode = 25

func EOR(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &EOROption{
		NewBaseTelOpt(usage),
	}
}

type EOROption struct {
	BaseTelOpt
}

func (o *EOROption) Code() telnet.TelOptCode {
	return eor
}

func (o *EOROption) String() string {
	return "EOR"
}

func (o *EOROption) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptRequested {
		o.Terminal().Keyboard().SetLock(eorKeyboardLock, telnet.DefaultKeyboardLock)
		return nil
	}

	o.Terminal().Keyboard().ClearLock(eorKeyboardLock)

	if newState == telnet.TelOptActive {
		o.Terminal().Keyboard().SetPromptCommand(telnet.PromptCommandEOR)
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Keyboard().ClearPromptCommand(telnet.PromptCommandEOR)
	}

	return nil
}

func (o *EOROption) TransitionRemoteState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptActive {
		o.Terminal().Printer().SetPromptCommand(telnet.PromptCommandEOR)
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Printer().ClearPromptCommand(telnet.PromptCommandEOR)
	}

	return nil
}

func (o *EOROption) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("eor: unknown subnegotiation: %+v", subnegotiation)
}

func (o *EOROption) SubnegotiationString(subnegotiation []byte) (string, error) {
	return "", fmt.Errorf("eor: unknown subnegotiation: %+v", subnegotiation)
}
