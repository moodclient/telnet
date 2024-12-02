package telopts

import (
	"github.com/moodclient/telnet"
)

const eorKeyboardLock string = "lock.eor"
const eor telnet.TelOptCode = 25

func RegisterEOR(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &EOR{
		NewBaseTelOpt(eor, "EOR", usage),
	}
}

type EOR struct {
	BaseTelOpt
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
