package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

const echo telnet.TelOptCode = 1

func ECHO(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &ECHOOption{
		NewBaseTelOpt(usage),
	}
}

// ECHOOption indicates whether the local will repeat text sent from the remote back to the remote.  In practice,
// clients will tend to echo locally if the remote is not set to echo, so ECHO is used far more often
// to stop the remote from echoing locally than actually echoing to the remote.  As a result, this
// telopt doesn't do anything at all, since the lib consumer needs to decide what ECHO being on actually
// means.
type ECHOOption struct {
	BaseTelOpt
}

func (o *ECHOOption) Code() telnet.TelOptCode {
	return echo
}

func (o *ECHOOption) String() string {
	return "ECHO"
}

func (o *ECHOOption) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("echo: unknown subnegotiation: %+v", subnegotiation)
}

func (o *ECHOOption) SubnegotiationString(subnegotiation []byte) (string, error) {
	return "", fmt.Errorf("echo: unknown subnegotiation: %+v", subnegotiation)
}
