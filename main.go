package main

import (
	"context"
	"fmt"
	"github.com/cannibalvox/moodclient/telnet"
	"github.com/charmbracelet/lipgloss/v2"
	"log"
	"net"
	"os"
)

func main() {
	addr, err := net.ResolveTCPAddr("tcp", "atp.pedia.szote.u-szeged.hu:3000")
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

	library := telnet.NewTelOptLibrary()

	c, err := telnet.NewTerminal(context.Background(), conn, library, telnet.TelOptPreferences{})
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
