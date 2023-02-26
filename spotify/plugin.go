package spotify

import (
	"context"
	"os"
	"path/filepath"
	"regexp"

	"github.com/eolso/discordgo"
	"github.com/eolso/threadsafe"
	"github.com/olympus-go/apollo"
	"github.com/olympus-go/apollo/spotify"
	"github.com/rs/zerolog"
)

const queryLimit = 10

var alphanumericRegex *regexp.Regexp

type Plugin struct {
	sessions     *threadsafe.Map[string, *session]
	callback     string
	clientId     string
	clientSecret string
	adminIds     []string
	logger       zerolog.Logger
}

// NewPlugin creates a new spotify.Plugin. If no logging is desired, a zerolog.Nop() should be supplied.
func NewPlugin(logger zerolog.Logger, callback string, clientId string, clientSecret string, adminIds ...string) *Plugin {
	plugin := Plugin{
		sessions:     threadsafe.NewMap[string, *session](),
		callback:     callback,
		clientId:     clientId,
		clientSecret: clientSecret,
		adminIds:     adminIds,
		logger:       logger.With().Str("plugin", "spotify").Logger(),
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
		Name:        "spotify",
		Description: "Spotify discord connector",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "play",
				Description: "Plays a specified song",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "query",
						Description: "Search query or spotify url",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
					{
						Name:        "remix",
						Description: "You want people to know you watch anime",
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
				Type: discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "queue",
				Description: "Shows the current song queue",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "join",
				Description: "Requests the bot to join your voice channel",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "leave",
				Description: "Requests the bot to leave the voice channel",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "resume",
				Description: "Resume playback",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "pause",
				Description: "Pause the currently playing song",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "next",
				Description: "Go to the next song",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "previous",
				Description: "Go back to the previous song",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "remove",
				Description: "Remove a song from queue",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "position",
						Description: "Queue position of the song to remove",
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    true,
					},
				},
			},
			{
				Name:        "login",
				Description: "Connect the bot to your spotify account",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "quiz",
				Description: "Start a spotify quiz game",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "playlist",
						Description: "Link to public playlist to use",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
					{
						Name:        "questions",
						Description: "Number of questions to play (default = 10)",
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

	return nil
}

func (p *Plugin) newSession(guildId string) *session {
	sessionConfig := spotify.DefaultSessionConfig()
	sessionConfig.ConfigHomeDir = filepath.Join(sessionConfig.ConfigHomeDir, guildId)
	sessionConfig.OAuthCallback = p.callback

	ctx, cancel := context.WithCancel(context.Background())

	spotSession := &session{
		session:          spotify.NewSession(sessionConfig),
		player:           apollo.NewPlayer(context.Background(), apollo.PlayerConfig{}, p.logger),
		playInteractions: threadsafe.NewMap[string, playInteraction](),
		framesProcessed:  0,
		voiceConnection:  nil,
		adminIds:         p.adminIds,
		ctx:              ctx,
		cancel:           cancel,
		logger:           p.logger,
	}

	return spotSession
}

func (p *Plugin) fileUploadHandlerInit() {
	err := os.MkdirAll("downloads", 0744)
	if err != nil {
		p.logger.Error().Err(err).Msg("failed to create downloads dir")
	}

	alphanumericRegex, err = regexp.Compile(`[^a-zA-Z0-9]+`)
	if err != nil {
		p.logger.Error().Err(err).Msg("failed to compile regex")
	}
}
