package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const eorKeyboardLock string = "lock.eor"
const CodeEOR telnet.TelOptCode = 25

func EORRegistration() telnet.TelOptFactory {
	return func(terminal *telnet.Terminal) telnet.TelnetOption {
		return &EOR{
			NewBaseTelOpt(terminal),
		}
	}
}

type EOR struct {
	BaseTelOpt
}

var _ telnet.TelnetOption = &EOR{}

func (o *EOR) Code() telnet.TelOptCode {
	return CodeEOR
}

func (o *EOR) String() string {
	return "EOR"
}

func (o *EOR) TransitionLocalState(newState telnet.TelOptState) error {
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
