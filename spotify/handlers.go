package spotify

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/eolso/threadsafe"
	"github.com/olympus-go/apollo"
	"github.com/olympus-go/apollo/spotify"
	"github.com/olympus-go/eris/utils"
	"golang.org/x/exp/slices"
)

const playQueryStr = "%s\n```Name: %s\nArtist: %s\n```%s"
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
				Message(p.config.GlobalResponses.GenericError).
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
		case "listify":
			p.listifyHandler(discordSession, i)
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
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	// If the session for the guild doesn't already exist, create it
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		logger.Debug("creating new spotify session for guild", slog.String("guild_id", i.Interaction.GuildID))

		sessionConfig := spotify.DefaultSessionConfig()
		sessionConfig.ConfigHomeDir = filepath.Join(sessionConfig.ConfigHomeDir, i.Interaction.GuildID)
		sessionConfig.OAuthCallback = p.config.SpotifyCallbackUrl
		spotSession = newSession(sessionConfig, p.logger.Handler(), p.config.AdminIds...)
		p.sessions.Set(i.Interaction.GuildID, spotSession)
	}

	voiceId := utils.GetInteractionUserVoiceStateId(discordSession, i.Interaction)

	if voiceId == "" {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	if spotSession.voiceConnection != nil && spotSession.voiceConnection.ChannelID == voiceId {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.JoinCommand.Responses.AlreadyJoined).
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	if spotSession.voiceConnection != nil {
		spotSession.player.Pause()
		spotSession.stop()
		if err := spotSession.voiceConnection.Disconnect(); err != nil {
			logger.Error("failed to disconnect from voice channel", slog.String("error", err.Error()))
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
				EditWithLog(logger)
			return
		}
	}

	// Attempt to connect to the voice channel. Sometimes this will timeout after joining, so retry a few times if that
	// happens.
	var err error
	for retries := 0; retries < 3; retries++ {
		spotSession.voiceConnection, err = discordSession.ChannelVoiceJoin(i.GuildID, voiceId, false, true)
		if err == nil {
			break
		}
		_ = spotSession.voiceConnection.Disconnect()
	}
	if err != nil {
		logger.Error("failed to join voice channel", slog.String("error", err.Error()))
		_ = spotSession.voiceConnection.Disconnect()
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			EditWithLog(logger)
		return
	}

	go spotSession.start()
	spotSession.player.Play()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(p.config.JoinCommand.Responses.JoinSuccess).
		EditWithLog(logger)
}

