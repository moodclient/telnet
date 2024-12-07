package telnet

import (
	"fmt"
)

func (t *Terminal) initTelopts(options []TelnetOption) error {
	for _, option := range options {
		oldOption, hasOldOption := t.options[option.Code()]
		if hasOldOption {
			return fmt.Errorf("telopt collision: TelOpt %d is already registered to an option of type %T. it cannot be registered to an option of type %T", option.Code(), oldOption, option)
		}

		option.Initialize(t)
		t.options[option.Code()] = option
	}

	return nil
}

func (t *Terminal) writeTelOptRequests() error {
	for _, option := range t.options {
		usage := option.Usage()
		if usage&telOptOnlyRequestLocal != 0 {
			t.keyboard.WriteCommand(Command{
				OpCode: WILL,
				Option: option.Code(),
			})

			oldState := option.LocalState()

			if oldState == TelOptInactive {
				err := option.TransitionLocalState(TelOptRequested)
				if err != nil {
					return err
				}

				t.RaiseTelOptEvent(TelOptStateChangeEvent{
					TelnetOption: option,
					Side:         TelOptSideLocal,
					OldState:     oldState,
					NewState:     TelOptRequested,
				})
			}
		}

		if usage&telOptOnlyRequestRemote != 0 {
			t.keyboard.WriteCommand(Command{
				OpCode: DO,
				Option: option.Code(),
			})

			oldState := option.RemoteState()

			if oldState == TelOptInactive {
				err := option.TransitionRemoteState(TelOptRequested)
				if err != nil {
					return err
				}

				t.RaiseTelOptEvent(TelOptStateChangeEvent{
					TelnetOption: option,
					Side:         TelOptSideRemote,
					OldState:     oldState,
					NewState:     TelOptRequested,
				})
			}
		}
	}

	return nil
}

func (t *Terminal) rejectNegotiationRequest(c Command) {
	if c.isActivateNegotiation() {
		t.keyboard.WriteCommand(c.reject())
	}
}

func (t *Terminal) processSubnegotiation(c Command) error {
	option, hasOption := t.options[c.Option]
	if !hasOption {
		// Getting subnegotiations for stuff we haven't agreed to
		return nil
	}

	if option.LocalState() != TelOptActive && option.RemoteState() != TelOptActive {
		// Getting subnegotiations for stuff we haven't agreed to
		return nil
	}

	return option.Subnegotiate(c.Subnegotiation)
}

func (t *Terminal) processTelOptCommand(c Command) error {
	if c.OpCode == SB {
		return t.processSubnegotiation(c)
	}

	// It's not a negotiation command
	if c.OpCode != DO && c.OpCode != DONT && c.OpCode != WILL && c.OpCode != WONT {
		return nil
	}

	// Is this an option we know about?
	option, hasOption := t.options[c.Option]
	if !hasOption {
		// Unregistered telopt
		t.rejectNegotiationRequest(c)

		return nil
	}

	oldState := option.RemoteState()
	side := TelOptSideRemote
	transitionFunc := option.TransitionRemoteState
	allowFlag := TelOptAllowRemote
	if c.isLocalNegotiation() {
		oldState = option.LocalState()
		side = TelOptSideLocal
		transitionFunc = option.TransitionLocalState
		allowFlag = TelOptAllowLocal
	}

	// They are requesting WONT/DONT
	if !c.isActivateNegotiation() && oldState == TelOptInactive {
		// already turned off
		return nil
	} else if !c.isActivateNegotiation() {
		// need to turn it off
		err := transitionFunc(TelOptInactive)
		if err != nil {
			return err
		}

		t.RaiseTelOptEvent(TelOptStateChangeEvent{
			TelnetOption: option,
			Side:         side,
			OldState:     oldState,
			NewState:     TelOptInactive,
		})

		return nil
	}

	// They are requesting DO/WILL
	if oldState == TelOptActive {
		// Already turned on
		return nil
	}

	if option.Usage()&allowFlag == 0 {
		// Disallowed telopt
		t.rejectNegotiationRequest(c)

		return nil
	}

	if oldState == TelOptInactive {
		// Need to send an accept command
		t.keyboard.WriteCommand(c.accept())
	}

	err := transitionFunc(TelOptActive)
	if err != nil {
		return err
	}

	t.RaiseTelOptEvent(TelOptStateChangeEvent{
		TelnetOption: option,
		Side:         side,
		OldState:     oldState,
		NewState:     TelOptActive,
	})
	return nil
}
