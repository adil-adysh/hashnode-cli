package diff

import (
	"fmt"
	"os"
	"path/filepath"

	"adil-adysh/hashnode-cli/internal/log"
	"adil-adysh/hashnode-cli/internal/state"
)

// ActionType defines the operation to be performed on the remote state.
type ActionType string

const (
	ActionCreate ActionType = "CREATE"
	ActionUpdate ActionType = "UPDATE"
	ActionSkip   ActionType = "SKIP"
	ActionDelete ActionType = "DELETE"
)

// PlanItem represents a single unit of work to be executed.
type PlanItem struct {
	Type     ActionType
	ID       string // Local ID from article.yml
	Title    string
	Path     string
	Reason   string
	OldPath  string // Source path if this is a RENAME
	RemoteID string // The Hashnode ID (if known)
}

// RegistryEntry is a lightweight representation of registry metadata used by diff
type RegistryEntry struct {
	LocalID      string
	Title        string
	MarkdownPath string
	SeriesID     string
	RemotePostID string
	Checksum     string
	LastSyncedAt string
}

// FullDiff checks the status of tracked articles against the disk.
// Used by `hnsync status` to show Modified/Deleted files.
func FullDiff(articles []RegistryEntry) []PlanItem {
	var plan []PlanItem

	for _, article := range articles {
		// 1. Resolve Path
		fsPath := resolveAbsPath(article.MarkdownPath)

		// 2. Check File Existence (Detect DELETE)
		info, err := os.Stat(fsPath)
		if err != nil || info.IsDir() {
			if article.RemotePostID != "" {
				plan = append(plan, PlanItem{
					Type:     ActionDelete,
					ID:       article.LocalID,
					Title:    article.Title,
					Path:     article.MarkdownPath,
					Reason:   "Local file missing; remote post exists",
					RemoteID: article.RemotePostID,
				})
			} else {
				plan = append(plan, PlanItem{
					Type:   ActionSkip,
					Path:   article.MarkdownPath,
					Reason: "Draft file missing (local-only)",
				})
			}
			continue
		}

		// 3. Read Content & Checksum
		content, err := os.ReadFile(fsPath)
		if err != nil {
			plan = append(plan, PlanItem{
				Type:   ActionSkip,
				Path:   article.MarkdownPath,
				Reason: fmt.Sprintf("Error reading file: %v", err),
			})
			continue
		}

		currentChecksum := state.ChecksumFromContent(content)

		// 4. Determine Action (Pure Logic)
		action, reason := determineAction(currentChecksum, article.Checksum, article.RemotePostID)

		plan = append(plan, PlanItem{
			Type:     action,
			ID:       article.LocalID,
			Title:    article.Title,
			Path:     article.MarkdownPath,
			Reason:   reason,
			RemoteID: article.RemotePostID,
		})
	}
	return plan
}

// GeneratePlan compares the STAGE against the LEDGER (Registry).
// Used by `hnsync plan` and `hnsync apply`.
func GeneratePlan(articles []RegistryEntry, st *state.Stage) []PlanItem {
	var plan []PlanItem

	// ---------------------------------------------------------
	// 1. OPTIMIZATION: Build Lookups ONCE (O(N))
	// ---------------------------------------------------------
	reg := make(map[string]RegistryEntry)
	checksumToPath := make(map[string]string) // Key: Checksum, Value: Path

	for _, a := range articles {
		norm := state.NormalizePath(a.MarkdownPath)
		reg[norm] = a
		// Only map valid synced articles for rename detection
		if a.RemotePostID != "" && a.Checksum != "" {
			checksumToPath[a.Checksum] = norm
		}
	}

	// ---------------------------------------------------------
	// 2. PROCESS STAGE (O(M))
	// ---------------------------------------------------------
	for rawPath, stagedItem := range st.Items {
		path := state.NormalizePath(rawPath)

		// Handle explicit delete intent
		if stagedItem.Operation == state.OpDelete {
			entry, exists := reg[path]
			if exists && entry.RemotePostID != "" {
				plan = append(plan, PlanItem{
					Type:     ActionDelete,
					ID:       entry.LocalID,
					Title:    entry.Title,
					Path:     path,
					RemoteID: entry.RemotePostID,
					Reason:   "Marked for deletion (staged)",
				})
			} else {
				plan = append(plan, PlanItem{Type: ActionSkip, Path: path, Reason: "Marked for deletion but not published remotely"})
			}
			continue
		}

		// Determine current checksum: prefer staged, then snapshot, then disk.
		var currentHash string
		if stagedItem.Checksum != "" {
			currentHash = stagedItem.Checksum
		} else if stagedItem.Snapshot != "" {
			snapStore := state.NewSnapshotStore()
			if content, err := snapStore.Get(stagedItem.Snapshot); err == nil {
				currentHash = state.ChecksumFromContent(content)
			}
		}
		if currentHash == "" {
			fsPath := resolveAbsPath(path)
			if content, err := os.ReadFile(fsPath); err == nil {
				currentHash = state.ChecksumFromContent(content)
			} else {
				plan = append(plan, PlanItem{Type: ActionSkip, Path: path, Reason: "Staged file missing from disk/snapshot"})
				continue
			}
		}

		// ---------------------------------------------------------
		// 3. DECISION ENGINE
		// ---------------------------------------------------------
		entry, exists := reg[path]

		// CASE A: NEW FILE (Not in Registry)
		if !exists {
			// RENAME HEURISTIC: Does this content exist elsewhere?
			if oldPath, found := checksumToPath[currentHash]; found {
				// SAFEGUARD: Only rename if old file is GONE (Deleted or Staged for Delete)
				isGone := false
				if _, err := os.Stat(resolveAbsPath(oldPath)); os.IsNotExist(err) {
					isGone = true
				}
				if oldItem, ok := st.Items[state.NormalizePath(oldPath)]; ok && oldItem.Operation == state.OpDelete {
					isGone = true
				}

				if isGone {
					oldEntry := reg[oldPath]
					plan = append(plan, PlanItem{
						Type:     ActionUpdate, // Treat Rename as an Update
						Path:     path,
						OldPath:  oldPath,
						RemoteID: oldEntry.RemotePostID,
						Title:    oldEntry.Title,
						Reason:   fmt.Sprintf("Rename detected (matches missing %s)", oldPath),
					})
					continue
				}
				// If old file exists, it's a COPY, so fall through to Create.
			}

			// Truly New
			plan = append(plan, PlanItem{
				Type:   ActionCreate,
				Path:   path,
				Reason: "New Article (Staged)",
			})
			continue
		}

		// CASE B: EXISTING FILE (In Registry)
		action, reason := determineAction(currentHash, entry.Checksum, entry.RemotePostID)

		if action == ActionCreate {
			reason = "Draft Promotion (First Push)"
		}

		plan = append(plan, PlanItem{
			Type:     action,
			ID:       entry.LocalID,
			Title:    entry.Title,
			Path:     path,
			RemoteID: entry.RemotePostID,
			Reason:   reason,
		})
	}

	return plan
}

