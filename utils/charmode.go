package utils

import (
	"github.com/moodclient/telnet"
	"github.com/moodclient/telnet/telopts"
)

type CharacterModeTracker struct {
	terminal *telnet.Terminal

	remoteSuppressGA     bool
	remoteEcho           bool
	localLineModeNonEdit bool
}

func NewCharacterModeTracker(t *telnet.Terminal) *CharacterModeTracker {
	tracker := &CharacterModeTracker{terminal: t}
	t.RegisterTelOptEventHook(tracker.TelOptEvent)

	sga, err := telnet.GetTelOpt[telopts.SUPPRESSGOAHEAD](t)
	if err == nil {
		tracker.remoteSuppressGA = sga.RemoteState() == telnet.TelOptActive
	}

	echo, err := telnet.GetTelOpt[telopts.ECHO](t)
	if err == nil {
		tracker.remoteEcho = echo.RemoteState() == telnet.TelOptActive
	}

	linemode, err := telnet.GetTelOpt[telopts.LINEMODE](t)
	if err == nil {
		tracker.localLineModeNonEdit = linemode.LocalState() == telnet.TelOptActive &&
			linemode.Mode()&telopts.LineModeEDIT == 0
	}

	return tracker
}

func (t *CharacterModeTracker) TelOptEvent(terminal *telnet.Terminal, data telnet.TelOptEvent) {
	switch typed := data.(type) {
	case telnet.TelOptStateChangeEvent:
		switch option := typed.Option().(type) {
		case *telopts.SUPPRESSGOAHEAD:
			if typed.NewState == telnet.TelOptActive {
				t.remoteSuppressGA = true
			} else if typed.NewState == telnet.TelOptInactive {
				t.remoteSuppressGA = false
			}
		case *telopts.ECHO:
			if typed.NewState == telnet.TelOptActive {
				t.remoteEcho = true
			} else if typed.NewState == telnet.TelOptInactive {
				t.remoteEcho = false
			}
		case *telopts.LINEMODE:
			if typed.NewState == telnet.TelOptActive {
				t.localLineModeNonEdit = option.Mode()&telopts.LineModeEDIT == 0
			} else if typed.NewState == telnet.TelOptInactive {
				t.localLineModeNonEdit = false
			}
		}

	case telopts.LINEMODEChangeEvent:
		t.localLineModeNonEdit = typed.NewMode&telopts.LineModeEDIT == 0
	}
}

// IsCharacterMode will return true if both the ECHO and SUPPRESS-GO-AHEAD options are
// enabled.  Technically this is supposed to be the case when NEITHER or BOTH are enabled,
// as traditionally, "kludge line mode", the line-at-a-time operation you might be familiar
// with, is supposed to occur when either ECHO or SUPPRESS-GO-AHEAD, but not both, are
// enabled.  However, MUDs traditionally operate in a line-at-a-time manner and do not
// usually request SUPPRESS-GO-AHEAD (instead using IAC GA to indicate the location of
// a prompt to clients), resulting in a relatively common expectation that
// kludge line mode is active when neither telopt is active.
//
// As a result, in order to allow the broadest support for the most clients possible,
// it's recommended that you activate both SUPPRESS-GO-AHEAD and EOR when you want to
// support line-at-a-time mode and activate both SUPPRESS-GO-AHEAD and ECHO when
// when you want to support character mode. If line-at-a-time is desired and EOR
// is not available, then leaving SUPPRESS-GO-AHEAD and ECHO both inactive and proceeding
// with line-at-a-time will generally work.
//
// BBS's additionally sometimes use LINEMODE which can negotiate whether to use line or character
// mode in the form of the EDIT flag in MODE
func (t *CharacterModeTracker) IsCharacterMode() bool {
	return t.localLineModeNonEdit || (t.remoteEcho && t.remoteSuppressGA)
}
