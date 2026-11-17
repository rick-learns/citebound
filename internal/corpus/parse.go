package corpus

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RFC is one parsed RFC: its sections in document order.
type RFC struct {
	Number   int
	Title    string
	Sections []Section
}

// Section is a numbered section, an appendix, or the abstract.
type Section struct {
	ID         string // "10.4", "A.1", or "Abstract"
	Title      string
	Paragraphs []Paragraph
}

// Paragraph is one block of text between blank lines.
type Paragraph struct {
	Ref Ref
	// Text is the paragraph as it should be quoted. Prose is joined onto one
	// line, with every run of whitespace collapsed to a single space. Figures keep their line breaks and relative
	// indentation.
	Text   string
	Figure bool
}

// Section returns the section with the given ID, or nil.
func (r *RFC) Section(id string) *Section {
	for i := range r.Sections {
		if r.Sections[i].ID == id {
			return &r.Sections[i]
		}
	}
	return nil
}

// Paragraph returns the paragraph ref points to, or nil.
func (r *RFC) Paragraph(ref Ref) *Paragraph {
	if ref.RFC != r.Number {
		return nil
	}
	s := r.Section(ref.Section)
	if s == nil || ref.Paragraph < 1 || ref.Paragraph > len(s.Paragraphs) {
		return nil
	}
	return &s.Paragraphs[ref.Paragraph-1]
}

var (
	// Headings start in column 0: "10.4.  Refresh Tokens", "A.1.  Title",
	// "Appendix A.  Title".
	headingPattern = regexp.MustCompile(`^(\d+(?:\.\d+)*|[A-Z](?:\.\d+)+|Appendix [A-Z])\.  +(\S.*)$`)
	// Page footers in pre-2019 RFCs: "Hardt   Standards Track   [Page 5]".
	footerPattern = regexp.MustCompile(`^\S.*\s\[Page \d+\]$`)
	// Page headers in pre-2019 RFCs: "RFC 6749   OAuth 2.0   October 2012".
	headerPattern = regexp.MustCompile(`^RFC \d+ {2,}.*\S {2,}\S.* \d{4}$`)
	// Lines that only make sense as part of a drawing, table, example
	// message, or grammar. One such line makes the whole block a figure.
	figurePatterns = []*regexp.Regexp{
		regexp.MustCompile(`[+|]-{2,}|-{2,}[+|]|\S\s+\|\s+\S`),               // boxes and tables
		regexp.MustCompile(`\S {3,}\S`),                                      // column alignment
		regexp.MustCompile(`^\s*(GET|POST|PUT|DELETE|PATCH|HEAD) \S+ HTTP/`), // HTTP requests
		regexp.MustCompile(`^\s*HTTP/\d(\.\d)? \d{3}`),                       // HTTP responses
		regexp.MustCompile(`^\s*[{}\[\]],?\s*$|^\s*"[^"]*"\s*:`),             // JSON
		regexp.MustCompile(`^\s*[&?]?[\w.-]+=\S*$`),                          // form-encoded bodies
		regexp.MustCompile(`^\s+[A-Za-z][\w-]*\s+=\s`),                       // ABNF rules
	}
)

var (
	// "1.   Text", "*  Text", "o  Text", "(a)  Text": the gap after a list
	// marker is not column alignment.
	listMarker = regexp.MustCompile(`^\s*(\d+\.|[*o-]|\([a-z0-9]+\))\s+`)
	// Asides in RFCs since 8650 are prefixed with "|" on every line.
	asideMarker = regexp.MustCompile(`^\s*\|( |$)`)
)

func isFigureLine(line string) bool {
	line = listMarker.ReplaceAllString(line, "")
	for _, p := range figurePatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}

// Unnumbered sections that are kept. Everything else unnumbered (status of
// this memo, copyright, table of contents, author addresses, index) is
// boilerplate and dropped.
var keptUnnumbered = map[string]bool{"Abstract": true}

