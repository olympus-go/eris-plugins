package logger

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

type Plugin struct {
	logger *slog.Logger
	level  slog.Level
	//logger zerolog.Logger
	//level  zerolog.Level
}

func NewPlugin(h slog.Handler, level slog.Level) *Plugin {
	return &Plugin{
		logger: slog.New(h),
		level:  level,
	}
}

func (p *Plugin) Name() string {
	return "Logger"
}

func (p *Plugin) Description() string {
	return "Logs high level interaction usage"
}

func (p *Plugin) Handlers() map[string]any {
	handlers := make(map[string]any)

	handlers["logger_handler"] = p.loggerHandler

	return handlers
}

func (p *Plugin) Commands() map[string]*discordgo.ApplicationCommand {
	return nil
}

func (p *Plugin) Intents() []discordgo.Intent {
	return nil
}
