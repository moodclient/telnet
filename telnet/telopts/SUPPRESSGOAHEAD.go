package telopts

import (
	"fmt"

	"github.com/moodclient/telnet/telnet"
)

const suppressgoaheadKeyboardLock string = "lock.suppress-go-ahead"
const suppressgoahead telnet.TelOptCode = 3

func RegisterSUPPRESSGOAHEAD(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &SUPPRESSGOAHEAD{
		NewBaseTelOpt(usage),
	}
}

type SUPPRESSGOAHEAD struct {
	BaseTelOpt
}

func (o *SUPPRESSGOAHEAD) Code() telnet.TelOptCode {
	return suppressgoahead
}

func (o *SUPPRESSGOAHEAD) String() string {
	return "SUPPRESS-GO-AHEAD"
}

func (o *SUPPRESSGOAHEAD) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptRequested {
		o.Terminal().Keyboard().SetLock(suppressgoaheadKeyboardLock, telnet.DefaultKeyboardLock)
		return nil
	}

	o.Terminal().Keyboard().ClearLock(suppressgoaheadKeyboardLock)

	if newState == telnet.TelOptActive {
		o.Terminal().Keyboard().ClearPromptCommand(telnet.PromptCommandGA)
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Keyboard().SetPromptCommand(telnet.PromptCommandGA)
	}

	return nil
}

func (o *SUPPRESSGOAHEAD) TransitionRemoteState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptActive {
		o.Terminal().Printer().ClearPromptCommand(telnet.PromptCommandGA)
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Printer().SetPromptCommand(telnet.PromptCommandGA)
	}

	return nil
}

func (o *SUPPRESSGOAHEAD) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("suppress-go-ahead: unknown subnegotiation: %+v", subnegotiation)
}

func (o *SUPPRESSGOAHEAD) SubnegotiationString(subnegotiation []byte) (string, error) {
	return "", fmt.Errorf("suppress-go-ahead: unknown subnegotiation: %+v", subnegotiation)
}

func (o *SUPPRESSGOAHEAD) EventString(eventData telnet.TelOptEventData) (eventName string, payload string, err error) {
	return "", "", fmt.Errorf("suppress-go-ahead: unknown event: %+v", eventData)
}
