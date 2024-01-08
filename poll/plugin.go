package poll

import (
	"log/slog"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/eolso/threadsafe"
)

type Plugin struct {
	polls        *threadsafe.Map[string, *Poll]
	optionsRegex *regexp.Regexp
	logger       *slog.Logger
	//logger       zerolog.Logger
}

func NewPlugin(h slog.Handler) *Plugin {
	return &Plugin{
		polls:        threadsafe.NewMap[string, *Poll](),
		optionsRegex: regexp.MustCompile(`"[^""]*"`),
		logger:       slog.New(h),
	}
}

func (p *Plugin) Name() string {
	return "Poll"
}

func (p *Plugin) Description() string {
	return "Enables polls"
}

func (p *Plugin) Handlers() map[string]any {
	handlers := make(map[string]any)

	handlers["poll_handler"] = p.pollHandler

	return handlers
}

func (p *Plugin) Commands() map[string]*discordgo.ApplicationCommand {
	commands := make(map[string]*discordgo.ApplicationCommand)
	negativeOne := -1.0

	commands["poll_cmd"] = &discordgo.ApplicationCommand{
		Name:        "poll",
		Description: "Start a poll",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "prompt",
				Description: "The prompt of your poll.",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
			{
				Name:        "options",
				Description: "The options of your poll. Surround each option with quotes. e.g. \"Option 1\" \"Option 2\"",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
			{
				Name:        "anonymous",
				Description: "Mark the poll as anonymous.",
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Required:    false,
			},
			{
				Name:        "duration",
				Description: "Duration to keep the poll open (in seconds). Use -1 for indefinitely. Defaults to 86400.",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    false,
				MinValue:    &negativeOne,
			},
		},
	}

	return commands
}

func (p *Plugin) Intents() []discordgo.Intent {
	return nil
}
