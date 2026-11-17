package eval

import (
	"errors"
	"fmt"
	"os"

	"github.com/rick-learns/citebound/internal/corpus"
	"github.com/rick-learns/citebound/internal/verify"
)

// Claim-support labels. The verifier proves a quote is in the paragraph it
// cites; these say whether that paragraph supports the claim.
const (
	Supported   = "supported"
	Partial     = "partial"
	Unsupported = "unsupported"
)

// SupportLabel judges one verified claim against its cited paragraph, from
// evals/support-labels.jsonl. A claim is identified by its ref and its text,
// so a label covers every answer that makes the same claim.
type SupportLabel struct {
	Ref   string `json:"ref"`
	Claim string `json:"claim"`
	Label string `json:"label"`
	// Note says what the paragraph does not support. It is required for
	// partial and unsupported labels, and empty for supported ones.
	Note string `json:"note,omitempty"`
}

func (l SupportLabel) key() string { return supportKey(l.Ref, l.Claim) }

func supportKey(ref, claim string) string { return ref + "\n" + claim }

func (l SupportLabel) validate() error {
	ref, err := corpus.ParseRef(l.Ref)
	if err != nil {
		return err
	}
	if ref.String() != l.Ref {
		return fmt.Errorf("ref %q: write it as %q", l.Ref, ref)
	}
	if l.Claim == "" {
		return fmt.Errorf("%s: claim is required", l.Ref)
	}
	switch l.Label {
	case Supported:
		if l.Note != "" {
			return fmt.Errorf("%s: a supported label has no note", l.Ref)
		}
	case Partial, Unsupported:
		if l.Note == "" {
			return fmt.Errorf("%s: a %s label needs a note", l.Ref, l.Label)
		}
	default:
		return fmt.Errorf("%s: label must be supported, partial, or unsupported, not %q", l.Ref, l.Label)
	}
	return nil
}

// SupportLabels holds labels by ref and claim text.
type SupportLabels map[string]SupportLabel

// LoadSupportLabels reads evals/support-labels.jsonl.
func LoadSupportLabels(path string) (SupportLabels, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is an operator-chosen eval set
	if err != nil {
		return nil, err
	}
	list, err := parseCases[SupportLabel](data)
	if err != nil {
		return nil, err
	}
	labels := SupportLabels{}
	for _, l := range list {
		labels[l.key()] = l
	}
	return labels, nil
}

// Support counts verified claims by label. Struck claims are not counted:
// the user never sees them as claims.
type Support struct {
	Claims      int     `json:"claims"`
	Supported   int     `json:"supported"`
	Partial     int     `json:"partial"`
	Unsupported int     `json:"unsupported"`
	Rate        float64 `json:"rate"`
}

// unlabeledShown is how many unlabeled claims an error lists.
const unlabeledShown = 3

// ScoreSupport labels every verified claim in r and counts the labels. A
// verified claim with no label is an error, so new recordings cannot skip
// labeling.
func (r *AnswerReport) ScoreSupport(labels SupportLabels) error {
	s := Support{}
	var missing []string
	for i := range r.Cases {
		for j := range r.Cases[i].Claims {
			cl := &r.Cases[i].Claims[j]
			if cl.Status != verify.Verified {
				continue
			}
			ref, err := corpus.ParseRef(cl.Citation)
			if err != nil {
				return fmt.Errorf("%s: %w", r.Cases[i].ID, err)
			}
			l, ok := labels[supportKey(ref.String(), cl.Text)]
			if !ok {
				missing = append(missing, fmt.Sprintf("%s: %s %q", r.Cases[i].ID, ref, cl.Text))
				continue
			}
			cl.Support = l.Label
			s.Claims++
			switch l.Label {
			case Supported:
				s.Supported++
			case Partial:
				s.Partial++
			case Unsupported:
				s.Unsupported++
			}
		}
	}
	if len(missing) > 0 {
		shown := missing[:min(len(missing), unlabeledShown)]
		msg := fmt.Sprintf("%d verified claims have no support label, starting with:", len(missing))
		for _, m := range shown {
			msg += "\n  " + m
		}
		return errors.New(msg)
	}
	s.Rate = ratio(s.Supported, s.Claims)
	r.Support = &s
	return nil
}
