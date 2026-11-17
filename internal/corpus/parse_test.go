package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A paginated (pre-RFC 8650) document: a page break falls inside the first
// paragraph of section 1 and between the paragraphs of section 1.1.
const paginated = `Internet Engineering Task Force (IETF)                          A. Author
Request for Comments: 9999                                       Example
Category: Standards Track                                   January 2012


                          A Test Protocol

Abstract

   This document describes a test protocol.

Table of Contents

   1. Introduction ....................................................2

1.  Introduction

   The first paragraph starts on one page and



Author                       Standards Track                    [Page 1]
` + "\f" + `
RFC 9999                    A Test Protocol                 January 2012


   continues on the next page.

   A second paragraph.

1.1.  A Heading That Is Long Enough to Wrap Onto
      a Second Line

   The last paragraph on the page ends here.



Author                       Standards Track                    [Page 2]
` + "\f" + `
RFC 9999                    A Test Protocol                 January 2012


   A new paragraph starts the next page.

     +--------+       +--------+
     | Client |------>| Server |
     +--------+       +--------+

Author's Address

   A. Author
   Example



Author                       Standards Track                    [Page 3]
`

func parseString(t *testing.T, text string) *RFC {
	t.Helper()
	r, err := Parse(9999, "A Test Protocol", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func paragraphTexts(s *Section) []string {
	out := make([]string, len(s.Paragraphs))
	for i, p := range s.Paragraphs {
		out[i] = p.Text
	}
	return out
}

func TestParsePaginated(t *testing.T) {
	r := parseString(t, paginated)

	var ids []string
	for _, s := range r.Sections {
		ids = append(ids, s.ID)
	}
	if got, want := strings.Join(ids, ","), "Abstract,1,1.1"; got != want {
		t.Fatalf("sections = %s, want %s (front matter, TOC, and addresses dropped)", got, want)
	}

	intro := r.Section("1")
	want := []string{
		"The first paragraph starts on one page and continues on the next page.",
		"A second paragraph.",
	}
	if got := paragraphTexts(intro); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("section 1 paragraphs =\n%q\nwant\n%q", got, want)
	}

	sub := r.Section("1.1")
	if want := "A Heading That Is Long Enough to Wrap Onto a Second Line"; sub.Title != want {
		t.Errorf("wrapped title = %q, want %q", sub.Title, want)
	}
	if len(sub.Paragraphs) != 3 {
		t.Fatalf("section 1.1 has %d paragraphs, want 3: %q", len(sub.Paragraphs), paragraphTexts(sub))
	}
	if got := sub.Paragraphs[1].Text; got != "A new paragraph starts the next page." {
		t.Errorf("paragraph after a between-paragraph page break = %q", got)
	}
	fig := sub.Paragraphs[2]
	if !fig.Figure || !strings.HasPrefix(fig.Text, "+--------+") || !strings.Contains(fig.Text, "\n| Client |") {
		t.Errorf("figure = %+v, want a dedented figure with line breaks kept", fig)
	}
	if want := (Ref{RFC: 9999, Section: "1.1", Paragraph: 3}); fig.Ref != want {
		t.Errorf("figure ref = %v, want %v", fig.Ref, want)
	}
	for _, s := range r.Sections {
		for _, p := range s.Paragraphs {
			if strings.Contains(p.Text, "[Page") || strings.Contains(p.Text, "RFC 9999 ") {
				t.Errorf("%v still contains a page header or footer: %q", p.Ref, p.Text)
			}
		}
	}
}

// An unpaginated (RFC 8650 and later) document with a byte-order mark,
// an aside, a list, and an HTTP example.
const unpaginated = "\ufeff" + `
Internet Engineering Task Force (IETF)                         A. Author

Abstract

   This document is unpaginated.

1.  Introduction

   |  Note: An aside is prose, even though every line starts with a
   |  vertical bar.

   1.   A list item whose marker is followed by three spaces.

   The client sends a client-
   side request:

     POST /token HTTP/1.1
     Host: server.example.com

Appendix A.  Acknowledgements

   Thanks.
`

func TestParseUnpaginated(t *testing.T) {
	r := parseString(t, unpaginated)
	intro := r.Section("1")
	if intro == nil || r.Section("A") == nil || r.Section("Abstract") == nil {
		t.Fatalf("missing sections: %+v", r.Sections)
	}
	tests := []struct {
		text   string
		figure bool
	}{
		{"Note: An aside is prose, even though every line starts with a vertical bar.", false},
		{"1. A list item whose marker is followed by three spaces.", false},
		{"The client sends a client-side request:", false},
		{"POST /token HTTP/1.1\nHost: server.example.com", true},
	}
	if len(intro.Paragraphs) != len(tests) {
		t.Fatalf("got %d paragraphs, want %d: %q", len(intro.Paragraphs), len(tests), paragraphTexts(intro))
	}
	for i, tt := range tests {
		p := intro.Paragraphs[i]
		if p.Text != tt.text || p.Figure != tt.figure {
			t.Errorf("¶%d = {%q, figure=%v}, want {%q, figure=%v}", i+1, p.Text, p.Figure, tt.text, tt.figure)
		}
	}
}

func TestParseRejects(t *testing.T) {
	if _, err := Parse(1, "", []byte("no sections here\n")); err == nil {
		t.Error("text with no section 1: want an error")
	}
	if _, err := Parse(1, "", []byte("1.  One\n\n1.  One again\n")); err == nil {
		t.Error("duplicate section: want an error")
	}
	if _, err := Parse(1, "", []byte{0xff, 0xfe}); err == nil {
		t.Error("invalid UTF-8: want an error")
	}
}

func loadPinned(t *testing.T) map[int]*RFC {
	t.Helper()
	m, err := LoadManifest("../../corpus")
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[int]*RFC, len(m.Documents))
	for _, d := range m.Documents {
		text, err := os.ReadFile(filepath.Join("../../corpus", d.File))
		if err != nil {
			t.Fatal(err)
		}
		r, err := Parse(d.Number, d.Title, text)
		if err != nil {
			t.Fatal(err)
		}
		out[d.Number] = r
	}
	return out
}

