package spotify

import (
	"gopkg.in/yaml.v3"
)

const defaultConfigStr = `Alias: spotify
Description: Plays spotify tracks in voice channels.
AdminIds: []
RestrictSkips: false
GlobalResponses:
  GenericError: Something went wrong.
  NotInVoice: I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯
  NotLoggedIn: Login first before playing.
  EmptyQueue: Nothing in queue.
  PermissionDenied: You don't have permissions to change this track.
PlayCommand:
  Alias: play
  Description: Plays a specified song
  QueryOption:
    Alias: query
    Description: Search query or spotify url
  PositionOption:
    Alias: position
    Description: Position to insert the song at (default = append)
  RemixOption:
    Alias: remix
    Description: You want people to know you watch anime
  Responses:
    SongPrompt: Is this your song?
    ListNotAvailable: This song list is no longer available. Try searching again.
    NoTracksFound: No tracks found.
    EndOfList: That's all of them! Try searching again.
    LoadingPlaylist: Queueing up playlist <a:loadingdots:1079304806881050644>
QueueCommand:
  Alias: queue
  Description: Shows the current song queue
  Responses:
    EmptyQueue: No songs in queue.
JoinCommand:
  Alias: join
  Description: Requests the bot to join your voice channel
  Responses:
    AlreadyJoined: I'm already here!
    JoinSuccess: ":tada:"
LeaveCommand:
  Alias: leave
  Description: Requests the bot to leave the voice channel
  KeepOption:
    Alias: keep
    Description: Don't clear the current running spotify session
  Responses:
    LeaveSuccess: ":wave:"
ResumeCommand:
  Alias: resume
  Description: Resume playback
  Responses:
    ResumeSuccess: ":arrow_forward:"
PauseCommand:
  Alias: pause
  Description: Pause the currently playing song
  Responses:
    PauseSuccess: ":pause_button:"
NextCommand:
  Alias: next
  Description: Go to the next song
  Responses:
    NextSuccess: ":fast_forward:"
PreviousCommand:
  Alias: previous
  Description: Go back to the previous song
  Responses:
    EmptyQueue: No queue history.
    PreviousSuccess: ":rewind:"
RemoveCommand:
  Alias: remove
  Description: Remove a song from queue
  PositionOption:
    Alias: position
    Description: Queue position of the song to remove
  Responses:
    InvalidPosition: Invalid position value.
    RemoveSuccess: ":gun:"
LoginCommand:
  Alias: login
  Description: Connect the bot to your spotify account
  Responses:
    LoginPrompt: Click here to login!
    AlreadyLoggedIn: Spotify session is already logged in. Log out now?
    LoginSuccess: "Login successful :tada:"
    LoginFail: "Login failed :("
    LoginCancel: ":+1:"
QuizCommand:
  Alias: quiz
  Description: Start a spotify quiz game
  PlaylistOption:
    Alias: playlist
    Description: Link to public playlist to use
  QuestionsOption:
    Alias: questions
    Description: Number of questions to play (default = 10)
`

