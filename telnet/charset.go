package telnet

import (
	"errors"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"strings"
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
	BinaryEncode bool
	BinaryDecode bool

	defaultCharset currentCharset
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

func (c *Charset) Name() string {
	return c.negotiated.name
}

func (c *Charset) Encode(utf8Text string) ([]byte, error) {
	if c.usage == CharsetUsageAlways || c.BinaryEncode {
		return c.negotiated.encoder.Bytes([]byte(utf8Text))
	}
	return c.defaultCharset.encoder.Bytes([]byte(utf8Text))
}

func (c *Charset) Decode(incomingText []byte) (string, error) {
	var charset currentCharset

	if c.usage == CharsetUsageAlways || c.BinaryDecode {
		charset = c.negotiated
	} else {
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

	return currentCharset{
		encoder: charset.NewEncoder(),
		decoder: charset.NewDecoder(),
		name:    name,
	}, nil
}

func (c *Charset) PromoteDefaultCharset(oldCodePage string, newCodePage string) error {
	if c.defaultCharset.name != oldCodePage {
		return nil
	}

	charset, err := c.buildCharset(newCodePage)
	if err != nil {
		return err
	}

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

	c.negotiated = charset
	return nil
}
