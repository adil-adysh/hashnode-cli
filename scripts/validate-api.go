package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	flag.Parse()

	report := &ValidationReport{}
	fmt.Printf("%s🔍 Starting Deep API Validation...%s\n", ColorBold, ColorReset)

	validateSchema(*schemaPath, report)
	printSummary(report)
}

func validateSchema(path string, report *ValidationReport) {
	// 1. Load Schema
	data, err := os.ReadFile(path)
	if err != nil {
		report.AddError(fmt.Sprintf("Read failed: %v", err))
		return
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		report.AddError(fmt.Sprintf("JSON Parse Error: %v", err))
		return
	}

	root := raw.(map[string]interface{})
	var schema map[string]interface{}
	if dataField, ok := root["data"].(map[string]interface{}); ok {
		schema, _ = dataField["__schema"].(map[string]interface{})
	} else {
		schema, _ = root["__schema"].(map[string]interface{})
	}

	types := schema["types"].([]interface{})

	// --- VALIDATION RULES ---

	// 2. Deep Check: PublishPostInput
	// We need to ensure the Input object has the specific fields we map our YAML to.
	fmt.Printf("%sChecking Input Types...%s\n", ColorCyan, ColorReset)
	requiredPostFields := []string{
		"title",
		"contentMarkdown", // Critical!
		"slug",
		"tags",
		"subtitle",
		"publishedAt",
		"seriesId", // Critical for attachment
	}
	validateTypeFields(types, "PublishPostInput", requiredPostFields, report)
	validateTypeFields(types, "UpdatePostInput", append(requiredPostFields, "id"), report)

	// 3. Deep Check: Series Inputs
	requiredSeriesFields := []string{
		"name",
		"slug",
		"descriptionMarkdown", // Critical for our Series Metadata feature
	}
	validateTypeFields(types, "CreateSeriesInput", requiredSeriesFields, report)

	// 4. Deep Check: Return Objects (What we read back)
	fmt.Printf("%sChecking Return Objects...%s\n", ColorCyan, ColorReset)

	// Check Post Object
	validateTypeFields(types, "Post", []string{"id", "slug", "title", "series"}, report)

	// Check Series Object
	validateTypeFields(types, "Series", []string{"id", "slug", "name"}, report)

	// 5. Verify Mutations exist (High Level)
	validateMutationExistence(types, report)
}

// --- Helpers ---

func validateTypeFields(types []interface{}, typeName string, requiredFields []string, report *ValidationReport) {
	typeObj := findTypeByName(types, typeName)
	if typeObj == nil {
		report.AddError(fmt.Sprintf("Type definition '%s' missing from schema", typeName))
		return
	}

	// Inputs use "inputFields", Objects use "fields"
	var fields []interface{}
	if val, ok := typeObj["inputFields"].([]interface{}); ok && val != nil {
		fields = val
	} else if val, ok := typeObj["fields"].([]interface{}); ok && val != nil {
		fields = val
	} else {
		report.AddError(fmt.Sprintf("Type '%s' has no readable fields", typeName))
		return
	}

	// Map existing fields for fast lookup
	existing := make(map[string]bool)
	for _, f := range fields {
		fMap := f.(map[string]interface{})
		existing[fMap["name"].(string)] = true
	}

	for _, req := range requiredFields {
		if !existing[req] {
			report.AddError(fmt.Sprintf("CRITICAL: Field '%s' missing in type '%s'", req, typeName))
		}
	}
}

func validateMutationExistence(types []interface{}, report *ValidationReport) {
	mutation := findTypeByName(types, "Mutation")
	if mutation == nil {
		report.AddError("Root Mutation type missing")
		return
	}

	required := []string{"publishPost", "updatePost", "createSeries"}
	fields := mutation["fields"].([]interface{})

	existing := make(map[string]bool)
	for _, f := range fields {
		existing[f.(map[string]interface{})["name"].(string)] = true
	}

	for _, req := range required {
		if !existing[req] {
			report.AddError(fmt.Sprintf("Mutation '%s' is missing", req))
		}
	}
}

func findTypeByName(types []interface{}, name string) map[string]interface{} {
	for _, t := range types {
		typeObj := t.(map[string]interface{})
		if typeObj["name"] == name {
			return typeObj
		}
	}
	return nil
}

func printSummary(r *ValidationReport) {
	fmt.Println("\n" + strings.Repeat("-", 40))
	for _, w := range r.Warnings {
		fmt.Println(w)
	}
	for _, e := range r.Errors {
		fmt.Println(e)
	}

	if len(r.Errors) == 0 {
		fmt.Printf("%sSUCCESS: Schema Deep Validation passed.%s\n", ColorGreen, ColorReset)
	} else {
		fmt.Printf("%sFAILED: %d critical schema mismatches found.%s\n", ColorRed, len(r.Errors), ColorReset)
		os.Exit(1)
	}
}
