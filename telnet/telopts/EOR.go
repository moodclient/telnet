package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const eor telnet.TelOptCode = 25

type EOR struct {
	BaseTelOpt
}

var _ telnet.TelnetOption = &EOR{}

func (o *EOR) Code() telnet.TelOptCode {
	return eor
}

func (o *EOR) String() string {
	return "EOR"
}

func (o *EOR) TransitionLocalState(newState telnet.TelOptState) error {
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
