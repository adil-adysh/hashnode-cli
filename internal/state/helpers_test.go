package state_test

import (
	"os"
	"path/filepath"
	"testing"

	st "adil-adysh/hashnode-cli/internal/state"
)

func TestFindProjectRootWithSumFile(t *testing.T) {
	root := t.TempDir()
	// create marker file
	sumPath := filepath.Join(root, st.SumFile)
	if err := os.WriteFile(sumPath, []byte("sum"), 0644); err != nil {
		t.Fatalf("failed to write sum file: %v", err)
	}

	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	found, err := st.FindProjectRoot(nested)
	if err != nil {
		t.Fatalf("FindProjectRoot returned error: %v", err)
	}
	if found != root {
		t.Fatalf("expected root %s, got %s", root, found)
	}
}

func TestFindProjectRootWithStateDir(t *testing.T) {
	root := t.TempDir()
	// create .hashnode dir marker
	marker := filepath.Join(root, st.StateDir)
	if err := os.MkdirAll(marker, 0755); err != nil {
		t.Fatalf("mkdir marker: %v", err)
	}

	nested := filepath.Join(root, "x", "y")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	found, err := st.FindProjectRoot(nested)
	if err != nil {
		t.Fatalf("FindProjectRoot returned error: %v", err)
	}
	if found != root {
		t.Fatalf("expected root %s, got %s", root, found)
	}
}

func TestAtomicWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.txt")
	data := []byte("hello world")

	if err := st.AtomicWriteFile(path, data, 0644); err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("content mismatch: want %q got %q", string(data), string(got))
	}

	// temp file should not remain
	tmp := filepath.Join(dir, ".tmp-foo.txt")
	if _, err := os.Stat(tmp); err == nil {
		t.Fatalf("temp file still exists: %s", tmp)
	}
}

func TestReadYAMLAndLoadYAMLEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example.yml")

	type sample struct {
		Name string `yaml:"name"`
		N    int    `yaml:"n"`
	}

	// write YAML manually
	content := "name: alice\nn: 7\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	var s sample
	if err := st.ReadYAML(path, &s); err != nil {
		t.Fatalf("ReadYAML failed: %v", err)
	}
	if s.Name != "alice" || s.N != 7 {
		t.Fatalf("unexpected parsed value: %#v", s)
	}

	// LoadYAMLOrEmpty on missing file should return nil and leave out zero-valued
	missing := filepath.Join(dir, "nope.yml")
	var s2 sample
	if err := st.LoadYAMLOrEmpty(missing, &s2); err != nil {
		t.Fatalf("LoadYAMLOrEmpty on missing returned error: %v", err)
	}
	if s2.Name != "" || s2.N != 0 {
		t.Fatalf("expected zero value for missing file, got %#v", s2)
	}
}
