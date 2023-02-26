package spotify

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"github.com/eolso/discordgo"
	"github.com/eolso/threadsafe"
	"github.com/olympus-go/apollo"
	"github.com/olympus-go/apollo/spotify"
	"github.com/olympus-go/eris/utils"
)

const playQueryStr = "Is this your song?\n```Name: %s\nArtist: %s\n```%s"
const playlistQueryStr = "Play all of the `%s` playlist?\n%s"

func (p *Plugin) spotifyHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		command := i.ApplicationCommandData()
		if command.Name != "spotify" || len(command.Options) == 0 {
			return
		}

		if i.Interaction.GuildID == "" {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("I can't do that in a DM, sry.").
				SendWithLog(p.logger)
			return
		}

		switch command.Options[0].Name {
		case "join":
			p.joinHandler(discordSession, i)
		case "leave":
			p.leaveHandler(discordSession, i)
		case "play":
			p.playHandler(discordSession, i)
		case "queue":
			p.queueHandler(discordSession, i)
		case "resume":
			p.resumeHandler(discordSession, i)
		case "pause":
			p.pauseHandler(discordSession, i)
		case "next":
			p.nextHandler(discordSession, i)
		case "previous":
			p.previousHandler(discordSession, i)
		case "remove":
			p.removeHandler(discordSession, i)
		case "login":
			p.loginHandler(discordSession, i)
		case "quiz":
			p.quizHandler(discordSession, i)
		}
	case discordgo.InteractionMessageComponent:
		switch {
		case utils.IsInteractionMessageComponent(i, "startsWith", "spotify_playlist"):
			p.playlistMessageHandler(discordSession, i)
		case utils.IsInteractionMessageComponent(i, "startsWith", "spotify_play"):
			p.playMessageHandler(discordSession, i)
		case utils.IsInteractionMessageComponent(i, "startsWith", "spotify_login"):
			p.loginMessageHandler(discordSession, i)
		case utils.IsInteractionMessageComponent(i, "startsWith", "spotify_quiz"):
			p.quizMessageHandler(discordSession, i)
		}
	}
}

func (p *Plugin) joinHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	// If the session for the guild doesn't already exist, create it
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		logger.Debug().Str("guild_id", i.Interaction.GuildID).Msg("creating new spotify session for guild")
		spotSession = p.newSession(i.Interaction.GuildID)
		p.sessions.Set(i.Interaction.GuildID, spotSession)
	}

	voiceId := utils.GetInteractionUserVoiceStateId(discordSession, i.Interaction)

	if voiceId == "" {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("You're not in a voice channel bub.").
			SendWithLog(logger)
		return
	}

	if spotSession.voiceConnection != nil && spotSession.voiceConnection.ChannelID == voiceId {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I'm already here!").
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	if spotSession.voiceConnection != nil {
		spotSession.player.Pause()
		if err := spotSession.voiceConnection.Disconnect(); err != nil {
			logger.Error().Err(err).Msg("failed to disconnect from voice channel")
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Something went wrong.").
				EditWithLog(logger)
			return
		}
	}

	var err error
	spotSession.voiceConnection, err = discordSession.ChannelVoiceJoin(i.GuildID, voiceId, false, true)
	if err != nil {
		logger.Error().Err(err).Msg("failed to join voice channel")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}

	go spotSession.start()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(":tada:").
		EditWithLog(logger)
}

func (p *Plugin) leaveHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok || spotSession.voiceConnection == nil {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	spotSession.player.Empty()

	if spotSession.quizGame != nil {
		spotSession.quizGame.cancelFunc()
		spotSession.quizGame = nil
	}

	if err := spotSession.voiceConnection.Disconnect(); err != nil {
		logger.Error().Err(err).Msg("failed to disconnect from voice channel")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			SendWithLog(logger)
		return
	}

	spotSession.voiceConnection = nil
	spotSession.stop()

	p.sessions.Delete(i.Interaction.GuildID)

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(":wave:").
		SendWithLog(logger)
}

