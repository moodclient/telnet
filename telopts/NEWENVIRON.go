package telopts

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/moodclient/telnet"
)

const newenviron telnet.TelOptCode = 39

var NEWENVIRONWellKnownVars = []string{"USER", "JOB", "ACCT", "PRINTER", "SYSTEMTYPE", "DISPLAY"}

const (
	newenvironIS byte = iota
	newenvironSEND
	newenvironINFO
)

const (
	newenvironVAR byte = iota
	newenvironVALUE
	newenvironESC
	newenvironUSERVAR
)

type NEWENVIRONRemoteVarsChangedEvent struct {
	BaseTelOptEvent
	UpdatedWellKnownVars []string
	UpdatedUserVars      []string
}

func (e NEWENVIRONRemoteVarsChangedEvent) String() string {
	return fmt.Sprintf("NEW-ENVIRON Remote Vars Changed- Well-Known: %+v, User: %+v", e.UpdatedWellKnownVars, e.UpdatedUserVars)
}

type NEWENVIRONConfig struct {
	WellKnownVarKeys []string

	InitialVars map[string]string
}

func RegisterNEWENVIRON(usage telnet.TelOptUsage, config NEWENVIRONConfig) telnet.TelnetOption {
	option := &NEWENVIRON{
		BaseTelOpt: NewBaseTelOpt(newenviron, "NEW-ENVIRON", usage),

		wellKnownVars: make(map[string]struct{}),

		localUserVars:       make(map[string]string),
		localWellKnownVars:  make(map[string]string),
		remoteUserVars:      make(map[string]string),
		remoteWellKnownVars: make(map[string]string),
	}

	for _, varKey := range config.WellKnownVarKeys {
		option.wellKnownVars[varKey] = struct{}{}
	}

	if config.InitialVars != nil {
		for key, value := range config.InitialVars {
			_, isWellKnown := option.wellKnownVars[key]
			if isWellKnown {
				option.localWellKnownVars[key] = value
			} else {
				option.localUserVars[key] = value
			}
		}
	}

	return option
}

type NEWENVIRON struct {
	BaseTelOpt

	localVarsLock  sync.Mutex
	remoteVarsLock sync.Mutex

	wellKnownVars map[string]struct{}

	localUserVars       map[string]string
	localWellKnownVars  map[string]string
	remoteUserVars      map[string]string
	remoteWellKnownVars map[string]string
}

func (o *NEWENVIRON) TransitionRemoteState(newState telnet.TelOptState) (func() error, error) {
	postSend, err := o.BaseTelOpt.TransitionRemoteState(newState)
	if err != nil {
		return postSend, err
	}

	if newState == telnet.TelOptInactive {
		o.remoteVarsLock.Lock()
		defer o.remoteVarsLock.Unlock()

		for key := range o.remoteUserVars {
			delete(o.remoteUserVars, key)
		}

		for key := range o.remoteWellKnownVars {
			delete(o.remoteWellKnownVars, key)
		}
	} else if newState == telnet.TelOptActive {
		o.localVarsLock.Lock()
		defer o.localVarsLock.Unlock()

		o.writeSendAll()
	}

	return postSend, nil
}

func (o *NEWENVIRON) encodeText(buffer *bytes.Buffer, text string) {
	textBytes := []byte(text)

	for _, b := range textBytes {
		if b <= newenvironUSERVAR {
			// VAR, VALUE, ESC, or USERVAR need to be escaped with an ESC
			buffer.WriteByte(newenvironESC)
		}

		buffer.WriteByte(b)
	}
}

func (o *NEWENVIRON) decodeText(buffer []byte) (int, string) {
	var sb strings.Builder

	var bufferIndex int
	for bufferIndex = 0; bufferIndex < len(buffer); bufferIndex++ {
		b := buffer[bufferIndex]
		if b == newenvironESC {
			bufferIndex++
			if bufferIndex >= len(buffer) {
				break
			}
		} else if b <= newenvironUSERVAR {
			break
		}

		sb.WriteByte(buffer[bufferIndex])
	}

	return bufferIndex, sb.String()
}

