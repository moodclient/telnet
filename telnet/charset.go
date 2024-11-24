package telnet

import (
	"errors"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"
	"strings"
)

type currentCharset struct {
	name    string
	encoder *encoding.Encoder
	decoder *encoding.Decoder
}

type Charset struct {
	current currentCharset
}

func NewCharset(codePage string) (*Charset, error) {
	charset := &Charset{}
	err := charset.SetCharset(codePage)
	return charset, err
}

func (c Charset) Name() string {
	return c.current.name
}

func (c Charset) Encode(utf8Text string) ([]byte, error) {
	return c.current.encoder.Bytes([]byte(utf8Text))
}

func (c Charset) Decode(incomingText []byte) (string, error) {
	b, err := c.current.decoder.Bytes(incomingText)
	if err != nil {
		return "", err
	}

	str := string(b)
	return strings.TrimSuffix(str, "\ufffd"), nil
}

func (c *Charset) SetCharset(codePage string) error {
	if strings.ToLower(codePage) == "utf-8" {
		c.current = currentCharset{
			encoder: encoding.Replacement.NewEncoder(),
			decoder: encoding.Replacement.NewDecoder(),
			name:    "UTF-8",
		}
		return nil
	}

	charset, err := ianaindex.IANA.Encoding(codePage)
	if err != nil {
		return err
	}
	if charset == nil {
		return errors.New("ianaindex: unsupported encoding")
	}
	name, err := ianaindex.IANA.Name(charset)
	if err != nil {
		return err
	}

	c.current = currentCharset{
		encoder: charset.NewEncoder(),
		decoder: charset.NewDecoder(),
		name:    name,
	}
	return nil
}
