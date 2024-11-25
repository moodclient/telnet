package telopts

import (
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
)

package telopts

import (
"fmt"
"github.com/cannibalvox/moodclient/telnet"
)

const echo telnet.TelOptCode = 1

// ECHO indicates whether the local will repeat text sent from the remote back to the remote.  In practice,
// clients will tend to echo locally if the remote is not set to echo, so ECHO is used far more often
// to stop the remote from echoing locally than actually echoing to the remote.  As a result, this
// telopt doesn't do anything at all, since the lib consumer needs to decide what ECHO being on actually
// means.
type ECHO struct {
	BaseTelOpt
}

var _ telnet.TelnetOption = &ECHO{}

func (o *ECHO) Code() telnet.TelOptCode {
	return echo
}

func (o *ECHO) String() string {
	return "ECHO"
}

func (o *ECHO) Subnegotiate(subnegotiation []byte) error {
	return fmt.Errorf("echo: unknown subnegotiation: %+v", subnegotiation)
}