func (o *NEWENVIRON) writeSendAll() {
	// Try to avoid repeated allocs by estimating the buffer size
	var estimatedBufferSize int
	for _, wellKnownVar := range o.localWellKnownVars {
		estimatedBufferSize += len(wellKnownVar)
	}

	buffer := bytes.NewBuffer(make([]byte, 0, estimatedBufferSize*2))

	buffer.WriteByte(newenvironSEND)

	// Spell out the well-known vars we want for the benefit of the remote- we want at least an
	// "I don't have that value" from them
	for wellKnownVar := range o.wellKnownVars {
		buffer.WriteByte(newenvironVAR)
		o.encodeText(buffer, wellKnownVar)
	}
	// Also send us anything else you might have
	buffer.WriteByte(newenvironVAR)
	buffer.WriteByte(newenvironUSERVAR)

	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         newenviron,
		Subnegotiation: buffer.Bytes(),
	}, nil)
}

func (o *NEWENVIRON) writeVarValues(buffer *bytes.Buffer, varKeys map[string]struct{}, userVarKeys map[string]struct{}) {
	for key := range varKeys {
		buffer.WriteByte(newenvironVAR)
		o.encodeText(buffer, key)

		value, hasValue := o.localWellKnownVars[key]
		if hasValue {
			buffer.WriteByte(newenvironVALUE)
			o.encodeText(buffer, value)
		}
	}

	for key := range userVarKeys {
		buffer.WriteByte(newenvironUSERVAR)
		o.encodeText(buffer, key)

		value, hasValue := o.localUserVars[key]
		if hasValue {
			buffer.WriteByte(newenvironVALUE)
			o.encodeText(buffer, value)
		}
	}
}

func (o *NEWENVIRON) subnegotiateSEND(subnegotiation []byte) {
	varKeys := make(map[string]struct{})
	userVarKeys := make(map[string]struct{})

	var includeAllVars, includeAllUservars bool

	if len(subnegotiation) > 0 {
		var index int
		for index < len(subnegotiation) {
			nextToken := subnegotiation[index]
			index++

			if nextToken == newenvironUSERVAR || nextToken == newenvironVAR {
				keySize, key := o.decodeText(subnegotiation[index:])
				index += keySize

				if keySize == 0 && nextToken == newenvironUSERVAR {
					includeAllUservars = true
				} else if keySize == 0 {
					includeAllVars = true
				} else if nextToken == newenvironUSERVAR {
					userVarKeys[key] = struct{}{}
				} else {
					varKeys[key] = struct{}{}
				}
			}
		}
	} else {
		includeAllVars = true
		includeAllUservars = true
	}

	if includeAllVars {
		for key := range o.wellKnownVars {
			varKeys[key] = struct{}{}
		}
		for key := range o.localWellKnownVars {
			varKeys[key] = struct{}{}
		}
	}

	if includeAllUservars {
		for key := range o.localUserVars {
			userVarKeys[key] = struct{}{}
		}
	}

	// Grab length of keys and values to estimate buffer size to reduce allocations
	estimatedBufferSize := 0
	for key, value := range o.localWellKnownVars {
		estimatedBufferSize += len(key)
		estimatedBufferSize += len(value)
	}

	for key, value := range o.localUserVars {
		estimatedBufferSize += len(key)
		estimatedBufferSize += len(value)
	}

	buffer := bytes.NewBuffer(make([]byte, 0, estimatedBufferSize*2))
	buffer.WriteByte(newenvironIS)
	o.writeVarValues(buffer, varKeys, userVarKeys)

	o.Terminal().Keyboard().WriteCommand(telnet.Command{
		OpCode:         telnet.SB,
		Option:         newenviron,
		Subnegotiation: buffer.Bytes(),
	}, nil)
}

