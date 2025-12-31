package diff

import (
	"adil-adysh/hashnode-cli/internal/log"
	"adil-adysh/hashnode-cli/internal/state"
	"fmt"
	"os"
)

type ActionType string

const (
	ActionCreate ActionType = "CREATE"
	ActionUpdate ActionType = "UPDATE"
	ActionSkip   ActionType = "SKIP"
	ActionDelete ActionType = "DELETE"
)

type PlanItem struct {
	Type   ActionType
	ID     string // local_id from article.yml
	Title  string
	Path   string
	Reason string
}

// GeneratePlan compares the authoritative article registry against the
// current filesystem and produces a plan of actions. The registry is the
// source of truth for identity; the file path is treated as metadata.
func GeneratePlan(articles []state.ArticleEntry) []PlanItem {
	var plan []PlanItem

	for _, article := range articles {
		articlePath := article.MarkdownPath

		// If the path is empty or not present on disk, decide whether this is
		// a remote-deletion candidate (DELETE) or simply missing locally (SKIP).
		info, statErr := os.Stat(articlePath)
		if statErr != nil || info.IsDir() {
			if article.RemotePostID != "" {
				plan = append(plan, PlanItem{
					Type:   ActionDelete,
					ID:     article.LocalID,
					Title:  article.Title,
					Path:   articlePath,
					Reason: "Local markdown missing; remote post exists and may be removed",
				})
			} else {
				plan = append(plan, PlanItem{
					Type:   ActionSkip,
					ID:     article.LocalID,
					Title:  article.Title,
					Path:   articlePath,
					Reason: "Markdown file missing or inaccessible",
				})
			}
			continue
		}

		// Read file and compute checksum
		content, readErr := os.ReadFile(articlePath)
		if readErr != nil {
			plan = append(plan, PlanItem{
				Type:   ActionSkip,
				ID:     article.LocalID,
				Title:  article.Title,
				Path:   articlePath,
				Reason: fmt.Sprintf("Failed to read file: %v", readErr),
			})
			continue
		}

		currentChecksum := state.ChecksumFromContent(content)

		// If not yet published remotely, create
		if article.RemotePostID == "" {
			plan = append(plan, PlanItem{
				Type:   ActionCreate,
				ID:     article.LocalID,
				Title:  article.Title,
				Path:   articlePath,
				Reason: "Not published remotely",
			})
			continue
		}

		// If checksums differ -> update
		if currentChecksum != article.Checksum {
			plan = append(plan, PlanItem{
				Type:   ActionUpdate,
				ID:     article.LocalID,
				Title:  article.Title,
				Path:   articlePath,
				Reason: "Local content differs from last synced checksum",
			})
			continue
		}

		// Otherwise up-to-date
		plan = append(plan, PlanItem{
			Type:   ActionSkip,
			ID:     article.LocalID,
			Title:  article.Title,
			Path:   articlePath,
			Reason: "Up to date",
		})
	}

	return plan
}

// PrintPlanSummary prints a summary of the plan to the terminal
func PrintPlanSummary(plan []PlanItem) {
	// Print a concise header that helps users scan the output quickly.
	log.Printf("📝 Execution Plan: %d changes detected\n", countChanges(plan))
	log.Println("---------------------------------------------------")
	for _, item := range plan {
		switch item.Type {
		case ActionCreate:
			log.Printf("🟢 [CREATE] %s (%s)\n   Reason: %s\n", item.Title, item.Path, item.Reason)
		case ActionUpdate:
			log.Printf("🟡 [UPDATE] %s (%s)\n   Reason: %s\n", item.Title, item.Path, item.Reason)
		case ActionDelete:
			log.Printf("🔴 [DELETE] %s (%s)\n   Reason: %s\n", item.Title, item.Path, item.Reason)
		case ActionSkip:
			log.Printf("⚪ [SKIP] %s (%s)\n   Reason: %s\n", item.Title, item.Path, item.Reason)
		}
	}
	log.Println("---------------------------------------------------")
	log.Println("Run 'hashnode apply' to execute these changes.")
}

// countChanges returns the number of actionable changes (create/update/delete)
// and excludes SKIP items which are informational.
func countChanges(plan []PlanItem) int {
	n := 0
	for _, item := range plan {
		if item.Type != ActionSkip {
			n++
		}
	}
	return n
}
