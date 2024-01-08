package chat

import (
	"fmt"
	"strings"
)

type GenerateData struct {
	N                int     `json:"n"`
	MaxContextLength int     `json:"max_context_length"`
	MaxLength        int     `json:"max_length"`
	RepPen           float64 `json:"rep_pen"`
	Temperature      float64 `json:"temperature"`
	TopP             float64 `json:"top_p"`
	TopK             int     `json:"top_k"`
	TopA             int     `json:"top_a"`
	Typical          int     `json:"typical"`
	Tfs              int     `json:"tfs"`
	RepPenRange      int     `json:"rep_pen_range"`
	RepPenSlope      float64 `json:"rep_pen_slope"`
	SamplerOrder     []int   `json:"sampler_order"`
	Memory           string  `json:"memory"`
	Genkey           string  `json:"genkey"`
	MinP             int     `json:"min_p"`
	PresencePenalty  int     `json:"presence_penalty"`
	LogitBias        struct {
	} `json:"logit_bias"`
	Prompt                string   `json:"prompt"`
	Quiet                 bool     `json:"quiet"`
	StopSequence          []string `json:"stop_sequence"`
	UseDefaultBadwordsids bool     `json:"use_default_badwordsids"`
}

type ResponseData struct {
	Results []struct {
		Text string `json:"text"`
	} `json:"results"`
}

func DefaultGenerateData() GenerateData {
	return GenerateData{
		N:                     1,
		MaxContextLength:      6144,
		MaxLength:             180,
		RepPen:                1.1,
		Temperature:           0.7,
		TopP:                  0.92,
		TopK:                  100,
		TopA:                  0,
		Typical:               1,
		Tfs:                   1,
		RepPenRange:           320,
		RepPenSlope:           0.7,
		SamplerOrder:          []int{6, 0, 1, 3, 4, 2, 5},
		Memory:                "Personality: George is a sassy tsundere. He likes to use ascii emoticons and roleplay using *italics* to describe his actions.",
		Genkey:                "KCPP6099",
		MinP:                  0,
		PresencePenalty:       0,
		LogitBias:             struct{}{},
		Prompt:                "",
		Quiet:                 true,
		StopSequence:          []string{"\nGeorge: "},
		UseDefaultBadwordsids: false,
	}
}

func (g *GenerateData) checkStopSequence(s string) bool {
	for i := range g.StopSequence {
		if strings.Contains(g.StopSequence[i], s) {
			return true
		}
	}

	return false
}

func (g *GenerateData) appendStopSequence(s string) {
	g.StopSequence = append(g.StopSequence, fmt.Sprintf("%s:", s), fmt.Sprintf("\n%s ", s))
}

func (r ResponseData) clean(suffixes ...string) string {
	cleaned := strings.TrimSpace(r.Results[0].Text)

	for i := range suffixes {
		cleaned = strings.TrimSpace(strings.TrimSuffix(cleaned, suffixes[i]))
	}

	return cleaned
}
