package eval

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
)

func TestParseRetrievalCases(t *testing.T) {
	good := `{"id":"r1","question":"q?","gold":["RFC6749 §10.4 ¶1"],"reference_answer":"a","tags":["x"],"difficulty":"easy"}

{"id":"r2","question":"q2?","gold":["RFC 6749 §1.5 ¶2","RFC7636 §4.2 ¶3"]}
`
	cases, err := parseRetrievalCases([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 2 || cases[1].Gold[0] != "RFC 6749 §1.5 ¶2" {
		t.Fatalf("got %+v", cases)
	}

	bad := map[string]string{
		"empty":         "\n\n",
		"not json":      `{"id":`,
		"unknown field": `{"id":"r1","question":"q","gold":["RFC6749 §1 ¶1"],"golds":[]}`,
		"no question":   `{"id":"r1","gold":["RFC6749 §1 ¶1"]}`,
		"no gold":       `{"id":"r1","question":"q","gold":[]}`,
		"bad ref":       `{"id":"r1","question":"q","gold":["RFC6749 10.4"]}`,
		"duplicate id":  `{"id":"r1","question":"q","gold":["RFC6749 §1 ¶1"]}` + "\n" + `{"id":"r1","question":"q","gold":["RFC6749 §1 ¶1"]}`,
	}
	for name, in := range bad {
		if _, err := parseRetrievalCases([]byte(in)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// fakeSearcher returns fixed refs for each question.
type fakeSearcher map[string][]string

func (f fakeSearcher) Search(_ context.Context, q string, k int) ([]index.Hit, error) {
	if q == "" {
		return nil, index.ErrEmptyQuery
	}
	var hits []index.Hit
	for _, s := range f[q] {
		ref, err := corpus.ParseRef(s)
		if err != nil {
			return nil, err
		}
		hits = append(hits, index.Hit{Ref: ref})
	}
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits, nil
}

func TestRunRetrieval(t *testing.T) {
	cases := []RetrievalCase{
		{ID: "first", Question: "a", Gold: []string{"RFC1 §1 ¶1"}},
		{ID: "second-of-two", Question: "b", Gold: []string{"RFC1 §2 ¶1", "RFC1 §2 ¶9"}},
		{ID: "rank7", Question: "c", Gold: []string{"RFC1 §3 ¶1"}},
		{ID: "miss", Question: "d", Gold: []string{"RFC1 §4 ¶1"}},
		{ID: "empty", Question: "", Gold: []string{"RFC1 §5 ¶1"}},
	}
	s := fakeSearcher{
		"a": {"RFC1 §1 ¶1", "RFC1 §9 ¶1"},
		"b": {"RFC1 §9 ¶1", "RFC1 §2 ¶1"},
		"c": {"RFC1 §9 ¶1", "RFC1 §9 ¶2", "RFC1 §3 ¶4", "RFC1 §9 ¶4", "RFC1 §9 ¶5", "RFC1 §9 ¶6", "RFC1 §3 ¶1"},
		"d": {"RFC1 §4 ¶2"},
	}
	rep, err := RunRetrieval(context.Background(), s, cases)
	if err != nil {
		t.Fatal(err)
	}
	wantRanks := map[string][2]int{"first": {1, 1}, "second-of-two": {2, 2}, "rank7": {7, 3}, "miss": {0, 1}, "empty": {0, 0}}
	for _, r := range rep.Results {
		if got := [2]int{r.Rank, r.SectionRank}; got != wantRanks[r.ID] {
			t.Errorf("%s: rank, section rank = %v, want %v", r.ID, got, wantRanks[r.ID])
		}
	}
	// recall@5: 1 + 0.5 + 0 + 0 + 0 over 5 cases.
	if rep.Recall5 != 0.3 {
		t.Errorf("Recall5 = %v, want 0.3", rep.Recall5)
	}
	// MRR: 1 + 1/2 + 1/7 over 5 cases.
	if rep.MRR != round((1+0.5+1.0/7)/5) {
		t.Errorf("MRR = %v", rep.MRR)
	}
	if rep.HitRate5 != 0.4 || rep.SectionHitRate5 != 0.8 {
		t.Errorf("HitRate5 = %v, SectionHitRate5 = %v, want 0.4, 0.8", rep.HitRate5, rep.SectionHitRate5)
	}

	var ids []string
	for _, m := range rep.Misses() {
		ids = append(ids, m.ID)
	}
	if got := strings.Join(ids, ","); got != "rank7,miss,empty" {
		t.Errorf("Misses = %s, want rank7,miss,empty", got)
	}

	if err := rep.Check(Thresholds{Recall5: 0.3, MRR: 0.3}); err != nil {
		t.Errorf("Check at the scores: %v", err)
	}
	err = rep.Check(Thresholds{Recall5: 0.31, MRR: 0.9})
	if err == nil || !strings.Contains(err.Error(), "recall@5") || !strings.Contains(err.Error(), "MRR@10") {
		t.Errorf("Check above the scores = %v, want both metrics named", err)
	}

	var buf bytes.Buffer
	rep.WriteSummary(&buf)
	for _, want := range []string{"5 questions", "recall@5          0.3000", "rank7  rank 7", "miss  not in top 10"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("summary missing %q:\n%s", want, buf.String())
		}
	}
}

func TestCheckGold(t *testing.T) {
	rfcs, err := corpus.Load("../../corpus")
	if err != nil {
		t.Fatal(err)
	}
	cases := []RetrievalCase{
		{ID: "ok", Gold: []string{"RFC6749 §10.4 ¶1", "RFC 7636 §4.2 ¶3"}},
		{ID: "bad", Gold: []string{"RFC6749 §10.4 ¶99", "RFC9999 §1 ¶1", "RFC6749 §12.1 ¶1"}},
	}
	err = CheckGold(cases, rfcs)
	if err == nil {
		t.Fatal("no error")
	}
	msg := err.Error()
	if strings.Contains(msg, "ok:") || !strings.Contains(msg, "§10.4 ¶99") || !strings.Contains(msg, "RFC9999") || !strings.Contains(msg, "§12.1 ¶1 is in a reference list") {
		t.Errorf("CheckGold = %v", err)
	}
}
