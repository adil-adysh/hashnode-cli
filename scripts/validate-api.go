package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// =============================================================================
// CONFIGURATION & COLORS
// =============================================================================

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorBold   = "\033[1m"
)

// =============================================================================
// REPORTING ENGINE (LINTER STYLE)
// =============================================================================

type ValidationReport struct {
	Errors   []string
	Warnings []string
}

func (r *ValidationReport) AddError(msg string) {
	r.Errors = append(r.Errors, fmt.Sprintf("%s❌ %s%s", ColorRed, msg, ColorReset))
}

func (r *ValidationReport) AddWarning(msg string) {
	r.Warnings = append(r.Warnings, fmt.Sprintf("%s⚠️  %s%s", ColorYellow, msg, ColorReset))
}

func (r *ValidationReport) HasFatalErrors() bool {
	return len(r.Errors) > 0
}

func (r *ValidationReport) PrintSummary() {
	fmt.Println("\n" + strings.Repeat("-", 50))
	
	// Print Warnings first
	for _, w := range r.Warnings {
		fmt.Println(w)
	}
	// Print Errors
	for _, e := range r.Errors {
		fmt.Println(e)
	}

	fmt.Println(strings.Repeat("-", 50))

	if len(r.Errors) > 0 {
		fmt.Printf("%sFAILED: Found %d errors and %d warnings.%s\n", ColorRed, len(r.Errors), len(r.Warnings), ColorReset)
		os.Exit(1)
	} else {
		fmt.Printf("%sSUCCESS: Integrity check passed (%d warnings).%s\n", ColorGreen, len(r.Warnings), ColorReset)
	}
}

// =============================================================================
// GRAPHQL TYPES (FOR SCHEMA PARSING)
// =============================================================================

type Introspection struct {
	Data struct {
		Schema struct {
			Types []Type `json:"types"`
		} `json:"__schema"`
	} `json:"data"`
	// Fallback for raw schema dump
	Schema *struct {
		Types []Type `json:"types"`
	} `json:"__schema"`
}

type Type struct {
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Fields []Field `json:"fields"`
}

type Field struct {
	Name string `json:"name"`
}

// =============================================================================
// MAIN EXECUTION
// =============================================================================

func main() {
	// 1. CLI Flags
	schemaPath := flag.String("schema", "schema/hashnode-schema.json", "Path to GraphQL schema JSON")
	postsPath := flag.String("posts", "posts", "Path to content directory")
	flag.Parse()

	report := &ValidationReport{}

	fmt.Printf("%s🔍 Starting hnsync validation...%s\n", ColorBold, ColorReset)

	// 2. Schema Validation
	validateSchema(*schemaPath, report)

	// 3. Content Validation
	// We pass the report so it can collect errors instead of crashing
	validateContent(*postsPath, report)

	// 4. Final Output
	report.PrintSummary()
}

// =============================================================================
// LOGIC: SCHEMA VALIDATION
// =============================================================================

func validateSchema(path string, report *ValidationReport) {
	fmt.Printf("%sChecking API Contract (%s)...%s\n", ColorCyan, path, ColorReset)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		report.AddError(fmt.Sprintf("Schema file not found: %s (Run introspection query first)", path))
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		report.AddError(fmt.Sprintf("Read failed: %v", err))
		return
	}

	var root Introspection
	if err := json.Unmarshal(data, &root); err != nil {
		report.AddError("Invalid JSON format in schema file")
		return
	}

	// normalize: handle wrapped vs unwrapped JSON
	var types []Type
	if root.Data.Schema.Types != nil {
		types = root.Data.Schema.Types
	} else if root.Schema != nil {
		types = root.Schema.Types
	} else {
		report.AddError("JSON does not contain '__schema' root")
		return
	}

	// Helper to find type
	getType := func(name string) *Type {
		for _, t := range types {
			if t.Name == name {
				return &t
			}
		}
		return nil
	}

	// Check Mutations
	mutation := getType("Mutation")
	if mutation == nil {
		report.AddError("Schema missing 'Mutation' type")
		return
	}

	requiredMutations := []string{"createSeries", "updatePost", "publishPost", "removePost"}
	foundMutations := make(map[string]bool)
	for _, f := range mutation.Fields {
		foundMutations[f.Name] = true
	}

	for _, req := range requiredMutations {
		if !foundMutations[req] {
			report.AddError(fmt.Sprintf("Missing critical mutation: '%s'", req))
		}
	}
}