func (p *Plugin) playHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	if !spotSession.session.LoggedIn() {
		if err := spotSession.session.Login("georgetuney"); err != nil {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Login first before playing.\n`/spotify login`").
				SendWithLog(logger)
			return
		}
		p.logger = p.logger.With().Str("spotify_user", spotSession.session.Username()).Logger()
		logger = logger.With().Str("spotify_user", spotSession.session.Username()).Logger()
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	playOption := utils.GetCommandOption(i.ApplicationCommandData(), "spotify", "play")
	if playOption == nil {
		logger.Error().Str("expected", "spotify play [...]").Msg("unexpected command data found for command")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}

	queryOption := utils.GetCommandOption(*playOption, "play", "query")
	if queryOption == nil {
		logger.Error().Str("field", "query").Msg("required field not set")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}
	query := queryOption.StringValue()

	frequency := discordFrequency
	//remixOption := utils.GetCommandOption(playOption, "play", "remix")
	//if remixOption != nil {
	//	frequency = int(remixOption.IntValue())
	//}

	// Check if the query is a local file. If it exists, queue that, otherwise continue.
	userId := utils.GetInteractionUserId(i.Interaction)
	username := utils.GetInteractionUserName(i.Interaction)
	if localFile, err := p.getLocalFile(query, userId, username); err == nil {
		spotSession.player.Enqueue(&localFile)
		if spotSession.player.State() == apollo.IdleState {
			spotSession.player.Play()
		}

		message := fmt.Sprintf("local file %s added to queue.", localFile.Name())
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(message).
			EditWithLog(logger)

		return
	}

	// Generate a uid for tracking future interactions
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s%s%d",
		i.Interaction.GuildID,
		utils.GetInteractionUserId(i.Interaction),
		time.Now().UnixNano(),
	)))
	uid := fmt.Sprintf("%x", h.Sum(nil))

	// Check if the query is a link to a playlist. If it is, we'll send a special message for queueing the entire thing.
	uri, ok := spotify.ConvertLinkToUri(query)
	if ok && uri.Authority == spotify.PlaylistResourceType {
		playlists, err := spotSession.session.Search(query).Limit(1).Playlists()
		if err != nil || len(playlists) == 0 {
			logger.Error().Err(err).Msg("playlist search failed")
			utils.InteractionResponse(discordSession, i.Interaction).Ephemeral().
				Message("Something went wrong.").EditWithLog(logger)
			return
		}

		trackIds := playlists[0].TrackIds()

		message := fmt.Sprintf(playlistQueryStr, playlists[0].Name(), playlists[0].Image())
		yesButton := utils.Button().Label("Yes").Id("spotify_playlist_yes_" + uid).Build()
		noButton := utils.Button().Style(discordgo.SecondaryButton).Label("No").Id("spotify_playlist_no_" + uid).Build()
		shuffleButton := utils.Button().Label("Shuffle").Id("spotify_playlist_shuffle_" + uid).Build()

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Components(utils.ActionsRow().Button(yesButton).Button(noButton).Button(shuffleButton).Build()).
			Message(message).
			EditWithLog(logger)

		spotSession.playInteractions.Set(uid, playInteraction{trackIds, true, frequency})
		logger.Debug().Str("uid", uid).Msg("play interaction created")

		return
	}

	trackIds, err := spotSession.session.Search(query).Limit(queryLimit).TrackIds()
	if err != nil {
		logger.Error().Err(err).Msg("spotify search failed")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}

	if len(trackIds) == 0 {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("No tracks found.").
			EditWithLog(logger)
		return
	}

	initialTrack, err := spotSession.session.GetTrackById(trackIds[0])
	if err != nil {
		logger.Error().Err(err).Str("id", trackIds[0]).Msg("failed to retrieve track by id")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}

	message := fmt.Sprintf(playQueryStr, initialTrack.Name(), initialTrack.Artist(), initialTrack.Image())

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(message).
		Components(yesNoButtons(uid, true)...).
		EditWithLog(logger)

	spotSession.playInteractions.Set(uid, playInteraction{trackIds, false, frequency})
	logger.Debug().Str("uid", uid).Msg("play interaction created")

	go func() {
		time.Sleep(60 * time.Second)
		if _, ok = spotSession.playInteractions.Get(uid); ok {
			spotSession.playInteractions.Delete(uid)
			logger.Debug().Str("uid", uid).Msg("play interaction timed out")
		}
	}()
}

