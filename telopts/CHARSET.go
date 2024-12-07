package telopts

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/moodclient/telnet"
	"golang.org/x/text/encoding/ianaindex"
)

const (
	charset telnet.TelOptCode = 42

	charsetREQUEST byte = iota
	charsetACCEPTED
	charsetREJECTED
	charsetTTABLEIS
	charsetTTABLEREJECTED
	charsetTTABLEACK
	charsetTTABLENAK
)

const charsetKeyboardLock = "lock.charset"

type CHARSETNegotiationSuccessEvent struct {
	BaseTelOptEvent
	NewCharsetName string
}

func (e CHARSETNegotiationSuccessEvent) String() string {
	return fmt.Sprintf("CHARSET Negotiated To: %s", e.NewCharsetName)
}

type CHARSETDefaultChangedEvent struct {
	BaseTelOptEvent
	NewDefaultCharset string
}

func (e CHARSETDefaultChangedEvent) String() string {
	return fmt.Sprintf("CHARSET Default Changed To: %s", e.NewDefaultCharset)
}

type CHARSETConfig struct {
	PreferredCharsets []string
	AllowAnyCharset   bool
}

func RegisterCHARSET(usage telnet.TelOptUsage, options CHARSETConfig) telnet.TelnetOption {
	charsets := make(map[string]struct{})
	for _, c := range options.PreferredCharsets {
		charsets[c] = struct{}{}
	}

	return &CHARSET{
		BaseTelOpt:           NewBaseTelOpt(charset, "CHARSET", usage),
		options:              options,
		localAllowedCharsets: charsets,
	}
}

type CHARSET struct {
	BaseTelOpt

	options CHARSETConfig

	bestRemoteEncoding   string
	localAllowedCharsets map[string]struct{}
}

func (o *CHARSET) writeRequest(charSets []string) error {
	// Estimate buffer size to reduce allocations
	var bufferSize int
	for _, charSet := range charSets {
		bufferSize += len(charSet) + 1
	}

	subnegotiation := bytes.NewBuffer(make([]byte, 0, bufferSize+5))
	err := subnegotiation.WriteByte(charsetREQUEST)
	if err != nil {
		return err
	}

	for _, preferredCharset := range charSets {
		err = subnegotiation.WriteByte(' ')
		if err != nil {
			return err
		}

		_, err = subnegotiation.Write([]byte(preferredCharset))
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
	subnegotiation := make([]byte, 0, len(acceptedCharset)+1)
	subnegotiation = append(subnegotiation, charsetACCEPTED)
	subnegotiation = append(subnegotiation, []byte(acceptedCharset)...)

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
		o.Terminal().Keyboard().SetLock(charsetKeyboardLock, telnet.DefaultKeyboardLock)
		return o.writeRequest(o.options.PreferredCharsets)
	}

	return nil
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
	// Some MUDs don't follow this rule!
	//if o.RemoteState() != telnet.TelOptActive {
	//	// Inactive sides shouldn't be sending charset requests
	//	o.writeReject()
	//	return nil
	//}

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
		if charSetList[i] == "UTF-8" {
			// We know the remote can handle UTF-8 so use it as our default charset no matter what happens
			// this will allow the consumer to ask the terminal whether the remote can handle UTF-8
			changed, _ := o.Terminal().Charset().PromoteDefaultCharset("US-ASCII", "UTF-8")
			if changed {
				o.Terminal().RaiseTelOptEvent(CHARSETDefaultChangedEvent{
					BaseTelOptEvent:   BaseTelOptEvent{o},
					NewDefaultCharset: "UTF-8",
				})
			}
		}

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
	err := o.Terminal().Charset().SetNegotiatedCharset(o.bestRemoteEncoding)
	if err != nil {
		o.writeReject()
		return err
	}
	o.Terminal().RaiseTelOptEvent(CHARSETNegotiationSuccessEvent{
		BaseTelOptEvent: BaseTelOptEvent{o},
		NewCharsetName:  o.bestRemoteEncoding,
	})

	o.writeAccept(o.bestRemoteEncoding)
	return nil
}

func (o *CHARSET) subnegotiateREJECTED() error {
	if o.LocalState() != telnet.TelOptActive {
		// We may have deactivated while the negotiation was ongoing
		return nil
	}

	if o.bestRemoteEncoding != "" && o.Terminal().Charset().NegotiatedCharsetName() != o.bestRemoteEncoding && o.Terminal().Side() == telnet.SideServer {
		// The client rejected us but they did send us some preferences that we rejected due to having
		// an active local negotiation- let's request that the client use it
		o.Terminal().Keyboard().SetLock(charsetKeyboardLock, telnet.DefaultKeyboardLock)
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

	// Clear the lock after setting charset to ensure that all outbound text uses the new charset
	// if we're going to change (we should clear the lock even if it fails since the negotiation
	// will not continue)
	err := o.Terminal().Charset().SetNegotiatedCharset(charSet)
	if err != nil {
		return err
	}

	o.Terminal().RaiseTelOptEvent(CHARSETNegotiationSuccessEvent{
		BaseTelOptEvent: BaseTelOptEvent{o},
		NewCharsetName:  charSet,
	})

	return nil
}

func (o *CHARSET) Subnegotiate(subnegotiation []byte) error {
	if len(subnegotiation) == 0 {
		return errors.New("charset: received empty subnegotiation")
	}

	if subnegotiation[0] == charsetREQUEST {
		err := o.subnegotiateREQUEST(subnegotiation)
		o.Terminal().Keyboard().ClearLock(charsetKeyboardLock)
		return err
	}

	if subnegotiation[0] == charsetREJECTED {
		// Depending on how we were rejected, we may keep the keyboard lock
		return o.subnegotiateREJECTED()
	}

	if subnegotiation[0] == charsetACCEPTED {
		err := o.subnegotiateACCEPTED(subnegotiation)
		o.Terminal().Keyboard().ClearLock(charsetKeyboardLock)
		return err
	}

	return o.BaseTelOpt.Subnegotiate(subnegotiation)
}

func (o *CHARSET) SubnegotiationString(subnegotiation []byte) (string, error) {
	if len(subnegotiation) == 0 {
		return "", fmt.Errorf("charset: empty subnegotiation")
	}

	if subnegotiation[0] == charsetREQUEST {
		var sb strings.Builder
		sb.WriteString("REQUEST ")
		sb.WriteString(string(subnegotiation[1:]))
		return sb.String(), nil
	}

	if subnegotiation[0] == charsetREJECTED {
		return "REJECTED", nil
	}

	if subnegotiation[0] == charsetACCEPTED {
		var sb strings.Builder
		sb.WriteString("ACCEPTED ")
		sb.WriteString(string(subnegotiation[1:]))
		return sb.String(), nil
	}

	if subnegotiation[0] == charsetTTABLEIS {
		return "TTABLE-IS", nil
	}

	if subnegotiation[0] == charsetTTABLEREJECTED {
		return "TTABLE-REJECTED", nil
	}

	if subnegotiation[0] == charsetTTABLEACK {
		return "TTABLE-ACK", nil
	}

	if subnegotiation[0] == charsetTTABLENAK {
		return "TTABLE-NAK", nil
	}

	return o.BaseTelOpt.SubnegotiationString(subnegotiation)
}
