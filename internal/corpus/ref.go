package corpus

import (
	"fmt"
	"regexp"
	"strconv"
)

// Ref identifies one paragraph in the corpus, for example RFC6749 §10.4 ¶2.
// Paragraph numbers start at 1 within each section.
type Ref struct {
	RFC       int
	Section   string // "10.4", "A.1", or "Abstract"
	Paragraph int
}

// String formats r the way citations appear in answers: "RFC6749 §10.4 ¶2".
func (r Ref) String() string {
	return fmt.Sprintf("RFC%d §%s ¶%d", r.RFC, r.Section, r.Paragraph)
}

var refPattern = regexp.MustCompile(`^RFC ?(\d+) §([0-9A-Za-z.]+?) ¶(\d+)$`)

// ParseRef parses a citation in the format produced by Ref.String.
// It also accepts "RFC 6749" with a space, which models often write.
func ParseRef(s string) (Ref, error) {
	m := refPattern.FindStringSubmatch(s)
	if m == nil {
		return Ref{}, fmt.Errorf("invalid citation %q: want the form RFC6749 §10.4 ¶2", s)
	}
	rfc, err := strconv.Atoi(m[1])
	if err != nil || rfc <= 0 {
		return Ref{}, fmt.Errorf("invalid RFC number in citation %q", s)
	}
	para, err := strconv.Atoi(m[3])
	if err != nil || para <= 0 {
		return Ref{}, fmt.Errorf("invalid paragraph number in citation %q", s)
	}
	return Ref{RFC: rfc, Section: m[2], Paragraph: para}, nil
}