func (p *Plugin) leaveHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	leaveOption := utils.GetCommandOption(i.ApplicationCommandData(), "spotify", "leave")
	if leaveOption == nil {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			SendWithLog(logger)
		return
	}

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok || spotSession.voiceConnection == nil {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	spotSession.player.Pause()

	if spotSession.quizGame != nil {
		spotSession.quizGame.cancelFunc()
		spotSession.quizGame = nil
	}

	if err := spotSession.voiceConnection.Disconnect(); err != nil {
		logger.Error("failed to disconnect from voice channel", slog.String("error", err.Error()))
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			SendWithLog(logger)
		return
	}

	keep := false
	if keepOption := utils.GetCommandOption(*leaveOption, "leave", "keep"); keepOption != nil {
		keep = keepOption.BoolValue()
	}

	// TODO make this configurable + better
	// Save session history somewhere
	//for _, playable := range spotSession.player.List(true) {
	//os.Open("")
	//playable.Name()
	//}

	spotSession.voiceConnection = nil
	if !keep {
		spotSession.player.Empty()
		spotSession.stop()
		p.sessions.Delete(i.Interaction.GuildID)
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(p.config.LeaveCommand.Responses.LeaveSuccess).
		SendWithLog(logger)
}

func (p *Plugin) playHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	if !spotSession.session.LoggedIn() {
		if err := spotSession.session.Login("georgetuney"); err != nil {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.NotLoggedIn).
				SendWithLog(logger)
			return
		}

		// Update base logger and this logger to include logged in user
		// TODO prolly move this logic elsewhere
		p.logger = p.logger.With(slog.String("spotify_user", spotSession.session.Username()))
		logger = logger.With(slog.String("spotify_user", spotSession.session.Username()))
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	playOption := utils.GetCommandOption(i.ApplicationCommandData(), "spotify", "play")
	if playOption == nil {
		logger.Error("unexpected command data found for command",
			slog.String("expected", "spotify play [...]"),
		)
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			EditWithLog(logger)
		return
	}

	queryOption := utils.GetCommandOption(*playOption, "play", "query")
	if queryOption == nil {
		slog.Error("required field not set", slog.String("field", "query"))
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			EditWithLog(logger)
		return
	}
	query := queryOption.StringValue()

	position := -1
	positionOption := utils.GetCommandOption(*playOption, "play", "position")
	if positionOption != nil {
		position = int(positionOption.IntValue())
	}

	frequency := discordFrequency
	//remixOption := utils.GetCommandOption(playOption, "play", "remix")
	//if remixOption != nil {
	//	frequency = int(remixOption.IntValue())
	//}

	// Check if the query is a local file. If it exists, queue that, otherwise continue.
	userId := utils.GetInteractionUserId(i.Interaction)
	username := utils.GetInteractionUserName(i.Interaction)
	if localFile, err := p.getLocalFile(query, userId, username); err == nil {
		if position != -1 {
			spotSession.player.Insert(spotSession.player.Cursor()+position-1, &localFile)
		} else {
			spotSession.player.Enqueue(&localFile)
		}
		if spotSession.player.State() == apollo.IdleState {
			spotSession.player.Play()
		}

		message := fmt.Sprintf("%s by %s added to queue.", localFile.Name(), localFile.Artist())
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(message).
			EditWithLog(logger)

		return
	}

	// Generate an uid for tracking future interactions
	uid := utils.ShaSum(fmt.Sprintf("%s%s%d",
		i.Interaction.GuildID,
		utils.GetInteractionUserId(i.Interaction),
		time.Now().UnixNano(),
	))

	// Check if the query is a link to a playlist. If it is, we'll send a special message for queueing the entire thing.
	uri, ok := spotify.ConvertLinkToUri(query)
	if ok && uri.Authority == spotify.PlaylistResourceType {
		playlists, err := spotSession.session.Search(query).Limit(1).Playlists()
		if err != nil || len(playlists) == 0 {
			logger.Error("playlist search failed", slog.String("error", err.Error()))
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
				EditWithLog(logger)
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

		spotSession.playInteractions.Set(uid, playInteraction{
			trackIds:     trackIds,
			playlistName: playlists[0].Name(),
			frequency:    frequency,
		})
		logger.Debug("play interaction created", slog.String("uid", uid))

		return
	}

	trackIds, err := spotSession.session.Search(query).Limit(queryLimit).TrackIds()
	if err != nil {
		logger.Error("spotify search failed", slog.String("error", err.Error()))
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			EditWithLog(logger)
		return
	}

	if len(trackIds) == 0 {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.PlayCommand.Responses.NoTracksFound).
			EditWithLog(logger)
		return
	}

	initialTrack, err := spotSession.session.GetTrackById(trackIds[0])
	if err != nil {
		logger.Error("failed to retrieve track by id",
			slog.String("error", err.Error()),
			slog.String("id", trackIds[0]),
		)
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			EditWithLog(logger)
		return
	}

	message := fmt.Sprintf(playQueryStr,
		p.config.PlayCommand.Responses.SongPrompt, initialTrack.Name(), initialTrack.Artist(), initialTrack.Image())

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(message).
		Components(yesNoButtons(uid, true)...).
		EditWithLog(logger)

	spotSession.playInteractions.Set(uid, playInteraction{
		trackIds:  trackIds,
		position:  position,
		frequency: frequency,
	})
	logger.Debug("play interaction created", slog.String("uid", uid))

	go func() {
		time.Sleep(60 * time.Second)
		if _, ok = spotSession.playInteractions.Get(uid); ok {
			utils.InteractionResponse(discordSession, i.Interaction).DeleteWithLog(logger)
			spotSession.playInteractions.Delete(uid)
			logger.Debug("play interaction timed out", slog.String("uid", uid))
		}
	}()
}

