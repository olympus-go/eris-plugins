package spotify

import (
	"github.com/bwmarrin/discordgo"
	"github.com/olympus-go/eris/utils"
)

func (p *Plugin) playCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
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
	}
}

func (p *Plugin) queueCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.QueueCommand.Alias,
		Description: p.config.QueueCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) joinCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.JoinCommand.Alias,
		Description: p.config.JoinCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) leaveCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.LeaveCommand.Alias,
		Description: p.config.LeaveCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) resumeCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.ResumeCommand.Alias,
		Description: p.config.ResumeCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) pauseCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.PauseCommand.Alias,
		Description: p.config.PauseCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) nextCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.NextCommand.Alias,
		Description: p.config.NextCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) previousCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.PreviousCommand.Alias,
		Description: p.config.PreviousCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) removeCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
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
	}
}

func (p *Plugin) loginCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.LoginCommand.Alias,
		Description: p.config.LoginCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) quizCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
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
	}
}

func (p *Plugin) listifyCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.ListifyCommand.Alias,
		Description: p.config.ListifyCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) clearCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.ClearCommand.Alias,
		Description: p.config.ClearCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}

func (p *Plugin) shuffleCommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Name:        p.config.ShuffleCommand.Alias,
		Description: p.config.ShuffleCommand.Description,
		Type:        discordgo.ApplicationCommandOptionSubCommand,
	}
}
