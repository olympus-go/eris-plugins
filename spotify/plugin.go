package spotify

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/eolso/threadsafe"
)

const queryLimit = 10
const sessionTimeout = 5 * time.Minute

var alphanumericRegex *regexp.Regexp

type Plugin struct {
	sessions *threadsafe.Map[string, *session]
	config   *Config
	logger   *slog.Logger
}

// NewPlugin creates a new spotify.Plugin. If no logging is desired, a zerolog.Nop() should be supplied.
func NewPlugin(config *Config, h slog.Handler) *Plugin {
	plugin := Plugin{
		sessions: threadsafe.NewMap[string, *session](),
		config:   config,
		logger:   slog.New(h).With(slog.String("plugin", "spotify")),
	}

	plugin.fileUploadHandlerInit()
	_ = plugin.pruneSessions(context.Background())

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
			p.playCommand(),
			p.queueCommand(),
			p.joinCommand(),
			p.leaveCommand(),
			p.resumeCommand(),
			p.pauseCommand(),
			p.nextCommand(),
			p.previousCommand(),
			p.removeCommand(),
			p.loginCommand(),
			p.quizCommand(),
			p.listifyCommand(),
			p.clearCommand(),
			p.shuffleCommand(),
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
		p.logger.Error("failed to create downloads dir",
			slog.String("error", err.Error()),
		)
	}

	alphanumericRegex, err = regexp.Compile(`[^a-zA-Z0-9 ]+`)
	if err != nil {
		p.logger.Error("failed to compile regex",
			slog.String("error", err.Error()),
		)
	}
}

func (p *Plugin) pruneSessions(ctx context.Context) context.CancelFunc {
	c, cancel := context.WithCancel(ctx)

	go func() {
		for {
			select {
			case <-c.Done():
				return
			case <-time.Tick(1 * time.Minute):
				k, v := p.sessions.Items()
				for i := 0; i < len(k); i++ {
					if v[i].voiceConnection == nil {
						if v[i].timeLastJoined.Before(time.Now().Add(-1 * sessionTimeout)) {
							p.sessions.Delete(k[i])
						}
					}
				}
			}
		}
	}()

	return cancel
}