// Parse splits the plain text of an RFC into sections and paragraphs.
// It handles both the paginated format used before RFC 8650 and the
// unpaginated format used since.
func Parse(number int, title string, text []byte) (*RFC, error) {
	if !utf8.Valid(text) {
		return nil, fmt.Errorf("RFC %d: text is not valid UTF-8", number)
	}
	text = bytes.TrimPrefix(text, []byte("\uFEFF"))
	text = bytes.ReplaceAll(text, []byte("\r\n"), []byte("\n"))
	lines := depaginate(strings.Split(string(text), "\n"))

	doc := &RFC{Number: number, Title: title}
	var cur *Section // nil while in front matter or a dropped section
	var block []string
	flush := func() {
		if cur != nil && len(block) > 0 {
			p := makeParagraph(block)
			p.Ref = Ref{RFC: number, Section: cur.ID, Paragraph: len(cur.Paragraphs) + 1}
			cur.Paragraphs = append(cur.Paragraphs, p)
		}
		block = block[:0]
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if startsInColumnZero(line) {
			flush()
			id, heading, ok := parseHeading(line)
			if !ok {
				// An unnumbered heading, or a stray front-matter line.
				name := strings.TrimSpace(line)
				if keptUnnumbered[name] {
					doc.Sections = append(doc.Sections, Section{ID: name, Title: name})
					cur = &doc.Sections[len(doc.Sections)-1]
				} else {
					cur = nil
				}
				continue
			}
			// Long headings wrap onto indented lines with no blank line between.
			for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" && !startsInColumnZero(lines[i+1]) {
				i++
				heading += " " + strings.TrimSpace(lines[i])
			}
			if doc.Section(id) != nil {
				return nil, fmt.Errorf("RFC %d: section %s appears twice", number, id)
			}
			doc.Sections = append(doc.Sections, Section{ID: id, Title: heading})
			cur = &doc.Sections[len(doc.Sections)-1]
			continue
		}
		block = append(block, line)
	}
	flush()

	if doc.Section("1") == nil {
		return nil, fmt.Errorf("RFC %d: no section 1 found; is this an RFC in plain-text format?", number)
	}
	return doc, nil
}

func startsInColumnZero(line string) bool {
	r, _ := utf8.DecodeRuneInString(line)
	return line != "" && !unicode.IsSpace(r)
}

func parseHeading(line string) (id, title string, ok bool) {
	m := headingPattern.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return strings.TrimPrefix(m[1], "Appendix "), strings.TrimSpace(m[2]), true
}

// depaginate removes page headers, footers, and form feeds. Where a page
// break falls inside a paragraph, the two halves are rejoined; where it falls
// between paragraphs, a blank line is kept.
func depaginate(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "\f") {
			if footerPattern.MatchString(line) {
				continue // the last page's footer has no form feed after it
			}
			out = append(out, line)
			continue
		}

		// Drop the footer and the blank padding above it.
		for len(out) > 0 && (strings.TrimSpace(out[len(out)-1]) == "" || footerPattern.MatchString(out[len(out)-1])) {
			out = out[:len(out)-1]
		}
		// Text after the form feed on the same line is ordinarily the header.
		if rest := strings.TrimPrefix(line, "\f"); rest != "" && !headerPattern.MatchString(rest) {
			out = append(out, rest)
			continue
		}
		// Drop the header and the blank padding below it.
		for i+1 < len(lines) && (strings.TrimSpace(lines[i+1]) == "" || headerPattern.MatchString(lines[i+1])) {
			i++
		}
		if len(out) > 0 && i+1 < len(lines) && !continuesParagraph(out[len(out)-1], lines[i+1]) {
			out = append(out, "")
		}
	}
	return out
}

// continuesParagraph guesses whether next continues the paragraph that prev
// ends. The plain-text format does not record this, so it is a heuristic: a
// paragraph that ends in sentence punctuation followed by a capitalized or
// differently indented line is treated as finished.
func continuesParagraph(prev, next string) bool {
	if startsInColumnZero(next) {
		return false
	}
	p := strings.TrimSpace(prev)
	n := strings.TrimSpace(next)
	if p == "" || n == "" {
		return false
	}
	if indent(prev) != indent(next) {
		// Hanging indents in lists continue deeper than they start.
		return indent(next) > indent(prev) && !endsSentence(p)
	}
	if !endsSentence(p) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(n)
	return unicode.IsLower(r)
}

func endsSentence(s string) bool {
	return strings.HasSuffix(s, ".") || strings.HasSuffix(s, ":")
}

func indent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func makeParagraph(block []string) Paragraph {
	if isAside(block) {
		stripped := make([]string, len(block))
		for i, l := range block {
			stripped[i] = "   " + asideMarker.ReplaceAllString(l, "")
		}
		block = stripped
	}
	figure := false
	for _, l := range block {
		if isFigureLine(l) {
			figure = true
			break
		}
	}
	if figure {
		return Paragraph{Text: dedent(block), Figure: true}
	}
	var b strings.Builder
	for i, l := range block {
		l = strings.TrimSpace(l)
		if i > 0 {
			prev := strings.TrimSpace(block[i-1])
			// A line that ends in a hyphen was broken at a compound word.
			if !strings.HasSuffix(prev, "-") || strings.HasSuffix(prev, " -") {
				b.WriteByte(' ')
			}
		}
		b.WriteString(l)
	}
	// Collapse the double spaces RFCs put after sentences and list markers.
	return Paragraph{Text: strings.Join(strings.Fields(b.String()), " ")}
}

func isAside(block []string) bool {
	for _, l := range block {
		if !asideMarker.MatchString(l) {
			return false
		}
	}
	return true
}

func dedent(block []string) string {
	minIndent := -1
	for _, l := range block {
		if n := indent(l); minIndent < 0 || n < minIndent {
			minIndent = n
		}
	}
	out := make([]string, len(block))
	for i, l := range block {
		out[i] = strings.TrimRight(l[minIndent:], " ")
	}
	return strings.Join(out, "\n")
}
