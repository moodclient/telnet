package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const suppressgoahead telnet.TelOptCode = 3

type SUPPRESSGOAHEAD struct {
	BaseTelOpt
}

var _ telnet.TelnetOption = &SUPPRESSGOAHEAD{}

func (o *SUPPRESSGOAHEAD) Code() telnet.TelOptCode {
	return suppressgoahead
}

func (o *SUPPRESSGOAHEAD) String() string {
	return "SUPPRESS-GO-AHEAD"
}

func (o *SUPPRESSGOAHEAD) TransitionLocalState(newState telnet.TelOptState) error {
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
