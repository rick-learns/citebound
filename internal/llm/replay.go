package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Recording is one request and the response it got, as stored on disk.
type Recording struct {
	Key      string   `json:"key"`
	Provider string   `json:"provider"`
	Request  Request  `json:"request"`
	Response Response `json:"response"`
	// Recorded is the UTC date of the recording, such as 2026-11-17. It is
	// not part of the key.
	Recorded string `json:"recorded"`
}

// Key identifies a request to a provider. Any change to the prompt, the
// question, the passages, or the model gives a new key, so a stale
// recording is never replayed for a changed request.
func Key(provider string, req Request) string {
	data, _ := json.Marshal(struct {
		Provider string  `json:"provider"`
		Request  Request `json:"request"`
	}{provider, req})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ErrNotRecorded is returned by Replay for a request it has no recording of.
var ErrNotRecorded = errors.New("no recording for this request")

// Replay answers requests from recordings, with no network. It is how tests
// and CI evals run model answers without an API key.
type Replay struct {
	name  string
	byKey map[string]Response
	// Oldest and Newest are the range of recording dates loaded.
	Oldest, Newest string
}

// LoadReplay reads the recordings for provider name from a JSON Lines file
// written by Recorder. Recordings for other providers are ignored.
func LoadReplay(path, name string) (*Replay, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is an operator-chosen recordings file
	if err != nil {
		return nil, err
	}
	r := &Replay{name: name, byKey: map[string]Response{}}
	// Lines are split by hand: a recording has no size limit, and escaping
	// can make one several times larger than the response it holds.
	for i, text := range bytes.Split(data, []byte("\n")) {
		line := i + 1
		if len(bytes.TrimSpace(text)) == 0 {
			continue
		}
		var rec Recording
		if err := json.Unmarshal(text, &rec); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if rec.Provider != name {
			continue
		}
		if want := Key(rec.Provider, rec.Request); rec.Key != want {
			return nil, fmt.Errorf("%s:%d: key does not match the request; the file was edited by hand", path, line)
		}
		r.byKey[rec.Key] = rec.Response
		if r.Oldest == "" || rec.Recorded < r.Oldest {
			r.Oldest = rec.Recorded
		}
		r.Newest = max(r.Newest, rec.Recorded)
	}
	return r, nil
}

// Name implements Provider. It is the name of the provider recorded, so keys
// match the ones the recorder wrote.
func (r *Replay) Name() string { return r.name }

// Len returns the number of recordings loaded.
func (r *Replay) Len() int { return len(r.byKey) }

// Complete implements Provider.
func (r *Replay) Complete(_ context.Context, req Request) (*Response, error) {
	resp, ok := r.byKey[Key(r.name, req)]
	if !ok {
		return nil, fmt.Errorf("%s: %w (key %s)", r.name, ErrNotRecorded, Key(r.name, req)[:12])
	}
	return &resp, nil
}

// Recorder passes requests to a live provider and appends each successful
// response to a JSON Lines file for Replay. API keys are never written:
// requests hold only the prompt.
type Recorder struct {
	Provider Provider
	Path     string
	mu       sync.Mutex
}

// Name implements Provider.
func (r *Recorder) Name() string { return r.Provider.Name() }

// Complete implements Provider.
func (r *Recorder) Complete(ctx context.Context, req Request) (*Response, error) {
	resp, err := r.Provider.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	line, err := json.Marshal(Recording{Key: Key(r.Name(), req), Provider: r.Name(), Request: req, Response: *resp, Recorded: time.Now().UTC().Format(time.DateOnly)})
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := appendLine(r.Path, line); err != nil {
		return nil, err
	}
	return resp, nil
}

// appendLine appends one JSON line to the recordings file at path, which is
// made private (0600) even if it already existed with wider permissions. If
// the file ends in a complete record with no newline, one is added first; if
// it ends in a partial record, nothing is written, because appending would
// leave the file unreadable.
func appendLine(path string, line []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600) //nolint:gosec // the path is an operator-chosen recordings file
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	existing, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if n := len(existing); n > 0 && existing[n-1] != '\n' {
		last := existing[bytes.LastIndexByte(existing, '\n')+1:]
		if !json.Valid(last) {
			return fmt.Errorf("%s ends in an incomplete record; fix or remove it before recording", path)
		}
		line = append([]byte("\n"), line...)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Close()
}
