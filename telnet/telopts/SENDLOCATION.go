package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
	"sync/atomic"
)

const sendlocation telnet.TelOptCode = 23

func SENDLOCATION(usage telnet.TelOptUsage, localLocation string) telnet.TelnetOption {
	option := &SENDLOCATIONOption{
		BaseTelOpt: NewBaseTelOpt(usage),
	}

	option.remoteLocation.Store("")
	option.localLocation.Store(localLocation)

	return option
}

type SENDLOCATIONOption struct {
	BaseTelOpt

	localLocation  atomic.Value
	remoteLocation atomic.Value
}

func (o *SENDLOCATIONOption) Code() telnet.TelOptCode {
	return sendlocation
}

func (o *SENDLOCATIONOption) String() string {
	return "SEND-LOCATION"
}

func (o *SENDLOCATIONOption) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptActive {
		o.Terminal().Keyboard().WriteCommand(telnet.Command{
			OpCode:         telnet.SB,
			Option:         sendlocation,
			Subnegotiation: []byte(o.localLocation.Load().(string)),
		})
	}

	return nil
}

func (o *SENDLOCATIONOption) TransitionRemoteState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptInactive {
		o.remoteLocation.Store("")
	}

	return nil
}

func (o *SENDLOCATIONOption) Subnegotiate(subnegotiation []byte) error {
	if o.RemoteState() == telnet.TelOptActive {
		o.remoteLocation.Store(string(subnegotiation))
	}

	return fmt.Errorf("send-location: unknown subnegotiation: %+v", subnegotiation)
}

func (o *SENDLOCATIONOption) SubnegotiationString(subnegotiation []byte) (string, error) {
	return string(subnegotiation), nil
}

func (o *SENDLOCATIONOption) SetLocalLocation(location string) {
	// This could hypothetically break, but probably not?
	o.localLocation.Store(location)
	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         sendlocation,
		Subnegotiation: []byte(location),
	})
}

func (o *SENDLOCATIONOption) RemoteLocation() string {
	return o.remoteLocation.Load().(string)
}
