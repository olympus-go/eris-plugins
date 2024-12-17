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
	"github.com/olympus-go/eris/utils"
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
	session *spotify.Session
	player  *apollo.Player

	playInteractions *threadsafe.Map[string, playInteraction]
	quizGame         *quiz

	guildId         string
	voiceConnection *discordgo.VoiceConnection

	adminIds []string

	cancelVoiceTimeout context.CancelFunc
	cancelVoiceSend    context.CancelFunc
	timeLastJoined     time.Time
}

func newSession(guildId string, sessionConfig spotify.SessionConfig, h slog.Handler, adminIds ...string) *session {
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
		guildId:          guildId,
		voiceConnection:  nil,
		adminIds:         adminIds,
	}
}

func (s *session) joinVoice(discordSession *discordgo.Session, interaction *discordgo.Interaction) error {
	voiceId := utils.GetInteractionUserVoiceStateId(discordSession, interaction)

	if voiceId == "" {
		return ErrNotInVoice
	}

	if s.voiceConnection != nil && s.voiceConnection.ChannelID == voiceId {
		return ErrAlreadyInVoice
	}

	// If we're already in a voice channel, disconnect from it first
	if s.voiceConnection != nil {
		if err := s.leaveVoice(); err != nil {
			return err
		}
	}

	if s.cancelVoiceTimeout != nil {
		s.cancelVoiceTimeout()
		s.cancelVoiceTimeout = nil
	}

	if s.cancelVoiceSend != nil {
		s.cancelVoiceSend()
		s.cancelVoiceSend = nil
	}

	var err error
	for retries := 0; retries < 3; retries++ {
		s.voiceConnection, err = discordSession.ChannelVoiceJoin(interaction.GuildID, voiceId, false, true)
		if err == nil {
			break
		}
		_ = s.voiceConnection.Disconnect()
	}

	if err != nil {
		_ = s.voiceConnection.Disconnect()
		return err
	}

	s.timeLastJoined = time.Now()

	s.cancelVoiceSend = s.start()
	s.cancelVoiceTimeout, err = s.timeoutVoice(context.Background(), discordSession)

	s.player.Play()

	return err
}

func (s *session) leaveVoice() error {
	if s.voiceConnection == nil {
		return ErrNotInVoice
	}

	s.player.Pause()

	if s.quizGame != nil {
		s.quizGame.cancelFunc()
		s.quizGame = nil
	}

	s.stop()

	if err := s.voiceConnection.Disconnect(); err != nil {
		return err
	}
	s.voiceConnection = nil

	return nil
}

func (s *session) start() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	out := s.player.Out()

	go func() {
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
	}()

	return cancel
}

func (s *session) stop() {
	if s.cancelVoiceSend != nil {
		s.cancelVoiceSend()
		s.cancelVoiceSend = nil
	}
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

func (s *session) timeoutVoice(ctx context.Context, discordSession *discordgo.Session) (context.CancelFunc, error) {
	c, cancel := context.WithCancel(ctx)

	guild, err := discordSession.State.Guild(s.guildId)
	if err != nil {
		cancel()
		return nil, err
	}

	go func() {
		for {
			select {
			case <-c.Done():
				return
			case <-time.Tick(sessionTimeout):
				if s.voiceConnection == nil {
					cancel()
				} else {
					count := 0
					for _, voiceState := range guild.VoiceStates {
						if voiceState.ChannelID == s.voiceConnection.ChannelID {
							count += 1
						}
					}

					if count <= 1 {
						_ = s.leaveVoice()
						cancel()
					}
				}
			}
		}
	}()

	return cancel, nil
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
