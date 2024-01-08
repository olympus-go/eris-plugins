package poll

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/olympus-go/eris/utils"
)

const maxOptions = 5

func (p *Plugin) pollHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		command := i.ApplicationCommandData()
		if command.Name != "poll" {
			return
		}

		p.pollApplicationCmdHandler(discordSession, i)
	case discordgo.InteractionMessageComponent:
		switch {
		case utils.IsInteractionMessageComponent(i, "startsWith", "poll"):
			p.pollMessageComponentHandler(discordSession, i)
		}
	}
}

func (p *Plugin) pollApplicationCmdHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	promptOption := utils.GetCommandOption(i.ApplicationCommandData(), "poll", "prompt")
	if promptOption == nil {
		p.logger.Error("required field not set", slog.String("field", "prompt"))
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			SendWithLog(p.logger)
		return
	}

	prompt := promptOption.StringValue()

	if _, ok := p.polls.Get(utils.ShaSum(prompt)); ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Poll already exists.").
			SendWithLog(p.logger)
		return
	}

	optionsOption := utils.GetCommandOption(i.ApplicationCommandData(), "poll", "options")
	if optionsOption == nil {
		p.logger.Error("required field not set", slog.String("field", "options"))
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			SendWithLog(p.logger)
		return
	}

	rawOptions := optionsOption.StringValue()
	matches := p.optionsRegex.FindAllString(rawOptions, -1)
	if len(matches) == 0 {
		p.logger.Error("invalid options supplied", slog.String("options", rawOptions))
		message := "Invalid options supplied. Surround each option with quotes. e.g. \"Option 1\" \"Option 2\""
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(message).
			SendWithLog(p.logger)
		return
	} else if len(matches) > maxOptions {
		p.logger.Error("too many options supplied", slog.String("options", rawOptions))
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(fmt.Sprintf("Too many options supplied. Please limit to %d or fewer.", maxOptions)).
			SendWithLog(p.logger)
		return
	}

	for index, _ := range matches {
		matches[index] = strings.Trim(matches[index], "\"")
	}

	anon := false
	anonymousOption := utils.GetCommandOption(i.ApplicationCommandData(), "poll", "anonymous")
	if anonymousOption != nil {
		anon = anonymousOption.BoolValue()
	}

	ttl := int64(86400)
	ttlOption := utils.GetCommandOption(i.ApplicationCommandData(), "poll", "duration")
	if ttlOption != nil {
		ttl = ttlOption.IntValue()
	}

	poll := NewPoll(prompt, anon, matches...)

	p.polls.Set(poll.uid, poll)

	utils.InteractionResponse(discordSession, i.Interaction).
		Components(pollButtons(poll, true)).
		Message(poll.String()).
		SendWithLog(p.logger)

	if ttl > 0 {
		go func() {
			time.Sleep(time.Second * time.Duration(ttl))
			utils.InteractionResponse(discordSession, i.Interaction).
				Components(pollButtons(poll, false)).
				Message(poll.String()).
				EditWithLog(p.logger)
			p.polls.Delete(poll.uid)
		}()
	}
}

func (p *Plugin) pollMessageComponentHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	customId := i.MessageComponentData().CustomID
	idSplit := strings.Split(customId, "_")
	if len(idSplit) != 3 {
		p.logger.Error("custom id did not match expected format",
			slog.String("custom_id", customId),
			slog.String("expected_format", "*_*_*"),
		)
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			SendWithLog(p.logger)
		return
	}

	pollId := idSplit[1]
	selectionStr := idSplit[2]

	selection, err := strconv.Atoi(selectionStr)
	if err != nil {
		p.logger.Error("failed to convert value to int",
			slog.String("error", err.Error()),
			slog.String("value", selectionStr),
		)
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			SendWithLog(p.logger)
		return
	}
	// Convert back to array notation
	selection--

	poll, ok := p.polls.Get(pollId)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Poll no longer exists.").
			SendWithLog(p.logger)
		return
	}

	if selection < 0 || selection >= len(poll.options) {
		p.logger.Error("selection was invalid for number of options",
			slog.Int("selection", selection),
			slog.Int("num_options", len(poll.options)),
		)

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Invalid selection for this poll.").
			SendWithLog(p.logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Type(discordgo.InteractionResponseDeferredMessageUpdate).
		SendWithLog(p.logger)

	poll.Vote(fmt.Sprintf("@%s", utils.GetInteractionUserName(i.Interaction)), selection)

	utils.InteractionResponse(discordSession, i.Interaction).
		Components(pollButtons(poll, true)).
		Message(poll.String()).
		EditWithLog(p.logger)
}

func pollButtons(poll *Poll, enabled bool) discordgo.ActionsRow {
	rowBuilder := utils.ActionsRow()

	for i, _ := range poll.options {
		button := utils.Button().Id(fmt.Sprintf("poll_%s_%d", poll.uid, i+1)).Label(fmt.Sprintf("%d", i+1)).Enabled(enabled)
		rowBuilder.Button(button.Build())
	}

	return rowBuilder.Build()
}
