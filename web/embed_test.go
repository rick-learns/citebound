//go:build embedui

package web

import (
	"io/fs"
	"regexp"
	"testing"
)

// TestFiles checks the embedded build: index.html exists, and every asset it
// references is embedded too.
func TestFiles(t *testing.T) {
	files := Files()
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	refs := regexp.MustCompile(`(?:src|href)="/(assets/[^"]+)"`).FindAllStringSubmatch(string(index), -1)
	if len(refs) == 0 {
		t.Fatalf("index.html references no assets:\n%s", index)
	}
	for _, m := range refs {
		if _, err := fs.Stat(files, m[1]); err != nil {
			t.Errorf("index.html references %s: %v", m[1], err)
		}
	}
}