func (p *Plugin) playMessageHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Interface("message_component", utils.MessageComponentInterface(i.MessageComponentData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	// If the session for the guild doesn't already exist, create it
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	if !spotSession.session.LoggedIn() {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Login first before playing.\n`/spotify login`").
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	messageData := i.MessageComponentData()
	idSplit := strings.Split(messageData.CustomID, "_")
	if len(idSplit) != 4 {
		logger.Error().
			Str("custom_id", messageData.CustomID).
			Msg("messageData interaction response had an unknown custom Id")

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}

	action := idSplit[2]
	uid := idSplit[3]

	if _, ok = spotSession.playInteractions.Get(uid); !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("This song list is no longer available. Try searching again.").
			EditWithLog(logger)
		return
	}

	switch action {
	case "yes":
		interaction, ok := spotSession.playInteractions.Get(uid)
		if !ok || len(interaction.trackIds) == 0 {
			logger.Error().Str("uid", uid).Msg("tracks no longer exist for uid")
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Something went wrong.").
				FollowUpCreateWithLog(logger)
			return
		}

		spotTrack, err := spotSession.session.GetTrackById(interaction.trackIds[0])
		if err != nil {
			logger.Error().Err(err).Str("trackId", interaction.trackIds[0]).Msg("failed to get track by id")
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Something went wrong.").
				FollowUpCreateWithLog(logger)

			spotSession.playInteractions.Delete(uid)

			return
		}

		t := &track{
			Track: spotTrack,
			metadata: map[string]string{
				"requesterId":   utils.GetInteractionUserId(i.Interaction),
				"requesterName": utils.GetInteractionUserName(i.Interaction),
				"frequency":     fmt.Sprintf("%d", discordFrequency),
			},
		}

		spotSession.player.Enqueue(t)

		message := fmt.Sprintf("%s by %s added to queue.", t.Name(), t.Artist())
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(message).
			EditWithLog(logger)

		spotSession.playInteractions.Delete(uid)

		if spotSession.player.State() == apollo.IdleState {
			spotSession.player.Play()
		}
	case "no":
		interaction, ok := spotSession.playInteractions.Get(uid)
		if !ok || len(interaction.trackIds) == 0 {
			logger.Error().Str("uid", uid).Msg("tracks no longer exist for uid")
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Something went wrong.").
				EditWithLog(logger)
			return
		}

		logger.Debug().Interface("trackId", interaction.trackIds[0]).Msg("user responded no to track")

		interaction.trackIds = interaction.trackIds[1:]
		spotSession.playInteractions.Set(uid, interaction)

		if len(interaction.trackIds) == 0 {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("That's all of them! Try searching again.").
				EditWithLog(logger)
			spotSession.playInteractions.Delete(uid)
			return
		}

		t, err := spotSession.session.GetTrackById(interaction.trackIds[0])
		if err != nil {
			logger.Error().Err(err).Str("trackId", interaction.trackIds[0]).Msg("failed to get track by id")
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Something went wrong.").
				EditWithLog(logger)

			spotSession.playInteractions.Delete(uid)

			return
		}

		message := fmt.Sprintf(playQueryStr, t.Name(), t.Artist(), t.Image())

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(message).
			Components(yesNoButtons(uid, true)...).
			EditWithLog(logger)
	}
}

func (p *Plugin) playlistMessageHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Interface("message_component", utils.MessageComponentInterface(i.MessageComponentData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	// If the session for the guild doesn't already exist, create it
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	if !spotSession.session.LoggedIn() {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Login first before playing.\n`/spotify login`").
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	messageData := i.MessageComponentData()
	idSplit := strings.Split(messageData.CustomID, "_")
	if len(idSplit) != 4 {
		logger.Error().
			Str("custom_id", messageData.CustomID).
			Msg("messageData interaction response had an unknown custom Id")

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}

	action := idSplit[2]
	uid := idSplit[3]

	if _, ok = spotSession.playInteractions.Get(uid); !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("This song list is no longer available. Try searching again.").
			EditWithLog(logger)
		return
	}

	switch action {
	case "yes", "shuffle":
		interaction, ok := spotSession.playInteractions.Get(uid)
		if !ok || len(interaction.trackIds) == 0 {
			logger.Error().Str("uid", uid).Msg("tracks no longer exist for uid")
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Something went wrong.").
				EditWithLog(logger)
			return
		}

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Fasho. Gimme a sec to get all those added.").
			EditWithLog(logger)

		trackIds := interaction.trackIds
		if action == "shuffle" {
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			rng.Shuffle(len(trackIds), func(i, j int) {
				trackIds[i], trackIds[j] = trackIds[j], trackIds[i]
			})
		}

		for _, trackId := range trackIds {
			spotTrack, err := spotSession.session.GetTrackById(trackId)
			if err != nil {
				logger.Error().Err(err).Str("trackId", interaction.trackIds[0]).Msg("failed to get track by id")
				continue
			}

			t := &track{
				Track: spotTrack,
				metadata: map[string]string{
					"requesterId":   utils.GetInteractionUserId(i.Interaction),
					"requesterName": utils.GetInteractionUserName(i.Interaction),
					"frequency":     fmt.Sprintf("%d", discordFrequency),
				},
			}

			spotSession.player.Enqueue(t)
		}

		spotSession.playInteractions.Delete(uid)

		if spotSession.player.State() == apollo.IdleState {
			spotSession.player.Play()
		}

		logger.Debug().Msg("user enqueued playlist")

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Done :white_check_mark:").
			FollowUpCreateWithLog(logger)
	case "no":
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Nevermind then, gosh.").
			EditWithLog(logger)

		spotSession.playInteractions.Delete(uid)
	}
}

func (p *Plugin) queueHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	np, ok := spotSession.player.NowPlaying()
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("No songs in queue.").
			EditWithLog(logger)
		return
	}

	remix := ""
	//timeRatio := 1.0
	elapsedDuration := time.Duration(float64(spotSession.player.BytesSent()) * 20.0 * float64(time.Millisecond)).Round(time.Second)
	totalDuration := np.Duration().Round(time.Second)
	elapsedPercent := elapsedDuration.Seconds() / totalDuration.Seconds()

	message := "```"
	message += "Currently playing:\n"
	message += fmt.Sprintf("  %s%s - %s (@%s)\n", np.Name(), remix, np.Artist(), np.Metadata()["requesterName"])
	message += fmt.Sprintf("  <%s%s> [%s/%s]\n", strings.Repeat("\u2588", int(elapsedPercent*30)),
		strings.Repeat("\u2591", int(30-(elapsedPercent*30))), elapsedDuration.String(),
		totalDuration.String())

	tracks := spotSession.player.List(false)
	if len(tracks) >= 1 {
		message += "Up next:\n"
	}
	for index, t := range tracks {
		if index >= 25 {
			message += fmt.Sprintf("...(+ %d more)", len(tracks)-index+1)
			break
		} else {
			message += fmt.Sprintf("  %d) %s%s - %s (@%s)\n", index+1, t.Name(), remix, t.Artist(), t.Metadata()["requesterName"])
		}
	}
	message += "```"

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(message).
		EditWithLog(logger)
}

