package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/moodclient/telnet"
	"github.com/moodclient/telnet/telopts"
	"github.com/moodclient/telnet/utils"
)

func encounteredError(t *telnet.Terminal, err error) {
	fmt.Println(err)
}

func printerOutput(t *telnet.Terminal, output telnet.PrinterOutput) {
	fmt.Print(output.String())
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("syntax: mudclient <host>:<port>")
	}

	addr, err := net.ResolveTCPAddr("tcp", os.Args[1])
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	terminal, err := telnet.NewTerminal(ctx, conn, telnet.TerminalConfig{
		Side:               telnet.SideClient,
		DefaultCharsetName: "CP437",
		TelOpts: []telnet.TelnetOption{
			telopts.RegisterCHARSET(telnet.TelOptAllowLocal|telnet.TelOptAllowRemote, telopts.CHARSETConfig{
				AllowAnyCharset:   true,
				PreferredCharsets: []string{"CP437", "UTF-8", "US-ASCII"},
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
		},
		EventHooks: telnet.EventHooks{
			PrinterOutput:    []telnet.PrinterOutputHandler{printerOutput},
			EncounteredError: []telnet.ErrorHandler{encounteredError},
		},
	})
	if err != nil {
		log.Fatalln(err)
	}

	feed, err := utils.NewKeyboardFeed(terminal, stdin, nil)
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		err := feed.FeedLoop()
		if err != nil {
			log.Println(err)
		}
	}()

	logStore := bytes.NewBuffer(nil)
	logHandler := slog.New(slog.NewTextHandler(logStore, nil))
	_ = utils.NewDebugLog(terminal, logHandler, utils.DebugLogConfig{
		EncounteredErrorLevel:  slog.LevelError,
		IncomingCommandLevel:   slog.LevelInfo,
		IncomingTextLevel:      utils.LevelNone,
		OutboundCommandLevel:   slog.LevelInfo,
		OutboundTextLevel:      utils.LevelNone,
		TelOptEventLevel:       slog.LevelDebug,
		TelOptStageChangeLevel: slog.LevelDebug,
	})

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

		<-sigs

		cancel()
	}()

	err = terminal.WaitForExit()
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("\x1b[0m")
	fmt.Println(logStore.String())
}
