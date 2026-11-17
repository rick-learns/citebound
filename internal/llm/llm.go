// Package llm calls hosted language models through one small interface.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Request is one completion: a system prompt and a single user turn.
// Citebound never holds a conversation; every question is answered from
// scratch.
type Request struct {
	System    string `json:"system"`
	User      string `json:"user"`
	MaxTokens int    `json:"max_tokens"`
}

// Response is the model's reply and what it cost.
type Response struct {
	Text         string `json:"text"`
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	// Truncated is set when the reply stopped at MaxTokens.
	Truncated bool `json:"truncated"`
}

// Provider completes requests with one model.
type Provider interface {
	// Name identifies the provider and model, such as "anthropic/claude-sonnet-5-5".
	Name() string
	Complete(ctx context.Context, req Request) (*Response, error)
}

// APIError is a non-2xx response from a provider.
type APIError struct {
	Provider string
	Status   int
	Type     string
	Message  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: HTTP %d %s: %s", e.Provider, e.Status, e.Type, e.Message)
}

// Retryable reports whether the request may succeed if sent again: rate
// limits, overload, and server errors.
func (e *APIError) Retryable() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= 500
}

// maxResponseBytes bounds how much of a provider response is read.
const maxResponseBytes = 4 << 20

// httpDoer sends HTTP requests; *http.Client is one.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// defaultClient has a timeout so a hung provider cannot hold a request open.
var defaultClient = &http.Client{Timeout: 90 * time.Second}

// postJSON sends body to url and decodes a 2xx response into out. On any
// other status it decodes the body with apiErr, which returns the error type
// and message.
func postJSON(ctx context.Context, client httpDoer, provider, url string, headers map[string]string, body, out any,
	apiErr func([]byte) (typ, msg string),
) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if client == nil {
		client = defaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", provider, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("%s: read response: %w", provider, err)
	}
	if resp.StatusCode/100 != 2 {
		typ, msg := apiErr(raw)
		if msg == "" {
			msg = strings.TrimSpace(string(raw[:min(len(raw), 200)]))
		}
		return &APIError{Provider: provider, Status: resp.StatusCode, Type: typ, Message: msg}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s: decode response: %w", provider, err)
	}
	return nil
}

// ErrEmpty is returned when a provider replies with no text.
var ErrEmpty = errors.New("the model returned no text")