func (p *Plugin) playMessageHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.Any("message_component", utils.MessageComponentInterface(i.MessageComponentData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	// If the session for the guild doesn't already exist, create it
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	if !spotSession.session.LoggedIn() {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotLoggedIn).
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		DeferredUpdate().
		SendWithLog(logger)

	messageData := i.MessageComponentData()
	idSplit := strings.Split(messageData.CustomID, "_")
	if len(idSplit) != 4 {
		logger.Error("message component data interaction response had an unknown custom ID",
			slog.String("custom_id", messageData.CustomID),
		)

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			EditWithLog(logger)
		return
	}

	action := idSplit[2]
	uid := idSplit[3]

	if _, ok = spotSession.playInteractions.Get(uid); !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.PlayCommand.Responses.ListNotAvailable).
			EditWithLog(logger)
		return
	}

	switch action {
	case "yes":
		interaction, ok := spotSession.playInteractions.Get(uid)
		if !ok || len(interaction.trackIds) == 0 {
			logger.Error("tracks no longer exist for uid", slog.String("uid", uid))
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
				FollowUpCreateWithLog(logger)
			return
		}

		spotTrack, err := spotSession.session.GetTrackById(interaction.trackIds[0])
		if err != nil {
			logger.Error("failed to get track by id",
				slog.String("error", err.Error()),
				slog.String("trackId", interaction.trackIds[0]),
			)
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
				FollowUpCreateWithLog(logger)

			spotSession.playInteractions.Delete(uid)

			return
		}

		if slices.Contains(p.config.BannedTracks, spotTrack.Id()) {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.PlayCommand.Responses.BannedTrack).
				EditWithLog(logger)

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

		if interaction.position != -1 {
			spotSession.player.Insert(spotSession.player.Cursor()+interaction.position-1, t)
		} else {
			spotSession.player.Enqueue(t)
		}

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
			logger.Error("tracks no longer exist for uid", slog.String("uid", uid))
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
				EditWithLog(logger)
			return
		}

		logger.Debug("user responded no to track", slog.Any("track", interaction.trackIds[0]))

		interaction.trackIds = interaction.trackIds[1:]
		spotSession.playInteractions.Set(uid, interaction)

		if len(interaction.trackIds) == 0 {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.PlayCommand.Responses.EndOfList).
				EditWithLog(logger)
			spotSession.playInteractions.Delete(uid)
			return
		}

		t, err := spotSession.session.GetTrackById(interaction.trackIds[0])
		if err != nil {
			logger.Error("failed to get track by id",
				slog.String("error", err.Error()),
				slog.String("trackId", interaction.trackIds[0]),
			)
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
				EditWithLog(logger)

			spotSession.playInteractions.Delete(uid)

			return
		}

		message := fmt.Sprintf(playQueryStr, p.config.PlayCommand.Responses.SongPrompt, t.Name(), t.Artist(), t.Image())

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(message).
			Components(yesNoButtons(uid, true)...).
			EditWithLog(logger)
	}
}

func (p *Plugin) playlistMessageHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.Any("message_component", utils.MessageComponentInterface(i.MessageComponentData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	// If the session for the guild doesn't already exist, create it
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	if !spotSession.session.LoggedIn() {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotLoggedIn).
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		DeferredUpdate().
		SendWithLog(logger)

	messageData := i.MessageComponentData()
	idSplit := strings.Split(messageData.CustomID, "_")
	if len(idSplit) != 4 {
		logger.Error("message component data interaction response had an unknown custom ID",
			slog.String("custom_id", messageData.CustomID),
		)

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			EditWithLog(logger)
		return
	}

	action := idSplit[2]
	uid := idSplit[3]

	if _, ok = spotSession.playInteractions.Get(uid); !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.PlayCommand.Responses.ListNotAvailable).
			EditWithLog(logger)
		return
	}

	switch action {
	case "yes", "shuffle":
		interaction, ok := spotSession.playInteractions.Get(uid)
		if !ok || len(interaction.trackIds) == 0 {
			logger.Error("tracks no longer exist for uid", slog.String("uid", uid))
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
				EditWithLog(logger)
			return
		}

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.PlayCommand.Responses.LoadingPlaylist).
			EditWithLog(logger)

		trackIds := interaction.trackIds
		if action == "shuffle" {
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			rng.Shuffle(len(trackIds), func(i, j int) {
				trackIds[i], trackIds[j] = trackIds[j], trackIds[i]
			})
		}

		for _, trackId := range trackIds {
			if slices.Contains(p.config.BannedTracks, trackId) {
				continue
			}

			spotTrack, err := spotSession.session.GetTrackById(trackId)
			if err != nil {
				logger.Error("failed to get track by id",
					slog.String("error", err.Error()),
					slog.String("trackId", interaction.trackIds[0]),
				)
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

		logger.Debug("user enqueued playlist")

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(fmt.Sprintf("Playlist `%s` added to queue.", interaction.playlistName)).
			EditWithLog(logger)
	case "no":
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.PlayCommand.Responses.EndOfList).
			EditWithLog(logger)

		spotSession.playInteractions.Delete(uid)
	}
}

