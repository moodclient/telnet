package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cannibalvox/moodclient/telnet"
	"github.com/cannibalvox/moodclient/telnet/telopts"
	"github.com/cannibalvox/moodclient/telnet/utils"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

func incomingCommand(t *telnet.Terminal, c telnet.Command) {
	fmt.Println("COMMAND:", t.CommandString(c))
}

func encounteredError(t *telnet.Terminal, err error) {
	fmt.Println(err)
}

func incomingText(t *telnet.Terminal, data telnet.IncomingTextData) {
	if data.OverwritePrevious {
		// Rewrite line
		fmt.Print(string('\r'))
	}

	fmt.Print(data.Text)
}

func outboundText(t *telnet.Terminal, text string) {
	fmt.Println("SENT:", text)
}

func outboundCommand(t *telnet.Terminal, c telnet.Command) {
	fmt.Println("OUTBOUND:", t.CommandString(c))
}

func telOptStateChange(t *telnet.Terminal, e telnet.TelOptStateChangeData) {
	if e.Side == telnet.TelOptSideLocal {
		fmt.Println(e.Option, "LOCAL", fmt.Sprintf("%s -> %s", e.OldState, e.Option.LocalState()))
		return
	}

	fmt.Println(e.Option, "REMOTE", fmt.Sprintf("%s -> %s", e.OldState, e.Option.RemoteState()))
}

func echo(t *telnet.Terminal, echo string) {
	o, err := telnet.GetTelOpt[telopts.ECHO](t)
	if err != nil {
		fmt.Println(err)
	}

	if o.RemoteState() != telnet.TelOptActive {
		fmt.Print(echo)
	}
}

func main() {
	addr, err := net.ResolveTCPAddr("tcp", "erionmud.com:1234")
	if err != nil {
		log.Fatalln(err)
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		log.Fatalln(err)
	}

	stdin := os.Stdin
	lipgloss.EnableLegacyWindowsANSI(os.Stdout)
	lipgloss.EnableLegacyWindowsANSI(stdin)
	state, err := term.MakeRaw(stdin.Fd())
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		err := term.Restore(stdin.Fd(), state)
		if err != nil {
			log.Println(err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	terminal, err := telnet.NewTerminal(context.Background(), conn, telnet.TerminalConfig{
		Side:               telnet.SideClient,
		DefaultCharsetName: "US-ASCII",
		TelOpts: []telnet.TelnetOption{
			telopts.RegisterCHARSET(telnet.TelOptAllowLocal|telnet.TelOptAllowRemote, telopts.CHARSETConfig{
				AllowAnyCharset:   true,
				PreferredCharsets: []string{"UTF-8", "US-ASCII"},
			}),
			telopts.RegisterTRANSMITBINARY(telnet.TelOptAllowLocal | telnet.TelOptAllowRemote),
			telopts.RegisterEOR(telnet.TelOptAllowRemote | telnet.TelOptAllowLocal),
			telopts.RegisterECHO(telnet.TelOptAllowRemote),
			telopts.RegisterTTYPE(telnet.TelOptAllowLocal, []string{
				"MOODCLIENT",
				"XTERM-256COLOR",
				"MTTS 299",
			}),
			telopts.RegisterSUPPRESSGOAHEAD(telnet.TelOptAllowLocal | telnet.TelOptAllowRemote),
			telopts.RegisterNAWS(telnet.TelOptAllowLocal),
			telopts.RegisterNEWENVIRON(telnet.TelOptAllowLocal, telopts.NEWENVIRONConfig{
				WellKnownVarKeys: telopts.NEWENVIRONWellKnownVars,
			}),
			telopts.RegisterSENDLOCATION(telnet.TelOptAllowLocal, "SOMEWHERE MYSTERIOUS"),
		},
		EventHooks: telnet.EventHooks{
			IncomingCommand:   []telnet.CommandEvent{incomingCommand},
			IncomingText:      []telnet.IncomingTextEvent{incomingText},
			OutboundCommand:   []telnet.CommandEvent{outboundCommand},
			OutboundText:      []telnet.OutboundTextEvent{outboundText},
			EncounteredError:  []telnet.ErrorEvent{encounteredError},
			TelOptStateChange: []telnet.TelOptStateChangeEvent{telOptStateChange},
		},
	})
	if err != nil {
		log.Fatalln(err)
	}

	feed, err := utils.NewKeyboardFeed(terminal, stdin, []utils.EchoEvent{echo})
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		err := feed.FeedLoop()
		if err != nil {
			log.Println(err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs

	cancel()
	err = terminal.WaitForExit()
	if err != nil {
		log.Fatalln(err)
	}
}
