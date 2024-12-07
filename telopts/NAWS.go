package telopts

import (
	"fmt"
	"sync"

	"github.com/moodclient/telnet"
)

const naws telnet.TelOptCode = 31

type NAWSRemoteSizeChangedEvent struct {
	BaseTelOptEvent
	NewRemoteWidth  int
	NewRemoteHeight int
}

func (e NAWSRemoteSizeChangedEvent) String() string {
	return fmt.Sprintf("NAWS Remote Size Changed- Width: %d, Height: %d", e.NewRemoteWidth, e.NewRemoteHeight)
}

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
	}, nil)
}

func (o *NAWS) TransitionLocalState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return postSend, err
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

	return postSend, nil
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
	o.Terminal().RaiseTelOptEvent(NAWSRemoteSizeChangedEvent{
		BaseTelOptEvent: BaseTelOptEvent{o},
		NewRemoteWidth:  width,
		NewRemoteHeight: height,
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
