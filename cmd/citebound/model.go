package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/llm"
)

// answererFlags choose who writes answers: the extractive answerer, a live
// model, or a model's recorded answers.
type answererFlags struct {
	model, record, replay *string
}

func addAnswererFlags(fs *flag.FlagSet) *answererFlags {
	return &answererFlags{
		model:  fs.String("model", "extractive", `answerer: "extractive", or provider/model with provider anthropic, openai, or openrouter`),
		record: fs.String("record", "", "append the model's answers to this recordings file"),
		replay: fs.String("replay", "", "answer from this recordings file instead of calling the model; needs no API key"),
	}
}

// apiKeyVars names the environment variable holding each provider's key.
var apiKeyVars = map[string]string{
	"anthropic":  "ANTHROPIC_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"openrouter": "OPENROUTER_API_KEY",
}

// answerer is what the flags chose: the answerer, the mode reported by the
// API ("extractive", "model", or "replay"), and the model name.
type answerer struct {
	answer.Answerer
	mode, model string
}

func (f *answererFlags) build() (answerer, error) {
	spec := *f.model
	if spec == "extractive" {
		if *f.record != "" || *f.replay != "" {
			return answerer{}, errors.New("-record and -replay need -model provider/model")
		}
		return answerer{answer.Extractive{}, "extractive", ""}, nil
	}
	provider, model, ok := strings.Cut(spec, "/")
	keyVar, known := apiKeyVars[provider]
	if !ok || !known || model == "" {
		return answerer{}, fmt.Errorf("-model %q: want extractive, or anthropic/, openai/, or openrouter/ followed by a model name", spec)
	}
	if *f.record != "" && *f.replay != "" {
		return answerer{}, errors.New("use -record or -replay, not both")
	}

	if *f.replay != "" {
		r, err := llm.LoadReplay(*f.replay, spec)
		if err != nil {
			return answerer{}, err
		}
		if r.Len() == 0 {
			return answerer{}, fmt.Errorf("%s has no recordings for %s", *f.replay, spec)
		}
		return answerer{answer.Model{Provider: r}, "replay", spec}, nil
	}

	key := os.Getenv(keyVar)
	if key == "" {
		return answerer{}, fmt.Errorf("-model %s needs %s, or -replay with a recordings file", spec, keyVar)
	}
	var p llm.Provider
	switch provider {
	case "anthropic":
		p = &llm.Anthropic{APIKey: key, Model: model}
	case "openai":
		p = &llm.OpenAI{APIKey: key, Model: model}
	case "openrouter":
		p = &llm.OpenAI{APIKey: key, Model: model, BaseURL: "https://openrouter.ai/api/v1", Label: "openrouter"}
	}
	if *f.record != "" {
		p = &llm.Recorder{Provider: p, Path: *f.record}
	}
	return answerer{answer.Model{Provider: p}, "model", spec}, nil
}
