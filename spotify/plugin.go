package spotify

import (
	"os"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/eolso/threadsafe"
	"github.com/olympus-go/eris/utils"
	"github.com/rs/zerolog"
)

const queryLimit = 10

var alphanumericRegex *regexp.Regexp

type Plugin struct {
	sessions *threadsafe.Map[string, *session]
	config   *Config
	logger   zerolog.Logger
}

// NewPlugin creates a new spotify.Plugin. If no logging is desired, a zerolog.Nop() should be supplied.
func NewPlugin(config *Config, logger zerolog.Logger) *Plugin {
	plugin := Plugin{
		sessions: threadsafe.NewMap[string, *session](),
		config:   config,
		logger:   logger.With().Str("plugin", "spotify").Logger(),
	}

	plugin.fileUploadHandlerInit()

	return &plugin
}

func (p *Plugin) Name() string {
	return "Spotify"
}

func (p *Plugin) Description() string {
	return "Play spotify songs in voice chats"
}

func (p *Plugin) Handlers() map[string]any {
	handlers := make(map[string]any)

	handlers["spotify_handler"] = p.spotifyHandler
	handlers["spotify_file_upload_handler"] = p.fileUploadHandler

	return handlers
}

func (p *Plugin) Commands() map[string]*discordgo.ApplicationCommand {
	commands := make(map[string]*discordgo.ApplicationCommand)

	commands["spotify_cmd"] = &discordgo.ApplicationCommand{
		Name:        p.config.Alias,
		Description: p.config.Description,
		Type:        discordgo.ChatApplicationCommand,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        p.config.PlayCommand.Alias,
				Description: p.config.PlayCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        p.config.PlayCommand.QueryOption.Alias,
						Description: p.config.PlayCommand.QueryOption.Description,
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
					{
						Name:        p.config.PlayCommand.PositionOption.Alias,
						Description: p.config.PlayCommand.PositionOption.Description,
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    false,
						MinValue:    utils.PointerTo(1.0),
					},
					{
						Name:        p.config.PlayCommand.RemixOption.Alias,
						Description: p.config.PlayCommand.RemixOption.Description,
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    false,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{
								Name:  "nightcore",
								Value: nightcoreFrequency,
							},
							{
								Name:  "chopped and screwed",
								Value: choppedFrequency,
							},
						},
					},
				},
			},
			{
				Name:        p.config.QueueCommand.Alias,
				Description: p.config.QueueCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        p.config.JoinCommand.Alias,
				Description: p.config.JoinCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        p.config.LeaveCommand.Alias,
				Description: p.config.LeaveCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        p.config.LeaveCommand.KeepOption.Alias,
						Description: p.config.LeaveCommand.KeepOption.Description,
						Type:        discordgo.ApplicationCommandOptionBoolean,
						Required:    false,
					},
				},
			},
			{
				Name:        p.config.ResumeCommand.Alias,
				Description: p.config.ResumeCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        p.config.PauseCommand.Alias,
				Description: p.config.PauseCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        p.config.NextCommand.Alias,
				Description: p.config.NextCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        p.config.PreviousCommand.Alias,
				Description: p.config.PreviousCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        p.config.RemoveCommand.Alias,
				Description: p.config.RemoveCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        p.config.RemoveCommand.PositionOption.Alias,
						Description: p.config.RemoveCommand.PositionOption.Description,
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    true,
					},
				},
			},
			{
				Name:        p.config.LoginCommand.Alias,
				Description: p.config.LoginCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        p.config.QuizCommand.Alias,
				Description: p.config.QuizCommand.Description,
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        p.config.QuizCommand.PlaylistOption.Alias,
						Description: p.config.QuizCommand.PlaylistOption.Description,
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
					{
						Name:        p.config.QuizCommand.QuestionsOption.Alias,
						Description: p.config.QuizCommand.QuestionsOption.Description,
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    false,
					},
				},
			},
		},
	}

	return commands
}

func (p *Plugin) Intents() []discordgo.Intent {
	return []discordgo.Intent{
		discordgo.IntentsGuilds,
		discordgo.IntentsGuildMessages,
		discordgo.IntentsGuildVoiceStates,
		discordgo.IntentMessageContent,
	}
}

func (p *Plugin) fileUploadHandlerInit() {
	err := os.MkdirAll("downloads", 0744)
	if err != nil {
		p.logger.Error().Err(err).Msg("failed to create downloads dir")
	}

	alphanumericRegex, err = regexp.Compile(`[^a-zA-Z0-9 ]+`)
	if err != nil {
		p.logger.Error().Err(err).Msg("failed to compile regex")
	}
}
