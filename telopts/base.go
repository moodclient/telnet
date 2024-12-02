package telopts

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/moodclient/telnet"
)

type BaseTelOpt struct {
	code        telnet.TelOptCode
	name        string
	terminal    *telnet.Terminal
	localState  uint32
	remoteState uint32
	usage       telnet.TelOptUsage
}

func NewBaseTelOpt(code telnet.TelOptCode, name string, usage telnet.TelOptUsage) BaseTelOpt {
	return BaseTelOpt{
		code:        code,
		name:        name,
		usage:       usage,
		localState:  uint32(telnet.TelOptInactive),
		remoteState: uint32(telnet.TelOptInactive),
	}
}

func (o *BaseTelOpt) Code() telnet.TelOptCode {
	return o.code
}

func (o *BaseTelOpt) String() string {
	return o.name
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

func (o *BaseTelOpt) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("%s: unexpected subnegotiation %+v", strings.ToLower(o.name), subnegotiation)
}

func (o *BaseTelOpt) SubnegotiationString(subnegotiation []byte) (string, error) {
	return "", fmt.Errorf("%s: unexpected subnegotiation %+v", strings.ToLower(o.name), subnegotiation)
}

func (o *BaseTelOpt) EventString(eventData telnet.TelOptEventData) (eventName string, payload string, err error) {
	return "", "", fmt.Errorf("%s: unexpected event %+v", strings.ToLower(o.name), eventData)
}
