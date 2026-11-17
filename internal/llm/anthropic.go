package llm

import (
	"cmp"
	"context"
	"encoding/json"
	"strings"
)

// Anthropic calls the Anthropic Messages API.
type Anthropic struct {
	APIKey string
	Model  string
	// BaseURL defaults to https://api.anthropic.com.
	BaseURL string
	Client  httpDoer
}

const anthropicVersion = "2023-06-01"

// Name implements Provider.
func (a *Anthropic) Name() string { return "anthropic/" + a.Model }

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Complete implements Provider.
func (a *Anthropic) Complete(ctx context.Context, req Request) (*Response, error) {
	var out anthropicResponse
	err := postJSON(ctx, a.Client, a.Name(), strings.TrimSuffix(cmp.Or(a.BaseURL, "https://api.anthropic.com"), "/")+"/v1/messages", a.APIKey,
		map[string]string{"x-api-key": a.APIKey, "anthropic-version": anthropicVersion},
		anthropicRequest{
			Model:     a.Model,
			MaxTokens: req.MaxTokens,
			System:    req.System,
			Messages:  []anthropicMessage{{Role: "user", Content: req.User}},
		},
		&out, anthropicError)
	if err != nil {
		return nil, err
	}
	var text strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	if text.Len() == 0 {
		return nil, emptyReply(out.StopReason == "max_tokens")
	}
	return &Response{
		Text:         text.String(),
		Model:        out.Model,
		InputTokens:  out.Usage.InputTokens,
		OutputTokens: out.Usage.OutputTokens,
		Truncated:    out.StopReason == "max_tokens",
	}, nil
}

func anthropicError(body []byte) (typ, msg string) {
	var e struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	return e.Error.Type, e.Error.Message
}