// determineAction contains the pure business logic for state transitions.
func determineAction(currentHash, knownHash, remoteID string) (ActionType, string) {
	if remoteID == "" {
		return ActionCreate, "Not published remotely"
	}
	if currentHash != knownHash {
		return ActionUpdate, "Content changed"
	}
	return ActionSkip, "Up to date"
}

// resolveAbsPath handles absolute/relative path conversion safely
func resolveAbsPath(p string) string {
	fsPath := filepath.FromSlash(p)
	if !filepath.IsAbs(fsPath) {
		fsPath = filepath.Join(state.ProjectRootOrCwd(), fsPath)
	}
	return fsPath
}

// PrintPlanSummary prints a High-Level, Risk-Aware summary
func PrintPlanSummary(plan []PlanItem) {
	var nCreate, nUpdate, nDelete, nSkip int
	for _, item := range plan {
		switch item.Type {
		case ActionCreate:
			nCreate++
		case ActionUpdate:
			nUpdate++
		case ActionDelete:
			nDelete++
		case ActionSkip:
			nSkip++
		}
	}
	totalOps := nCreate + nUpdate + nDelete

	log.Println("---------------------------------------------------")
	log.Printf("📝  PLAN SUMMARY: %d changes to be applied\n", totalOps)

	if totalOps == 0 {
		log.Println("   (No changes detected. Working tree matches remote.)")
		log.Println("---------------------------------------------------")
		return
	}

	if nCreate > 0 {
		log.Printf("   🟢  Creates: %d\n", nCreate)
	}
	if nUpdate > 0 {
		log.Printf("   🟡  Updates: %d\n", nUpdate)
	}
	if nDelete > 0 {
		log.Printf("   🔴  Deletes: %d\n", nDelete)
	}
	if nSkip > 0 {
		log.Printf("   ⚪  Skipped: %d\n", nSkip)
	}

	log.Println("---------------------------------------------------")

	// Print Groups: Risk First
	printGroup(plan, ActionDelete, "🔴  DELETIONS")
	printGroup(plan, ActionCreate, "🟢  CREATIONS")
	printGroup(plan, ActionUpdate, "🟡  UPDATES")

	log.Println("---------------------------------------------------")
}

func printGroup(plan []PlanItem, action ActionType, header string) {
	hasItems := false
	for _, item := range plan {
		if item.Type == action {
			hasItems = true
			break
		}
	}
	if !hasItems {
		return
	}

	log.Printf("\n%s\n", header)
	for _, item := range plan {
		if item.Type == action {
			log.Printf("   %s (%s)\n", item.Title, item.Path)
			log.Printf("     └─ Reason: %s\n", item.Reason)
		}
	}
}

func countChanges(plan []PlanItem) int {
	n := 0
	for _, item := range plan {
		if item.Type != ActionSkip {
			n++
		}
	}
	return n
}
