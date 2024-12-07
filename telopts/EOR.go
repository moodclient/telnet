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

func (o *EOR) TransitionLocalState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return postSend, err
	}

	if newState == telnet.TelOptRequested {
		o.Terminal().Keyboard().SetLock(eorKeyboardLock, telnet.DefaultKeyboardLock)
		return postSend, nil
	}

	return func() error {
		if newState == telnet.TelOptActive {
			o.Terminal().Keyboard().SetPromptCommand(telnet.PromptCommandEOR)
		} else if newState == telnet.TelOptInactive {
			o.Terminal().Keyboard().ClearPromptCommand(telnet.PromptCommandEOR)
		}
		o.Terminal().Keyboard().ClearLock(eorKeyboardLock)
		return nil
	}, nil
}

func (o *EOR) TransitionRemoteState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return postSend, err
	}

	if newState == telnet.TelOptActive {
		o.Terminal().Printer().SetPromptCommand(telnet.PromptCommandEOR)
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Printer().ClearPromptCommand(telnet.PromptCommandEOR)
	}

	return postSend, nil
}
