package main

import (
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

	err = telnet.RegisterOption[telopts.CHARSET](telopts.CHARSETRegistration(telopts.CHARSETOptions{
		AllowAnyCharset:   true,
		PreferredCharsets: []string{"UTF-8", "US-ASCII"},
	}))
	if err != nil {
		log.Fatalln(err)
	}

	err = telnet.RegisterOption[telopts.ECHO](telopts.ECHORegistration())
	if err != nil {
		log.Fatalln(err)
	}

	err = telnet.RegisterOption[telopts.EOR](telopts.EORRegistration())
	if err != nil {
		log.Fatalln(err)
	}

	err = telnet.RegisterOption[telopts.NAWS](telopts.NAWSRegistration())
	if err != nil {
		log.Fatalln(err)
	}

	err = telnet.RegisterOption[telopts.SUPPRESSGOAHEAD](telopts.SUPPRESSGOAHEADRegistration())
	if err != nil {
		log.Fatalln(err)
	}

	err = telnet.RegisterOption[telopts.TTYPE](telopts.TTYPERegistration([]string{
		"MOODCLIENT",
		"XTERM-256COLOR",
		"MTTS 299",
	}))
	if err != nil {
		log.Fatalln(err)
	}

	c, err := telnet.NewTerminal(context.Background(), conn, telnet.SideClient, telnet.TelOptPreferences{
		AllowLocal:  []telnet.TelOptCode{telopts.CodeEOR, telopts.CodeCHARSET, telopts.CodeNAWS, telopts.CodeTTYPE, telopts.CodeSUPPRESSGOAHEAD},
		AllowRemote: []telnet.TelOptCode{telopts.CodeECHO, telopts.CodeEOR, telopts.CodeCHARSET, telopts.CodeSUPPRESSGOAHEAD},
	})
	if err != nil {
		log.Fatalln(err)
	}

	_, err = fmt.Scanln()
	if err != nil {
		log.Fatalln(err)
	}

	cancel()
	err = c.WaitForExit()
	if err != nil {
		log.Fatalln(err)
	}
}
