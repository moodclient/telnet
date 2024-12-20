package telnet

import (
	"errors"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/transform"

	"github.com/moodclient/telnet/charset"
)

type currentCharset struct {
	name string

	encoder *encoding.Encoder
	decoder transform.Transformer
}

// Charset represents the full encoding landscape for this terminal.  Terminals have
// both a default charset and separate negotiated charsets for encoding and decoding.
// On terminal creation, the negotiated charsets are the same as the default charset.
// Through the CHARSET telopt, a new negotiated charset can be established with the remote.
// However, according to the RFC, the negotiated charset should only be used when
//
//	TRANSMIT-BINARY is active. When it is not active, the default charset should continue
//
// to be used. Not all implementors follow that requirement, though, so CharsetUsage is used
// to establish when the negotiated charset should be used.
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
//
// Lastly, a fallback character set can be established that will be used during decoding
// if the correct charset for decoding fails.
type Charset struct {
	usage        CharsetUsage
	binaryEncode atomic.Bool
	binaryDecode atomic.Bool

	defaultCharset     atomic.Pointer[currentCharset]
	negotiatedEncoding atomic.Pointer[currentCharset]
	negotiatedDecoding atomic.Pointer[currentCharset]
	fallback           atomic.Pointer[currentCharset]
}

// NewCharset creates a new charset with a default charset, an optional fallback charset,
//
//	& a CharsetUsage to decide how the negotiated charset will be used if one is negotiated.
func NewCharset(defaultCodePage string, fallbackCodePage string, usage CharsetUsage) (*Charset, error) {
	charset := &Charset{
		usage: usage,
	}

	defaultCharset, err := charset.buildCharset(defaultCodePage)
	if err != nil {
		return nil, err
	}

	charset.defaultCharset.Store(defaultCharset)
	charset.negotiatedDecoding.Store(defaultCharset)
	charset.negotiatedEncoding.Store(defaultCharset)

	if fallbackCodePage != "" {
		fallback, err := charset.buildCharset(fallbackCodePage)
		if err != nil {
			return nil, err
		}

		charset.fallback.Store(fallback)
	}

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

func (c *Charset) loadEncodingCharset() *currentCharset {
	var charset *currentCharset
	if c.usage == CharsetUsageAlways || c.binaryEncode.Load() {
		charset = c.negotiatedEncoding.Load()
	}

	if charset == nil {
		charset = c.defaultCharset.Load()
	}

	return charset
}

func (c *Charset) loadDecodingCharset() *currentCharset {
	var charset *currentCharset
	if c.usage == CharsetUsageAlways || c.binaryDecode.Load() {
		charset = c.negotiatedDecoding.Load()
	}

	if charset == nil {
		charset = c.defaultCharset.Load()
	}

	return charset
}

// DefaultCharsetName returns the name of the default character set
func (c *Charset) DefaultCharsetName() string {
	return c.defaultCharset.Load().name
}

// EncodingName returns the name of the character set currently used by the keyboard.
// This method takes into account the default & negotiated character sets, the CharsetUsage
// value, and whether the keyboard is in binary mode
func (c *Charset) EncodingName() string {
	return c.loadEncodingCharset().name
}

// DecodingName returns the name of the character set currently used by the printer.
// This method takes into account the default & negotiated character sets, the
// CharsetUsage value, and whether the printer is in binary mode
func (c *Charset) DecodingName() string {
	return c.loadDecodingCharset().name
}

// Encode accepts a string of UTF-8 text and returns a byte slice that is encoded
// in the keyboard's current encoding
func (c *Charset) Encode(utf8Text string) ([]byte, error) {
	return c.loadEncodingCharset().encoder.Bytes([]byte(utf8Text))
}

func validEncoding(charset *currentCharset, incomingText []byte) EncodingState {
	var buffer [1000]byte
	buffered, _, err := charset.decoder.Transform(buffer[:], incomingText, false)

	bufferEmpty := errors.Is(err, transform.ErrShortDst) || errors.Is(err, transform.ErrShortSrc)
	if !bufferEmpty && (buffered == 0 || err != nil) {
		return EncodingInvalid
	}

	size := 1
	valid := false
	for i := 0; i < buffered; i += size {
		var decoded rune
		decoded, size = utf8.DecodeRune(buffer[i:])
		if decoded == utf8.RuneError {
			return EncodingInvalid
		} else if size > 1 {
			valid = true
		}
	}

	if valid {
		return EncodingValid
	}

	return EncodingUnsure
}

type EncodingState int

const (
	EncodingUnsure EncodingState = iota
	EncodingInvalid
	EncodingValid
)

// Decode accepts a byte slice that is encoded in the printer's current encoding as well as a
// destination buffer for decoded bytes.  Additionally, it accepts a bool indicating whether
// the decode process should skip the default/negotiated charset and immediately use the fallback
// charset.
//
// The method returns how many bytes were consumed from the incoming text, how many bytes were
// written to the buffer, whether the charset had to move to fallback mode due to decoding failure,
// and potentially an error.
func (c *Charset) Decode(buffer []byte, incomingText []byte, fallback EncodingState) (consumed int, buffered int, fellback EncodingState, err error) {
	if len(incomingText) == 0 {
		return 0, 0, fallback, nil
	}

	charset := c.loadDecodingCharset()
	fallbackCharset := c.fallback.Load()
	charsetToUse := charset

	if fallbackCharset != nil && fallback == EncodingUnsure {
		fallback = validEncoding(charset, incomingText)

		if fallback == EncodingInvalid {
			fallbackEncodingState := validEncoding(fallbackCharset, incomingText)
			if fallbackEncodingState == EncodingInvalid {
				fallback = EncodingValid
			}
		}
	}

	if fallbackCharset != nil && fallback == EncodingInvalid {
		charsetToUse = fallbackCharset
	}

	buffered, consumed, err = charsetToUse.decoder.Transform(buffer, incomingText, false)

	return consumed, buffered, fallback, err
}

func (c *Charset) buildCharset(codePage string) (*currentCharset, error) {
	if strings.ToLower(codePage) == "utf-8" {
		// A utf-8 character set will replace bad runes with the replacement character
		// but otherwise not touch the text
		return &currentCharset{
			encoder: encoding.Replacement.NewEncoder(),
			// We use an encoder instead of decoder because the Replacement encoding works weird-
			// see the difference between the decoder & encoder behaviors
			decoder: encoding.Replacement.NewEncoder(),
			name:    "UTF-8",
		}, nil
	}

	if strings.ToLower(codePage) == "cp437-full" {
		encoding := charset.CP437Full{}
		return &currentCharset{
			encoder: encoding.NewEncoder(),
			decoder: encoding.NewDecoder(),
			name:    "CP437-FULL",
		}, nil
	}

	charset, err := ianaindex.IANA.Encoding(codePage)
	if err != nil {
		return nil, err
	}
	if charset == nil {
		return nil, errors.New("ianaindex: unsupported encoding")
	}
	name, err := ianaindex.IANA.Name(charset)
	if err != nil {
		return nil, err
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

	return &currentCharset{
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
	defaultCharset := c.defaultCharset.Load()

	if defaultCharset.name != oldCodePage {
		return false, nil
	}

	charset, err := c.buildCharset(newCodePage)
	if err != nil {
		return false, err
	}

	return c.defaultCharset.CompareAndSwap(defaultCharset, charset), nil
}

// SetNegotiatedEncodingCharset modifies the negotiated keyboard charset to the requested
// character set
func (c *Charset) SetNegotiatedEncodingCharset(codePage string) error {
	charset, err := c.buildCharset(codePage)
	if err != nil {
		return err
	}

	c.negotiatedEncoding.Store(charset)
	return nil
}

// SetNegotiatedDecodingCHarset modifies the negotiated printer charset to the requested character
// set.
func (c *Charset) SetNegotiatedDecodingCharset(codePage string) error {
	charset, err := c.buildCharset(codePage)
	if err != nil {
		return err
	}

	c.negotiatedDecoding.Store(charset)
	return nil
}
