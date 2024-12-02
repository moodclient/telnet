package telopts

import (
	"github.com/moodclient/telnet"
)

const echo telnet.TelOptCode = 1

func RegisterECHO(usage telnet.TelOptUsage) telnet.TelnetOption {
	return &ECHO{
		NewBaseTelOpt(echo, "ECHO", usage),
	}
}

// ECHO indicates whether the local will repeat text sent from the remote back to the remote.  In practice,
// clients will tend to echo locally if the remote is not set to echo, so ECHO is used far more often
// to stop the remote from echoing locally than actually echoing to the remote.  As a result, this
// telopt doesn't do anything at all, since the lib consumer needs to decide what ECHO being on actually
// means.
type ECHO struct {
	BaseTelOpt
}
