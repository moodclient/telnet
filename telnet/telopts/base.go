package telopts

import (
	"github.com/cannibalvox/moodclient/telnet"
	"sync/atomic"
)

type BaseTelOpt struct {
	terminal    *telnet.Terminal
	localState  uint32
	remoteState uint32
	usage       telnet.TelOptUsage
}

func NewBaseTelOpt(usage telnet.TelOptUsage) BaseTelOpt {
	return BaseTelOpt{
		usage:       usage,
		localState:  uint32(telnet.TelOptInactive),
		remoteState: uint32(telnet.TelOptInactive),
	}
}

func (o *BaseTelOpt) LocalState() telnet.TelOptState {
	return telnet.TelOptState(atomic.LoadUint32(&o.localState))
}

func (o *BaseTelOpt) RemoteState() telnet.TelOptState {
	return telnet.TelOptState(atomic.LoadUint32(&o.remoteState))
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
	atomic.StoreUint32(&o.localState, uint32(newState))

	return nil
}

func (o *BaseTelOpt) TransitionRemoteState(newState telnet.TelOptState) error {
	atomic.StoreUint32(&o.remoteState, uint32(newState))
	return nil
}
