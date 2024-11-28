package telnet

type TerminalSide byte

const (
	SideUnknown TerminalSide = iota
	SideClient
	SideServer
)

type CharsetUsage byte

const (
	// CharsetUsageBinary indicates that text communications should use a CHARSET-negotiated character set
	// if the connection is in BINARY mode, and the default character set otherwise
	CharsetUsageBinary CharsetUsage = iota
	// CharsetUsageAlways indicates that text communications should always use a CHARSET-negotiated character
	// set (if any) instead of the default character set
	CharsetUsageAlways
)

type TerminalConfig struct {
	// DefaultCharsetName is the registered IANA name of the character set to use for all communications not
	// sent via a negotiated charset (via the CHARSET telopt). RFC 854 (Telnet Protocol) specifies that by
	// default, communications take place in ASCII encoding.  RFC 5198 specified that since 2008, communications
	// should by default take place in UTF-8.  However, many active telnet services and a vanishingly small
	// number of telnet clients have not been updated to use UTF-8. While UTF-8, as a superset of ASCII,
	// will generally function just fine as a communications protocol with ASCII systems, it can be useful
	// to make US-ASCII the default character set, allow the remote to negotiate to UTF-8 if they want, and
	// use the current character set to determine support for sending things like emojis.
	//
	// Lastly, in the pre-2008 period, many telnet services were established in languages that could not use
	// US-ASCII under any circumstances and used other character sets as the default rather than implementing
	// CHARSET appropriately. For these services, launching with an alternative charset such as Big5 can be
	// necessary.
	//
	// The charset specified here will be used initially for all text communications until a different character
	// set is negotiated with the CHARSET telopt.  If there are non-charset text communications (see CharsetUsage),
	// this will be used for them.  Text sent in telopt subnegotiations will always use UTF-8 regardless of this
	// setting.
	//
	// If this characters set is US-ASCII and the remote indicates support for UTF-8 via a CHARSET negotiation
	// or some other mechanism, the default character set will be promoted to UTF-8.
	DefaultCharsetName string

	// CharsetUsage is only relevant if a new characters set has been negotiated via the CHARSET telopt.
	// This field indicates when the negotiated character set will be used
	// to send and receive text. According to RFC 2066, the charset is only to be used in BINARY mode
	// (RFC 856).  However, some systems will use it all the time, or only use CHARSET to advertise that the
	// server is speaking UTF-8 without actually implementing any encoding functionality. As a result, we offer
	// the option to always use the negotiated charset or only use it when BINARY mode is active.
	//
	// Text sent in telopt subnegotiations will always use UTF-8 regardless of this setting.
	CharsetUsage CharsetUsage

	// Side indicates whether this terminal is intended to be the client or server. Even though RFC 854
	// (Telnet Protocol) does not have the concept of a client or server, just local and remote, some TelOpts,
	// such as CHARSET, indicate different behaviors for clients and servers.
	Side TerminalSide

	// TelOpts indicates which TelOpts the terminal should request from the remote, and which the remote
	// should be permitted to request from us.
	TelOpts []TelnetOption

	EventHooks EventHooks
}