// =============================================================================
// LOGIC: CONTENT VALIDATION
// =============================================================================

func validateContent(rootPath string, report *ValidationReport) {
	fmt.Printf("%sLinting Content (%s)...%s\n", ColorCyan, rootPath, ColorReset)

	// MANDATORY FOLDER CHECK
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		report.AddError(fmt.Sprintf("Content directory '%s' does not exist.", rootPath))
		return // Cannot continue if folder is missing
	}

	slugMap := make(map[string]string)   // slug_lower -> filepath
	seriesMap := make(map[string]string) // series_lower -> series_canonical_case

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		
		// Skip dotfiles and non-markdown
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir // Skip .git, etc
			}
			return nil
		}
		if filepath.Ext(path) != ".md" { return nil }

		// Read File
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			report.AddError(fmt.Sprintf("Unreadable file: %s", path))
			return nil
		}
		
		// Parse Frontmatter
		frontmatter, hasFM := extractFrontmatter(string(contentBytes))
		if !hasFM {
			report.AddWarning(fmt.Sprintf("Skipping (no frontmatter): %s", path))
			return nil
		}

		// 1. Title Check
		title := extractField(frontmatter, "title")
		if title == "" {
			report.AddError(fmt.Sprintf("[%s] Missing 'title' in frontmatter", info.Name()))
		}

		// 2. Slug Collision Check (Case Insensitive)
		slug := extractField(frontmatter, "slug")
		if slug != "" {
			lowerSlug := strings.ToLower(slug)
			if existingFile, exists := slugMap[lowerSlug]; exists {
				report.AddError(fmt.Sprintf(
					"[%s] Duplicate Slug Detected!\n      Slug: '%s'\n      Conflict: %s", 
					info.Name(), slug, filepath.Base(existingFile),
				))
			} else {
				slugMap[lowerSlug] = path
			}
		}

		// 3. Series Consistency (Typo Check)
		series := extractField(frontmatter, "series")
		if series != "" {
			lowerSeries := strings.ToLower(series)
			if canonical, exists := seriesMap[lowerSeries]; exists {
				if series != canonical {
					report.AddWarning(fmt.Sprintf(
						"[%s] Series Casing Mismatch.\n      Found: '%s'\n      Expected: '%s'", 
						info.Name(), series, canonical,
					))
				}
			} else {
				seriesMap[lowerSeries] = series
			}
		}

		return nil
	})

	if err != nil {
		report.AddError(fmt.Sprintf("Walk error: %v", err))
	}
}

// =============================================================================
// PARSING HELPERS
// =============================================================================

// Extract YAML block between "---"
func extractFrontmatter(content string) (string, bool) {
	if !strings.HasPrefix(content, "---") {
		return "", false
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return "", false
	}
	return parts[1], true
}

// Extract value from "key: value" line
func extractField(yamlBlock, key string) string {
	lines := strings.Split(yamlBlock, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if strings.HasPrefix(trimmed, "#") { continue } // Skip comments

		// Case-insensitive key check
		if strings.HasPrefix(strings.ToLower(trimmed), key+":") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) < 2 { return "" }
			
			val := strings.TrimSpace(parts[1])
			
			// Remove inline comments
			if idx := strings.Index(val, "#"); idx > -1 {
				val = strings.TrimSpace(val[:idx])
			}

			// Remove quotes
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}
