package logger

import (
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
)

type Plugin struct {
	logger zerolog.Logger
	level  zerolog.Level
}

func NewPlugin(logger zerolog.Logger, level zerolog.Level) *Plugin {
	return &Plugin{
		logger: logger,
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
