package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rick-learns/citebound/internal/answer"
	"github.com/rick-learns/citebound/internal/index"
)

// eventStream writes server-sent events and flushes each one.
type eventStream struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func newEventStream(w http.ResponseWriter) (*eventStream, error) {
	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no") // ask reverse proxies not to buffer the stream
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return nil, err
	}
	return &eventStream{w: w, rc: rc}, nil
}

// send writes one event. JSON has no raw newlines, so data is one line.
func (s *eventStream) send(name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := s.w.Write([]byte("event: " + name + "\ndata: " + string(data) + "\n\n")); err != nil {
		return err
	}
	return s.rc.Flush()
}

type passageJSON struct {
	Ref          string `json:"ref"`
	RFC          int    `json:"rfc"`
	RFCTitle     string `json:"rfcTitle"`
	Section      string `json:"section"`
	SectionTitle string `json:"sectionTitle"`
	Paragraph    int    `json:"paragraph"`
	Text         string `json:"text"`
	Figure       bool   `json:"figure"`
}

type retrievalEvent struct {
	Passages []passageJSON `json:"passages"`
}

type claimEvent struct {
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Ref   string `json:"ref"`
	Quote string `json:"quote"`
}

type verdictEvent struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type abstainEvent struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

type doneEvent struct {
	Mode         string  `json:"mode"`
	Model        string  `json:"model"`
	Claims       int     `json:"claims"`
	Verified     int     `json:"verified"`
	Struck       int     `json:"struck"`
	LatencyMs    int64   `json:"latencyMs"`
	InputTokens  int     `json:"inputTokens"`
	OutputTokens int     `json:"outputTokens"`
	CostUSD      float64 `json:"costUsd"`
}

// abstainMessages explain each abstention to a reader.
var abstainMessages = map[answer.AbstainReason]string{
	answer.NoPassages:       "Nothing in the corpus matched the question.",
	answer.AnswererAbstains: "The passages found do not answer the question.",
	answer.NoVerifiedClaims: "No claim could be verified against the cited RFC text.",
	answer.TooManyStruck:    "Too few claims could be verified to trust the answer.",
}

// answer runs the pipeline and streams its progress. Once the stream has
// started, failures are reported as an error event; the status is already
// 200.
func (s *Server) answer(ctx context.Context, stream *eventStream, question string) {
	start := s.clock()
	nextID := 0
	var sendErr error
	emit := func(e answer.Event) {
		if sendErr != nil {
			return
		}
		switch {
		case e.Passages != nil:
			sendErr = stream.send("retrieval", retrievalEvent{Passages: passages(e.Passages)})
		case e.Claim != nil:
			ref := e.Claim.Citation
			if e.Claim.Ref.RFC != 0 {
				ref = e.Claim.Ref.String()
			}
			sendErr = stream.send("claim", claimEvent{ID: nextID, Text: e.Claim.Text, Ref: ref, Quote: e.Claim.Quote})
		case e.Verdict != nil:
			sendErr = stream.send("verdict", verdictEvent{ID: nextID, Status: string(e.Verdict.Status), Reason: string(e.Verdict.Reason)})
			nextID++
		}
	}
	res, err := s.cfg.Pipeline.Ask(ctx, question, emit)
	if sendErr != nil {
		return // the client went away
	}
	if err != nil {
		code, msg := "internal", "Something went wrong while answering."
		if errors.Is(err, context.DeadlineExceeded) {
			code, msg = "unavailable", "Answering took too long. Try again."
		}
		if !errors.Is(err, context.Canceled) {
			s.cfg.Logger.Error("ask failed", "err", err)
		}
		_ = stream.send("error", errorBody{Code: code, Message: msg})
		return
	}
	if res.Abstained {
		if err := stream.send("abstain", abstainEvent{Code: string(res.Reason), Reason: abstainMessages[res.Reason]}); err != nil {
			return
		}
	}
	_ = stream.send("done", doneEvent{
		Mode:      s.cfg.Mode,
		Model:     s.cfg.Model,
		Claims:    len(res.Claims),
		Verified:  res.Verified,
		Struck:    len(res.Claims) - res.Verified,
		LatencyMs: s.clock().Sub(start).Milliseconds(),
	})
}

func passages(hits []index.Hit) []passageJSON {
	out := make([]passageJSON, 0, len(hits))
	for _, h := range hits {
		out = append(out, passageJSON{
			Ref: h.Ref.String(), RFC: h.Ref.RFC, RFCTitle: h.RFCTitle, Section: h.Ref.Section,
			SectionTitle: h.SectionTitle, Paragraph: h.Ref.Paragraph, Text: h.Text, Figure: h.Figure,
		})
	}
	return out
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, struct {
		Error errorBody `json:"error"`
	}{errorBody{Code: code, Message: message}})
}
