package telnet

import "sync/atomic"

// PromptCommands is a set of flags indicating which IAC opcodes indicate
// the end of a prompt line. MUDs like to use GA or EOR to indicate where
// to place the cursor at the end of a prompt. But GA can be turned off
// with the SUPPRESS-GO-AHEAD telopt, and EOR has to be turned on with the
// EOR telopt, so this helps us track where we're at.
type PromptCommands uint32

const (
	// PromptCommandGA refers to the IAC GA command. This command was initially used as part
	// of telnet's scheme for supporting half-duplex terminals.  However, half-duplex terminals
	// were rapidly phased out after the telnet protocol was introduced and eventually came to be
	// used for a variety of hacky boutique purposes.  For MUDs and BBSs it is often deactivated
	// via the SUPPRESS-GO-AHEAD telopt in order to activate character mode. It is sometimes used
	// as a prompt indicator on MUDs
	PromptCommandGA PromptCommands = 1 << iota
	// PromptCommandEOR refers to the IAC EOR command. This was introduced as part of the EOR telopt
	// and can be used for any purpose an application would like to use it for.  It is mainly only
	// used as a prompt indicator on MUDs.
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