func (p *Plugin) queueHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
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
			Message(p.config.QueueCommand.Responses.EmptyQueue).
			EditWithLog(logger)
		return
	}

	remix := ""
	//timeRatio := 1.0
	elapsedDuration := time.Duration(float64(spotSession.player.BytesSent()) * 20.0 * float64(time.Millisecond)).Round(time.Second)
	totalDuration := np.Duration().Round(time.Second)
	elapsedPercent := elapsedDuration.Seconds() / totalDuration.Seconds()
	if elapsedPercent > 1 {
		elapsedPercent = 1
	}

	message := "```"
	message += "Currently playing:\n"
	message += fmt.Sprintf("  %s%s - %s (@%s)\n", np.Name(), remix, np.Artist(), np.Metadata()["requesterName"])
	message += fmt.Sprintf("  <%s%s> [%s/%s]\n",
		strings.Repeat("\u2588", int(elapsedPercent*30)),
		strings.Repeat("\u2591", int(30-(elapsedPercent*30))),
		elapsedDuration.String(),
		totalDuration.String(),
	)

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
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	if spotSession.player.State() == apollo.IdleState {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.EmptyQueue).
			SendWithLog(logger)
		return
	}

	spotSession.player.Play()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(p.config.ResumeCommand.Responses.ResumeSuccess).
		SendWithLog(logger)
}

func (p *Plugin) pauseHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	if spotSession.player.State() != apollo.PlayState {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.EmptyQueue).
			SendWithLog(logger)
		return
	}

	spotSession.player.Pause()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(p.config.PauseCommand.Responses.PauseSuccess).
		SendWithLog(logger)
}

func (p *Plugin) nextHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	t, ok := spotSession.player.NowPlaying()
	if spotSession.player.State() == apollo.IdleState || !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.EmptyQueue).
			SendWithLog(logger)
		return
	}

	userId := utils.GetInteractionUserId(i.Interaction)
	if strings.ToLower(p.config.RestrictSkips) == "true" && !spotSession.checkPermissions(t, userId) {
		logger.Debug("user tried to skip a track they don't own",
			slog.String("author_id", t.Metadata()["requesterId"]),
			slog.String("track", t.Name()),
		)
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.PermissionDenied).
			SendWithLog(logger)
		return
	}

	logger.Debug("user skipped track", slog.String("track", t.Name()))
	spotSession.player.Next()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(p.config.NextCommand.Responses.NextSuccess).
		SendWithLog(logger)
}

func (p *Plugin) previousHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	if len(spotSession.player.List(true)) == 0 {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.PreviousCommand.Responses.EmptyQueue).
			SendWithLog(logger)
		return
	}

	// If something is currently playing, verify permissions if applicable.
	if t, ok := spotSession.player.NowPlaying(); ok {
		userId := utils.GetInteractionUserId(i.Interaction)
		if p.config.RestrictSkips == "true" && !spotSession.checkPermissions(t, userId) {
			logger.Debug("user tried to skip a track they don't own",
				slog.String("author_id", t.Metadata()["requesterId"]),
				slog.String("track", t.Name()),
			)
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.PermissionDenied).
				SendWithLog(logger)
			return
		}
	}

	logger.Debug("player changed to previous track")
	spotSession.player.Previous()

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(p.config.PreviousCommand.Responses.PreviousSuccess).
		SendWithLog(logger)
}

