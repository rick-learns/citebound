// Package corpus loads the pinned RFC corpus and checks it against its manifest.
package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Manifest lists the RFCs in the corpus and pins each file by checksum.
type Manifest struct {
	Version   int        `json:"version"`
	Documents []Document `json:"documents"`
}

// Document is one pinned RFC.
type Document struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	File   string `json:"file"`   // relative to the corpus directory
	Source string `json:"source"` // where the file was fetched from
	SHA256 string `json:"sha256"`
}

// LoadManifest reads manifest.json from the corpus directory.
func LoadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Version != 1 {
		return nil, fmt.Errorf("unsupported manifest version %d", m.Version)
	}
	seen := make(map[int]bool, len(m.Documents))
	for _, d := range m.Documents {
		if d.Number <= 0 || d.File == "" || d.SHA256 == "" {
			return nil, fmt.Errorf("manifest entry for RFC %d is incomplete", d.Number)
		}
		if !filepath.IsLocal(d.File) {
			return nil, fmt.Errorf("RFC %d: file %q must be a path inside the corpus directory", d.Number, d.File)
		}
		if seen[d.Number] {
			return nil, fmt.Errorf("RFC %d is listed twice", d.Number)
		}
		seen[d.Number] = true
	}
	return &m, nil
}

// ErrChecksum reports a corpus file whose contents do not match the manifest.
var ErrChecksum = errors.New("checksum mismatch")

// Verify checks every file in the manifest against its pinned checksum.
// It reports all failures, not just the first.
func (m *Manifest) Verify(dir string) error {
	var errs []error
	for _, d := range m.Documents {
		data, err := os.ReadFile(filepath.Join(dir, d.File))
		if err != nil {
			errs = append(errs, fmt.Errorf("RFC %d: %w", d.Number, err))
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != d.SHA256 {
			errs = append(errs, fmt.Errorf("RFC %d (%s): %w: got %s, want %s", d.Number, d.File, ErrChecksum, got, d.SHA256))
		}
	}
	return errors.Join(errs...)
}