func TestParsePinnedCorpus(t *testing.T) {
	rfcs := loadPinned(t)

	for n, r := range rfcs {
		if len(r.Sections) < 10 {
			t.Errorf("RFC %d: only %d sections parsed", n, len(r.Sections))
		}
		for _, s := range r.Sections {
			for i, p := range s.Paragraphs {
				if p.Ref.Paragraph != i+1 || p.Ref.Section != s.ID || p.Ref.RFC != n {
					t.Errorf("RFC %d §%s: paragraph %d has ref %v", n, s.ID, i+1, p.Ref)
				}
				if strings.Contains(p.Text, "[Page ") || strings.ContainsRune(p.Text, '\f') {
					t.Errorf("%v: pagination left in text: %q", p.Ref, p.Text)
				}
				if got := r.Paragraph(p.Ref); got == nil || got.Text != p.Text {
					t.Errorf("%v does not resolve back to itself", p.Ref)
				}
			}
		}
	}

	// Spot checks against the published text.
	checks := []struct {
		ref      Ref
		contains string
	}{
		// Spans the page 10/11 break in RFC 6749.
		{Ref{6749, "1.5", 1}, "Refresh tokens are credentials used to obtain access tokens."},
		{Ref{6749, "1.5", 2}, "Unlike access tokens, refresh tokens are intended for use only with authorization servers"},
		{Ref{6749, "10.4", 1}, "Authorization servers MAY issue refresh tokens to web application clients and native application clients."},
		{Ref{7636, "4.2", 3}, "code_challenge = BASE64URL-ENCODE(SHA256(ASCII(code_verifier)))"},
		{Ref{9700, "2.1.1", 1}, "Clients MUST prevent authorization code injection attacks"},
	}
	for _, c := range checks {
		p := rfcs[c.ref.RFC].Paragraph(c.ref)
		if p == nil {
			t.Errorf("%v: not found", c.ref)
			continue
		}
		if !strings.Contains(strings.Join(strings.Fields(p.Text), " "), c.contains) {
			t.Errorf("%v = %q\nwant it to contain %q", c.ref, p.Text, c.contains)
		}
	}
	if got := rfcs[6749].Section("10.4").Title; got != "Refresh Tokens" {
		t.Errorf("RFC 6749 §10.4 title = %q", got)
	}
}
