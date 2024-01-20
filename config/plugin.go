package config

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/eolso/threadsafe"
)

// Plugin stores configs from other plugins. The commands for this plugin are dynamically generated based on what is
// added to it at the time of initialization, so this plugin should be added last.
type Plugin struct {
	configs  *threadsafe.Map[string, *configReloader]
	adminIds []string
	logger   *slog.Logger
	//logger   zerolog.Logger
}

type configReloader struct {
	config any
	fn     func()
}

// NewPlugin creates a new config plugin. A list of adminIds can be specified to limit set operations to a specific
// set of user ids. If none are supplied set calls will be left unrestricted.
func NewPlugin(h slog.Handler, adminIds ...string) *Plugin {
	return &Plugin{
		adminIds: adminIds,
		configs:  threadsafe.NewMap[string, *configReloader](),
		logger:   slog.New(h),
	}
}

func (p *Plugin) Name() string {
	return "Config"
}

func (p *Plugin) Description() string {
	return "Helps manage configs for all your plugins."
}

func (p *Plugin) Handlers() map[string]any {
	handlers := make(map[string]any)

	handlers["config_handler"] = p.configHandler

	return handlers
}

func (p *Plugin) Commands() map[string]*discordgo.ApplicationCommand {
	commands := make(map[string]*discordgo.ApplicationCommand)

	var options []*discordgo.ApplicationCommandOption

	keys := p.configs.Keys()
	for i := 0; i < len(keys); i++ {
		options = append(options, ConfigApplicationCommandOptions(keys[i])...)
	}

	commands["config_cmd"] = &discordgo.ApplicationCommand{
		Name:        "config",
		Description: "Config plugin",
		Options:     options,
	}

	return commands
}

func (p *Plugin) Intents() []discordgo.Intent {
	return nil
}

func (p *Plugin) AddConfig(name string, config any, fn func()) {
	p.configs.Set(name, &configReloader{
		config: config,
		fn:     fn,
	})
}

// ConfigApplicationCommandOptions is a shortcut to generating a slice of *discordgo.ApplicationCommandOption used
// for getting and setting config options. Plugins can leverage this to quickly generate get & set sub-commands.
func ConfigApplicationCommandOptions(name string) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Name:        name,
			Description: fmt.Sprintf("Get or update a config %s settings", name),
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "get",
					Description: fmt.Sprintf("Print out the current %s config", name),
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "key",
							Description: "The key of the config setting. An empty value here will return the entire file.",
							Type:        discordgo.ApplicationCommandOptionString,
							Required:    false,
						},
					},
				},
				{
					Name:        "set",
					Description: fmt.Sprintf("Update a %s config setting", name),
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:        "key",
							Description: "The key of the config setting",
							Type:        discordgo.ApplicationCommandOptionString,
							Required:    true,
						},
						{
							Name:        "value",
							Description: "The new desired value",
							Type:        discordgo.ApplicationCommandOptionString,
							Required:    true,
						},
					},
				},
			},
		},
	}
}
