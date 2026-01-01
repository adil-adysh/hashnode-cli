package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"adil-adysh/hashnode-cli/internal/state"
)

// TestStageWorkflow validates the staging workflow:
// 1. Stage a file -> creates snapshot
// 2. Modify file on disk
// 3. Stage again -> creates new snapshot
// 4. Verify checksums are tracked correctly
func TestStageWorkflow(t *testing.T) {
	// Setup temp directory as project root
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	// Create state directory (marks as hashnode project)
	stateDir := filepath.Join(tempDir, ".hashnode")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("mkdir .hashnode failed: %v", err)
	}

	// Create a test markdown file
	testFile := filepath.Join(tempDir, "test-post.md")
	content1 := []byte("---\ntitle: Test Post V1\n---\n\nContent version 1")
	if err := os.WriteFile(testFile, content1, 0644); err != nil {
		t.Fatalf("write test file failed: %v", err)
	}

	// Stage the file
	if err := state.StageAdd(testFile); err != nil {
		t.Fatalf("StageAdd failed: %v", err)
	}

	// Load stage and verify
	st, err := state.LoadStage()
	if err != nil {
		t.Fatalf("LoadStage failed: %v", err)
	}

	normPath := state.NormalizePath(testFile)
	item, ok := st.Items[normPath]
	if !ok {
		t.Fatalf("staged item not found for %s", testFile)
	}

	if item.Operation != state.OpModify {
		t.Errorf("expected operation MODIFY, got %s", item.Operation)
	}

	checksum1 := state.ChecksumFromContent(content1)
	if item.Checksum != checksum1 {
		t.Errorf("checksum mismatch: expected %s, got %s", checksum1, item.Checksum)
	}

	// Verify snapshot was created
	if item.Snapshot == "" {
		t.Fatal("snapshot not created")
	}
	snapshotPath := filepath.Join(stateDir, "snapshots", item.Snapshot)
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		t.Errorf("snapshot file not found: %s", snapshotPath)
	}

	// Modify the file on disk
	content2 := []byte("---\ntitle: Test Post V2\n---\n\nContent version 2")
	if err := os.WriteFile(testFile, content2, 0644); err != nil {
		t.Fatalf("write modified file failed: %v", err)
	}

	// Stage again
	if err := state.StageAdd(testFile); err != nil {
		t.Fatalf("StageAdd (v2) failed: %v", err)
	}

	// Verify new checksum
	st2, err := state.LoadStage()
	if err != nil {
		t.Fatalf("LoadStage (v2) failed: %v", err)
	}

	item2, ok := st2.Items[normPath]
	if !ok {
		t.Fatalf("staged item not found after restage")
	}

	checksum2 := state.ChecksumFromContent(content2)
	if item2.Checksum != checksum2 {
		t.Errorf("checksum v2 mismatch: expected %s, got %s", checksum2, item2.Checksum)
	}

	// Verify new snapshot created (content-addressable)
	if item2.Snapshot == item.Snapshot {
		t.Error("expected different snapshot for modified content")
	}
}

// TestTitleResolution validates centralized title resolution
func TestTitleResolution(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	// Create state directory
	stateDir := filepath.Join(tempDir, ".hashnode")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("mkdir .hashnode failed: %v", err)
	}

	// Create test file with frontmatter
	testFile := filepath.Join(tempDir, "post.md")
	content := []byte("---\ntitle: My Awesome Post\n---\n\nBody content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("write test file failed: %v", err)
	}

	// Test title resolution without ledger/stage
	title, err := state.ResolveTitleForPath(testFile, nil, nil)
	if err != nil {
		t.Fatalf("ResolveTitleForPath failed: %v", err)
	}
	if title != "My Awesome Post" {
		t.Errorf("expected 'My Awesome Post', got '%s'", title)
	}

	// Create ledger with cached title
	sum := &state.Sum{
		Version: 1,
		Articles: map[string]state.ArticleSum{
			state.NormalizePath(testFile): {
				PostID:   "p_123",
				Checksum: state.ChecksumFromContent(content),
				Slug:     "my-awesome-post",
				Title:    "Cached Title",
			},
		},
	}

	// Title resolution should prefer ledger cache
	title2, err := state.ResolveTitleForPath(testFile, sum, nil)
	if err != nil {
		t.Fatalf("ResolveTitleForPath with sum failed: %v", err)
	}
	if title2 != "Cached Title" {
		t.Errorf("expected cached title 'Cached Title', got '%s'", title2)
	}
}

// TestUnstageWorkflow validates unstaging removes items
func TestUnstageWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	defer state.ResetProjectRootCache()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	// Reset cache so FindProjectRoot searches from new tempDir
	state.ResetProjectRootCache()

	stateDir := filepath.Join(tempDir, ".hashnode")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("mkdir .hashnode failed: %v", err)
	}

	testFile := "post.md" // Relative path after chdir
	content := []byte("---\ntitle: Test\n---\nBody")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Stage
	if err := state.StageAdd(testFile); err != nil {
		t.Fatalf("StageAdd failed: %v", err)
	}

	// Verify staged
	st, _ := state.LoadStage()
	normPath := state.NormalizePath(testFile)
	if _, ok := st.Items[normPath]; !ok {
		t.Fatal("file not staged")
	}

	// Unstage
	if err := state.Unstage(testFile); err != nil {
		t.Fatalf("Unstage failed: %v", err)
	}

	// Verify removed
	st2, _ := state.LoadStage()
	if _, ok := st2.Items[normPath]; ok {
		t.Error("file still staged after unstage")
	}
}
