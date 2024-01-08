package logger

import (
	"context"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/olympus-go/eris/utils"
)

func (p *Plugin) loggerHandler(_ *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		p.logger.Log(context.Background(), p.level, "user used slash command",
			slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
			slog.Any("user", utils.GetInteractionUser(i.Interaction)),
		)
	case discordgo.InteractionMessageComponent:
		p.logger.Log(context.Background(), p.level, "user interacted with message component",
			slog.Any("message_component", utils.MessageComponentInterface(i.MessageComponentData())),
			slog.Any("user", utils.GetInteractionUser(i.Interaction)),
		)
	}
}
