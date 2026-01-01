package applyutil

import (
	"fmt"
	"os"
	"path/filepath"

	"adil-adysh/hashnode-cli/internal/state"
)

// LoadContentForPath returns parsed frontmatter and the markdown body (without frontmatter)
// for a given path using staged snapshot if available, otherwise disk. Frontmatter errors
// are returned to prevent bad publishes.
func LoadContentForPath(st *state.Stage, path string) (*state.Frontmatter, string, error) {
	np := state.NormalizePath(path)

	var contentBytes []byte
	var rerr error
	if st != nil {
		if si, ok := st.Items[np]; ok && si.Snapshot != "" {
			snapStore := state.NewSnapshotStore()
			contentBytes, rerr = snapStore.Get(si.Snapshot)
		}
	}
	if contentBytes == nil {
		fsPath := filepath.FromSlash(np)
		if !filepath.IsAbs(fsPath) {
			fsPath = filepath.Join(state.ProjectRootOrCwd(), fsPath)
		}
		contentBytes, rerr = os.ReadFile(fsPath)
	}
	if rerr != nil {
		return nil, "", fmt.Errorf("failed to read content for %s: %w", path, rerr)
	}

	fm, bodyBytes, berr := state.ExtractFrontmatter(contentBytes)
	if berr != nil {
		return nil, "", fmt.Errorf("failed to parse frontmatter for %s: %w", path, berr)
	}
	body := string(bodyBytes)
	return fm, body, nil
}
