package telopts

import (
	"fmt"

	"github.com/moodclient/telnet/telnet"
)

const eorKeyboardLock string = "lock.eor"
const eor telnet.TelOptCode = 25

func RegisterEOR(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &EOR{
		NewBaseTelOpt(usage),
	}
}

type EOR struct {
	BaseTelOpt
}

func (o *EOR) Code() telnet.TelOptCode {
	return eor
}

func (o *EOR) String() string {
	return "EOR"
}

func (o *EOR) TransitionLocalState(newState telnet.TelOptState) error {
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

func (o *EOR) TransitionRemoteState(newState telnet.TelOptState) error {
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

func (o *EOR) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("eor: unknown subnegotiation: %+v", subnegotiation)
}

func (o *EOR) SubnegotiationString(subnegotiation []byte) (string, error) {
	return "", fmt.Errorf("eor: unknown subnegotiation: %+v", subnegotiation)
}

func (o *EOR) EventString(eventData telnet.TelOptEventData) (eventName string, payload string, err error) {
	return "", "", fmt.Errorf("eor: unknown event: %+v", eventData)
}