func (p *Plugin) removeHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	queue := spotSession.player.List(false)
	if len(queue) == 0 {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.EmptyQueue).
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	removeOption := utils.GetCommandOption(i.ApplicationCommandData(), "spotify", "remove")
	if removeOption == nil {

		logger.Error("unexpected command data found for command",
			slog.String("expected", "spotify remove [...]"),
		)
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			EditWithLog(logger)
		return
	}

	positionOption := utils.GetCommandOption(*removeOption, "remove", "position")
	if positionOption == nil {
		logger.Error("required field not set", slog.String("field", "position"))
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			EditWithLog(logger)
		return
	}

	position := int(positionOption.IntValue())
	if position <= 0 || position > len(queue) {
		logger.Error("invalid position value", slog.Int("position", position))
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.RemoveCommand.Responses.InvalidPosition).
			EditWithLog(logger)
		return
	}

	if !spotSession.checkPermissions(queue[position-1], utils.GetInteractionUserId(i.Interaction)) {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.PermissionDenied).
			EditWithLog(logger)
		return
	}

	removed := spotSession.player.Get(spotSession.player.Cursor() + position - 1)
	spotSession.player.Remove(spotSession.player.Cursor() + position - 1)
	logger.Debug("user removed track",
		slog.Int("position", position),
		slog.Any("title", removed.Name()),
	)

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(p.config.RemoveCommand.Responses.RemoveSuccess).
		EditWithLog(logger)
}

func (p *Plugin) loginHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	// If the session for the guild doesn't already exist, create it.
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		sessionConfig := spotify.DefaultSessionConfig()
		sessionConfig.ConfigHomeDir = filepath.Join(sessionConfig.ConfigHomeDir, i.Interaction.GuildID)
		sessionConfig.OAuthCallback = p.config.SpotifyCallbackUrl
		spotSession = newSession(sessionConfig, p.logger.Handler(), p.config.AdminIds...)
		p.sessions.Set(i.Interaction.GuildID, spotSession)
	}

	if spotSession.session.LoggedIn() {
		yesButton := utils.Button().Label("Yes").Id("spotify_login_yes").Build()
		noButton := utils.Button().Style(discordgo.SecondaryButton).Label("No").Id("spotify_login_no").Build()
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Components(utils.ActionsRow().Button(yesButton).Button(noButton).Build()).
			Message(p.config.LoginCommand.Responses.AlreadyLoggedIn).
			SendWithLog(logger)
		return
	}

	if err := spotSession.session.Login("georgetuney"); err == nil {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.LoginCommand.Responses.LoginSuccess).
			SendWithLog(logger)

		p.logger = p.logger.With(slog.String("spotify_user", spotSession.session.Username()))
		return
	}

	url := spotify.StartLocalOAuth(p.config.SpotifyClientId, p.config.SpotifyClientSecret, p.config.SpotifyCallbackUrl)

	linkButton := utils.Button().Style(discordgo.LinkButton).Label("Login").URL(url).Build()
	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Components(utils.ActionsRow().Button(linkButton).Build()).
		Message(p.config.LoginCommand.Responses.LoginPrompt).
		SendWithLog(logger)

	go func() {
		token := spotify.GetOAuthToken()
		if err := spotSession.session.LoginWithToken("georgetuney", token); err != nil {
			logger.Error("spotify login failed", slog.String("error", err.Error()))

			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.LoginCommand.Responses.LoginFail).
				FollowUpCreateWithLog(logger)
		} else {
			logger.Info("spotify login succeeded", slog.String("spotify_user", spotSession.session.Username()))

			p.logger = p.logger.With(slog.String("spotify_user", spotSession.session.Username()))

			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.LoginCommand.Responses.LoginSuccess).
				FollowUpCreateWithLog(logger)
		}
	}()
}

