package telopts

import (
	"fmt"
	"sync/atomic"

	"github.com/moodclient/telnet"
)

const sendlocation telnet.TelOptCode = 23

type SENDLOCATIONRemoteUpdatedEvent struct {
	BaseTelOptEvent
	NewLocation string
}

func (e SENDLOCATIONRemoteUpdatedEvent) String() string {
	return fmt.Sprintf("SEND-LOCATION Remote Updated: %s", e.NewLocation)
}

func RegisterSENDLOCATION(usage telnet.TelOptUsage, localLocation string) telnet.TelnetOption {
	option := &SENDLOCATION{
		BaseTelOpt: NewBaseTelOpt(sendlocation, "SEND-LOCATION", usage),
	}

	option.remoteLocation.Store("")
	option.localLocation.Store(localLocation)

	return option
}

type SENDLOCATION struct {
	BaseTelOpt

	localLocation  atomic.Value
	remoteLocation atomic.Value
}

func (o *SENDLOCATION) TransitionLocalState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return postSend, err
	}

	if newState == telnet.TelOptActive {
		o.Terminal().Keyboard().WriteCommand(telnet.Command{
			OpCode:         telnet.SB,
			Option:         sendlocation,
			Subnegotiation: []byte(o.localLocation.Load().(string)),
		}, nil)
	}

	return postSend, nil
}

func (o *SENDLOCATION) TransitionRemoteState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return postSend, err
	}

	if newState == telnet.TelOptInactive {
		o.remoteLocation.Store("")
	}

	return postSend, nil
}

func (o *SENDLOCATION) Subnegotiate(subnegotiation []byte) error {
	if o.RemoteState() == telnet.TelOptActive {
		o.remoteLocation.Store(string(subnegotiation))
		o.Terminal().RaiseTelOptEvent(SENDLOCATIONRemoteUpdatedEvent{
			BaseTelOptEvent: BaseTelOptEvent{o},
			NewLocation:     string(subnegotiation),
		})
	}

	return o.BaseTelOpt.Subnegotiate(subnegotiation)
}

func (o *SENDLOCATION) SubnegotiationString(subnegotiation []byte) (string, error) {
	return string(subnegotiation), nil
}

func (o *SENDLOCATION) SetLocalLocation(location string) {
	// This could hypothetically break, but probably not?
	o.localLocation.Store(location)
	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         sendlocation,
		Subnegotiation: []byte(location),
	}, nil)
}

func (o *SENDLOCATION) RemoteLocation() string {
	return o.remoteLocation.Load().(string)
}
