package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const suppressgoaheadKeyboardLock string = "lock.suppress-go-ahead"
const CodeSUPPRESSGOAHEAD telnet.TelOptCode = 3

func SUPPRESSGOAHEADRegistration() telnet.TelOptFactory {
	return func(terminal *telnet.Terminal) telnet.TelnetOption {
		return &SUPPRESSGOAHEAD{
			NewBaseTelOpt(terminal),
		}
	}
}

type SUPPRESSGOAHEAD struct {
	BaseTelOpt
}

var _ telnet.TelnetOption = &SUPPRESSGOAHEAD{}

func (o *SUPPRESSGOAHEAD) Code() telnet.TelOptCode {
	return CodeSUPPRESSGOAHEAD
}

func (o *SUPPRESSGOAHEAD) String() string {
	return "SUPPRESS-GO-AHEAD"
}

func (o *SUPPRESSGOAHEAD) TransitionLocalState(newState telnet.TelOptState) error {
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
