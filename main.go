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

func main() {
	addr, err := net.ResolveTCPAddr("tcp", "20forbeers.com:1337")
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
	})
	if err != nil {
		log.Fatalln(err)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		fmt.Println(line)
		break
	}

	cancel()
	err = c.WaitForExit()
	if err != nil {
		log.Fatalln(err)
	}
}
