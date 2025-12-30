package diff

import (
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

// GeneratePlan compares the authoritative .hashnode/article.yml entries
// against the current filesystem. The article registry is the source of truth
// for identity; paths are metadata.
func GeneratePlan(articles []state.ArticleEntry) []PlanItem {
	var plan []PlanItem

	for _, a := range articles {
		path := a.MarkdownPath
		// if path is empty or file missing, mark accordingly
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			// If the article was previously published remotely but the local file is gone,
			// show a DELETE action so the user can remove the remote post.
			if a.RemotePostID != "" {
				plan = append(plan, PlanItem{
					Type:   ActionDelete,
					ID:     a.LocalID,
					Title:  a.Title,
					Path:   path,
					Reason: "Local markdown missing; remote post exists and may be removed",
				})
			} else {
				plan = append(plan, PlanItem{
					Type:   ActionSkip,
					ID:     a.LocalID,
					Title:  a.Title,
					Path:   path,
					Reason: "Markdown file missing or inaccessible",
				})
			}
			continue
		}

		// Read file and compute checksum
		data, err := os.ReadFile(path)
		if err != nil {
			plan = append(plan, PlanItem{
				Type:   ActionSkip,
				ID:     a.LocalID,
				Title:  a.Title,
				Path:   path,
				Reason: fmt.Sprintf("Failed to read file: %v", err),
			})
			continue
		}

		current := state.ChecksumFromContent(data)

		// If not yet published remotely
		if a.RemotePostID == "" {
			// If checksum matches registry, still needs create; if differs, still create
			plan = append(plan, PlanItem{
				Type:   ActionCreate,
				ID:     a.LocalID,
				Title:  a.Title,
				Path:   path,
				Reason: "Not published remotely",
			})
			continue
		}

		// If checksums differ -> update
		if current != a.Checksum {
			plan = append(plan, PlanItem{
				Type:   ActionUpdate,
				ID:     a.LocalID,
				Title:  a.Title,
				Path:   path,
				Reason: "Local content differs from last synced checksum",
			})
			continue
		}

		// Otherwise up-to-date
		plan = append(plan, PlanItem{
			Type:   ActionSkip,
			ID:     a.LocalID,
			Title:  a.Title,
			Path:   path,
			Reason: "Up to date",
		})
	}

	return plan
}

// PrintPlanSummary prints a summary of the plan to the terminal
func PrintPlanSummary(plan []PlanItem) {
	fmt.Printf("📝 Execution Plan: %d changes detected\n", countNonSkip(plan))
	fmt.Println("---------------------------------------------------")
	for _, item := range plan {
		switch item.Type {
		case ActionCreate:
			fmt.Printf("🟢 [CREATE] %s (%s)\n   Reason: %s\n", item.Title, item.Path, item.Reason)
		case ActionUpdate:
			fmt.Printf("🟡 [UPDATE] %s (%s)\n   Reason: %s\n", item.Title, item.Path, item.Reason)
		case ActionDelete:
			fmt.Printf("🔴 [DELETE] %s (%s)\n   Reason: %s\n", item.Title, item.Path, item.Reason)
		}
	}
	fmt.Println("---------------------------------------------------")
	fmt.Println("Run 'hashnode apply' to execute these changes.")
}

func countNonSkip(plan []PlanItem) int {
	n := 0
	for _, item := range plan {
		if item.Type != ActionSkip {
			n++
		}
	}
	return n
}
