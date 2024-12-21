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
	"github.com/charmbracelet/x/term"
	"github.com/moodclient/mudopts"
	"github.com/moodclient/telnet"
	"github.com/moodclient/telnet/telopts"
	"github.com/moodclient/telnet/utils"
)

func encounteredError(t *telnet.Terminal, err error) {
	fmt.Println(err)
}

func printerOutput(t *telnet.Terminal, output telnet.TerminalData) {
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

	state, err := term.MakeRaw(stdin.Fd())
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		_ = term.Restore(stdin.Fd(), state)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	terminal, err := telnet.NewTerminal(ctx, conn, telnet.TerminalConfig{
		Side:                telnet.SideClient,
		DefaultCharsetName:  "US-ASCII",
		FallbackCharsetName: "CP437-FULL",
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
			mudopts.RegisterMCCP2(telnet.TelOptAllowRemote),
			mudopts.RegisterMCCP3(telnet.TelOptAllowRemote),
		},
		EventHooks: telnet.EventHooks{
			PrinterOutput:    []telnet.TerminalDataHandler{printerOutput},
			EncounteredError: []telnet.ErrorHandler{encounteredError},
		},
	})
	if err != nil {
		log.Fatalln(err)
	}

	charMode := utils.NewCharacterModeTracker(terminal)
	lineFeed := utils.NewLineFeed(terminal, terminal.Keyboard().LineOut, printerOutput, utils.LineFeedConfig{})

	feed, err := utils.NewKeyboardFeed(terminal, stdin, lineFeed, charMode)
	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		err := feed.FeedLoop()
		if err != nil {
			log.Println(err)
		}
		cancel()
	}()

	logStore := bytes.NewBuffer(nil)
	logHandler := slog.New(slog.NewTextHandler(logStore, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	_ = utils.NewDebugLog(terminal, logHandler, utils.DebugLogConfig{
		EncounteredErrorLevel:  slog.LevelError,
		IncomingCommandLevel:   slog.LevelInfo,
		IncomingTextLevel:      slog.LevelDebug,
		OutboundCommandLevel:   slog.LevelInfo,
		OutboundTextLevel:      slog.LevelDebug,
		TelOptEventLevel:       slog.LevelDebug,
		TelOptStageChangeLevel: slog.LevelDebug,
	})

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

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
