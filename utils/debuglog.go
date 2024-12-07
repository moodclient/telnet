package utils

import (
	"context"
	"log/slog"

	"github.com/moodclient/telnet"
)

const LevelNone slog.Level = -8

type DebugLogConfig struct {
	EncounteredErrorLevel  slog.Level
	IncomingCommandLevel   slog.Level
	IncomingTextLevel      slog.Level
	OutboundCommandLevel   slog.Level
	OutboundTextLevel      slog.Level
	TelOptEventLevel       slog.Level
	TelOptStageChangeLevel slog.Level
}

type DebugLog struct {
	logger *slog.Logger
	config DebugLogConfig
}

func NewDebugLog(terminal *telnet.Terminal, logger *slog.Logger, config DebugLogConfig) *DebugLog {
	log := &DebugLog{logger: logger, config: config}

	terminal.RegisterEncounteredErrorHook(log.logError)
	terminal.RegisterPrinterOutputHook(log.logPrinterOutput)
	terminal.RegisterOutboundCommandHook(log.logOutboundCommand)
	terminal.RegisterOutboundTextHook(log.logOutboundText)
	terminal.RegisterTelOptEventHook(log.logTelOptEvent)
	terminal.RegisterTelOptStateChangeEventHook(log.logTelOptStateChange)

	return log
}

func (l *DebugLog) logError(terminal *telnet.Terminal, err error) {
	l.logger.LogAttrs(context.Background(), l.config.EncounteredErrorLevel, "Encountered error", slog.Any("error", err))
}

func (l *DebugLog) logPrinterOutput(terminal *telnet.Terminal, output telnet.PrinterOutput) {
	switch o := output.(type) {
	case telnet.CommandOutput:
		l.logger.LogAttrs(context.Background(), l.config.IncomingCommandLevel, "Received command", slog.String("command", o.EscapedString(terminal)))
	default:
		l.logger.LogAttrs(context.Background(), l.config.IncomingTextLevel, output.EscapedString(terminal))
	}
}

func (l *DebugLog) logOutboundCommand(terminal *telnet.Terminal, c telnet.Command) {
	l.logger.LogAttrs(context.Background(), l.config.OutboundCommandLevel, "Sent command", slog.String("command", terminal.CommandString(c)))
}

func (l *DebugLog) logOutboundText(terminal *telnet.Terminal, text string) {
	l.logger.LogAttrs(context.Background(), l.config.OutboundTextLevel, "Sent text", slog.String("contents", text))
}

func (l *DebugLog) logTelOptEvent(terminal *telnet.Terminal, event telnet.TelOptEventData) {
	attrs := []slog.Attr{slog.String("option", event.Option.String())}

	name, payload, err := event.Option.EventString(event)
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	} else {
		attrs = append(attrs, slog.String("event", name))
		if payload != "" {
			attrs = append(attrs, slog.String("payload", payload))
		}
	}

	l.logger.LogAttrs(context.Background(), l.config.TelOptEventLevel, "TelOpt Event", attrs...)
}

func (l *DebugLog) logTelOptStateChange(terminal *telnet.Terminal, event telnet.TelOptStateChangeData) {
	newState := event.Option.LocalState()
	if event.Side == telnet.TelOptSideRemote {
		newState = event.Option.RemoteState()
	}

	l.logger.LogAttrs(context.Background(), l.config.TelOptStageChangeLevel, "TelOpt State Change",
		slog.String("oldState", event.OldState.String()),
		slog.String("newState", newState.String()),
		slog.String("side", event.Side.String()),
	)
}