type Config struct {
	Alias               string   `yaml:"Alias"`
	Description         string   `yaml:"Description"`
	AdminIds            []string `yaml:"AdminIds"`
	SpotifyCallbackUrl  string   `yaml:"-"`
	SpotifyClientId     string   `yaml:"-"`
	SpotifyClientSecret string   `yaml:"-"`
	RestrictSkips       string   `yaml:"RestrictSkips"`
	GlobalResponses     struct {
		GenericError     string `yaml:"GenericError"`
		NotInVoice       string `yaml:"NotInVoice"`
		NotLoggedIn      string `yaml:"NotLoggedIn"`
		EmptyQueue       string `yaml:"EmptyQueue"`
		PermissionDenied string `yaml:"PermissionDenied"`
	} `yaml:"GlobalResponses"`
	PlayCommand struct {
		Alias          string              `yaml:"Alias"`
		Description    string              `yaml:"Description"`
		QueryOption    CommandOptionConfig `yaml:"QueryOption"`
		PositionOption CommandOptionConfig `yaml:"PositionOption"`
		RemixOption    CommandOptionConfig `yaml:"RemixOption"`
		Responses      struct {
			SongPrompt       string `yaml:"SongPrompt"`
			ListNotAvailable string `yaml:"ListNotAvailable"`
			NoTracksFound    string `yaml:"NoTracksFound"`
			EndOfList        string `yaml:"EndOfList"`
			LoadingPlaylist  string `yaml:"LoadingPlaylist"`
		} `yaml:"Responses"`
	} `yaml:"PlayCommand"`
	QueueCommand struct {
		Alias       string `yaml:"Alias"`
		Description string `yaml:"Description"`
		Responses   struct {
			EmptyQueue string `yaml:"EmptyQueue"`
		} `yaml:"Responses"`
	} `yaml:"QueueCommand"`
	JoinCommand struct {
		Alias       string `yaml:"Alias"`
		Description string `yaml:"Description"`
		Responses   struct {
			AlreadyJoined string `yaml:"AlreadyJoined"`
			JoinSuccess   string `yaml:"JoinSuccess"`
		} `yaml:"Responses"`
	} `yaml:"JoinCommand"`
	LeaveCommand struct {
		Alias       string              `yaml:"Alias"`
		Description string              `yaml:"Description"`
		KeepOption  CommandOptionConfig `yaml:"KeepOption"`
		Responses   struct {
			LeaveSuccess string `yaml:"LeaveSuccess"`
		} `yaml:"Responses"`
	} `yaml:"LeaveCommand"`
	ResumeCommand struct {
		Alias       string `yaml:"Alias"`
		Description string `yaml:"Description"`
		Responses   struct {
			ResumeSuccess string `yaml:"ResumeSuccess"`
		} `yaml:"Responses"`
	} `yaml:"ResumeCommand"`
	PauseCommand struct {
		Alias       string `yaml:"Alias"`
		Description string `yaml:"Description"`
		Responses   struct {
			PauseSuccess string `yaml:"PauseSuccess"`
		} `yaml:"Responses"`
	} `yaml:"PauseCommand"`
	NextCommand struct {
		Alias       string `yaml:"Alias"`
		Description string `yaml:"Description"`
		Responses   struct {
			NextSuccess string `yaml:"NextSuccess"`
		} ` yaml:"Responses"`
	} `yaml:"NextCommand"`
	PreviousCommand struct {
		Alias       string `yaml:"Alias"`
		Description string `yaml:"Description"`
		Responses   struct {
			EmptyQueue      string `yaml:"EmptyQueue"`
			PreviousSuccess string `yaml:"PreviousSuccess"`
		} `yaml:"Responses"`
	} `yaml:"PreviousCommand"`
	RemoveCommand struct {
		Alias          string              `yaml:"Alias"`
		Description    string              `yaml:"Description"`
		PositionOption CommandOptionConfig `yaml:"PositionOption"`
		Responses      struct {
			InvalidPosition string `yaml:"InvalidPosition"`
			RemoveSuccess   string `yaml:"RemoveSuccess"`
		} `yaml:"Responses"`
	} `yaml:"RemoveCommand"`
	LoginCommand struct {
		Alias       string `yaml:"Alias"`
		Description string `yaml:"Description"`
		Responses   struct {
			LoginPrompt     string `yaml:"LoginPrompt"`
			AlreadyLoggedIn string `yaml:"AlreadyLoggedIn"`
			LoginSuccess    string `yaml:"LoginSuccess"`
			LoginFail       string `yaml:"LoginFail"`
			LoginCancel     string `yaml:"LoginCancel"`
		} `yaml:"Responses"`
	} `yaml:"LoginCommand"`
	QuizCommand struct {
		Alias           string              `yaml:"Alias"`
		Description     string              `yaml:"Description"`
		PlaylistOption  CommandOptionConfig `yaml:"PlaylistOption"`
		QuestionsOption CommandOptionConfig `yaml:"QuestionsOption"`
	} `yaml:"QuizCommand"`
}

type CommandOptionConfig struct {
	Alias       string `yaml:"Alias"`
	Description string `yaml:"Description"`
}

func DefaultConfig() Config {
	var config Config

	_ = yaml.Unmarshal([]byte(defaultConfigStr), &config)

	return config
}

func ValidateConfig(c Config) error {
	return nil
}
