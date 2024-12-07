package telopts

import (
	"github.com/moodclient/telnet"
)

const suppressgoaheadKeyboardLock string = "lock.suppress-go-ahead"
const suppressgoahead telnet.TelOptCode = 3

func RegisterSUPPRESSGOAHEAD(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &SUPPRESSGOAHEAD{
		NewBaseTelOpt(suppressgoahead, "SUPPRESS-GO-AHEAD", usage),
	}
}

type SUPPRESSGOAHEAD struct {
	BaseTelOpt
}

func (o *SUPPRESSGOAHEAD) TransitionLocalState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return postSend, err
	}

	if newState == telnet.TelOptRequested {
		o.Terminal().Keyboard().SetLock(suppressgoaheadKeyboardLock, telnet.DefaultKeyboardLock)
		return postSend, nil
	}

	return func() error {
		if newState == telnet.TelOptActive {
			o.Terminal().Keyboard().ClearPromptCommand(telnet.PromptCommandGA)
		} else if newState == telnet.TelOptInactive {
			o.Terminal().Keyboard().SetPromptCommand(telnet.PromptCommandGA)
		}

		o.Terminal().Keyboard().ClearLock(suppressgoaheadKeyboardLock)
		return nil
	}, nil
}

func (o *SUPPRESSGOAHEAD) TransitionRemoteState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return postSend, err
	}

	if newState == telnet.TelOptActive {
		o.Terminal().Printer().ClearPromptCommand(telnet.PromptCommandGA)
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Printer().SetPromptCommand(telnet.PromptCommandGA)
	}

	return postSend, nil
}
