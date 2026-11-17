package llm

import (
	"cmp"
	"context"
	"encoding/json"
	"strings"
)

// OpenAI calls an OpenAI-compatible Chat Completions API. With a different
// BaseURL it also reaches compatible gateways such as OpenRouter.
type OpenAI struct {
	APIKey string
	Model  string
	// BaseURL defaults to https://api.openai.com/v1.
	BaseURL string
	// Label names the provider in Name; it defaults to "openai".
	Label  string
	Client httpDoer
}

// Name implements Provider.
func (o *OpenAI) Name() string { return cmp.Or(o.Label, "openai") + "/" + o.Model }

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model               string          `json:"model"`
	Messages            []openAIMessage `json:"messages"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
}

type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Complete implements Provider.
func (o *OpenAI) Complete(ctx context.Context, req Request) (*Response, error) {
	msgs := []openAIMessage{{Role: "user", Content: req.User}}
	if req.System != "" {
		msgs = append([]openAIMessage{{Role: "system", Content: req.System}}, msgs...)
	}
	var out openAIResponse
	err := postJSON(ctx, o.Client, o.Name(), strings.TrimSuffix(cmp.Or(o.BaseURL, "https://api.openai.com/v1"), "/")+"/chat/completions", o.APIKey,
		map[string]string{"Authorization": "Bearer " + o.APIKey},
		openAIRequest{Model: o.Model, Messages: msgs, MaxCompletionTokens: req.MaxTokens},
		&out, openAIError)
	if err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return nil, ErrEmpty
	}
	if out.Choices[0].Message.Content == "" {
		return nil, emptyReply(out.Choices[0].FinishReason == "length")
	}
	return &Response{
		Text:         out.Choices[0].Message.Content,
		Model:        out.Model,
		InputTokens:  out.Usage.PromptTokens,
		OutputTokens: out.Usage.CompletionTokens,
		Truncated:    out.Choices[0].FinishReason == "length",
	}, nil
}

// openAIError reads {"error": {"type", "message"}}, or {"error": "text"} as
// some gateways send.
func openAIError(body []byte) (typ, msg string) {
	var e struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &e) != nil || len(e.Error) == 0 {
		return "", ""
	}
	var obj struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(e.Error, &obj) == nil {
		return obj.Type, obj.Message
	}
	var s string
	_ = json.Unmarshal(e.Error, &s)
	return "", s
}
