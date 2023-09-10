package spotify

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/eolso/threadsafe"
	"github.com/olympus-go/apollo/spotify"
	"github.com/olympus-go/eris/utils"
)

type quiz struct {
	playlist          []string
	questions         int
	previousQuestions *threadsafe.Map[int, bool]

	questionNumber      int
	questionAnswer      int
	questionAnswerTrack spotify.Track

	questionStartTime     time.Time
	questionResponseTimes *threadsafe.Map[string, float64]

	// scoreboard contains the players as keys, and their points as values
	scoreboard *threadsafe.Map[string, struct {
		score     int
		totalTime float64
	}]

	startInteraction *discordgo.Interaction
	startMessage     string

	rng        *rand.Rand
	cancelFunc context.CancelFunc
}

// validatePlaylist checks if the playlist is still in a good state.
//func (s *quiz) validatePlaylist() bool {
//	return s.playlist.Len()len(s.playlist)-s.previousQuestions.Len() < 5
//}

func (s *quiz) getRandomTracks(player *spotify.Session, n int) []spotify.Track {
	if s.rng == nil {
		return nil
	}

	randomIndexes := make(map[int]bool)
	for len(randomIndexes) < n {
		randomIndexes[s.rng.Intn(len(s.playlist))] = true
	}

	var tracks []spotify.Track
	for key, _ := range randomIndexes {
		t, err := player.GetTrackById(s.playlist[key])
		if err != nil {
			return nil
		}

		tracks = append(tracks, t)
	}

	return tracks
}

func (s *quiz) getRandomTrackIndexes(n int) []int {
	if s.rng == nil {
		return nil
	}

	indexMap := make(map[int]bool)
	for len(indexMap) < n {
		randNum := s.rng.Intn(len(s.playlist))
		if _, ok := s.previousQuestions.Get(randNum); ok {
			continue
		}
		indexMap[randNum] = true
	}

	indexSlice := make([]int, n)
	for key, _ := range indexMap {
		indexSlice = append(indexSlice, key)
	}

	return indexSlice
}

func (s *quiz) getTracks(session *spotify.Session, trackIds ...string) []spotify.Track {
	tracks := make([]spotify.Track, len(trackIds))
	for _, trackId := range trackIds {
		track, err := session.GetTrackById(trackId)
		if err != nil {
			return nil
		}

		tracks = append(tracks, track)
	}

	return tracks
}

func (s *quiz) generateQuestion(tracks []spotify.Track) *discordgo.InteractionResponse {
	message := fmt.Sprintf("Question %d:\n```\n", s.questionNumber)
	var actionsRow utils.ActionsRowBuilder
	for index, track := range tracks {
		rowNum := fmt.Sprintf("%d", index+1)
		message += fmt.Sprintf("%s) %s || %s\n", rowNum, track.Name(), track.Artist())
		button := utils.Button().Id(fmt.Sprintf("spotify_quiz_answer_%s", rowNum)).Label(rowNum).Build()
		actionsRow.Button(button)
	}
	message += "```"

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    message,
			Components: []discordgo.MessageComponent{actionsRow.Build()},
		},
	}
}

func (s *quiz) generateQuestionWinner() string {
	message := fmt.Sprintf("The correct answer was: `%s || %s`\n", s.questionAnswerTrack.Name(),
		s.questionAnswerTrack.Artist())

	keys, values := s.questionResponseTimes.Items()
	if len(keys) == 0 || len(values) == 0 {
		return message
	}

	sort.SliceStable(keys, func(i, j int) bool {
		return values[i] < values[j]
	})

	sort.SliceStable(values, func(i, j int) bool {
		return values[i] < values[j]
	})

	// Update the winner's score
	noWinner := true
	if responseTime, _ := s.questionResponseTimes.Get(keys[0]); responseTime < 15.0 {
		currentScore, _ := s.scoreboard.Get(keys[0])
		currentScore.score++
		s.scoreboard.Set(keys[0], currentScore)
		noWinner = false
	}

	if noWinner {
		message += fmt.Sprintf("Y'all are dumb :unamused:\n```")
	} else {
		message += fmt.Sprintf("%s answered the fastest <:gottagofast:1079304836534784022>\n```", keys[0])
	}

	for i := 0; i < len(keys); i++ {
		if values[i] < 15.0 {
			message += fmt.Sprintf("%s - %.3fs\n", keys[i], values[i])
		} else {
			message += fmt.Sprintf("%s - WRONG\n", keys[i])
		}
	}
	message += "```"

	return message
}

func (s *quiz) generateGameWinner() string {
	players, scores := s.scoreboard.Items()
	if len(players) == 0 || len(scores) == 0 {
		return "No one was playing."
	}

	sort.SliceStable(players, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	scoreboardMessage := "```"
	for index, _ := range players {
		scoreboardMessage += fmt.Sprintf("%s - %dpts (Total: %.3fs, Avg: %.3fs)\n",
			players[index], scores[index].score, scores[index].totalTime, scores[index].totalTime/float64(s.questions))
	}
	scoreboardMessage += "```"

	winners := []string{players[0]}
	for index := 1; index < len(players); index++ {
		if scores[index].score == scores[0].score {
			if scores[index].totalTime < scores[0].totalTime {
				winners = []string{players[index]}
				scores[0].totalTime = scores[index].totalTime
			} else if scores[index].totalTime == scores[0].totalTime {
				winners = append(winners, players[index])
			}
		}
	}

	var message string
	if len(winners) > 1 {
		message = fmt.Sprintf("%s are the winners!\n", strings.Join(winners, " and "))
	} else {
		message = fmt.Sprintf("%s is the winner!\n", winners[0])
	}

	return message + scoreboardMessage
}