func (o *NEWENVIRON) subnegotiationLoadValues(subnegotiation []byte) ([]string, []string, error) {
	o.remoteVarsLock.Lock()
	defer o.remoteVarsLock.Unlock()

	var modifiedWellKnownKeys, modifiedUserKeys []string
	var index int
	for index < len(subnegotiation) {
		nextToken := subnegotiation[index]
		index++

		if nextToken == newenvironUSERVAR || nextToken == newenvironVAR {
			keySize, key := o.decodeText(subnegotiation[index:])
			if keySize == 0 {
				return nil, nil, errors.New("new-environ: received 0-sized key with IS/INFO subnegotiation")
			}

			if nextToken == newenvironUSERVAR {
				modifiedUserKeys = append(modifiedUserKeys, key)
			} else {
				modifiedWellKnownKeys = append(modifiedWellKnownKeys, key)
			}

			index += keySize

			if index < len(subnegotiation) && subnegotiation[index] == newenvironVALUE {
				index++

				valueSize, value := o.decodeText(subnegotiation[index:])
				index += valueSize

				if nextToken == newenvironUSERVAR {
					o.remoteUserVars[key] = value
				} else {
					o.remoteWellKnownVars[key] = value
				}
			} else if nextToken == newenvironUSERVAR {
				delete(o.remoteUserVars, key)
			} else {
				delete(o.remoteWellKnownVars, key)
			}
		}
	}

	return modifiedWellKnownKeys, modifiedUserKeys, nil
}

func (o *NEWENVIRON) Subnegotiate(subnegotiation []byte) error {
	if len(subnegotiation) == 0 {
		return fmt.Errorf("new-environ received empty subnegotiation")
	}

	if subnegotiation[0] == newenvironSEND && o.LocalState() == telnet.TelOptActive {
		o.localVarsLock.Lock()
		defer o.localVarsLock.Unlock()

		o.subnegotiateSEND(subnegotiation[1:])
		return nil
	}

	if o.RemoteState() == telnet.TelOptActive && (subnegotiation[0] == newenvironIS || subnegotiation[0] == newenvironINFO) {
		// This method locks remote locks
		modifiedWellKnownKeys, modifiedUserKeys, err := o.subnegotiationLoadValues(subnegotiation[1:])
		if err != nil {
			return err
		}

		o.Terminal().RaiseTelOptEvent(NEWENVIRONRemoteVarsChangedEvent{
			BaseTelOptEvent:      BaseTelOptEvent{o},
			UpdatedWellKnownVars: modifiedWellKnownKeys,
			UpdatedUserVars:      modifiedUserKeys,
		})
	}

	return o.BaseTelOpt.Subnegotiate(subnegotiation)
}

func (o *NEWENVIRON) subnegotiationSENDString(sb *strings.Builder, subnegotiation []byte) error {
	var index int
	for index < len(subnegotiation) {
		nextToken := subnegotiation[index]
		index++

		if nextToken == newenvironVAR {
			sb.WriteString("VAR ")
		} else if nextToken == newenvironUSERVAR {
			sb.WriteString("USERVAR ")
		} else {
			return fmt.Errorf("new-environ: unexpected token %d", nextToken)
		}

		keyLen, key := o.decodeText(subnegotiation[index:])
		if keyLen == 0 {
			sb.WriteString("(ALL) ")
		} else {
			sb.WriteString(key)
			sb.WriteString(" ")
		}
		index += keyLen
	}

	return nil
}

func (o *NEWENVIRON) subnegotiationValueString(sb *strings.Builder, subnegotiation []byte) error {
	var index int
	for index < len(subnegotiation) {
		nextToken := subnegotiation[index]
		index++

		if nextToken == newenvironVAR {
			sb.WriteString("VAR ")
		} else if nextToken == newenvironUSERVAR {
			sb.WriteString("USERVAR ")
		} else {
			return fmt.Errorf("new-environ: unexpected token %d", nextToken)
		}

		keyLen, key := o.decodeText(subnegotiation[index:])
		if keyLen == 0 {
			return fmt.Errorf("new-environ: 0-length key in IS/INFO subnegotiation")
		}
		sb.WriteString(key)
		sb.WriteString(" ")
		index += keyLen

		if index < len(subnegotiation) && subnegotiation[index] == newenvironVALUE {
			sb.WriteString("VALUE ")
			index++

			valueLen, value := o.decodeText(subnegotiation[index:])
			index += valueLen
			sb.WriteString(value)
			sb.WriteString(" ")
		} else {
			sb.WriteString("(DELETE) ")
		}
	}

	return nil
}

