package spotify

import "errors"

var ErrNotInVoice = errors.New("not in voice")
var ErrAlreadyInVoice = errors.New("already in voice")
