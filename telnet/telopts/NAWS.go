package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const naws telnet.TelOptCode = 31

type NAWS struct {
	BaseTelOpt

	localWidth   int
	localHeight  int
	remoteWidth  int
	remoteHeight int
}

var _ telnet.TelnetOption = &NAWS{}

func (o *NAWS) Code() telnet.TelOptCode {
	return naws
}

func (o *NAWS) String() string {
	return "NAWS"
}

func (o *NAWS) writeSizeSubnegotiation() {
	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode: telnet.SB,
		Option: naws,
		Subnegotiation: []byte{
			byte(o.localWidth >> 8),
			byte(o.localWidth & 0xff),
			byte(o.localHeight >> 8),
			byte(o.localHeight & 0xff),
		},
	})
}

func (o *NAWS) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptActive {
		// NAWS works by having the client subnegotiate its bounds to the server after activation
		// and whenever it changes
		if o.localHeight > 0 && o.localWidth > 0 {
			o.writeSizeSubnegotiation()
		}
	}

	return nil
}

func (o *NAWS) Subnegotiate(subnegotiation []byte) error {
	if o.RemoteState() != telnet.TelOptActive {
		return nil
	}

	if len(subnegotiation) != 4 {
		return fmt.Errorf("naws: expected a four byte subnegotiation but received %d", len(subnegotiation))
	}

	o.remoteWidth = (int(subnegotiation[0]) << 8) | int(subnegotiation[1])
	o.remoteHeight = (int(subnegotiation[0]) << 8) | int(subnegotiation[1]) // height

	return nil
}

func (o *NAWS) SetLocalSize(width, height int) {
	if o.localWidth == width && o.localHeight == height {
		return
	}

	o.localWidth = width
	o.localHeight = height

	if o.LocalState() == telnet.TelOptActive {
		o.writeSizeSubnegotiation()
	}
}

func (o *NAWS) GetRemoteSize() (width, height int) {
	return o.remoteWidth, o.remoteHeight
}