func (p *Plugin) resumeHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	if spotSession.player.State() == apollo.IdleState {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Nothing in queue.").
			SendWithLog(logger)
		return
	}

	spotSession.player.Play()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(":arrow_forward:").
		SendWithLog(logger)
}

func (p *Plugin) pauseHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	if spotSession.player.State() != apollo.PlayState {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Nothing is currently playing.").
			SendWithLog(logger)
		return
	}

	spotSession.player.Pause()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(":pause_button:").
		SendWithLog(logger)
}

func (p *Plugin) nextHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	t, ok := spotSession.player.NowPlaying()
	if spotSession.player.State() == apollo.IdleState || !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Nothing is currently playing.").
			SendWithLog(logger)
		return
	}

	logger.Debug().Str("track", t.Name()).Msg("user skipped track")
	spotSession.player.Next()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(":fast_forward:").
		SendWithLog(logger)
}

func (p *Plugin) previousHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	_, ok = spotSession.player.NowPlaying()
	if len(spotSession.player.List(true)) == 0 {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("No queue history.").
			SendWithLog(logger)
	}

	//userId := utils.GetInteractionUserId(i.Interaction)
	//if spotSession.player.State() != apollo.IdleState && !spotSession.checkPermissions(t, userId) {
	//	logger.Debug().
	//		Str("author_id", t.Metadata()["requesterId"]).
	//		Str("track", t.Name()).
	//		Msg("user tried to skip a track they don't own")
	//	utils.InteractionResponse(discordSession, i.Interaction).
	//		Ephemeral().
	//		Message("You don't have permissions to previous on this track.").
	//		SendWithLog(logger)
	//	return
	//}
	logger.Debug().Msg("player changed to previous track")
	spotSession.player.Previous()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(":rewind:").
		SendWithLog(logger)
}

func (p *Plugin) removeHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯").
			SendWithLog(logger)
		return
	}

	queue := spotSession.player.List(false)
	if len(queue) == 0 {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Nothing in queue to remove.").
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	removeOption := utils.GetCommandOption(i.ApplicationCommandData(), "spotify", "remove")
	if removeOption == nil {
		logger.Error().Str("expected", "spotify remove [...]").Msg("unexpected command data found for command")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}

	positionOption := utils.GetCommandOption(*removeOption, "remove", "position")
	if positionOption == nil {
		logger.Error().Str("field", "position").Msg("required field not set")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}

	position := int(positionOption.IntValue())
	if position <= 0 || position > len(queue) {
		logger.Error().Int("position", position).Msg("invalid position value")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Invalid position value.").
			EditWithLog(logger)
		return
	}

	if !spotSession.checkPermissions(queue[position-1], utils.GetInteractionUserId(i.Interaction)) {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("You don't have permission to skip that track.").
			EditWithLog(logger)
		return
	}

	spotSession.player.Remove(position)

	logger.Debug().Interface("position", position).Msg("user removed track")

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(":gun:").
		EditWithLog(logger)
}

