package telnet

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"
)

type currentCharset struct {
	name string

	encoder *encoding.Encoder
	decoder transform.Transformer
}

// Charset represents the full encoding landscape for this terminal.  Terminals have
// both a default charset and negotiated charset.  On terminal creation, the negotiated
// charset is the same as the default charset.  Through the CHARSET telopt, a new
// negotiated charset can be established with the remote.  However, according to the
// RFC, the negotiated charset should only be used when TRANSMIT-BINARY is active. When
// it is not active, the default charset should continue to be used. Not
// all implementors follow that requirement, though, so CharsetUsage is used to establish
// when the negotiated charset should be used.
//
// Additionally, the RFC required the default charset to be US-ASCII prior to 2008 and
// requires it to be UTF-8 since 2008. However, not all peers have been updated to support
// UTF-8, so it is useful for us to use the default charset to establish whether
// our peer actually supports UTF-8 and make that information available to the consumer.
// The consumer can use that information to decide whether to send UTF-8 text to the peer
// or limit itself to US-ASCII.
//
// Finally, some non-english services written prior to 2008 broke RFC and do not use
// US-ASCII as their default charset.  So in some cases, we will establish a default
// character set other than US-ASCII to support these services.
type Charset struct {
	usage        CharsetUsage
	binaryEncode atomic.Bool
	binaryDecode atomic.Bool

	defaultLock    sync.Mutex
	defaultCharset currentCharset

	negotiatedLock sync.Mutex
	negotiated     currentCharset
}

// NewCharset creates a new charset with a default charset & a CharsetUsage to decide
// how the negotiated charset will be used if one is negotiated.
func NewCharset(defaultCodePage string, usage CharsetUsage) (*Charset, error) {
	charset := &Charset{
		usage: usage,
	}

	defaultCharset, err := charset.buildCharset(defaultCodePage)
	if err != nil {
		return nil, err
	}

	charset.defaultCharset = defaultCharset
	charset.negotiated = defaultCharset

	return charset, nil
}

// SetBinaryEncode is used by the TRANSMIT-BINARY telopt to establish whether
// the keyboard should use binary mode
func (c *Charset) SetBinaryEncode(encode bool) {
	c.binaryEncode.Store(encode)
}

// SetBinaryDecode is used by the TRANSMIT-BINARY telopt to establish whether the
// printer should use binary mode
func (c *Charset) SetBinaryDecode(decode bool) {
	c.binaryDecode.Store(decode)
}

// BinaryEncode returns a bool indicating whether the keyboard should use binary mode
func (c *Charset) BinaryEncode() bool {
	return c.binaryEncode.Load()
}

// BinaryDecode returns a bool indicating whether the printer should use binary mode
func (c *Charset) BinaryDecode() bool {
	return c.binaryDecode.Load()
}

// NegotiatedCharsetName returns the name of the character set negotiated by the
// CHARSET telopt. If the CHARSET telopt has not negotiated a charset, this will
// return the name of the default charset
func (c *Charset) NegotiatedCharsetName() string {
	c.negotiatedLock.Lock()
	defer c.negotiatedLock.Unlock()

	return c.negotiated.name
}

// DefaultCharsetName returns the name of the default character set
func (c *Charset) DefaultCharsetName() string {
	c.defaultLock.Lock()
	defer c.defaultLock.Unlock()

	return c.defaultCharset.name
}

// EncodingName returns the name of the character set currently used by the keyboard.
// This method takes into account the default & negotiated character sets, the CharsetUsage
// value, and whether the keyboard is in binary mode
func (c *Charset) EncodingName() string {
	if c.usage == CharsetUsageAlways || c.binaryEncode.Load() {
		c.negotiatedLock.Lock()
		defer c.negotiatedLock.Unlock()

		return c.negotiated.name
	}

	c.defaultLock.Lock()
	defer c.defaultLock.Unlock()

	return c.defaultCharset.name
}

// DecodingName returns the name of the character set currently used by the printer.
// This method takes into account the default & negotiated character sets, the
// CharsetUsage value, and whether the printer is in binary mode
func (c *Charset) DecodingName() string {
	if c.usage == CharsetUsageAlways || c.binaryDecode.Load() {
		c.negotiatedLock.Lock()
		defer c.negotiatedLock.Unlock()

		return c.negotiated.name
	}

	c.defaultLock.Lock()
	defer c.defaultLock.Unlock()

	return c.defaultCharset.name
}

