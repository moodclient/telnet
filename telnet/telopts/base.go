package telopts

import (
	"github.com/cannibalvox/moodclient/telnet"
	"time"
)

const LocalBlockTimeout = 5 * time.Second
const LocalBlockLocalRequest = "block.localRequested"
const LocalBlockRemoteRequest = "block.remoteRequested"

type BaseTelOpt struct {
	terminal    *telnet.Terminal
	localState  telnet.TelOptState
	remoteState telnet.TelOptState
}

func NewBaseTelOpt(terminal *telnet.Terminal) BaseTelOpt {
	return BaseTelOpt{
		terminal:    terminal,
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