func (p *Plugin) loginHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	// If the session for the guild doesn't already exist, create it.
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		spotSession = p.newSession(i.Interaction.GuildID)
		p.sessions.Set(i.Interaction.GuildID, spotSession)
	}

	if spotSession.session.LoggedIn() {
		yesButton := utils.Button().Label("Yes").Id("spotify_login_yes").Build()
		noButton := utils.Button().Style(discordgo.SecondaryButton).Label("No").Id("spotify_login_no").Build()
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Components(utils.ActionsRow().Button(yesButton).Button(noButton).Build()).
			Message("Spotify session is already logged in. Log out now?").
			SendWithLog(logger)
		return
	}

	if err := spotSession.session.Login("georgetuney"); err == nil {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Login successful :tada:").
			SendWithLog(logger)
		p.logger = p.logger.With().Str("spotify_user", spotSession.session.Username()).Logger()
		return
	}

	url := spotify.StartLocalOAuth(p.clientId, p.clientSecret, p.callback)

	linkButton := utils.Button().Style(discordgo.LinkButton).Label("Login").URL(url).Build()
	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Components(utils.ActionsRow().Button(linkButton).Build()).
		Message("Click here to login!").
		SendWithLog(logger)

	go func() {
		token := spotify.GetOAuthToken()
		if err := spotSession.session.LoginWithToken("georgetuney", token); err != nil {
			logger.Error().Msg("spotify login failed")

			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Login failed :(").
				FollowUpCreateWithLog(logger)
		} else {
			logger.Debug().
				Str("spotify_user", spotSession.session.Username()).
				Msg("spotify login succeeded")
			p.logger = p.logger.With().Str("spotify_user", spotSession.session.Username()).Logger()

			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Login successful :tada:").
				FollowUpCreateWithLog(logger)
		}
	}()
}

func (p *Plugin) loginMessageHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Interface("message_component", utils.MessageComponentInterface(i.MessageComponentData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	// If the session for the guild doesn't already exist, create it.
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		spotSession = p.newSession(i.Interaction.GuildID)
		p.sessions.Set(i.Interaction.GuildID, spotSession)
	}

	messageData := i.MessageComponentData()
	idSplit := strings.Split(messageData.CustomID, "_")
	if len(idSplit) != 3 {
		logger.Error().
			Str("custom_id", messageData.CustomID).
			Msg("message interaction response had an unknown custom Id")

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			SendWithLog(logger)
		return
	}

	action := idSplit[2]

	if action == "yes" {
		url := spotify.StartLocalOAuth(p.clientId, p.clientSecret, p.callback)

		linkButton := utils.Button().Style(discordgo.LinkButton).Label("Login").URL(url).Build()
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Components(utils.ActionsRow().Button(linkButton).Build()).
			Message("Click here to login!").
			SendWithLog(logger)

		go func() {
			token := spotify.GetOAuthToken()
			if err := spotSession.session.LoginWithToken("georgetuney", token); err != nil {
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message("Login failed :(").
					FollowUpCreateWithLog(logger)
			} else {
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message("Login successful :tada:").
					FollowUpCreateWithLog(logger)
				p.logger = p.logger.With().Str("spotify_user", spotSession.session.Username()).Logger()
			}
		}()
	} else if action == "no" {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(":+1:").
			SendWithLog(logger)
	}
}

