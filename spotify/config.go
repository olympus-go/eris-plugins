package spotify

type Config struct {
	CommandPrefix string   `yaml:"command_prefix"`
	AdminIds      []string `yaml:"admin_ids"`
	PlayCommand   struct {
		Alias          string              `yaml:"alias"`
		Description    string              `yaml:"description"`
		QueryOption    CommandOptionConfig `yaml:"query_option"`
		PositionOption CommandOptionConfig `yaml:"position_option"`
		RemixOption    CommandOptionConfig `yaml:"remix_option"`
		Responses      struct {
			NotInVoice       string `yaml:"not_in_voice"`
			NotLoggedIn      string `yaml:"not_logged_in"`
			NoTracksFound    string `yaml:"no_tracks_found"`
			ListNotAvailable string `yaml:"list_not_available"`
			EndOfList        string `yaml:"end_of_list"`
		} `yaml:"responses"`
	} `yaml:"play_command"`
	GenericError string `yaml:"generic_error"`
}

type CommandOptionConfig struct {
	Alias       string `yaml:"alias"`
	Description string `yaml:"description"`
}

func DefaultConfig() Config {
	return Config{
		CommandPrefix: "spotify",
		AdminIds:      []string{"404108775935442944", "154827595186307072"},
		PlayCommand: struct {
			Alias          string              `yaml:"alias"`
			Description    string              `yaml:"description"`
			QueryOption    CommandOptionConfig `yaml:"query_option"`
			PositionOption CommandOptionConfig `yaml:"position_option"`
			RemixOption    CommandOptionConfig `yaml:"remix_option"`
			Responses      struct {
				NotInVoice       string `yaml:"not_in_voice"`
				NotLoggedIn      string `yaml:"not_logged_in"`
				NoTracksFound    string `yaml:"no_tracks_found"`
				ListNotAvailable string `yaml:"list_not_available"`
				EndOfList        string `yaml:"end_of_list"`
			} `yaml:"responses"`
		}{
			Alias:       "play",
			Description: "Plays a specified song",
			QueryOption: CommandOptionConfig{
				Alias:       "query",
				Description: "Search query or spotify url",
			},
			PositionOption: CommandOptionConfig{
				Alias:       "position",
				Description: "Position to insert the song at (default = append)",
			},
			RemixOption: CommandOptionConfig{
				Alias:       "remix",
				Description: "You want people to know you watch anime",
			},
			Responses: struct {
				NotInVoice       string `yaml:"not_in_voice"`
				NotLoggedIn      string `yaml:"not_logged_in"`
				NoTracksFound    string `yaml:"no_tracks_found"`
				ListNotAvailable string `yaml:"list_not_available"`
				EndOfList        string `yaml:"end_of_list"`
			}{
				NotInVoice:       "I don't think I'm in a voice chat here. ¯\\_(ツ)_/¯",
				NotLoggedIn:      "Login first before playing.\n`/spotify login`",
				NoTracksFound:    "No tracks found.",
				ListNotAvailable: "This song list is no longer available. Try searching again.",
				EndOfList:        "That's all of them! Try searching again.",
			},
		},
		GenericError: "Something went wrong.",
	}
}

func ValidateConfig(c Config) error {
	return nil
}
