package telnet

import "sync/atomic"

// PromptCommands is a set of flags indicating which IAC opcodes indicate
// the end of a prompt line. MUDs like to use GA or EOR to indicate where
// to place the cursor at the end of a prompt. But GA can be turned off
// with the SUPPRESS-GO-AHEAD telopt, and EOR has to be turned on with the
// EOR telopt, so this helps us track where we're at.
type PromptCommands uint32

const (
	PromptCommandGA PromptCommands = 1 << iota
	PromptCommandEOR
)

type atomicPromptCommands struct {
	promptCommands atomic.Uint32
}

func (p *atomicPromptCommands) Init() {
	p.promptCommands.Store(uint32(PromptCommandGA))
}

func (p *atomicPromptCommands) Get() PromptCommands {
	return PromptCommands(p.promptCommands.Load())
}

func (p *atomicPromptCommands) SetPromptCommand(flag PromptCommands) {
	for {
		oldValue := p.promptCommands.Load()
		if p.promptCommands.CompareAndSwap(oldValue, oldValue|uint32(flag)) {
			break
		}
	}
}

func (p *atomicPromptCommands) ClearPromptCommand(flag PromptCommands) {
	for {
		oldValue := p.promptCommands.Load()
		if p.promptCommands.CompareAndSwap(oldValue, oldValue&uint32(^flag)) {
			break
		}
	}
}
