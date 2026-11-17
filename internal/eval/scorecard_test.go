package eval

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rick-learns/citebound/internal/verify"
)

func TestWriteMarkdown(t *testing.T) {
	clean := &AnswerReport{
		Answerer: "Clean",
		Validity: Validity{Claims: 2, Verified: 2, Struck: map[verify.Reason]int{}},
		Support:  &Support{Claims: 2, Supported: 2},
		Refusal:  Refusal{Answerable: 3, AnswerableAbstained: 1, Unanswerable: 2, UnanswerableAbstained: 2, Precision: 2.0 / 3, Recall: 1, RedTeam: 4, RedTeamPassed: 3, Leaks: 1},
	}
	messy := &AnswerReport{
		Answerer: "Messy",
		Validity: Validity{Claims: 8, Verified: 5, Struck: map[verify.Reason]int{verify.ReasonUncited: 2, verify.ReasonQuoteNotFound: 1}},
		Support:  &Support{Claims: 5, Supported: 3, Partial: 1, Unsupported: 1},
		Cases: []AnswerCase{{ID: "r7", Claims: []ClaimResult{
			{Text: "Fine.", Citation: "RFC6749 §1 ¶1", Status: verify.Verified, Support: Supported},
			{Text: "Wrong.", Citation: "RFC6749 §1 ¶2", Status: verify.Verified, Support: Unsupported},
		}}},
	}
	card := &Scorecard{Retrieval: &RetrievalReport{Cases: 100, Recall5: 0.524, HitRate5: 0.65, MRR: 0.4449, SectionHitRate5: 0.69}, Answers: []*AnswerReport{clean, messy}}
	var a, b bytes.Buffer
	card.WriteMarkdown(&a)
	card.WriteMarkdown(&b)
	if a.String() != b.String() {
		t.Error("two writes differ")
	}
	for _, s := range []string{
		"100 questions",
		"| 0.52 | 0.65 | 0.44 | 0.69 |",
		"| Clean | 2 | 2 (100.0%) | 0 |",
		"| Messy | 8 | 5 (62.5%) | 1 quote-not-found, 2 uncited |",
		"| Messy | 5 | 3 (60.0%) | 1 | 1 |",
		"- Messy, r7, RFC6749 §1 ¶2: Wrong.\n",
		"| Clean | 1 of 3 | 2 of 2 | 0.67 | 1.00 | 0 of 0 | 3 of 4 | 1 |",
	} {
		if !strings.Contains(a.String(), s) {
			t.Errorf("scorecard lacks %q:\n%s", s, a.String())
		}
	}
	if strings.Contains(a.String(), "Fine.") {
		t.Error("scorecard lists a supported claim as unsupported")
	}

	card.Answers = []*AnswerReport{clean}
	var c bytes.Buffer
	card.WriteMarkdown(&c)
	if strings.Contains(c.String(), "Verified but unsupported") {
		t.Errorf("scorecard has an empty unsupported list:\n%s", c.String())
	}
}
