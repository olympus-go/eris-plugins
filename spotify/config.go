package spotify

import (
	_ "embed"
	"encoding/json"
)

//go:embed default_config.json
var DefaultConfigStr string

type Config struct {
	Alias               string   `json:"Alias"`
	Description         string   `json:"Description"`
	AdminIds            []string `json:"AdminIds"`
	SpotifyCallbackUrl  string   `json:"-"`
	SpotifyClientId     string   `json:"-"`
	SpotifyClientSecret string   `json:"-"`
	RestrictSkips       string   `json:"RestrictSkips"`
	BannedTracks        []string `json:"BannedTracks"`
	GlobalResponses     struct {
		GenericSuccess   string `json:"GenericSuccess"`
		GenericError     string `json:"GenericError"`
		NotInVoice       string `json:"NotInVoice"`
		NotLoggedIn      string `json:"NotLoggedIn"`
		EmptyQueue       string `json:"EmptyQueue"`
		PermissionDenied string `json:"PermissionDenied"`
	} `json:"GlobalResponses"`
	PlayCommand struct {
		Alias          string              `json:"Alias"`
		Description    string              `json:"Description"`
		QueryOption    CommandOptionConfig `json:"QueryOption"`
		PositionOption CommandOptionConfig `json:"PositionOption"`
		RemixOption    CommandOptionConfig `json:"RemixOption"`
		Responses      struct {
			SongPrompt       string `json:"SongPrompt"`
			ListNotAvailable string `json:"ListNotAvailable"`
			NoTracksFound    string `json:"NoTracksFound"`
			EndOfList        string `json:"EndOfList"`
			LoadingPlaylist  string `json:"LoadingPlaylist"`
			BannedTrack      string `json:"BannedTrack"`
		} `json:"Responses"`
	} `json:"PlayCommand"`
	QueueCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
		Responses   struct {
			EmptyQueue string `json:"EmptyQueue"`
		} `json:"Responses"`
	} `json:"QueueCommand"`
	JoinCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
		Responses   struct {
			AlreadyJoined string `json:"AlreadyJoined"`
			JoinSuccess   string `json:"JoinSuccess"`
		} `json:"Responses"`
	} `json:"JoinCommand"`
	LeaveCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
		Responses   struct {
			LeaveSuccess string `json:"LeaveSuccess"`
		} `json:"Responses"`
	} `json:"LeaveCommand"`
	ResumeCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
		Responses   struct {
			ResumeSuccess string `json:"ResumeSuccess"`
		} `json:"Responses"`
	} `json:"ResumeCommand"`
	PauseCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
		Responses   struct {
			PauseSuccess string `json:"PauseSuccess"`
		} `json:"Responses"`
	} `json:"PauseCommand"`
	NextCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
		Responses   struct {
			NextSuccess string `json:"NextSuccess"`
		} ` json:"Responses"`
	} `json:"NextCommand"`
	PreviousCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
		Responses   struct {
			EmptyQueue      string `json:"EmptyQueue"`
			PreviousSuccess string `json:"PreviousSuccess"`
		} `json:"Responses"`
	} `json:"PreviousCommand"`
	RemoveCommand struct {
		Alias          string              `json:"Alias"`
		Description    string              `json:"Description"`
		PositionOption CommandOptionConfig `json:"PositionOption"`
		Responses      struct {
			InvalidPosition string `json:"InvalidPosition"`
			RemoveSuccess   string `json:"RemoveSuccess"`
		} `json:"Responses"`
	} `json:"RemoveCommand"`
	LoginCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
		Responses   struct {
			LoginPrompt     string `json:"LoginPrompt"`
			AlreadyLoggedIn string `json:"AlreadyLoggedIn"`
			LoginSuccess    string `json:"LoginSuccess"`
			LoginFail       string `json:"LoginFail"`
			LoginCancel     string `json:"LoginCancel"`
		} `json:"Responses"`
	} `json:"LoginCommand"`
	QuizCommand struct {
		Alias           string              `json:"Alias"`
		Description     string              `json:"Description"`
		PlaylistOption  CommandOptionConfig `json:"PlaylistOption"`
		QuestionsOption CommandOptionConfig `json:"QuestionsOption"`
	} `json:"QuizCommand"`
	ListifyCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
	} `json:"ListifyCommand"`
	ClearCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
	} `json:"ClearCommand"`
	ShuffleCommand struct {
		Alias       string `json:"Alias"`
		Description string `json:"Description"`
	} `json:"ShuffleCommand"`
}

type CommandOptionConfig struct {
	Alias       string `json:"Alias"`
	Description string `json:"Description"`
}

func DefaultConfig() Config {
	var config Config

	_ = json.Unmarshal([]byte(DefaultConfigStr), &config)

	return config
}

func ValidateConfig(c Config) error {
	return nil
}
