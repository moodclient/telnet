package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
	"sync/atomic"
)

const naws telnet.TelOptCode = 31

func NAWS(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &NAWSOption{
		BaseTelOpt: NewBaseTelOpt(usage),
	}
}

type NAWSOption struct {
	BaseTelOpt

	localWidth   uint32
	localHeight  uint32
	remoteWidth  uint32
	remoteHeight uint32
}

func (o *NAWSOption) Code() telnet.TelOptCode {
	return naws
}

func (o *NAWSOption) String() string {
	return "NAWS"
}

func (o *NAWSOption) writeSizeSubnegotiation(width, height uint16) {
	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode: telnet.SB,
		Option: naws,
		Subnegotiation: []byte{
			byte(width >> 8),
			byte(width & 0xff),
			byte(height >> 8),
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
		width := uint16(atomic.LoadUint32(&o.localWidth))
		height := uint16(atomic.LoadUint32(&o.localHeight))

		// NAWS works by having the client subnegotiate its bounds to the server after activation
		// and whenever it changes
		if width > 0 && height > 0 {
			o.writeSizeSubnegotiation(width, height)
		}
	}

	return nil
}

func (o *NAWSOption) Subnegotiate(subnegotiation []byte) error {
	if o.RemoteState() != telnet.TelOptActive {
		return nil
	}

	if len(subnegotiation) != 4 {
		return fmt.Errorf("naws: expected a four byte subnegotiation but received %d", len(subnegotiation))
	}

	width := uint32((int(subnegotiation[0]) << 8) | int(subnegotiation[1]))
	height := uint32((int(subnegotiation[2]) << 8) | int(subnegotiation[3]))

	atomic.StoreUint32(&o.remoteWidth, width)
	atomic.StoreUint32(&o.remoteHeight, height)

	return nil
}

func (o *NAWSOption) SubnegotiationString(subnegotiation []byte) (string, error) {
	return fmt.Sprintf("%+v", subnegotiation), nil
}

func (o *NAWSOption) SetLocalSize(newWidth, newHeight uint16) {
	oldWidth := uint16(atomic.LoadUint32(&o.localWidth))
	oldHeight := uint16(atomic.LoadUint32(&o.localHeight))

	if oldWidth == newWidth && oldHeight == newHeight {
		return
	}

	swappedWidth := atomic.CompareAndSwapUint32(&o.localWidth, uint32(oldWidth), uint32(newWidth))
	swappedHeight := atomic.CompareAndSwapUint32(&o.localHeight, uint32(oldHeight), uint32(newHeight))

	if swappedWidth && !swappedHeight {
		// We collided with another process doing something similar, whoever swapped the width wins
		atomic.StoreUint32(&o.localHeight, uint32(newHeight))
	} else if !swappedWidth {
		return
	}

	if o.LocalState() == telnet.TelOptActive {
		o.writeSizeSubnegotiation(newWidth, newHeight)
	}
}

func (o *NAWSOption) GetRemoteSize() (width, height uint16) {
	width = uint16(atomic.LoadUint32(&o.remoteWidth))
	height = uint16(atomic.LoadUint32(&o.remoteHeight))
	return width, height
}