func (p *Plugin) loginMessageHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.Any("message_component", utils.MessageComponentInterface(i.MessageComponentData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	// If the session for the guild doesn't already exist, create it.
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		sessionConfig := spotify.DefaultSessionConfig()
		sessionConfig.ConfigHomeDir = filepath.Join(sessionConfig.ConfigHomeDir, i.Interaction.GuildID)
		sessionConfig.OAuthCallback = p.config.SpotifyCallbackUrl
		spotSession = newSession(sessionConfig, p.logger.Handler(), p.config.AdminIds...)
		p.sessions.Set(i.Interaction.GuildID, spotSession)
	}

	messageData := i.MessageComponentData()
	idSplit := strings.Split(messageData.CustomID, "_")
	if len(idSplit) != 3 {
		logger.Error("message component data interaction response had an unknown custom ID",
			slog.String("custom_id", messageData.CustomID),
		)

		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
			SendWithLog(logger)
		return
	}

	action := idSplit[2]

	if action == "yes" {
		url := spotify.StartLocalOAuth(p.config.SpotifyClientId, p.config.SpotifyClientSecret, p.config.SpotifyCallbackUrl)

		linkButton := utils.Button().Style(discordgo.LinkButton).Label("Login").URL(url).Build()
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Components(utils.ActionsRow().Button(linkButton).Build()).
			Message(p.config.LoginCommand.Responses.LoginPrompt).
			SendWithLog(logger)

		go func() {
			token := spotify.GetOAuthToken()
			if err := spotSession.session.LoginWithToken("georgetuney", token); err != nil {
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message(p.config.LoginCommand.Responses.LoginFail).
					FollowUpCreateWithLog(logger)
			} else {
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message(p.config.LoginCommand.Responses.LoginSuccess).
					FollowUpCreateWithLog(logger)
				p.logger = p.logger.With(slog.String("spotify_user", spotSession.session.Username()))
			}
		}()
	} else if action == "no" {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.LoginCommand.Responses.LoginCancel).
			SendWithLog(logger)
	}
}

