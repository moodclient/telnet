package telopts

import (
	"fmt"
	"sync"

	"github.com/moodclient/telnet"
)

const naws telnet.TelOptCode = 31

const (
	NAWSEventRemoteSize int = iota
)

func RegisterNAWS(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &NAWS{
		BaseTelOpt: NewBaseTelOpt(naws, "NAWS", usage),
	}
}

type NAWS struct {
	BaseTelOpt

	localLock  sync.Mutex
	remoteLock sync.Mutex

	localWidth   int
	localHeight  int
	remoteWidth  int
	remoteHeight int
}

func (o *NAWS) writeSizeSubnegotiation(width, height int) {
	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode: telnet.SB,
		Option: naws,
		Subnegotiation: []byte{
			byte((width >> 8) & 0xff),
			byte(width & 0xff),
			byte((height >> 8) & 0xff),
			byte(height & 0xff),
		},
	})
}

func (o *NAWS) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptActive {
		o.localLock.Lock()
		defer o.localLock.Unlock()

		// NAWS works by having the client subnegotiate its bounds to the server after activation
		// and whenever it changes
		if o.localWidth > 0 && o.localHeight > 0 {
			o.writeSizeSubnegotiation(o.localWidth, o.localHeight)
		}
	}

	return nil
}

func (o *NAWS) storeRemoteSize(width, height int) {
	o.remoteLock.Lock()
	defer o.remoteLock.Unlock()

	o.remoteWidth = width
	o.remoteHeight = height
}

func (o *NAWS) Subnegotiate(subnegotiation []byte) error {
	if o.RemoteState() != telnet.TelOptActive {
		return nil
	}

	if len(subnegotiation) != 4 {
		return fmt.Errorf("naws: expected a four byte subnegotiation but received %d", len(subnegotiation))
	}

	width := (int(subnegotiation[0]) << 8) | int(subnegotiation[1])
	height := (int(subnegotiation[2]) << 8) | int(subnegotiation[3])

	o.storeRemoteSize(width, height)
	o.Terminal().RaiseTelOptEvent(telnet.TelOptEventData{
		Option:    o,
		EventType: NAWSEventRemoteSize,
	})

	return nil
}

func (o *NAWS) SubnegotiationString(subnegotiation []byte) (string, error) {
	return fmt.Sprintf("%+v", subnegotiation), nil
}

func (o *NAWS) SetLocalSize(newWidth, newHeight int) {
	o.localLock.Lock()
	defer o.localLock.Unlock()

	if o.localWidth == newWidth && o.localHeight == newHeight {
		return
	}

	o.localWidth = newWidth
	o.localHeight = newHeight

	if o.LocalState() == telnet.TelOptActive {
		o.writeSizeSubnegotiation(newWidth, newHeight)
	}
}

func (o *NAWS) GetRemoteSize() (width, height int) {
	o.remoteLock.Lock()
	defer o.remoteLock.Unlock()

	return o.remoteWidth, o.remoteHeight
}

func (o *NAWS) EventString(eventData telnet.TelOptEventData) (eventName string, payload string, err error) {
	if eventData.EventType == NAWSEventRemoteSize {
		return "Updated Remote Size", "", nil
	}

	return o.BaseTelOpt.EventString(eventData)
}