// Encode accepts a string of UTF-8 text and returns a byte slice that is encoded
// in the keyboard's current encoding
func (c *Charset) Encode(utf8Text string) ([]byte, error) {
	if c.usage == CharsetUsageAlways || c.binaryEncode.Load() {
		c.negotiatedLock.Lock()
		defer c.negotiatedLock.Unlock()

		return c.negotiated.encoder.Bytes([]byte(utf8Text))
	}

	c.defaultLock.Lock()
	defer c.defaultLock.Unlock()

	return c.defaultCharset.encoder.Bytes([]byte(utf8Text))
}

// Decode accepts a byte slice that is encoded in the printer's current encoding
// and returns a string of UTF-8 text
func (c *Charset) Decode(buffer []byte, incomingText []byte) (consumed int, buffered int, err error) {
	if len(incomingText) == 0 {
		return 0, 0, nil
	}

	var charset currentCharset

	if c.usage == CharsetUsageAlways || c.binaryDecode.Load() {
		c.negotiatedLock.Lock()
		defer c.negotiatedLock.Unlock()

		charset = c.negotiated
	} else {
		c.defaultLock.Lock()
		defer c.defaultLock.Unlock()

		charset = c.defaultCharset
	}

	for i := 0; i < len(incomingText); i++ {
		var dstBytes, srcBytes int
		dstBytes, srcBytes, err = charset.decoder.Transform(buffer, incomingText[:i+1], false)
		if err != nil && !errors.Is(err, transform.ErrShortDst) && !errors.Is(err, transform.ErrShortSrc) {
			return srcBytes, dstBytes, err
		}

		if dstBytes > 0 {
			return srcBytes, dstBytes, nil
		}
	}

	return 0, 0, err
}

func (c *Charset) buildCharset(codePage string) (currentCharset, error) {
	if strings.ToLower(codePage) == "utf-8" {
		// A utf-8 character set will replace bad runes with the replacement character
		// but otherwise not touch the text
		return currentCharset{
			encoder: encoding.Replacement.NewEncoder(),
			// We use an encoder instead of decoder because the Replacement encoding works weird-
			// see the difference between the decoder & encoder behaviors
			decoder: encoding.Replacement.NewEncoder(),
			name:    "UTF-8",
		}, nil
	}

	charset, err := ianaindex.IANA.Encoding(codePage)
	if err != nil {
		return currentCharset{}, err
	}
	if charset == nil {
		return currentCharset{}, errors.New("ianaindex: unsupported encoding")
	}
	name, err := ianaindex.IANA.Name(charset)
	if err != nil {
		return currentCharset{}, err
	}

	encoder := charset.NewEncoder()
	var decoder transform.Transformer

	if strings.ToLower(codePage) == "us-ascii" {
		// Allow the remote to send us UTF-8 even if we think we're ascii. We'll be good citizens
		// and only send ASCII.
		decoder = encoding.Replacement.NewEncoder()
	} else {
		decoder = charset.NewDecoder()
	}

	return currentCharset{
		encoder: encoder,
		decoder: decoder,
		name:    name,
	}, nil
}

// PromoteDefaultCharset will change the default character set to the new code page
// if it is currently set to the old code page.  If the default character set is changed,
// the negotiated character set will also be changed if it's the same as the default
// character set.
//
// This is primarily used when we get some indication that the remote supports UTF-8
// to promote the default charset from US-ASCII to UTF-8. The US-ASCII decoder will
// always decode UTF-8, but it's useful for the consumer to know whether the remote
// actually supports UTF-8, in order to decide whether to send things like emojis.
func (c *Charset) PromoteDefaultCharset(oldCodePage string, newCodePage string) (bool, error) {
	c.defaultLock.Lock()
	defer c.defaultLock.Unlock()

	if c.defaultCharset.name != oldCodePage {
		return false, nil
	}

	charset, err := c.buildCharset(newCodePage)
	if err != nil {
		return false, err
	}

	c.negotiatedLock.Lock()
	defer c.negotiatedLock.Unlock()

	if c.negotiated.name == oldCodePage {
		c.negotiated = charset
	}

	c.defaultCharset = charset
	return true, nil
}

// SetNegotiatedCharset modifies the negotiated charset to the requested character set
func (c *Charset) SetNegotiatedCharset(codePage string) error {
	charset, err := c.buildCharset(codePage)
	if err != nil {
		return err
	}

	c.negotiatedLock.Lock()
	defer c.negotiatedLock.Unlock()

	c.negotiated = charset
	return nil
}