func (p *Plugin) quizHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Str("command", utils.CommandDataString(i.ApplicationCommandData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	// If the session for the guild doesn't already exist, create it.
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		spotSession = p.newSession(i.Interaction.GuildID)
		p.sessions.Set(i.Interaction.GuildID, spotSession)
	}

	if !spotSession.session.LoggedIn() {
		if err := spotSession.session.Login("georgetuney"); err != nil {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Login first before playing.\n`/spotify login`").
				SendWithLog(logger)
			return
		}
	}

	if spotSession.quizGame != nil {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Game is already running!").
			SendWithLog(logger)
		return
	}

	if spotSession.voiceConnection == nil {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Summon me into a voice channel before starting.").
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Deferred().
		SendWithLog(logger)

	quizOption := utils.GetCommandOption(i.ApplicationCommandData(), "spotify", "quiz")
	if quizOption == nil {
		logger.Error().Str("expected", "spotify quiz [...]").Msg("unexpected command data found for command")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Something went wrong.").
			EditWithLog(logger)
		return
	}

	// Set defaults and try and fetch options
	playlist := ""
	questions := 10
	for _, option := range quizOption.Options {
		switch option.Name {
		case "playlist":
			playlist, _ = option.Value.(string)
		case "questions":
			v, _ := option.Value.(float64)
			questions = int(v)
		default:
			logger.Error().Str("unknown_option", option.Name).Msg("interaction received unknown option")
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Something went wrong.").
				EditWithLog(logger)
			return
		}
	}

	results, err := spotSession.session.Search(playlist).Limit(1).Playlists()
	if err != nil || len(results) == 0 {
		logger.Error().Err(err).Msg("failed to search playlist")
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("I had trouble finding that playlist :/").
			EditWithLog(logger)
		return
	}

	trackIds := results[0].TrackIds()
	if len(trackIds) < 5 {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Playlist too small for a quiz game.").
			EditWithLog(logger)
		return
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	quizGame := &quiz{
		playlist:          trackIds,
		questionNumber:    1,
		previousQuestions: threadsafe.NewMap[int, bool](),

		questions: questions,
		scoreboard: threadsafe.NewMap[string, struct {
			score     int
			totalTime float64
		}](),
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		cancelFunc: cancelFunc,
	}

	spotSession.quizGame = quizGame

	userId := utils.GetInteractionUserId(i.Interaction)
	quizGame.startMessage = fmt.Sprintf("<@%s> started a spotify quiz game! Click the button to join.\n", userId)
	quizGame.startMessage += fmt.Sprintf("Category: `%s`", results[0].Name())
	buttonBuilder := utils.Button().Id("spotify_quiz_join").Label("Join game")
	components := utils.ActionsRow().Button(buttonBuilder.Build()).Build()

	utils.InteractionResponse(discordSession, i.Interaction).
		Components(components).
		Message(quizGame.startMessage).
		EditWithLog(logger)

	quizGame.startInteraction = i.Interaction

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
			break
		}

		// Close out the join option
		message := fmt.Sprintf("%s\n```", spotSession.quizGame.startMessage)
		for _, user := range spotSession.quizGame.scoreboard.Keys() {
			message += fmt.Sprintf("%s joined.\n", user)
		}
		message += "```"
		components = utils.ActionsRow().Button(buttonBuilder.Enabled(false).Build()).Build()
		utils.InteractionResponse(discordSession, i.Interaction).
			Message(message).
			Components(components).
			EditWithLog(logger)

		if len(quizGame.scoreboard.Data) == 0 {
			utils.InteractionResponse(discordSession, i.Interaction).
				Message("No one joined :disappointed:").
				FollowUpCreateWithLog(logger)
			spotSession.quizGame = nil
			return
		}

		for index := 0; index < quizGame.questions; index++ {
			if spotSession.quizGame == nil {
				return
			} else if len(quizGame.playlist) < 5 {
				break
			}

			quizGame.questionAnswer = quizGame.rng.Intn(5)
			tracks := quizGame.getRandomTracks(spotSession.session, 5)
			quizGame.questionAnswerTrack = tracks[quizGame.questionAnswer]

			trackIndex := slices.Index(quizGame.playlist, quizGame.questionAnswerTrack.Id())
			if trackIndex != -1 {
				quizGame.playlist = slices.Delete(quizGame.playlist, trackIndex, trackIndex+1)
			}

			quizGame.questionResponseTimes = threadsafe.NewMap[string, float64]()
			for _, username := range quizGame.scoreboard.Keys() {
				quizGame.questionResponseTimes.Set(username, 16.0)
			}

			t := track{
				Track: tracks[quizGame.questionAnswer],
				metadata: map[string]string{
					"requesterId":   "george",
					"requesterName": "george",
					"frequency":     fmt.Sprintf("%d", discordFrequency),
				},
			}
			spotSession.player.Enqueue(&t)
			spotSession.player.Play()
			quizGame.questionStartTime = time.Now()
			questionMessage, err := utils.InteractionResponse(discordSession, i.Interaction).
				Response(quizGame.generateQuestion(tracks)).
				FollowUpCreate()
			if err != nil {
				logger.Error().Err(err).Msg("failed to create followup")
			}

			go func() {
				select {
				case <-ctx.Done():
					return
				case <-time.After(15 * time.Second):
					break
				}

				spotSession.player.Next()

				utils.InteractionResponse(discordSession, i.Interaction).
					Components().
					FollowUpEditWithLog(questionMessage.ID, logger)

				utils.InteractionResponse(discordSession, i.Interaction).
					Message(quizGame.generateQuestionWinner()).
					FollowUpCreateWithLog(logger)
			}()

			select {
			case <-ctx.Done():
				return
			case <-time.After(18 * time.Second):
				break
			}

			quizGame.questionNumber++
		}

		utils.InteractionResponse(discordSession, i.Interaction).
			Message(quizGame.generateGameWinner()).
			FollowUpCreateWithLog(logger)

		spotSession.quizGame = nil
	}()
}

