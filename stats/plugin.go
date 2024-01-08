package stats

import (
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/eolso/threadsafe"
)

type Plugin struct {
	functions *threadsafe.Map[string, func() string]

	logger *slog.Logger
}

func NewPlugin(h slog.Handler) *Plugin {
	p := Plugin{
		functions: threadsafe.NewMap[string, func() string](),
		logger:    slog.New(h),
	}

	startTime := time.Now()
	p.AddStatFunc("Uptime", func() string { return time.Since(startTime).Round(time.Second).String() })
	p.AddStat("Runtime", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))

	return &p
}

func (p *Plugin) Name() string {
	return "Stats"
}

func (p *Plugin) Description() string {
	return "Prints out various technical stats"
}

func (p *Plugin) Handlers() map[string]any {
	handlers := make(map[string]any)

	handlers["stats_handler"] = p.statsHandler

	return handlers
}

func (p *Plugin) Commands() map[string]*discordgo.ApplicationCommand {
	commands := make(map[string]*discordgo.ApplicationCommand)

	commands["stats_command"] = &discordgo.ApplicationCommand{
		Name:        "stats",
		Description: "Displays bot stats",
	}

	return commands
}

func (p *Plugin) Intents() []discordgo.Intent {
	return nil
}

func (p *Plugin) AddStat(key string, value string) {
	p.functions.Set(key, func() string { return value })
}

func (p *Plugin) AddStatFunc(key string, fn func() string) {
	p.functions.Set(key, fn)
}
