package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/term"
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
	// Launch server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ServerListener(ctx)

	// Build public cert with certificate authority
	certFile, err := os.ReadFile("cert.pem")
	if err != nil {
		log.Fatalln(err)
	}
	signingAuthorities := x509.NewCertPool()

	addedCert := signingAuthorities.AppendCertsFromPEM(certFile)
	if !addedCert {
		log.Fatalln("failed to parse a valid certificate in cert.pem")
	}

	// Configure pty
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

	// Wait a moment for the server to start, then connect
	time.Sleep(50 * time.Millisecond)

	conn, err := tls.Dial("tcp", "localhost:23235", &tls.Config{RootCAs: signingAuthorities})
	if err != nil {
		log.Fatalln(err)
	}

	terminal, err := telnet.NewTerminal(ctx, conn, telnet.TerminalConfig{
		Side:               telnet.SideClient,
		DefaultCharsetName: "US-ASCII",
		TelOpts: []telnet.TelnetOption{
			telopts.RegisterTRANSMITBINARY(telnet.TelOptAllowLocal | telnet.TelOptAllowRemote),
			telopts.RegisterECHO(telnet.TelOptAllowRemote),
			telopts.RegisterSUPPRESSGOAHEAD(telnet.TelOptAllowLocal | telnet.TelOptAllowRemote),
		},
		EventHooks: telnet.EventHooks{
			PrinterOutput:    []telnet.TerminalDataHandler{printerOutput},
			EncounteredError: []telnet.ErrorHandler{encounteredError},
		},
	})
	if err != nil {
		log.Fatalln(err)
	}

	lineFeed := utils.NewLineFeed(terminal, terminal.Keyboard().LineOut, printerOutput, utils.LineFeedConfig{})

	feed, err := utils.NewKeyboardFeed(terminal, stdin, lineFeed)
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
