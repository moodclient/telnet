package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
	"sync"
)

const naws telnet.TelOptCode = 31

const (
	NAWSEventRemoteSize int = iota
)

func NAWS(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &NAWSOption{
		BaseTelOpt: NewBaseTelOpt(usage),
	}
}

type NAWSOption struct {
	BaseTelOpt

	localLock  sync.Mutex
	remoteLock sync.Mutex

	localWidth   int
	localHeight  int
	remoteWidth  int
	remoteHeight int
}

func (o *NAWSOption) Code() telnet.TelOptCode {
	return naws
}

func (o *NAWSOption) String() string {
	return "NAWS"
}

func (o *NAWSOption) writeSizeSubnegotiation(width, height int) {
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

func (o *NAWSOption) TransitionLocalState(newState telnet.TelOptState) error {
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

func (o *NAWSOption) storeRemoteSize(width, height int) {
	o.remoteLock.Lock()
	defer o.remoteLock.Unlock()

	o.remoteWidth = width
	o.remoteHeight = height
}

func (o *NAWSOption) Subnegotiate(subnegotiation []byte) error {
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

func (o *NAWSOption) SubnegotiationString(subnegotiation []byte) (string, error) {
	return fmt.Sprintf("%+v", subnegotiation), nil
}

func (o *NAWSOption) SetLocalSize(newWidth, newHeight int) {
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

func (o *NAWSOption) GetRemoteSize() (width, height int) {
	o.remoteLock.Lock()
	defer o.remoteLock.Unlock()

	return o.remoteWidth, o.remoteHeight
}
