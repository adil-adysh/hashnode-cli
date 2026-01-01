package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"adil-adysh/hashnode-cli/internal/state"
)

// TestStageProtection verifies .hashnode directory cannot be staged
func TestStageProtection(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	defer state.ResetProjectRootCache()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	state.ResetProjectRootCache()

	// Create .hashnode directory structure
	stateDir := filepath.Join(tempDir, ".hashnode")
	snapshotsDir := filepath.Join(stateDir, "snapshots")
	if err := os.MkdirAll(snapshotsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Create a fake snapshot file
	snapshotFile := filepath.Join(snapshotsDir, "abc123def456.md")
	if err := os.WriteFile(snapshotFile, []byte("fake snapshot"), 0644); err != nil {
		t.Fatalf("write snapshot failed: %v", err)
	}

	// Create other state files
	stageFile := filepath.Join(stateDir, "hashnode.stage")
	if err := os.WriteFile(stageFile, []byte("items: {}"), 0644); err != nil {
		t.Fatalf("write stage failed: %v", err)
	}

	// Test 1: Try to stage snapshot file directly - should be rejected
	err := state.StageAdd(snapshotFile)
	if err == nil {
		t.Error("expected error when staging snapshot file, got nil")
	}
	if err != nil && err.Error() != "cannot stage files from .hashnode directory" {
		t.Errorf("expected specific error message, got: %v", err)
	}

	// Test 2: Try to stage .hashnode/hashnode.stage - should be rejected
	err = state.StageAdd(stageFile)
	if err == nil {
		t.Error("expected error when staging state file, got nil")
	}
	if err != nil && err.Error() != "cannot stage files from .hashnode directory" {
		t.Errorf("expected specific error message, got: %v", err)
	}

	// Test 3: Create a valid article and stage it - should succeed
	articleFile := filepath.Join(tempDir, "article.md")
	articleContent := []byte("---\ntitle: Test Article\n---\nContent")
	if err := os.WriteFile(articleFile, articleContent, 0644); err != nil {
		t.Fatalf("write article failed: %v", err)
	}

	err = state.StageAdd(articleFile)
	if err != nil {
		t.Errorf("expected success staging valid article, got error: %v", err)
	}

	// Test 4: StageDir on project root should skip .hashnode entirely
	// Create another article in a subdir
	postsDir := filepath.Join(tempDir, "posts")
	if err := os.MkdirAll(postsDir, 0755); err != nil {
		t.Fatalf("mkdir posts failed: %v", err)
	}

	postFile := filepath.Join(postsDir, "post.md")
	if err := os.WriteFile(postFile, []byte("---\ntitle: Post\n---\nBody"), 0644); err != nil {
		t.Fatalf("write post failed: %v", err)
	}

	// Stage entire directory - should include posts/post.md but not .hashnode/*
	staged, skipped, err := state.StageDir(tempDir)
	if err != nil {
		t.Fatalf("StageDir failed: %v", err)
	}

	// Should have staged the articles, but not snapshot files
	hasSnapshotFile := false
	hasStateFile := false
	for _, s := range staged {
		if filepath.Base(s) == "abc123def456.md" {
			hasSnapshotFile = true
		}
		if filepath.Base(s) == "hashnode.stage" {
			hasStateFile = true
		}
	}

	if hasSnapshotFile {
		t.Error("StageDir incorrectly staged snapshot file")
	}
	if hasStateFile {
		t.Error("StageDir incorrectly staged state file")
	}

	// Verify .hashnode files are in skipped or not processed at all
	// (they should be skipped via SkipDir, so won't even appear in skipped list)
	for _, s := range skipped {
		if filepath.Base(s) == "abc123def456.md" || filepath.Base(s) == "hashnode.stage" {
			t.Logf("State file in skipped list (acceptable): %s", s)
		}
	}

	t.Logf("Staged %d files, skipped %d files", len(staged), len(skipped))
}
