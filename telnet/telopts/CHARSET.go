package telopts

import (
	"bytes"
	"encoding/ascii85"
	"errors"
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
	"golang.org/x/text/encoding/ianaindex"
	"strings"
)

const (
	charset telnet.TelOptCode = 42

	charsetREQUEST byte = iota
	charsetACCEPTED
	charsetREJECTED
)

const charsetKeyboardLock = "lock.charset"

type CHARSETOptions struct {
	PreferredCharsets []string
	AllowAnyCharset   bool
}

type CHARSET struct {
	BaseTelOpt

	options CHARSETOptions

	bestRemoteEncoding   string
	localAllowedCharsets map[string]struct{}
}

var _ telnet.TelnetOption = &CHARSET{}

func (o *CHARSET) writeRequest(charSets []string) error {
	subnegotiation := bytes.NewBuffer(nil)
	err := subnegotiation.WriteByte(charsetREQUEST)
	if err != nil {
		return err
	}

	for _, preferredCharset := range charSets {
		err = subnegotiation.WriteByte(' ')
		if err != nil {
			return err
		}

		charsetBytes := make([]byte, len(preferredCharset))
		ascii85.Encode(charsetBytes, []byte(preferredCharset))
		_, err = subnegotiation.Write(charsetBytes)
		if err != nil {
			return err
		}
	}

	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         charset,
		Subnegotiation: subnegotiation.Bytes(),
	})

	return nil
}

func (o *CHARSET) writeAccept(acceptedCharset string) {
	subnegotiation := make([]byte, len(acceptedCharset)+1)
	subnegotiation[0] = charsetACCEPTED
	ascii85.Encode(subnegotiation[1:], []byte(acceptedCharset))

	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         charset,
		Subnegotiation: subnegotiation,
	})
}

func (o *CHARSET) writeReject() {
	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         charset,
		Subnegotiation: []byte{charsetREJECTED},
	})
}

func (o *CHARSET) TransitionRemoteState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptInactive {
		o.bestRemoteEncoding = ""
	}

	return nil
}

func (o *CHARSET) TransitionLocalState(newState telnet.TelOptState) error {
	err := o.BaseTelOpt.TransitionLocalState(newState)
	if err != nil {
		return err
	}

	if newState == telnet.TelOptInactive {
		o.Terminal().Keyboard().ClearLock(charsetKeyboardLock)
	}

	if newState != telnet.TelOptActive {
		return nil
	}

	// Send REQUEST- if we don't have any preferred charsets we don't care so we won't
	// send anything
	if len(o.options.PreferredCharsets) > 0 {
		o.Terminal().Keyboard().SetLock(charsetKeyboardLock, LocalBlockTimeout)
		return o.writeRequest(o.options.PreferredCharsets)
	}

	return nil
}

func (o *CHARSET) Code() telnet.TelOptCode {
	return charset
}

func (o *CHARSET) String() string {
	return "CHARSET"
}

func (o *CHARSET) isAcceptableCharset(charSet string) bool {
	// Has to be a valid IANA encoding name
	_, err := ianaindex.IANA.Encoding(charSet)
	if err != nil {
		return false
	}

	// We have to allow all encodings or have it in our list of allowed encodings
	if !o.options.AllowAnyCharset {
		_, inAllowedEncodings := o.localAllowedCharsets[charSet]
		if !inAllowedEncodings {
			return false
		}
	}

	return true
}

func (o *CHARSET) subnegotiateREQUEST(subnegotiation []byte) error {
	if o.RemoteState() != telnet.TelOptActive {
		// Inactive sides shouldn't be sending charset requests
		o.writeReject()
		return nil
	}

	o.bestRemoteEncoding = ""
	charSets := subnegotiation[1:]

	if len(charSets) > 8 {
		possibleTTABLE := charSets[:8]
		if string(possibleTTABLE) == "[TTABLE]" {
			charSets = charSets[8:]
		}
	}

	charSetList := strings.Split(string(charSets), string(charSets[0]))
	var bestCharSet string

	for i := 1; i < len(charSetList); i++ {
		if o.isAcceptableCharset(charSetList[i]) {
			bestCharSet = charSetList[i]
			break
		}
	}

	if bestCharSet == "" {
		o.writeReject()
		return nil
	}

	o.bestRemoteEncoding = bestCharSet

	if o.Terminal().Side() == telnet.SideServer && o.Terminal().Keyboard().HasActiveLock(charsetKeyboardLock) {
		// We have worked on a negotiation originating from local in the last 5 seconds
		// and we are set up to demand priority for our negotiations, so reject the remote negotiation
		o.writeReject()
		return nil
	}

	// We have no reason not to accept the encoding
	err := o.Terminal().Charset().SetCharset(o.bestRemoteEncoding)
	if err != nil {
		return err
	}

	// Stop waiting on our local negotiation
	o.Terminal().Keyboard().ClearLock(charsetKeyboardLock)
	o.writeAccept(o.bestRemoteEncoding)
	return nil
}

func (o *CHARSET) subnegotiateREJECTED() error {
	if o.LocalState() != telnet.TelOptActive {
		// We may have deactivated while the negotiation was ongoing
		return nil
	}

	if o.bestRemoteEncoding != "" && o.Terminal().Charset().Name() != o.bestRemoteEncoding && o.Terminal().Side() == telnet.SideServer {
		// The client rejected us but they did send us some preferences that we rejected due to having
		// an active local negotiation- let's request that the client use it
		o.Terminal().Keyboard().SetLock(charsetKeyboardLock, LocalBlockTimeout)
		return o.writeRequest([]string{o.bestRemoteEncoding})
	}

	o.Terminal().Keyboard().ClearLock(charsetKeyboardLock)
	return nil
}

func (o *CHARSET) subnegotiateACCEPTED(subnegotiation []byte) error {
	if o.LocalState() != telnet.TelOptActive {
		// We may have deactivated while the negotiation was ongoing
		return nil
	}

	charSet := string(subnegotiation[1:])
	if !o.isAcceptableCharset(charSet) {
		return fmt.Errorf("charset: client sent ACCEPT for invalid charset %s", charSet)
	}

	o.bestRemoteEncoding = charSet
	o.Terminal().Keyboard().ClearLock(charsetKeyboardLock)

	return o.Terminal().Charset().SetCharset(charSet)
}

func (o *CHARSET) Subnegotiate(subnegotiation []byte) error {
	if len(subnegotiation) == 0 {
		return errors.New("charset: received empty subnegotiation")
	}

	if subnegotiation[0] == charsetREQUEST {
		return o.subnegotiateREQUEST(subnegotiation)
	}

	if subnegotiation[0] == charsetREJECTED {
		return o.subnegotiateREJECTED()
	}

	if subnegotiation[0] == charsetACCEPTED {
		return o.subnegotiateACCEPTED(subnegotiation)
	}

	return fmt.Errorf("charset: unexpected subnegotiation %+v", subnegotiation)
}