func (o *NEWENVIRON) SubnegotiationString(subnegotiation []byte) (string, error) {
	var sb strings.Builder

	if subnegotiation[0] == newenvironSEND {
		sb.WriteString("SEND ")
		err := o.subnegotiationSENDString(&sb, subnegotiation[1:])
		if err != nil {
			return "", err
		}

		str := sb.String()
		return str[:len(str)-1], nil
	}

	if subnegotiation[0] == newenvironIS || subnegotiation[0] == newenvironINFO {
		if subnegotiation[0] == newenvironIS {
			sb.WriteString("IS ")
		} else {
			sb.WriteString("INFO ")
		}

		err := o.subnegotiationValueString(&sb, subnegotiation[1:])
		if err != nil {
			return "", err
		}

		str := sb.String()
		return str[:len(str)-1], nil
	}

	return o.BaseTelOpt.SubnegotiationString(subnegotiation)
}

func (o *NEWENVIRON) SetVars(keysAndValues ...string) error {
	o.localVarsLock.Lock()
	defer o.localVarsLock.Unlock()

	if len(keysAndValues)%2 != 0 {
		return fmt.Errorf("new-environ: uneven numbers of keys and values. dangling value: %s", keysAndValues[len(keysAndValues)-1])
	}

	var estimatedBufferSize int

	for _, item := range keysAndValues {
		estimatedBufferSize += len(item)
	}

	buffer := bytes.NewBuffer(make([]byte, 0, estimatedBufferSize*2))
	buffer.WriteByte(newenvironINFO)

	for index := 0; index < len(keysAndValues); index += 2 {
		key := keysAndValues[index]
		value := keysAndValues[index+1]

		_, isWellKnown := o.wellKnownVars[key]
		if isWellKnown {
			buffer.WriteByte(newenvironVAR)
			o.localWellKnownVars[key] = value
		} else {
			buffer.WriteByte(newenvironUSERVAR)
			o.localUserVars[key] = value
		}

		o.encodeText(buffer, key)
		buffer.WriteByte(newenvironVALUE)
		o.encodeText(buffer, value)
	}

	if o.LocalState() == telnet.TelOptActive {
		o.Terminal().Keyboard().WriteCommand(telnet.Command{
			OpCode:         telnet.SB,
			Option:         newenviron,
			Subnegotiation: buffer.Bytes(),
		}, nil)
	}

	return nil
}

func (o *NEWENVIRON) ClearVars(keys ...string) {
	o.localVarsLock.Lock()
	defer o.localVarsLock.Unlock()

	var estimatedBufferSize int

	for _, key := range keys {
		estimatedBufferSize += len(key)
	}

	buffer := bytes.NewBuffer(make([]byte, 0, estimatedBufferSize*2))
	buffer.WriteByte(newenvironINFO)

	for _, key := range keys {
		_, isWellKnown := o.wellKnownVars[key]
		if isWellKnown {
			buffer.WriteByte(newenvironVAR)
			delete(o.localWellKnownVars, key)
		} else {
			buffer.WriteByte(newenvironUSERVAR)
			delete(o.localUserVars, key)
		}

		o.encodeText(buffer, key)
	}

	if o.LocalState() == telnet.TelOptActive {
		o.Terminal().Keyboard().WriteCommand(telnet.Command{
			OpCode:         telnet.SB,
			Option:         newenviron,
			Subnegotiation: buffer.Bytes(),
		}, nil)
	}
}

func (o *NEWENVIRON) RemoteWellKnownVar(key string) (string, bool) {
	o.remoteVarsLock.Lock()
	defer o.remoteVarsLock.Unlock()

	value, hasValue := o.remoteWellKnownVars[key]
	return value, hasValue
}

func (o *NEWENVIRON) RemoteUserVar(key string) (string, bool) {
	o.remoteVarsLock.Lock()
	defer o.remoteVarsLock.Unlock()

	value, hasValue := o.remoteUserVars[key]
	return value, hasValue
}
