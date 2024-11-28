package telopts

import (
	"github.com/cannibalvox/moodclient/telnet"
)

type BaseTelOpt struct {
	terminal    *telnet.Terminal
	localState  telnet.TelOptState
	remoteState telnet.TelOptState
	usage       telnet.TelOptUsage
}

func NewBaseTelOpt(usage telnet.TelOptUsage) BaseTelOpt {
	return BaseTelOpt{
		usage:       usage,
		localState:  telnet.TelOptInactive,
		remoteState: telnet.TelOptInactive,
	}
}

func (o *BaseTelOpt) LocalState() telnet.TelOptState {
	return o.localState
}

func (o *BaseTelOpt) RemoteState() telnet.TelOptState {
	return o.remoteState
}

func (o *BaseTelOpt) Usage() telnet.TelOptUsage {
	return o.usage
}

func (o *BaseTelOpt) Initialize(terminal *telnet.Terminal) {
	o.terminal = terminal
}

func (o *BaseTelOpt) Terminal() *telnet.Terminal {
	return o.terminal
}

func (o *BaseTelOpt) TransitionLocalState(newState telnet.TelOptState) error {
	o.localState = newState

	return nil
}

func (o *BaseTelOpt) TransitionRemoteState(newState telnet.TelOptState) error {
	o.remoteState = newState
	return nil
}
