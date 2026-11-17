// Package eval measures Citebound against labeled question sets.
package eval

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/index"
)

// RetrievalCase is one labeled question from evals/retrieval.jsonl.
type RetrievalCase struct {
	ID              string   `json:"id"`
	Question        string   `json:"question"`
	Gold            []string `json:"gold"`
	ReferenceAnswer string   `json:"reference_answer"`
	Tags            []string `json:"tags"`
	Difficulty      string   `json:"difficulty"`
}

// LoadRetrievalCases reads a JSON Lines file of retrieval cases. Every case
// needs an id, a question, and at least one gold ref that parses; ids must be
// unique.
func LoadRetrievalCases(path string) ([]RetrievalCase, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is an operator-chosen eval set
	if err != nil {
		return nil, err
	}
	return parseRetrievalCases(data)
}

func parseRetrievalCases(data []byte) ([]RetrievalCase, error) {
	return parseCases[RetrievalCase](data)
}

// evalCase is one line of an eval set.
type evalCase interface {
	key() string
	validate() error
}

// parseCases reads JSON Lines: one case per line, with no unknown fields, no
// trailing data, and unique ids.
func parseCases[T evalCase](data []byte) ([]T, error) {
	var cases []T
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, 1<<20)
	for line := 1; sc.Scan(); line++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.DisallowUnknownFields()
		var c T
		if err := dec.Decode(&c); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if dec.Decode(&struct{}{}) != io.EOF {
			return nil, fmt.Errorf("line %d: more than one JSON value", line)
		}
		if err := c.validate(); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if seen[c.key()] {
			return nil, fmt.Errorf("line %d: duplicate id %q", line, c.key())
		}
		seen[c.key()] = true
		cases = append(cases, c)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(cases) == 0 {
		return nil, errors.New("no cases")
	}
	return cases, nil
}

func (c RetrievalCase) key() string { return c.ID }

func (c RetrievalCase) validate() error {
	if c.ID == "" || c.Question == "" {
		return errors.New("id and question are required")
	}
	if len(c.Gold) == 0 {
		return fmt.Errorf("%s: no gold refs", c.ID)
	}
	for _, g := range c.Gold {
		if _, err := corpus.ParseRef(g); err != nil {
			return fmt.Errorf("%s: %w", c.ID, err)
		}
	}
	return nil
}

// CheckGold reports every gold ref that is not a paragraph in rfcs. A parser
// change that renumbers paragraphs breaks labels silently unless this runs.
func CheckGold(cases []RetrievalCase, rfcs []*corpus.RFC) error {
	byNumber := map[int]*corpus.RFC{}
	for _, r := range rfcs {
		byNumber[r.Number] = r
	}
	var errs []error
	for _, c := range cases {
		for _, g := range c.Gold {
			ref, _ := corpus.ParseRef(g) // validated on load
			r := byNumber[ref.RFC]
			switch {
			case r == nil || r.Paragraph(ref) == nil:
				errs = append(errs, fmt.Errorf("%s: gold %s is not in the corpus", c.ID, g))
			case index.IsReferenceList(r.Section(ref.Section).Title):
				errs = append(errs, fmt.Errorf("%s: gold %s is in a reference list, which is not searched", c.ID, g))
			}
		}
	}
	return errors.Join(errs...)
}

// Searcher is the part of the index the retrieval eval needs.
type Searcher interface {
	Search(ctx context.Context, query string, k int) ([]index.Hit, error)
}

// Depth is how many results each question retrieves. Ranks past it count as
// misses.
const Depth = 10

// CaseResult is how one question fared.
type CaseResult struct {
	ID string `json:"id"`
	// Rank is the 1-based rank of the first gold paragraph, or 0 if none was
	// in the top Depth.
	Rank int `json:"rank"`
	// SectionRank is the same for the first hit in a gold paragraph's section.
	SectionRank int `json:"section_rank"`
	// Recall5 is the fraction of gold paragraphs in the top 5.
	Recall5 float64  `json:"recall_at_5"`
	Top     []string `json:"top"`
}

// RetrievalReport summarizes a run. Means are over all cases.
type RetrievalReport struct {
	Cases int `json:"cases"`
	// Recall5 is the mean fraction of gold paragraphs in the top 5.
	Recall5 float64 `json:"recall_at_5"`
	// HitRate5 is the fraction of cases with any gold paragraph in the top 5.
	HitRate5 float64 `json:"hit_rate_at_5"`
	// MRR is the mean reciprocal rank of the first gold paragraph, to Depth.
	MRR float64 `json:"mrr_at_10"`
	// SectionHitRate5 is HitRate5 counting any paragraph of a gold section.
	SectionHitRate5 float64      `json:"section_hit_rate_at_5"`
	Results         []CaseResult `json:"results"`
}

