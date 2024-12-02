# Moodclient/Telnet

[![Go Version](https://img.shields.io/github/go-mod/go-version/gomods/athens.svg)](https://github.com/moodclient/telnet) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/moodclient/telnet) [![GoReportCard](https://goreportcard.com/badge/github.com/nanomsg/mangos)](https://goreportcard.com/report/github.com/moodclient/telnet)

This library provides a wrapper that can fit around any net.Conn in order to provide Telnet services for any arbitrary connection.  In addition to basic line-level read and write that is compatible with RFC854/RFC5198, this library also provides an extensible base for Telnet Options (telopts), handles telopt negotiation and subnegotiation routing, and provides implementations for 9 heavily-used telopts.

In the examples folder, an example for a dead-simple terminal-based MUD client can be found.

## How To Use?

Initialize a new terminal with your connection and configuration:

```go
	terminal, err := telnet.NewTerminal(context.Background(), conn, telnet.TerminalConfig{
		Side:               telnet.SideClient,
		DefaultCharsetName: "US-ASCII",
	})
	if err != nil {
		log.Fatalln(err)
	}
```

The terminal will immediately begin communicating on the connection and negotiating options.  It will continue to do so until the connection is closed or the provided context is cancelled (or if the context times out, but that would be weird).

You can call `terminal.WaitForExit()` to block the current goroutine until the terminal finishes.

You can write lines of text to the terminal using `terminal.Keyboard().WriteString`.

### Hooks

Get data from the telnet connection using the Terminal's many, many event hooks:


```go
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

...
	terminal, err := telnet.NewTerminal(ctx, conn, telnet.TerminalConfig{
		Side:               telnet.SideClient,
		DefaultCharsetName: "US-ASCII",
		EventHooks: telnet.EventHooks{
			IncomingText:     []telnet.IncomingTextEvent{incomingText},
			EncounteredError: []telnet.ErrorEvent{encounteredError},
		},
	})
```

You can also register hook with the terminal after creation:

```go
terminal.RegisterIncomingTextHook(incomingText)
```

### TelOpt Support

By default, the terminal will reject all attempts at telopt negotiation by the remote party.  You can register telopts with the terminal on creation. The first argument of a registration event is whether and how the telopt is permitted.  Other parameters are telopt-specific.

```go
	terminal, err := telnet.NewTerminal(ctx, conn, telnet.TerminalConfig{
		Side:               telnet.SideClient,
		DefaultCharsetName: "US-ASCII",
		TelOpts: []telnet.TelnetOption{
			telopts.RegisterCHARSET(telnet.TelOptAllowLocal|telnet.TelOptAllowRemote, telopts.CHARSETConfig{
				AllowAnyCharset:   true,
				PreferredCharsets: []string{"UTF-8", "US-ASCII"},
			}),
			telopts.RegisterEOR(telnet.TelOptRequestRemote | telnet.TelOptAllowLocal),
			telopts.RegisterECHO(telnet.TelOptAllowRemote),
			telopts.RegisterSUPPRESSGOAHEAD(telnet.TelOptAllowLocal | telnet.TelOptAllowRemote),
			telopts.RegisterNAWS(telnet.TelOptAllowLocal),
		},
		EventHooks: telnet.EventHooks{
			IncomingText:     []telnet.IncomingTextEvent{incomingText},
			EncounteredError: []telnet.ErrorEvent{encounteredError},
		},
	})
```


## Why Another Telnet Library In Go?

[There](https://github.com/gbazil/telnet) [are](https://github.com/reiver/go-telnet) [a](https://github.com/aprice/telnet) [great](https://github.com/plyul/telnet) [many](https://github.com/Tanjmaxalb/telnet-client) telnet libraries written in go.  However, telopt support in these libraries is usually spotty, and never extensible.  If one wants to write a mud client (check the org name) in go, strong support for many boutique telopts is required.  Concepts that are not part of the telnet RFC but are central to modern use of the telnet protocol, such as the weird rules around IAC GA/IAC EOR, are important and not represented in these libraries.

The ultimate goal of this library is for it not just implement the basics of the telnet protocol, but be a useful core for real-world uses of telnet, such as MUD and BBS clients and servers, strange online games that use vt100 for TUIs, and other oddities. Making this work will take long-term, dedicated work on a telnet protocol library.

## What Is Missing?

A lot!  The example(s) provided make clear that this library works well when communications occur in linemode, with basic ANSI colors. Character mode works... less well.  VT100 worse than that.  

Additionally, this has not been used in an environment where one server is tracking several different terminals for different connected users. I suspect that the library will grow difficult to work with.

The next step is going to be working with [Bubbletea](https://github.com/charmbracelet/bubbletea) to understand Raw Mode better in order to improve how the library interacts with character mode and VT100, with the ultimate goal of an example BBS client to sit alongside the example MUD client.