func (p *Plugin) quizHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	// If the session for the guild doesn't already exist, create it.
	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		sessionConfig := spotify.DefaultSessionConfig()
		sessionConfig.ConfigHomeDir = filepath.Join(sessionConfig.ConfigHomeDir, i.Interaction.GuildID)
		sessionConfig.OAuthCallback = p.config.SpotifyCallbackUrl
		spotSession = newSession(sessionConfig, p.logger.Handler(), p.config.AdminIds...)
		p.sessions.Set(i.Interaction.GuildID, spotSession)
	}

	if !spotSession.session.LoggedIn() {
		if err := spotSession.session.Login("georgetuney"); err != nil {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.NotLoggedIn).
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
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	utils.InteractionResponse(discordSession, i.Interaction).
		Deferred().
		SendWithLog(logger)

	quizOption := utils.GetCommandOption(i.ApplicationCommandData(), "spotify", "quiz")
	if quizOption == nil {
		logger.Error("unexpected command data found for command",
			slog.String("expected", "spotify quiz [...]"),
		)
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.GenericError).
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
			logger.Error("interaction received unknown option", slog.String("option", option.Name))
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
				EditWithLog(logger)
			return
		}
	}

	results, err := spotSession.session.Search(playlist).Limit(1).Playlists()
	if err != nil || len(results) == 0 {
		logger.Error("failed to search playlist", slog.String("error", err.Error()))
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
				logger.Error("failed to create followup", slog.String("error", err.Error()))
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
	logger := p.logger.With(
		slog.Any("message_component", utils.MessageComponentInterface(i.MessageComponentData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

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
			logger.Error("message component data interaction response had an unknown custom ID",
				slog.String("custom_id", messageData.CustomID),
			)
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
				EditWithLog(logger)
			return
		}

		answer, err := strconv.Atoi(idSplit[3])
		if err != nil {
			logger.Error("failed to convert id to int",
				slog.String("error", err.Error()),
				slog.String("id", idSplit[3]),
			)

			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(p.config.GlobalResponses.GenericError).
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

func (p *Plugin) listifyHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	logger := p.logger.With(
		slog.String("command", utils.CommandDataString(i.ApplicationCommandData())),
		slog.Any("user", utils.GetInteractionUser(i.Interaction)),
	)

	spotSession, ok := p.sessions.Get(i.Interaction.GuildID)
	if !ok {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.NotInVoice).
			SendWithLog(logger)
		return
	}

	queue := spotSession.player.List(true)
	if len(queue) == 0 {
		utils.InteractionResponse(discordSession, i.Interaction).
			Ephemeral().
			Message(p.config.GlobalResponses.EmptyQueue).
			SendWithLog(logger)
		return
	}

	// Send a deferred message that way we can follow up repeatedly
	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Deferred().
		SendWithLog(logger)

	message := "```\n"
	for index := range queue {
		line := fmt.Sprintf("%d) %s - %s (@%s)\n", index+1, queue[index].Name(), queue[index].Artist(), queue[index].Metadata()["requesterName"])

		// Cut off slightly early before the line limit, just so we never risk not being able to send
		if len(message)+len(line)+3 >= 1995 {
			message += "```"
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(message).
				FollowUpCreateWithLog(logger)

			message = "```\n"
		}

		message += line
	}
	message += "```"

	utils.InteractionResponse(discordSession, i.Interaction).
		Ephemeral().
		Message(message).
		FollowUpCreateWithLog(logger)
}

func (p *Plugin) fileUploadHandler(discordSession *discordgo.Session, message *discordgo.MessageCreate) {
	if message == nil || message.Author == nil {
		return
	}

	uploadTriggerWords := []string{
		"here",
		"take",
		"have",
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
		"ls",
	}

	lowercaseContent := strings.ToLower(alphanumericRegex.ReplaceAllString(message.Content, ""))
	contentWords := strings.Fields(lowercaseContent)

	// Upload check
	if len(message.Attachments) > 0 && len(p.config.AdminIds) > 0 {
		for _, word := range uploadTriggerWords {
			if slices.Contains(contentWords, "george") && slices.Contains(contentWords, word) {
				//if strings.Contains(squishedContent, "george") && strings.Contains(squishedContent, word) {
				if !slices.Contains(p.config.AdminIds, message.Author.ID) {
					m := fmt.Sprintf("<@%s> told me not to accept candy from strangers", p.config.AdminIds[0])
					_, _ = discordSession.ChannelMessageSend(message.ChannelID, m)
					break
				}

				for _, attachment := range message.Attachments {
					path := filepath.Join("downloads", attachment.Filename)
					out, err := os.Create(path)
					if err != nil {
						p.logger.Error("failed to create file",
							slog.String("error", err.Error()),
							slog.String("path", path),
						)
						return
					}
					defer out.Close()

					resp, err := http.Get(attachment.URL)
					if err != nil {
						p.logger.Error("failed to download file",
							slog.String("error", err.Error()),
							slog.String("url", attachment.URL),
						)
						return
					}
					defer resp.Body.Close()

					if _, err = io.Copy(out, resp.Body); err != nil {
						p.logger.Error("failed to copy file",
							slog.String("error", err.Error()),
							slog.String("path", path),
						)
					}
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
			p.logger.Error("rename message was not formatted correctly", slog.String("content", message.Content))
			return
		}

		filename := splitContent[2]
		newName := splitContent[3]

		entries, err := os.ReadDir("downloads/")
		if err != nil {
			p.logger.Error("failed to read downloads directory", slog.String("error", err.Error()))
			return
		}
		for _, entry := range entries {
			if entry.Name() == filename {
				from := strings.TrimSpace(filepath.Join("downloads/", filename))
				to := strings.TrimSpace(filepath.Join("downloads/", newName))

				if err = os.Rename(from, to); err != nil {
					p.logger.Error("failed to rename file",
						slog.String("error", err.Error()),
						slog.String("from", from),
						slog.String("to", to),
					)
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
			p.logger.Error("remove message was not formatted correctly", slog.String("content", message.Content))
			return
		}

		filename := splitContent[2]

		entries, err := os.ReadDir("downloads/")
		if err != nil {
			p.logger.Error("failed to read downloads directory", slog.String("error", err.Error()))
			return
		}
		for _, entry := range entries {
			if entry.Name() == filename {
				path := strings.TrimSpace(filepath.Join("downloads/", filename))

				if err = os.Remove(path); err != nil {
					p.logger.Error("failed to remove file",
						slog.String("error", err.Error()),
						slog.String("path", path),
					)
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
		if entry.Name() == name && !entry.IsDir() {
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
