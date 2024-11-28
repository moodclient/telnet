package telnet

import (
	"errors"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"strings"
	"sync"
	"sync/atomic"
)

type Coder interface {
	Bytes(b []byte) ([]byte, error)
}

type currentCharset struct {
	name string

	encoder Coder
	decoder Coder
}

type Charset struct {
	usage        CharsetUsage
	binaryEncode atomic.Bool
	binaryDecode atomic.Bool

	defaultLock    sync.Mutex
	defaultCharset currentCharset

	negotiatedLock sync.Mutex
	negotiated     currentCharset
}

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

func (c *Charset) SetBinaryEncode(encode bool) {
	c.binaryEncode.Store(encode)
}

func (c *Charset) SetBinaryDecode(decode bool) {
	c.binaryDecode.Store(decode)
}

func (c *Charset) BinaryEncode() bool {
	return c.binaryEncode.Load()
}

func (c *Charset) BinaryDecode() bool {
	return c.binaryDecode.Load()
}

func (c *Charset) NegotiatedCharsetName() string {
	c.negotiatedLock.Lock()
	defer c.negotiatedLock.Unlock()

	return c.negotiated.name
}

func (c *Charset) DefaultCharsetName() string {
	c.defaultLock.Lock()
	defer c.defaultLock.Unlock()

	return c.defaultCharset.name
}

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

func (c *Charset) Decode(incomingText []byte) (string, error) {
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

	b, err := charset.decoder.Bytes(incomingText)
	if err != nil {
		return "", err
	}

	str := string(b)
	return strings.TrimSuffix(str, "\ufffd"), nil
}

func (c *Charset) buildCharset(codePage string) (currentCharset, error) {
	if strings.ToLower(codePage) == "utf-8" {
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
	var decoder Coder

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

func (c *Charset) PromoteDefaultCharset(oldCodePage string, newCodePage string) error {
	c.defaultLock.Lock()
	defer c.defaultLock.Unlock()

	if c.defaultCharset.name != oldCodePage {
		return nil
	}

	charset, err := c.buildCharset(newCodePage)
	if err != nil {
		return err
	}

	c.negotiatedLock.Lock()
	defer c.negotiatedLock.Unlock()

	if c.negotiated.name == oldCodePage {
		c.negotiated = charset
	}

	c.defaultCharset = charset
	return nil
}

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
