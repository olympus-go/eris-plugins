package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/olympus-go/eris/utils"
)

func (p *Plugin) chatHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if i.ApplicationCommandData().Name == "chat" {
			// Start a thread to use for this chat history.
			ch, err := discordSession.ThreadStart(i.ChannelID,
				fmt.Sprintf("Chat%d", time.Now().UnixNano()),
				discordgo.ChannelTypeGuildPublicThread,
				60,
			)
			if err != nil {
				p.logger.Error("failed to start thread", slog.String("error", err.Error()))
				return
			}

			// Load in some sane defaults and the custom personality if provided.
			data := DefaultGenerateData()
			if personalityOption := utils.GetCommandOption(i.ApplicationCommandData(), "chat", "personality"); personalityOption != nil {
				personalityString := personalityOption.StringValue()
				if personalityString != "" {
					if !strings.HasPrefix(strings.ToLower(personalityString), "personality:") {
						personalityString = "Personality: " + personalityString
					}

					data.Memory = personalityString
				}
			}

			sessionData := SessionData{
				Name:       "George",
				ExpireTime: time.Now().Add(time.Minute * 60),
			}

			if nameOption := utils.GetCommandOption(i.ApplicationCommandData(), "chat", "name"); nameOption != nil {
				if nameOption.StringValue() != "" {
					sessionData.Name = nameOption.StringValue()
				}
			}

			// Update the stop sequences to include the character name + the user who created this thread.
			username := utils.GetInteractionUserName(i.Interaction)
			data.StopSequence[0] = fmt.Sprintf("\n%s: ", sessionData.Name)
			data.appendStopSequence(username)

			sessionData.Data = &data
			p.threads.Set(ch.ID, sessionData)

			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(":white_check_mark:").
				SendWithLog(p.logger)
		}
	}
}

func (p *Plugin) chatMessageHandler(discordSession *discordgo.Session, m *discordgo.MessageCreate) {
	sessionData, ok := p.threads.Get(m.ChannelID)
	if !ok || m.Message == nil || m.Message.Author.Bot {
		return
	}

	if err := discordSession.ChannelTyping(m.ChannelID); err != nil {
		p.logger.Error("failed to broadcast typing event", slog.String("error", err.Error()))
		return
	}

	// Check if the user is a new unique user, and if so add them to the stop sequence.
	if !sessionData.Data.checkStopSequence(m.Message.Author.Username) {
		sessionData.Data.appendStopSequence(m.Message.Author.Username)
	}

	sessionData.Data.Prompt += fmt.Sprintf("\n%s: %s \n%s:", m.Message.Author.Username, m.Message.Content, sessionData.Name)

	b, _ := json.Marshal(sessionData.Data)

	resp, err := p.c.Post("http://192.168.0.69:5001/api/v1/generate", "application/json", bytes.NewReader(b))
	if err != nil {
		discordSession.ChannelMessageSendReply(m.ChannelID, "I don't want to talk now >.<", m.Reference())
		p.logger.Error("failed to post to chat endpoint", slog.String("error", err.Error()))
		return
	}

	var respData ResponseData
	if err = json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		panic(err)
	}

	cleanResp := respData.clean(m.Message.Author.Username+":", "</s>")
	//cleanResp := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(respData.Results[0].Text), m.Message.Author.Username+":"))

	if _, err = discordSession.ChannelMessageSendReply(m.ChannelID, cleanResp, m.Reference()); err != nil {
		p.logger.Error("failed to send reply", slog.String("error", err.Error()))
		return
	}

	sessionData.Data.Prompt += cleanResp

	p.threads.Set(m.ChannelID, sessionData)
}
