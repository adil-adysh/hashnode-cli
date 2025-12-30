package diff

import (
	"adil-adysh/hashnode-cli/internal/state"
	"fmt"
)

type ActionType string

const (
	ActionCreate ActionType = "CREATE"
	ActionUpdate ActionType = "UPDATE"
	ActionSkip   ActionType = "SKIP"
	ActionDelete ActionType = "DELETE"
)

type PlanItem struct {
	Type      ActionType
	Slug      string
	Path      string
	Reason    string
	LocalPost state.LocalPost
}

// GeneratePlan compares Local Files against Stored State
func GeneratePlan(localPosts map[string]state.LocalPost, storedState map[string]state.RemoteIdentity) []PlanItem {
	var plan []PlanItem

	// 1. Check every local post
	for slug, post := range localPosts {
		identity, exists := storedState[slug]

		if !exists {
			// Case A: It's a new file we haven't tracked yet
			plan = append(plan, PlanItem{
				Type:      ActionCreate,
				Slug:      slug,
				Path:      post.Path,
				Reason:    "New local file detected",
				LocalPost: post,
			})
			continue
		}

		// Case B: We track it, but did the content change?
		if post.Checksum != identity.LastChecksum {
			plan = append(plan, PlanItem{
				Type:      ActionUpdate,
				Slug:      slug,
				Path:      post.Path,
				Reason:    "Content changed since last sync",
				LocalPost: post,
			})
		} else {
			// Case C: Everything matches
			plan = append(plan, PlanItem{
				Type:   ActionSkip,
				Slug:   slug,
				Reason: "Up to date",
			})
		}
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
			fmt.Printf("🟢 [CREATE] %s\n   Reason: %s\n", item.Slug, item.Reason)
		case ActionUpdate:
			fmt.Printf("🟡 [UPDATE] %s\n   Reason: %s\n", item.Slug, item.Reason)
		case ActionDelete:
			fmt.Printf("🔴 [DELETE] %s\n   Reason: %s\n", item.Slug, item.Reason)
		}
	}
	fmt.Println("---------------------------------------------------")
	fmt.Println("Run 'hnsync apply' to execute these changes.")
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