func (p *Plugin) quizMessageHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With().
		Interface("message_component", utils.MessageComponentInterface(i.MessageComponentData())).
		Interface("user", utils.GetInteractionUser(i.Interaction)).
		Logger()

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok || spotSession.quizGame == nil {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("Game no longer exists.").SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	messageData := i.MessageComponentData()
	switch {
	case strings.HasPrefix(messageData.CustomID, "spotify_quiz_join"):
		username := utils.GetInteractionUserName(i.Interaction)

		if _, ok = spotSession.quizGame.scoreboard.Get(username); ok {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("gl;hf").
				EditWithLog(logger)
			return
		}

		spotSession.quizGame.scoreboard.Set("@"+username, struct {
			score     int
			totalTime float64
		}{score: 0, totalTime: 0.0})

		message := fmt.Sprintf("%s\n```", spotSession.quizGame.startMessage)
		for _, user := range spotSession.quizGame.scoreboard.Keys() {
			message += fmt.Sprintf("%s joined.\n", user)
		}
		message += "```"

		buttonBuilder := utils.Button().Id("spotify_quiz_join").Label("Join game")
		components := utils.ActionsRow().Button(buttonBuilder.Build()).Build()
		utils.InteractionResponse(discordSession, spotSession.quizGame.startInteraction).
			Components(components).
			Message(message).EditWithLog(logger)

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message("gl;hf").
			EditWithLog(logger)

	case strings.HasPrefix(messageData.CustomID, "spotify_quiz_answer"):
		username := fmt.Sprintf("@%s", utils.GetInteractionUserName(i.Interaction))

		if _, ok = spotSession.quizGame.scoreboard.Get(username); !ok {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("You aren't a part of this round.").
				EditWithLog(logger)
			return
		}

		if responseTime, _ := spotSession.quizGame.questionResponseTimes.Get(username); responseTime != 16.0 {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("You already selected an answer for this round.").
				EditWithLog(logger)
			return
		}

		idSplit := strings.Split(messageData.CustomID, "_")
		if len(idSplit) != 4 {
			logger.Error().Msg("message interaction response had an unknown custom Id")
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Something went wrong.").
				EditWithLog(logger)
			return
		}

		answer, err := strconv.Atoi(idSplit[3])
		if err != nil {
			logger.Error().Err(err).Str("id", idSplit[3]).Msg("failed to convert id to int")
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Something went wrong.").
				EditWithLog(logger)
			return

		}

		timeElapsed := time.Since(spotSession.quizGame.questionStartTime).Round(time.Millisecond).Seconds()
		if answer-1 == spotSession.quizGame.questionAnswer {
			spotSession.quizGame.questionResponseTimes.Set(username, timeElapsed)

			score, _ := spotSession.quizGame.scoreboard.Get(username)
			score.totalTime += timeElapsed
			spotSession.quizGame.scoreboard.Set(username, score)
		} else {
			spotSession.quizGame.questionResponseTimes.Set(username, 15.0)

			score, _ := spotSession.quizGame.scoreboard.Get(username)
			score.totalTime += 15.0
			spotSession.quizGame.scoreboard.Set(username, score)
		}

		message := fmt.Sprintf("You answered in: %.3fs :stopwatch:", timeElapsed)
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(message).
			EditWithLog(logger)
	}
}

