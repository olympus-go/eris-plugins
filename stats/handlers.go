package stats

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/olympus-go/eris/utils"
)

func (p *Plugin) statsHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if i.ApplicationCommandData().Name == "stats" {
			message := "```"
			for key, fn := range p.functions.Data {
				message += fmt.Sprintf("%s: %s\n", key, fn())
			}
			message += "```"
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(message).
				SendWithLog(p.logger)
		}
	}
}
