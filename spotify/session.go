package spotify

import (
	"context"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/eolso/threadsafe"
	"github.com/olympus-go/apollo"
	"github.com/olympus-go/apollo/ffmpeg"
	"github.com/olympus-go/apollo/ffmpeg/formats"
	"github.com/olympus-go/apollo/ogg"
	"github.com/olympus-go/apollo/spotify"
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
	trackIds []string
	// playlistName is set when a playlist was sent. "" == not a playlist.
	playlistName string
	position     int
	frequency    int
}

type session struct {
	session          *spotify.Session
	player           *apollo.Player
	playInteractions *threadsafe.Map[string, playInteraction]
	quizGame         *quiz
	voiceConnection  *discordgo.VoiceConnection
	adminIds         []string
	cancel           context.CancelFunc
}

func newSession(sessionConfig spotify.SessionConfig, h slog.Handler, adminIds ...string) *session {
	opts := ffmpeg.Options{
		Decoder:          nil,
		Encoder:          formats.DiscordOpusFormat(),
		Input:            ffmpeg.Stdin,
		Output:           ffmpeg.Stdout,
		Channels:         "2",
		FrameRate:        "48000",
		Bitrate:          "64000",
		CompressionLevel: "10",
	}

	codec := ffmpeg.New(opts).WithCodec(&ogg.Decoder{})
	playerConfig := apollo.PlayerConfig{PacketBuffer: ogg.MaxPageSize}
	player := apollo.NewPlayer(playerConfig, h).WithCodec(codec)

	return &session{
		session:          spotify.NewSession(sessionConfig, h),
		player:           player,
		playInteractions: threadsafe.NewMap[string, playInteraction](),
		voiceConnection:  nil,
		adminIds:         adminIds,
	}
}

func (s *session) start() {
	var ctx context.Context
	ctx, s.cancel = context.WithCancel(context.Background())
	out := s.player.Out()

	// Wait until the voice channel becomes available
	for s.voiceConnection == nil {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Pray that it didn't become unavailable in that instant
	voiceSend := s.voiceConnection.OpusSend

	for {
		select {
		case <-ctx.Done():
			return
		case b := <-out:
			select {
			case <-ctx.Done():
				return
			case voiceSend <- b:
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
