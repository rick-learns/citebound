package corpus

import "testing"

func TestRefRoundTrip(t *testing.T) {
	for _, r := range []Ref{
		{RFC: 6749, Section: "10.4", Paragraph: 2},
		{RFC: 9110, Section: "A.1", Paragraph: 1},
		{RFC: 9700, Section: "Abstract", Paragraph: 1},
	} {
		got, err := ParseRef(r.String())
		if err != nil {
			t.Fatalf("ParseRef(%q) error: %v", r.String(), err)
		}
		if got != r {
			t.Errorf("ParseRef(%q) = %+v, want %+v", r.String(), got, r)
		}
	}
}

func TestParseRefAcceptsSpaceAfterRFC(t *testing.T) {
	got, err := ParseRef("RFC 6749 §10.4 ¶2")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Ref{RFC: 6749, Section: "10.4", Paragraph: 2}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseRefRejects(t *testing.T) {
	for _, s := range []string{
		"",
		"RFC6749",
		"RFC6749 §10.4",
		"RFC6749 §10.4 ¶0",
		"RFC0 §1 ¶1",
		"rfc6749 §10.4 ¶2",
		"RFC6749 section 10.4 paragraph 2",
		"RFC6749 §10.4 ¶2 extra",
	} {
		if _, err := ParseRef(s); err == nil {
			t.Errorf("ParseRef(%q) = nil error, want an error", s)
		}
	}
}
