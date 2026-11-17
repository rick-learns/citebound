package corpus

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinnedCorpusVerifies(t *testing.T) {
	m, err := LoadManifest("../../corpus")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Documents) == 0 {
		t.Fatal("manifest lists no documents")
	}
	if err := m.Verify("../../corpus"); err != nil {
		t.Fatalf("pinned corpus does not verify: %v", err)
	}
}

func writeCorpus(t *testing.T, manifest, file, contents string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if file != "" {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// sha256("hello\n")
const helloSum = "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"

func TestVerify(t *testing.T) {
	manifest := `{"version":1,"documents":[{"number":1,"title":"t","file":"a.txt","sha256":"` + helloSum + `"}]}`

	t.Run("match", func(t *testing.T) {
		dir := writeCorpus(t, manifest, "a.txt", "hello\n")
		m, err := LoadManifest(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Verify(dir); err != nil {
			t.Fatalf("Verify() = %v, want nil", err)
		}
	})

	t.Run("edited file", func(t *testing.T) {
		dir := writeCorpus(t, manifest, "a.txt", "hello, edited\n")
		m, err := LoadManifest(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Verify(dir); !errors.Is(err, ErrChecksum) {
			t.Fatalf("Verify() = %v, want ErrChecksum", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		dir := writeCorpus(t, manifest, "", "")
		m, err := LoadManifest(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Verify(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Verify() = %v, want a not-exist error", err)
		}
	})
}

func TestLoadManifestRejects(t *testing.T) {
	tests := map[string]string{
		"bad version":   `{"version":2,"documents":[]}`,
		"incomplete":    `{"version":1,"documents":[{"number":1,"file":"a.txt"}]}`,
		"escaping path": `{"version":1,"documents":[{"number":1,"file":"../a.txt","sha256":"x"}]}`,
		"duplicate":     `{"version":1,"documents":[{"number":1,"file":"a","sha256":"x"},{"number":1,"file":"b","sha256":"y"}]}`,
		"not json":      `{`,
	}
	for name, manifest := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadManifest(writeCorpus(t, manifest, "", "")); err == nil {
				t.Fatal("LoadManifest() = nil error, want an error")
			}
		})
	}
	if _, err := LoadManifest(t.TempDir()); err == nil || !strings.Contains(err.Error(), "read manifest") {
		t.Fatalf("missing manifest: err = %v", err)
	}
}
