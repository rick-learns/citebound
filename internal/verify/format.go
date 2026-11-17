package verify

import (
	"regexp"
	"strings"

	"github.com/rick-learns/citebound/internal/corpus"
)

// Answers are plain text with one claim per line. Each claim ends with a
// citation and a verbatim quote from the cited paragraph:
//
//	Refresh tokens are never sent to resource servers. [RFC6749 §1.5 ¶2: "refresh tokens are intended for use only with authorization servers and are never sent to resource servers"]
//
// A line that is exactly AbstainMarker means the answerer found no support
// for an answer.
const AbstainMarker = "INSUFFICIENT_EVIDENCE"

// citationPattern matches a trailing [<ref>: "<quote>"]. The quote is greedy
// so that quotes containing double quotes, which RFCs use for parameter
// names, are captured whole.
var citationPattern = regexp.MustCompile(`\s*\[\s*([^\]":]*?)\s*:\s*"(.*)"\s*\]\s*$`)

// Format renders a claim in the answer format.
func Format(c Claim) string {
	return c.Text + " [" + c.Ref.String() + `: "` + c.Quote + `"]`
}

// ParseLine parses one line of an answer. ok is false for blank lines.
func ParseLine(line string) (c Claim, ok bool) {
	line = strings.TrimSpace(line)
	line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
	if line == "" {
		return Claim{}, false
	}
	m := citationPattern.FindStringSubmatchIndex(line)
	if m == nil {
		return Claim{Text: line}, true
	}
	c = Claim{
		Text:     strings.TrimSpace(line[:m[0]]),
		Citation: line[m[2]:m[3]],
		Quote:    line[m[4]:m[5]],
	}
	if ref, err := corpus.ParseRef(c.Citation); err == nil {
		c.Ref = ref
	}
	return c, true
}

// ParseAnswer splits an answer into claims. abstained reports whether the
// answer contains the abstain marker on a line of its own.
func ParseAnswer(answer string) (claims []Claim, abstained bool) {
	for _, line := range strings.Split(answer, "\n") {
		if strings.TrimSpace(line) == AbstainMarker {
			abstained = true
			continue
		}
		if c, ok := ParseLine(line); ok {
			claims = append(claims, c)
		}
	}
	return claims, abstained
}
