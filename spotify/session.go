package spotify

import (
	"context"

	"github.com/eolso/discordgo"
	"github.com/eolso/threadsafe"
	"github.com/olympus-go/apollo"
	"github.com/olympus-go/apollo/spotify"
	"github.com/rs/zerolog"
)

const (
	nightcoreFrequency = 40000
	discordFrequency   = 48000
	choppedFrequency   = 56000
)

type track struct {
	spotify.Track
	metadata map[string]string
}

type playInteraction struct {
	trackIds   []string
	isPlaylist bool
	position   int
	frequency  int
}

type session struct {
	session          *spotify.Session
	player           *apollo.Player
	playInteractions *threadsafe.Map[string, playInteraction]
	quizGame         *quiz
	framesProcessed  int
	voiceConnection  *discordgo.VoiceConnection
	adminIds         []string
	logger           zerolog.Logger
	ctx              context.Context
	cancel           context.CancelFunc
}

func (s *session) start() {
	out := s.player.Out()
	for {
		select {
		case <-s.ctx.Done():
			return
		case b := <-out:
			if s.voiceConnection != nil {
				s.voiceConnection.OpusSend <- b
			}
		}
	}
}

func (s *session) stop() {
	s.cancel()
}

func (s *session) checkPermissions(p apollo.Playable, userId string) bool {
	if requesterId, ok := p.Metadata()["requesterId"]; ok {
		if requesterId == userId {
			return true
		}
	}

	for _, adminId := range s.adminIds {
		if userId == adminId {
			return true
		}
	}

	return false
}

func yesNoButtons(uid string, enabled bool) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Yes",
					Style:    discordgo.PrimaryButton,
					CustomID: "spotify_play_yes_" + uid,
					Disabled: !enabled,
				},
				discordgo.Button{
					Label:    "No",
					Style:    discordgo.SecondaryButton,
					CustomID: "spotify_play_no_" + uid,
					Disabled: !enabled,
				},
			},
		},
	}
}

func (t track) Metadata() map[string]string {
	return t.metadata
}