func (p *Plugin) fileUploadHandler(discordSession *discordgo.Session, message *discordgo.MessageCreate) {
	if message == nil || message.Author == nil {
		return
	}

	uploadTriggerWords := []string{
		"here",
		"take",
		"have",
		//"foryou",
		"download",
		"save",
		"hold",
		"keep",
		"cp",
	}

	listTriggerWords := []string{
		"list",
		"show",
		"see",
		"peep",
		"peek",
		"view",
		"glance",
		"eye",
		//"workingwit",
		"ls",
	}

	lowercaseContent := strings.ToLower(message.Content)
	contentWords := strings.Fields(lowercaseContent)

	// Upload check
	if len(message.Attachments) > 0 && len(p.adminIds) > 0 {
		for _, word := range uploadTriggerWords {
			if slices.Contains(contentWords, "george") && slices.Contains(contentWords, word) {
				//if strings.Contains(squishedContent, "george") && strings.Contains(squishedContent, word) {
				if !slices.Contains(p.adminIds, message.Author.ID) {
					m := fmt.Sprintf("<@%s> told me not to accept candy from strangers", p.adminIds[0])
					_, _ = discordSession.ChannelMessageSend(message.ChannelID, m)
					break
				}

				for _, attachment := range message.Attachments {
					path := filepath.Join("downloads", attachment.Filename)
					out, err := os.Create(path)
					if err != nil {
						p.logger.Error().Err(err).Str("path", path).Msg("failed to create file")
						return
					}

					resp, err := http.Get(attachment.URL)
					if err != nil {
						p.logger.Error().Err(err).Str("url", attachment.URL).Msg("failed to download file")
						return
					}

					if _, err = io.Copy(out, resp.Body); err != nil {
						p.logger.Error().Err(err).Str("path", path).Msg("failed to copy file")
					}

					_ = out.Close()
					_ = resp.Body.Close()
				}

				_, _ = discordSession.ChannelMessageSend(message.ChannelID, "omnomnomnom delicioso :yum:")

				return
			}
		}
	}

	// List Check
	for _, word := range listTriggerWords {
		if slices.Contains(contentWords, "george") && slices.Contains(contentWords, word) {
			//if strings.Contains(squishedContent, "george") && strings.Contains(squishedContent, word) {
			entries, _ := os.ReadDir("downloads/")
			if len(entries) == 0 {
				_, _ = discordSession.ChannelMessageSend(message.ChannelID, "I don't have anything :pleading_face:")
				return
			}

			_, _ = discordSession.ChannelMessageSend(message.ChannelID, "I have:")
			m := "```\n"
			for _, entry := range entries {
				m += entry.Name() + "\n"
			}
			m += "```"
			_, _ = discordSession.ChannelMessageSend(message.ChannelID, m)

			return
		}
	}

	// Rename check
	if strings.HasPrefix(lowercaseContent, "george rename") || strings.HasPrefix(lowercaseContent, "george mv") {
		splitContent := strings.Split(message.Content, " ")
		if len(splitContent) != 4 {
			p.logger.Debug().Str("content", message.Content).Msg("rename message was not formatted correctly")
			return
		}

		filename := splitContent[2]
		newName := splitContent[3]

		entries, err := os.ReadDir("downloads/")
		if err != nil {
			p.logger.Error().Err(err).Msg("failed to read downloads directory")
			return
		}
		for _, entry := range entries {
			if entry.Name() == filename {
				from := strings.TrimSpace(filepath.Join("downloads/", filename))
				to := strings.TrimSpace(filepath.Join("downloads/", newName))

				if err = os.Rename(from, to); err != nil {
					p.logger.Error().Str("from", from).Str("to", to).Err(err).Msg("failed to rename file")
					_, _ = discordSession.ChannelMessageSend(message.ChannelID, "I done goofed.")
					return
				}

				_, _ = discordSession.ChannelMessageSend(message.ChannelID, "Done :sunglasses:")

				return
			}
		}

		_, _ = discordSession.ChannelMessageSend(message.ChannelID, "I don't think I have that file.")
	}

	// Remove check
	if strings.HasPrefix(lowercaseContent, "george remove") || strings.HasPrefix(lowercaseContent, "george rm") {
		splitContent := strings.Split(message.Content, " ")
		if len(splitContent) != 3 {
			p.logger.Debug().Str("content", message.Content).Msg("remove message was not formatted correctly")
			return
		}

		filename := splitContent[2]

		entries, err := os.ReadDir("downloads/")
		if err != nil {
			p.logger.Error().Err(err).Msg("failed to read downloads directory")
			return
		}
		for _, entry := range entries {
			if entry.Name() == filename {
				path := strings.TrimSpace(filepath.Join("downloads/", filename))

				if err = os.Remove(path); err != nil {
					p.logger.Error().Str("path", path).Err(err).Msg("failed to remove file")
					_, _ = discordSession.ChannelMessageSend(message.ChannelID, "I done goofed.")
					return
				}

				_, _ = discordSession.ChannelMessageSend(message.ChannelID, "Done :sunglasses:")

				return
			}
		}

		_, _ = discordSession.ChannelMessageSend(message.ChannelID, "I don't think I have that file.")
	}
}

func (p *Plugin) getLocalFile(name string, userId string, username string) (apollo.LocalFile, error) {
	var localFile apollo.LocalFile

	entries, err := os.ReadDir("downloads/")
	if err != nil {
		return localFile, err
	}

	for _, entry := range entries {
		if entry.Name() == name {
			localFile, err = apollo.NewLocalFile(filepath.Join("downloads", entry.Name()))
			localFile.Mdata = map[string]string{
				"requesterId":   userId,
				"requesterName": username,
				"frequency":     fmt.Sprintf("%d", discordFrequency),
			}

			return localFile, err
		}
	}

	return apollo.LocalFile{}, fmt.Errorf("no local file found")
}