// RunRetrieval searches every case and scores the results against its gold
// paragraphs.
func RunRetrieval(ctx context.Context, s Searcher, cases []RetrievalCase) (*RetrievalReport, error) {
	rep := &RetrievalReport{Cases: len(cases)}
	var hits5, sectionHits5 int
	for _, c := range cases {
		hits, err := s.Search(ctx, c.Question, Depth)
		if err != nil && !errors.Is(err, index.ErrEmptyQuery) {
			return nil, fmt.Errorf("%s: %w", c.ID, err)
		}
		res := score(c, hits)
		rep.Results = append(rep.Results, res)
		rep.Recall5 += res.Recall5
		if res.Rank > 0 {
			rep.MRR += 1 / float64(res.Rank)
		}
		if res.Rank > 0 && res.Rank <= 5 {
			hits5++
		}
		if res.SectionRank > 0 && res.SectionRank <= 5 {
			sectionHits5++
		}
	}
	// Scores are kept unrounded, so Check compares exactly; only the summary
	// rounds them.
	n := float64(len(cases))
	rep.Recall5 /= n
	rep.MRR /= n
	rep.HitRate5 = float64(hits5) / n
	rep.SectionHitRate5 = float64(sectionHits5) / n
	return rep, nil
}

func score(c RetrievalCase, hits []index.Hit) CaseResult {
	gold := map[string]bool{}
	goldSections := map[string]bool{}
	for _, g := range c.Gold {
		ref, _ := corpus.ParseRef(g)
		gold[ref.String()] = true
		goldSections[sectionKey(ref)] = true
	}
	res := CaseResult{ID: c.ID}
	found5 := 0
	for i, h := range hits {
		ref := h.Ref.String()
		res.Top = append(res.Top, ref)
		if gold[ref] {
			if res.Rank == 0 {
				res.Rank = i + 1
			}
			if i < 5 {
				found5++
			}
		}
		if res.SectionRank == 0 && goldSections[sectionKey(h.Ref)] {
			res.SectionRank = i + 1
		}
	}
	res.Recall5 = float64(found5) / float64(len(gold))
	return res
}

func sectionKey(r corpus.Ref) string {
	return fmt.Sprintf("RFC%d §%s", r.RFC, r.Section)
}

// Thresholds are the minimum scores a run must reach. They live in
// evals/thresholds.json so that lowering one shows up in review.
type Thresholds struct {
	Recall5 float64 `json:"recall_at_5"`
	MRR     float64 `json:"mrr_at_10"`
}

// LoadThresholds reads the retrieval thresholds from a thresholds file.
func LoadThresholds(path string) (Thresholds, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is an operator-chosen eval config
	if err != nil {
		return Thresholds{}, err
	}
	var f struct {
		Retrieval *Thresholds `json:"retrieval"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return Thresholds{}, fmt.Errorf("%s: %w", path, err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return Thresholds{}, fmt.Errorf("%s: more than one JSON value", path)
	}
	if f.Retrieval == nil {
		return Thresholds{}, fmt.Errorf("%s: no retrieval thresholds", path)
	}
	return *f.Retrieval, nil
}

// Check returns an error naming every metric below its threshold.
func (r *RetrievalReport) Check(t Thresholds) error {
	var errs []error
	if r.Recall5 < t.Recall5 {
		errs = append(errs, fmt.Errorf("recall@5 %.6f is below the threshold %.6f", r.Recall5, t.Recall5))
	}
	if r.MRR < t.MRR {
		errs = append(errs, fmt.Errorf("MRR@10 %.6f is below the threshold %.6f", r.MRR, t.MRR))
	}
	return errors.Join(errs...)
}

// Misses returns the results with no gold paragraph in the top 5: near
// misses first, then questions with no gold paragraph in the top Depth.
func (r *RetrievalReport) Misses() []CaseResult {
	var out []CaseResult
	for _, res := range r.Results {
		if res.Rank == 0 || res.Rank > 5 {
			out = append(out, res)
		}
	}
	slices.SortStableFunc(out, func(a, b CaseResult) int {
		return missOrder(a) - missOrder(b)
	})
	return out
}

func missOrder(r CaseResult) int {
	if r.Rank == 0 {
		return Depth + 1
	}
	return r.Rank
}

// WriteSummary prints the headline metrics and the misses.
func (r *RetrievalReport) WriteSummary(w io.Writer) {
	fmt.Fprintf(w, "Retrieval eval: %d questions\n", r.Cases)
	fmt.Fprintf(w, "  recall@5          %.4f\n", r.Recall5)
	fmt.Fprintf(w, "  hit rate@5        %.4f\n", r.HitRate5)
	fmt.Fprintf(w, "  MRR@10            %.4f\n", r.MRR)
	fmt.Fprintf(w, "  section hit@5     %.4f\n", r.SectionHitRate5)
	misses := r.Misses()
	if len(misses) == 0 {
		return
	}
	fmt.Fprintf(w, "\nNo gold paragraph in the top 5 (%d):\n", len(misses))
	for _, m := range misses {
		rank := "not in top 10"
		if m.Rank > 0 {
			rank = fmt.Sprintf("rank %d", m.Rank)
		}
		fmt.Fprintf(w, "  %s  %s\n", m.ID, rank)
	}
}
