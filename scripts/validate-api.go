package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- ANSI Colors ---
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorBold   = "\033[1m"
)

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

func main() {
	schemaPath := flag.String("schema", "schema/hashnode-schema.json", "Path to GraphQL schema JSON")
	postsPath := flag.String("posts", "posts", "Path to content directory")
	flag.Parse()

	report := &ValidationReport{}
	fmt.Printf("%s🔍 Starting hnsync validation...%s\n", ColorBold, ColorReset)

	validateSchema(*schemaPath, report)
	validateContent(*postsPath, report)

	printSummary(report)
}

func validateSchema(path string, report *ValidationReport) {
	fmt.Printf("%sChecking API Contract (%s)...%s\n", ColorCyan, path, ColorReset)

	data, err := os.ReadFile(path)
	if err != nil {
		report.AddError(fmt.Sprintf("Read failed: %v", err))
		return
	}

	// Use interface{} to handle dynamic wrappers
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		report.AddError(fmt.Sprintf("JSON Parse Error: %v", err))
		return
	}

	root, ok := raw.(map[string]interface{})
	if !ok {
		report.AddError("JSON root is not an object")
		return
	}

	// Navigate to __schema (Handles { "data": { "__schema": ... } } or raw { "__schema": ... })
	var schema map[string]interface{}
	if dataField, ok := root["data"].(map[string]interface{}); ok {
		schema, _ = dataField["__schema"].(map[string]interface{})
	} else {
		schema, _ = root["__schema"].(map[string]interface{})
	}

	if schema == nil {
		report.AddError("Could not find '__schema' key in JSON")
		return
	}

	// Extract Types
	types, ok := schema["types"].([]interface{})
	if !ok {
		report.AddError("Schema missing 'types' array")
		return
	}

	// Find Mutation Type
	var mutationFields []interface{}
	for _, t := range types {
		typeObj := t.(map[string]interface{})
		if typeObj["name"] == "Mutation" {
			mutationFields, _ = typeObj["fields"].([]interface{})
			break
		}
	}

	if mutationFields == nil {
		report.AddError("Mutation type not found in schema")
		return
	}

	// Check for required fields
	required := map[string]bool{"publishPost": false, "updatePost": false, "createSeries": false}
	for _, f := range mutationFields {
		name := f.(map[string]interface{})["name"].(string)
		if _, exists := required[name]; exists {
			required[name] = true
		}
	}

	for name, found := range required {
		if !found {
			report.AddError(fmt.Sprintf("API Breakage: Mutation '%s' is missing", name))
		}
	}
}

func validateContent(rootPath string, report *ValidationReport) {
	fmt.Printf("%sLinting Content (%s)...%s\n", ColorCyan, rootPath, ColorReset)

	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		report.AddError(fmt.Sprintf("Directory '%s' not found", rootPath))
		return
	}

	slugMap := make(map[string]string)
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		raw, _ := os.ReadFile(path)
		content := string(raw)

		if !strings.HasPrefix(content, "---") {
			report.AddWarning(fmt.Sprintf("[%s] Missing frontmatter", info.Name()))
			return nil
		}

		// Simple slug extraction for the spike
		if strings.Contains(content, "slug:") {
			slug := extractValue(content, "slug")
			if existing, exists := slugMap[slug]; exists {
				report.AddError(fmt.Sprintf("Slug Collision: '%s' in %s and %s", slug, existing, info.Name()))
			}
			slugMap[slug] = info.Name()
		}

		return nil
	})

	if err != nil {
		report.AddError(fmt.Sprintf("File walk failed: %v", err))
	}
}

func extractValue(content, key string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}

func printSummary(r *ValidationReport) {
	fmt.Println("\n" + strings.Repeat("-", 40))
	for _, w := range r.Warnings { fmt.Println(w) }
	for _, e := range r.Errors { fmt.Println(e) }
	fmt.Println(strings.Repeat("-", 40))

	if len(r.Errors) > 0 {
		fmt.Printf("%sFAILED: %d errors found.%s\n", ColorRed, len(r.Errors), ColorReset)
		os.Exit(1)
	}
	fmt.Printf("%sSUCCESS: Integrity check passed.%s\n", ColorGreen, ColorReset)
}
