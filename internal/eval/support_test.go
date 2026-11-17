package eval

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rick-learns/citebound/internal/verify"
)

func TestParseSupportLabels(t *testing.T) {
	ok := `{"ref": "RFC6749 §1.5 ¶1", "claim": "Real.", "label": "supported"}
{"ref": "RFC6749 §1.5 ¶1", "claim": "Also real.", "label": "partial", "note": "SHOULD, not MUST."}`
	labels, err := parseCases[SupportLabel]([]byte(ok))
	if err != nil || len(labels) != 2 {
		t.Fatalf("parse = %v, %v; want 2 labels", labels, err)
	}
	for _, c := range []struct{ name, line, wantErr string }{
		{"spaced ref", `{"ref": "RFC 6749 §1.5 ¶1", "claim": "Real.", "label": "supported"}`, `write it as "RFC6749 §1.5 ¶1"`},
		{"bad ref", `{"ref": "RFC6749 1.5", "claim": "Real.", "label": "supported"}`, "invalid citation"},
		{"no claim", `{"ref": "RFC6749 §1.5 ¶1", "label": "supported"}`, "claim is required"},
		{"unknown label", `{"ref": "RFC6749 §1.5 ¶1", "claim": "Real.", "label": "maybe"}`, `not "maybe"`},
		{"supported with a note", `{"ref": "RFC6749 §1.5 ¶1", "claim": "Real.", "label": "supported", "note": "fine"}`, "has no note"},
		{"partial without a note", `{"ref": "RFC6749 §1.5 ¶1", "claim": "Real.", "label": "partial"}`, "partial label needs a note"},
		{"unsupported without a note", `{"ref": "RFC6749 §1.5 ¶1", "claim": "Real.", "label": "unsupported"}`, "unsupported label needs a note"},
		{"duplicate", ok + "\n" + `{"ref": "RFC6749 §1.5 ¶1", "claim": "Real.", "label": "unsupported", "note": "no"}`, "duplicate"},
		{"unknown field", `{"ref": "RFC6749 §1.5 ¶1", "claim": "Real.", "label": "supported", "id": "c1"}`, "unknown field"},
	} {
		if _, err := parseCases[SupportLabel]([]byte(c.line)); err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: err = %v, want %q", c.name, err, c.wantErr)
		}
	}
}

func supportReport(claims ...ClaimResult) *AnswerReport {
	return &AnswerReport{Cases: []AnswerCase{{ID: "r1", Claims: claims}}}
}

func labelsOf(ls ...SupportLabel) SupportLabels {
	m := SupportLabels{}
	for _, l := range ls {
		m[l.key()] = l
	}
	return m
}

func TestScoreSupport(t *testing.T) {
	ref := "RFC6749 §1.5 ¶1"
	labels := labelsOf(
		SupportLabel{Ref: ref, Claim: "A.", Label: Supported},
		SupportLabel{Ref: ref, Claim: "B.", Label: Partial, Note: "n"},
		SupportLabel{Ref: ref, Claim: "C.", Label: Unsupported, Note: "n"},
		SupportLabel{Ref: "RFC7636 §4.1 ¶1", Claim: "Unused.", Label: Supported},
	)
	r := supportReport(
		ClaimResult{Text: "A.", Citation: ref, Status: verify.Verified},
		ClaimResult{Text: "A.", Citation: "RFC 6749 §1.5 ¶1", Status: verify.Verified}, // the spaced form is the same ref
		ClaimResult{Text: "B.", Citation: ref, Status: verify.Verified},
		ClaimResult{Text: "C.", Citation: ref, Status: verify.Verified},
		ClaimResult{Text: "Struck, so needs no label.", Citation: ref, Status: verify.Struck, Reason: verify.ReasonQuoteNotFound},
	)
	if err := r.ScoreSupport(labels); err != nil {
		t.Fatal(err)
	}
	want := Support{Claims: 4, Supported: 2, Partial: 1, Unsupported: 1, Rate: 0.5}
	if *r.Support != want {
		t.Errorf("support = %+v, want %+v", *r.Support, want)
	}
	var got []string
	for _, cl := range r.Cases[0].Claims {
		got = append(got, cl.Support)
	}
	if strings.Join(got, ",") != "supported,supported,partial,unsupported," {
		t.Errorf("claim labels = %q", got)
	}
	var out bytes.Buffer
	r.WriteSummary(&out)
	for _, s := range []string{"Claim support: 2 of 4 verified claims supported (50.0%)", "partial      1", "unsupported r1 RFC6749 §1.5 ¶1"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("summary lacks %q:\n%s", s, out.String())
		}
	}
}

func TestScoreSupportNeedsEveryLabel(t *testing.T) {
	ref := "RFC6749 §1.5 ¶1"
	labels := labelsOf(SupportLabel{Ref: ref, Claim: "A.", Label: Supported})
	var claims []ClaimResult
	for _, text := range []string{"A.", "New 1.", "New 2.", "New 3.", "New 4."} {
		claims = append(claims, ClaimResult{Text: text, Citation: ref, Status: verify.Verified})
	}
	claims = append(claims, ClaimResult{Text: "A.", Citation: "RFC6749 §1.5 ¶2", Status: verify.Verified}) // same text, other ref
	r := supportReport(claims...)
	err := r.ScoreSupport(labels)
	if err == nil {
		t.Fatal("want an error for unlabeled claims")
	}
	for _, s := range []string{"5 verified claims have no support label", `r1: RFC6749 §1.5 ¶1 "New 1."`, `"New 3."`} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error lacks %q: %v", s, err)
		}
	}
	if strings.Contains(err.Error(), "New 4.") {
		t.Errorf("error lists more than %d claims: %v", unlabeledShown, err)
	}
	if r.Support != nil {
		t.Errorf("support set despite the error: %+v", r.Support)
	}
}

func TestLoadSupportLabels(t *testing.T) {
	if _, err := LoadSupportLabels("testdata/none.jsonl"); err == nil {
		t.Error("want an error for a missing file")
	}
}
