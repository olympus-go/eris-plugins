package logger

import (
	"github.com/eolso/discordgo"
	"github.com/olympus-go/eris/utils"
)

func (p *Plugin) loggerHandler(_ *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		p.logger.WithLevel(p.level).
			Str("command", utils.CommandDataString(i.ApplicationCommandData())).
			Interface("user", utils.GetInteractionUser(i.Interaction)).
			Msg("user used slash command")
	case discordgo.InteractionMessageComponent:
		p.logger.WithLevel(p.level).
			Interface("message_component", utils.MessageComponentInterface(i.MessageComponentData())).
			Interface("user", utils.GetInteractionUser(i.Interaction)).
			Msg("user interacted with message component")
	}
}
