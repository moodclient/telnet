package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
	"github.com/cannibalvox/moodclient/telnet/telopts"
	"github.com/charmbracelet/lipgloss/v2"
	"log"
	"net"
	"os"
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

func main() {
	addr, err := net.ResolveTCPAddr("tcp", "erionmud.com:1234")
	if err != nil {
		log.Fatalln(err)
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		log.Fatalln(err)
	}

	lipgloss.EnableLegacyWindowsANSI(os.Stdout)
	lipgloss.EnableLegacyWindowsANSI(os.Stdin)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	c, err := telnet.NewTerminal(context.Background(), conn, telnet.TerminalConfig{
		Side:               telnet.SideClient,
		DefaultCharsetName: "US-ASCII",
		TelOpts: []telnet.TelnetOption{
			telopts.CHARSET(telnet.TelOptAllowLocal|telnet.TelOptAllowRemote, telopts.CHARSETConfig{
				AllowAnyCharset:   true,
				PreferredCharsets: []string{"UTF-8", "US-ASCII"},
			}),
			telopts.TRANSMITBINARY(telnet.TelOptAllowLocal | telnet.TelOptAllowRemote),
			telopts.EOR(telnet.TelOptAllowRemote | telnet.TelOptAllowLocal),
			telopts.ECHO(telnet.TelOptAllowRemote),
			telopts.TTYPE(telnet.TelOptAllowLocal, []string{
				"MOODCLIENT",
				"XTERM-256COLOR",
				"MTTS 299",
			}),
			telopts.SUPPRESSGOAHEAD(telnet.TelOptAllowLocal | telnet.TelOptAllowRemote),
			telopts.NAWS(telnet.TelOptAllowLocal),
			telopts.NEWENVIRON(telnet.TelOptAllowRemote|telnet.TelOptAllowLocal, telopts.NEWENVIRONConfig{
				WellKnownVarKeys: telopts.NEWENVIRONWellKnownVars,
			}),
			telopts.SENDLOCATION(telnet.TelOptAllowLocal, "SOMEWHERE MYSTERIOUS"),
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

	reader := bufio.NewReader(os.Stdin)
	for {
		_, err = reader.ReadString('\n')
		if err != nil {
			break
		}
		break
	}

	cancel()
	err = c.WaitForExit()
	if err != nil {
		log.Fatalln(err)
	}
}
