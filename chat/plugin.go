package chat

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/eolso/threadsafe"
)

type Plugin struct {
	c       *http.Client
	threads *threadsafe.Map[string, SessionData]
	logger  *slog.Logger
}

type SessionData struct {
	Name       string
	Data       *GenerateData
	ExpireTime time.Time
}

func NewPlugin(h slog.Handler) *Plugin {
	p := Plugin{
		c:       &http.Client{Timeout: time.Second * 10},
		threads: threadsafe.NewMap[string, SessionData](),
		logger:  slog.New(h),
	}

	return &p
}

func (p *Plugin) Name() string {
	return "Chat"
}

func (p *Plugin) Description() string {
	return "Start chats with George"
}

func (p *Plugin) Handlers() map[string]any {
	handlers := make(map[string]any)

	handlers["chat_handler"] = p.chatHandler
	handlers["chat_message_handler"] = p.chatMessageHandler

	return handlers
}

func (p *Plugin) Commands() map[string]*discordgo.ApplicationCommand {
	commands := make(map[string]*discordgo.ApplicationCommand)

	commands["chat_command"] = &discordgo.ApplicationCommand{
		Name:        "chat",
		Description: "Start a chat with George.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "name",
				Description: "Make George think he has a different name.",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    false,
			},
			{
				Name:        "personality",
				Description: "Personality you want George to have.",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    false,
			},
		},
	}

	return commands
}

func (p *Plugin) Intents() []discordgo.Intent {
	return nil
}
