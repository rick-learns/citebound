package corpus

import (
	"fmt"
	"os"
)

// Load verifies the corpus in dir against its manifest and parses every RFC,
// in manifest order.
func Load(dir string) ([]*RFC, error) {
	m, err := LoadManifest(dir)
	if err != nil {
		return nil, err
	}
	if err := m.Verify(dir); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open corpus: %w", err)
	}
	defer root.Close()
	rfcs := make([]*RFC, 0, len(m.Documents))
	for _, d := range m.Documents {
		text, err := root.ReadFile(d.File)
		if err != nil {
			return nil, fmt.Errorf("RFC %d: %w", d.Number, err)
		}
		r, err := Parse(d.Number, d.Title, text)
		if err != nil {
			return nil, err
		}
		rfcs = append(rfcs, r)
	}
	return rfcs, nil
}
