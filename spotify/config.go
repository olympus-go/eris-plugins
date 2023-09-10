package spotify

import (
	_ "embed"

	"github.com/BurntSushi/toml"
)

//go:embed default_config.toml
var defaultConfigStr string

type Config struct {
	Alias               string   `toml:"Alias"`
	Description         string   `toml:"Description"`
	AdminIds            []string `toml:"AdminIds"`
	SpotifyCallbackUrl  string   `toml:"-"`
	SpotifyClientId     string   `toml:"-"`
	SpotifyClientSecret string   `toml:"-"`
	RestrictSkips       string   `toml:"RestrictSkips"`
	GlobalResponses     struct {
		GenericError     string `toml:"GenericError"`
		NotInVoice       string `toml:"NotInVoice"`
		NotLoggedIn      string `toml:"NotLoggedIn"`
		EmptyQueue       string `toml:"EmptyQueue"`
		PermissionDenied string `toml:"PermissionDenied"`
	} `toml:"GlobalResponses"`
	PlayCommand struct {
		Alias          string              `toml:"Alias"`
		Description    string              `toml:"Description"`
		QueryOption    CommandOptionConfig `toml:"QueryOption"`
		PositionOption CommandOptionConfig `toml:"PositionOption"`
		RemixOption    CommandOptionConfig `toml:"RemixOption"`
		Responses      struct {
			SongPrompt       string `toml:"SongPrompt"`
			ListNotAvailable string `toml:"ListNotAvailable"`
			NoTracksFound    string `toml:"NoTracksFound"`
			EndOfList        string `toml:"EndOfList"`
			LoadingPlaylist  string `toml:"LoadingPlaylist"`
		} `toml:"Responses"`
	} `toml:"PlayCommand"`
	QueueCommand struct {
		Alias       string `toml:"Alias"`
		Description string `toml:"Description"`
		Responses   struct {
			EmptyQueue string `toml:"EmptyQueue"`
		} `toml:"Responses"`
	} `toml:"QueueCommand"`
	JoinCommand struct {
		Alias       string `toml:"Alias"`
		Description string `toml:"Description"`
		Responses   struct {
			AlreadyJoined string `toml:"AlreadyJoined"`
			JoinSuccess   string `toml:"JoinSuccess"`
		} `toml:"Responses"`
	} `toml:"JoinCommand"`
	LeaveCommand struct {
		Alias       string              `toml:"Alias"`
		Description string              `toml:"Description"`
		KeepOption  CommandOptionConfig `toml:"KeepOption"`
		Responses   struct {
			LeaveSuccess string `toml:"LeaveSuccess"`
		} `toml:"Responses"`
	} `toml:"LeaveCommand"`
	ResumeCommand struct {
		Alias       string `toml:"Alias"`
		Description string `toml:"Description"`
		Responses   struct {
			ResumeSuccess string `toml:"ResumeSuccess"`
		} `toml:"Responses"`
	} `toml:"ResumeCommand"`
	PauseCommand struct {
		Alias       string `toml:"Alias"`
		Description string `toml:"Description"`
		Responses   struct {
			PauseSuccess string `toml:"PauseSuccess"`
		} `toml:"Responses"`
	} `toml:"PauseCommand"`
	NextCommand struct {
		Alias       string `toml:"Alias"`
		Description string `toml:"Description"`
		Responses   struct {
			NextSuccess string `toml:"NextSuccess"`
		} ` toml:"Responses"`
	} `toml:"NextCommand"`
	PreviousCommand struct {
		Alias       string `toml:"Alias"`
		Description string `toml:"Description"`
		Responses   struct {
			EmptyQueue      string `toml:"EmptyQueue"`
			PreviousSuccess string `toml:"PreviousSuccess"`
		} `toml:"Responses"`
	} `toml:"PreviousCommand"`
	RemoveCommand struct {
		Alias          string              `toml:"Alias"`
		Description    string              `toml:"Description"`
		PositionOption CommandOptionConfig `toml:"PositionOption"`
		Responses      struct {
			InvalidPosition string `toml:"InvalidPosition"`
			RemoveSuccess   string `toml:"RemoveSuccess"`
		} `toml:"Responses"`
	} `toml:"RemoveCommand"`
	LoginCommand struct {
		Alias       string `toml:"Alias"`
		Description string `toml:"Description"`
		Responses   struct {
			LoginPrompt     string `toml:"LoginPrompt"`
			AlreadyLoggedIn string `toml:"AlreadyLoggedIn"`
			LoginSuccess    string `toml:"LoginSuccess"`
			LoginFail       string `toml:"LoginFail"`
			LoginCancel     string `toml:"LoginCancel"`
		} `toml:"Responses"`
	} `toml:"LoginCommand"`
	QuizCommand struct {
		Alias           string              `toml:"Alias"`
		Description     string              `toml:"Description"`
		PlaylistOption  CommandOptionConfig `toml:"PlaylistOption"`
		QuestionsOption CommandOptionConfig `toml:"QuestionsOption"`
	} `toml:"QuizCommand"`
}

type CommandOptionConfig struct {
	Alias       string `toml:"Alias"`
	Description string `toml:"Description"`
}

func DefaultConfig() Config {
	var config Config

	_ = toml.Unmarshal([]byte(defaultConfigStr), &config)

	return config
}

func ValidateConfig(c Config) error {
	return nil
}
