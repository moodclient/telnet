package telopts

import (
	"fmt"
	"sync/atomic"

	"github.com/moodclient/telnet"
)

const sendlocation telnet.TelOptCode = 23

const (
	SENDLOCATIONEventRemoteLocation int = iota
)

func RegisterSENDLOCATION(usage telnet.TelOptUsage, localLocation string) telnet.TelnetOption {
	option := &SENDLOCATION{
		BaseTelOpt: NewBaseTelOpt(usage),
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

func (o *SENDLOCATION) Code() telnet.TelOptCode {
	return sendlocation
}

func (o *SENDLOCATION) String() string {
	return "SEND-LOCATION"
}

func (o *SENDLOCATION) TransitionLocalState(newState telnet.TelOptState) error {
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

func (o *SENDLOCATION) TransitionRemoteState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptInactive {
		o.remoteLocation.Store("")
	}

	return nil
}

func (o *SENDLOCATION) Subnegotiate(subnegotiation []byte) error {
	if o.RemoteState() == telnet.TelOptActive {
		o.remoteLocation.Store(string(subnegotiation))
		o.Terminal().RaiseTelOptEvent(telnet.TelOptEventData{
			Option:    o,
			EventType: SENDLOCATIONEventRemoteLocation,
		})
	}

	return fmt.Errorf("send-location: unknown subnegotiation: %+v", subnegotiation)
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
	})
}

func (o *SENDLOCATION) RemoteLocation() string {
	return o.remoteLocation.Load().(string)
}

func (o *SENDLOCATION) EventString(eventData telnet.TelOptEventData) (eventName string, payload string, err error) {
	if eventData.EventType == SENDLOCATIONEventRemoteLocation {
		return "Update Location", "", nil
	}

	return "", "", fmt.Errorf("send-location: unknown error: %+v", eventData)
}
