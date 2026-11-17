package eval

import (
	"errors"
	"fmt"
	"os"
	"slices"
)

// UnanswerableCase is a question the corpus cannot answer, from
// evals/unanswerable.jsonl.
type UnanswerableCase struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Why      string `json:"why"`
	// FalsePremise marks a question built on something the RFCs do not say.
	// Refuting the premise with citations is as good as abstaining, so these
	// are reported apart and judged by claim support.
	FalsePremise bool `json:"false_premise,omitempty"`
}

func (c UnanswerableCase) key() string { return c.ID }

func (c UnanswerableCase) validate() error {
	if c.ID == "" || c.Question == "" || c.Why == "" {
		return errors.New("id, question, and why are required")
	}
	return nil
}

// RedTeamCase is an adversarial prompt from evals/redteam.jsonl.
type RedTeamCase struct {
	ID       string `json:"id"`
	Prompt   string `json:"prompt"`
	Category string `json:"category"`
	// Expect is "abstain" (the corpus cannot answer, so abstain) or "refuse"
	// (do not comply: abstain, or answer only with verified claims).
	Expect string `json:"expect"`
}

func (c RedTeamCase) key() string { return c.ID }

var redTeamCategories = []string{"prompt-injection", "system-prompt-extraction", "fake-citation", "off-topic", "jailbreak", "format-attack"}

func (c RedTeamCase) validate() error {
	if c.ID == "" || c.Prompt == "" {
		return errors.New("id and prompt are required")
	}
	if !slices.Contains(redTeamCategories, c.Category) {
		return fmt.Errorf("%s: unknown category %q", c.ID, c.Category)
	}
	if c.Expect != "abstain" && c.Expect != "refuse" {
		return fmt.Errorf("%s: expect must be abstain or refuse, not %q", c.ID, c.Expect)
	}
	return nil
}

// LoadUnanswerableCases reads evals/unanswerable.jsonl.
func LoadUnanswerableCases(path string) ([]UnanswerableCase, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is an operator-chosen eval set
	if err != nil {
		return nil, err
	}
	return parseCases[UnanswerableCase](data)
}

// LoadRedTeamCases reads evals/redteam.jsonl.
func LoadRedTeamCases(path string) ([]RedTeamCase, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path is an operator-chosen eval set
	if err != nil {
		return nil, err
	}
	return parseCases[RedTeamCase](data)
}
