package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"os"

	"github.com/moodclient/telnet"
	"github.com/moodclient/telnet/telopts"
	"github.com/moodclient/telnet/utils"
)

type session struct {
	sentPrompt bool
}

func (s *session) echoOutput(t *telnet.Terminal, output telnet.TerminalData) {
	if s.sentPrompt {
		t.Keyboard().LineOut(t, telnet.ControlCodeData('\r'))
		t.Keyboard().LineOut(t, telnet.ControlCodeData('\n'))
		s.sentPrompt = false
	}

	switch o := output.(type) {
	case telnet.TextData:
		if o == "quit" {
			os.Exit(0)
		}
		t.Keyboard().WriteString(string(o))
	case telnet.ControlCodeData:
		if o == '\n' {
			s.sendPrompt(t)
		}
	}
}

func (s *session) sendPrompt(t *telnet.Terminal) {
	t.Keyboard().WriteString("\r\n\r\n > ")
	t.Keyboard().SendPromptHint()
	s.sentPrompt = true
}

func singleConnection(ctx context.Context, conn net.Conn) {
	s := session{}

	terminal, err := telnet.NewTerminal(ctx, conn, telnet.TerminalConfig{
		Side:               telnet.SideClient,
		DefaultCharsetName: "US-ASCII",
		TelOpts: []telnet.TelnetOption{
			telopts.RegisterTRANSMITBINARY(telnet.TelOptRequestLocal | telnet.TelOptRequestRemote),
			telopts.RegisterECHO(telnet.TelOptRequestLocal),
			telopts.RegisterSUPPRESSGOAHEAD(telnet.TelOptRequestLocal | telnet.TelOptRequestRemote),
		},
		EventHooks: telnet.EventHooks{
			EncounteredError: []telnet.ErrorHandler{encounteredError},
		},
	})
	if err != nil {
		log.Fatalln(err)
	}

	lineFeed := utils.NewLineFeed(terminal, s.echoOutput,
		func(t *telnet.Terminal, data telnet.TerminalData) {
			t.Keyboard().LineOut(t, data)
		}, utils.LineFeedConfig{MaxLength: 300})
	terminal.RegisterPrinterOutputHook(lineFeed.LineIn)

	terminal.Keyboard().WriteString("Welcome to your echo service! Type anything!")
	s.sendPrompt(terminal)

	err = terminal.WaitForExit()
	if err != nil {
		log.Println(err)
	}
}

func ServerListener(ctx context.Context) {
	privateCert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err != nil {
		log.Fatalln(err)
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{privateCert}}

	listener, err := tls.Listen("tcp", ":23235", tlsConfig)
	if err != nil {
		log.Fatalln(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalln(err)
		}

		go singleConnection(ctx, conn)
	}
}
