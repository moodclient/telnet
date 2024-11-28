package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const suppressgoaheadKeyboardLock string = "lock.suppress-go-ahead"
const suppressgoahead telnet.TelOptCode = 3

func SUPPRESSGOAHEAD(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &SUPPRESSGOAHEADOption{
		NewBaseTelOpt(usage),
	}
}

type SUPPRESSGOAHEADOption struct {
	BaseTelOpt
}

var _ telnet.TelnetOption = &SUPPRESSGOAHEADOption{}

func (o *SUPPRESSGOAHEADOption) Code() telnet.TelOptCode {
	return suppressgoahead
}

func (o *SUPPRESSGOAHEADOption) String() string {
	return "SUPPRESS-GO-AHEAD"
}

func (o *SUPPRESSGOAHEADOption) TransitionLocalState(newState telnet.TelOptState) error {
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

func (o *SUPPRESSGOAHEADOption) TransitionRemoteState(newState telnet.TelOptState) error {
	if newState == telnet.TelOptActive {
		o.Terminal().Printer().ClearPromptCommand(telnet.PromptCommandGA)
	} else if newState == telnet.TelOptInactive {
		o.Terminal().Printer().SetPromptCommand(telnet.PromptCommandGA)
	}

	return nil
}

func (o *SUPPRESSGOAHEADOption) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("suppress-go-ahead: unknown subnegotiation: %+v", subnegotiation)
}

func (o *SUPPRESSGOAHEADOption) SubnegotiationString(subnegotiation []byte) (string, error) {
	return "", fmt.Errorf("suppress-go-ahead: unknown subnegotiation: %+v", subnegotiation)
}
