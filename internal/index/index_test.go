package index

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/rick-learns/citebound/internal/corpus"
)

var (
	pinnedOnce  sync.Once
	pinnedIndex *Index
	pinnedErr   error
)

func pinned(t *testing.T) *Index {
	t.Helper()
	pinnedOnce.Do(func() {
		rfcs, err := corpus.Load("../../corpus")
		if err != nil {
			pinnedErr = err
			return
		}
		pinnedIndex, pinnedErr = Build(context.Background(), rfcs)
	})
	if pinnedErr != nil {
		t.Fatal(pinnedErr)
	}
	return pinnedIndex
}

func TestSearchFindsTheRightSection(t *testing.T) {
	ix := pinned(t)
	tests := []struct {
		query string
		want  []string // any of these sections in the top 5 passes
	}{
		{"How should a server rotate refresh tokens?", []string{"RFC6819 §5.2.2.3", "RFC9700 §4.14.2"}},
		{"What is the code_challenge_method S256 transformation?", []string{"RFC7636 §4.2", "RFC7636 §4.3", "RFC7636 §B"}},
		{"bearer token in the URI query parameter", []string{"RFC6750 §2.3"}},
		{"DPoP proof JWT nonce", []string{"RFC9449 §8", "RFC9449 §9"}},
	}
	for _, tt := range tests {
		hits, err := ix.Search(context.Background(), tt.query, 5)
		if err != nil {
			t.Fatalf("Search(%q): %v", tt.query, err)
		}
		var got []string
		found := false
		for _, h := range hits {
			got = append(got, h.Ref.String())
			sec := fmt.Sprintf("RFC%d §%s", h.Ref.RFC, h.Ref.Section)
			for _, w := range tt.want {
				if sec == w || strings.HasPrefix(sec, w+".") {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("Search(%q) top 5 = %v, want a hit in one of %v", tt.query, got, tt.want)
		}
	}
}

func TestSearchScoresDescend(t *testing.T) {
	hits, err := pinned(t).Search(context.Background(), "authorization code injection", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 10 {
		t.Fatalf("got %d hits, want 10", len(hits))
	}
	for i := 1; i < len(hits); i++ {
		if hits[i].Score > hits[i-1].Score {
			t.Errorf("hit %d score %v > hit %d score %v", i, hits[i].Score, i-1, hits[i-1].Score)
		}
	}
}

func TestSearchTreatsFTSSyntaxAsText(t *testing.T) {
	for _, q := range []string{`"unbalanced`, `NEAR(token, 2)`, `token* OR -client`, `col:refresh`, `a"b`} {
		if _, err := pinned(t).Search(context.Background(), q, 5); err != nil {
			t.Errorf("Search(%q) error: %v", q, err)
		}
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	for _, q := range []string{"", "   ", "what is the", "?!", ")("} {
		if _, err := pinned(t).Search(context.Background(), q, 5); !errors.Is(err, ErrEmptyQuery) {
			t.Errorf("Search(%q) error = %v, want ErrEmptyQuery", q, err)
		}
	}
}

func TestReferenceListsAreNotIndexed(t *testing.T) {
	hits, err := pinned(t).Search(context.Background(), "Bradner Key words for use in RFCs to Indicate Requirement Levels", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if strings.HasSuffix(h.SectionTitle, "References") {
			t.Errorf("hit from a reference list: %v (%s)", h.Ref, h.SectionTitle)
		}
	}
}
