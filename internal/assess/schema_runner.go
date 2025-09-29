package assess

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bytes"
	"encoding/json"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/fulmenhq/goneat/pkg/work"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// isOfflineMode checks if offline schema validation is enabled via environment variable
func isOfflineMode() bool {
	return os.Getenv("GONEAT_OFFLINE_SCHEMA_VALIDATION") == "true"
}

// SchemaAssessmentRunner implements AssessmentRunner for schema-aware validation (preview)
type SchemaAssessmentRunner struct {
	commandName string
}

func NewSchemaAssessmentRunner() *SchemaAssessmentRunner {
	return &SchemaAssessmentRunner{commandName: "schema"}
}

func (r *SchemaAssessmentRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	start := time.Now()
	var issues []Issue

	// Discover candidate files via include patterns or by extension (yaml/json) under target
	candidates, err := r.findCandidates(target, config)
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategorySchema,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(start)),
			Error:         fmt.Sprintf("discovery failed: %v", err),
		}, nil
	}

	for _, f := range candidates {
		select {
		case <-ctx.Done():
			return &AssessmentResult{CommandName: r.commandName, Category: CategorySchema, Success: false, ExecutionTime: HumanReadableDuration(time.Since(start)), Error: ctx.Err().Error()}, nil
		default:
		}

		// Syntax validation first
		if strings.HasSuffix(strings.ToLower(f), ".json") {
			if err := r.checkJSONSyntax(f); err != nil {
				issues = append(issues, Issue{
					File:          f,
					Severity:      SeverityHigh,
					Message:       fmt.Sprintf("JSON syntax error: %v", err),
					Category:      CategorySchema,
					SubCategory:   "json_syntax",
					AutoFixable:   false,
					EstimatedTime: HumanReadableDuration(2 * time.Minute),
				})
				continue
			}
		}
		if strings.HasSuffix(strings.ToLower(f), ".yaml") || strings.HasSuffix(strings.ToLower(f), ".yml") {
			if err := r.checkYAMLSyntax(f); err != nil {
				issues = append(issues, Issue{
					File:          f,
					Severity:      SeverityHigh,
					Message:       fmt.Sprintf("YAML syntax error: %v", err),
					Category:      CategorySchema,
					SubCategory:   "yaml_syntax",
					AutoFixable:   false,
					EstimatedTime: HumanReadableDuration(2 * time.Minute),
				})
				continue
			}
		}

		// Enhanced schema validation logic
		isSchema, draft, err := r.detectSchemaInfo(f)
		if err != nil {
			issues = append(issues, Issue{
				File:          f,
				Severity:      SeverityMedium,
				Message:       fmt.Sprintf("Failed to analyze schema info: %v", err),
				Category:      CategorySchema,
				SubCategory:   "schema_analysis",
				AutoFixable:   false,
				EstimatedTime: HumanReadableDuration(1 * time.Minute),
			})
			continue
		}

		if isSchema {
			// Check if draft is allowed by configuration
			if !r.isDraftAllowed(draft, config) {
				issues = append(issues, Issue{
					File:          f,
					Severity:      SeverityLow,
					Message:       fmt.Sprintf("Schema draft '%s' not in allowed drafts list", draft),
					Category:      CategorySchema,
					SubCategory:   "draft_filter",
					AutoFixable:   false,
					EstimatedTime: HumanReadableDuration(1 * time.Minute),
				})
				continue
			}

			// Basic structural validation
			if err := r.checkJSONSchemaStructure(f); err != nil {
				issues = append(issues, Issue{
					File:          f,
					Severity:      SeverityHigh,
					Message:       fmt.Sprintf("Schema structural validation failed: %v", err),
					Category:      CategorySchema,
					SubCategory:   "schema_structure",
					AutoFixable:   false,
					EstimatedTime: HumanReadableDuration(3 * time.Minute),
				})
				continue
			}

			// Enhanced meta-validation using pkg/schema
			if config.SchemaEnableMeta {
				metaIssues := r.validateSchemaMeta(f, draft)
				issues = append(issues, metaIssues...)
			}
		}
	}

	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategorySchema,
		Success:       true,
		ExecutionTime: HumanReadableDuration(time.Since(start)),
		Issues:        issues,
	}, nil
}

func (r *SchemaAssessmentRunner) CanRunInParallel() bool          { return true }
func (r *SchemaAssessmentRunner) GetCategory() AssessmentCategory { return CategorySchema }
func (r *SchemaAssessmentRunner) GetEstimatedTime(target string) time.Duration {
	return 2 * time.Second
}
func (r *SchemaAssessmentRunner) IsAvailable() bool { return true }

func (r *SchemaAssessmentRunner) findCandidates(target string, config AssessmentConfig) ([]string, error) {
	var files []string

	// Helper to derive scope roots from include dirs and force-include anchors
	deriveScopeRoots := func() []string {
		var roots []string
		// Include directories
		for _, inc := range config.IncludeFiles {
			p := filepath.Clean(inc)
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				roots = append(roots, p)
			}
		}
		// Force-include anchors
		for _, pat := range config.ForceInclude {
			s := strings.TrimSpace(filepath.ToSlash(pat))
			if s == "" {
				continue
			}
			if strings.HasSuffix(s, "/**") {
				roots = append(roots, filepath.FromSlash(strings.TrimSuffix(s, "/**")))
				continue
			}
			cut := strings.IndexAny(s, "*[?")
			var anchor string
			if cut >= 0 {
				anchor = s[:cut]
			} else {
				anchor = s
			}
			if anchor == "" {
				continue
			}
			roots = append(roots, filepath.FromSlash(anchor))
		}
		// Dedup
		uniq := make(map[string]struct{}, len(roots))
		var out []string
		for _, r := range roots {
			if r == "" {
				continue
			}
			if _, ok := uniq[r]; !ok {
				uniq[r] = struct{}{}
				out = append(out, r)
			}
		}
		return out
	}

	// If include files provided, use them directly (subject to exclude and extension)
	if len(config.IncludeFiles) > 0 {
		for _, inc := range config.IncludeFiles {
			p := filepath.Clean(inc)
			info, err := os.Stat(p)
			if err == nil && info.IsDir() {
				// If scoped, walk the directory directly to guarantee all matching files are collected
				// regardless of ignore interplay, then let per-file checks decide schema relevance.
				if config.Scope {
					_ = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
						if err != nil {
							return nil
						}
						if d.IsDir() {
							return nil
						}
						low := strings.ToLower(path)
						if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".json") {
							if !r.isExcluded(path, config) && r.isSchemaCandidate(path, config) {
								files = append(files, path)
							}
						}
						return nil
					})
					continue
				}
				// Use planner for this directory to respect .goneatignore
				pcfg := work.PlannerConfig{
					Command:              "schema",
					Paths:                []string{p},
					ExecutionStrategy:    "sequential",
					IgnoreFile:           ".goneatignore",
					Verbose:              false,
					NoIgnore:             config.NoIgnore,
					ForceIncludePatterns: append([]string(nil), config.ForceInclude...),
				}
				if config.Scope {
					if roots := deriveScopeRoots(); len(roots) > 0 {
						// Keep only roots under p
						var under []string
						for _, s := range roots {
							sp := filepath.ToSlash(s) + "/"
							pp := filepath.ToSlash(p) + "/"
							if strings.HasPrefix(sp, pp) {
								under = append(under, s)
							}
						}
						if len(under) > 0 {
							pcfg.Paths = under
						}
					}
				}
				planner := work.NewPlanner(pcfg)
				manifest, perr := planner.GenerateManifest()
				if perr == nil {
					for _, item := range manifest.WorkItems {
						low := strings.ToLower(item.Path)
						if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".json") {
							if !r.isExcluded(item.Path, config) && r.isSchemaCandidate(item.Path, config) {
								files = append(files, item.Path)
							}
						}
					}
				}
				continue
			}
			// File path include
			low := strings.ToLower(p)
			if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".json") {
				if !r.isExcluded(p, config) && r.isSchemaCandidate(p, config) {
					files = append(files, p)
				}
			}
		}
		return files, nil
	}

	// Use unified planner to respect .goneatignore
	pcfg := work.PlannerConfig{
		Command:              "schema",
		Paths:                []string{target},
		ExecutionStrategy:    "sequential",
		IgnoreFile:           ".goneatignore",
		Verbose:              false,
		NoIgnore:             config.NoIgnore,
		ForceIncludePatterns: append([]string(nil), config.ForceInclude...),
	}
	if config.Scope {
		if roots := deriveScopeRoots(); len(roots) > 0 {
			pcfg.Paths = roots
		}
	}
	planner := work.NewPlanner(pcfg)
	manifest, err := planner.GenerateManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to discover files: %w", err)
	}
	for _, item := range manifest.WorkItems {
		p := item.Path
		low := strings.ToLower(p)
		if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".json") {
			if !r.isExcluded(p, config) && r.isSchemaCandidate(p, config) {
				files = append(files, p)
			}
		}
	}
	return files, nil
}

func (r *SchemaAssessmentRunner) isExcluded(path string, config AssessmentConfig) bool {
	for _, ex := range config.ExcludeFiles {
		if strings.Contains(path, ex) {
			return true
		}
	}
	return false
}

// isSchemaCandidate determines if a file should be considered for schema validation
// based on the discovery mode and configured patterns
func (r *SchemaAssessmentRunner) isSchemaCandidate(path string, config AssessmentConfig) bool {
	// Check custom patterns first if provided
	if len(config.SchemaPatterns) > 0 {
		for _, pattern := range config.SchemaPatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return true
			}
		}
		// If patterns are specified but none match, it's not a candidate
		return false
	}

	// Determine discovery mode (default to "schemas-dir" for backward compatibility)
	discoveryMode := config.SchemaDiscoveryMode
	if discoveryMode == "" {
		discoveryMode = "schemas-dir"
	}

	switch discoveryMode {
	case "schemas-dir":
		// Original behavior: only files under directories named "schemas"
		return isUnderSchemas(path)
	case "all":
		// Enhanced behavior: check if file has $schema field
		return r.hasSchemaField(path)
	default:
		// Unknown mode, fall back to schemas-dir
		return isUnderSchemas(path)
	}
}

// hasSchemaField checks if a JSON/YAML file contains a $schema field
func (r *SchemaAssessmentRunner) hasSchemaField(path string) bool {
	data, err := safeReadFile(path)
	if err != nil {
		return false
	}

	// Try to parse as YAML first, then JSON
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// Try JSON if YAML fails
		if err := json.Unmarshal(data, &doc); err != nil {
			return false
		}
	}

	// Check for $schema field
	_, hasSchema := doc["$schema"]
	return hasSchema
}

// extractDraftFromURL extracts the draft version from a $schema URL
func (r *SchemaAssessmentRunner) extractDraftFromURL(schemaURL string) string {
	// Common schema URL patterns:
	// "https://json-schema.org/draft-07/schema" -> "draft-07"
	// "https://json-schema.org/draft/2020-12/schema" -> "2020-12"
	// "http://json-schema.org/draft-07/schema#" -> "draft-07"

	if strings.Contains(schemaURL, "draft-07") {
		return "draft-07"
	}
	if strings.Contains(schemaURL, "2020-12") {
		return "2020-12"
	}
	if strings.Contains(schemaURL, "2019-09") {
		return "2019-09"
	}
	if strings.Contains(schemaURL, "draft-04") {
		return "draft-04"
	}

	// Default to 2020-12 if we can't determine
	return "2020-12"
}

// detectSchemaInfo detects if a file is a schema and returns its draft version
func (r *SchemaAssessmentRunner) detectSchemaInfo(path string) (isSchema bool, draft string, err error) {
	data, err := safeReadFile(path)
	if err != nil {
		return false, "", err
	}

	// Try to parse as YAML first, then JSON
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// Try JSON if YAML fails
		if err := json.Unmarshal(data, &doc); err != nil {
			return false, "", nil // Not a valid JSON/YAML file
		}
	}

	// Check for $schema field
	if schemaURL, ok := doc["$schema"].(string); ok {
		draft := r.extractDraftFromURL(schemaURL)
		return true, draft, nil
	}

	return false, "", nil
}

// isDraftAllowed checks if the detected draft is in the allowed list
func (r *SchemaAssessmentRunner) isDraftAllowed(draft string, config AssessmentConfig) bool {
	if len(config.SchemaDrafts) == 0 {
		// No filter specified, allow all supported drafts
		return draft == "draft-07" || draft == "2020-12" || draft == "2019-09"
	}

	for _, allowedDraft := range config.SchemaDrafts {
		if draft == allowedDraft {
			return true
		}
	}
	return false
}

func isUnderSchemas(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, seg := range parts {
		if seg == "schemas" {
			return true
		}
	}
	return false
}

// validateSchemaMeta performs meta-validation using the pkg/schema library
func (r *SchemaAssessmentRunner) validateSchemaMeta(path string, draft string) []Issue {
	var issues []Issue

	// Use the enhanced pkg/schema library for validation
	validator, err := schema.NewValidatorFromMetaSchema(draft)
	if err != nil {
		issues = append(issues, Issue{
			File:          path,
			Severity:      SeverityMedium,
			Message:       fmt.Sprintf("Unsupported schema draft '%s': %v", draft, err),
			Category:      CategorySchema,
			SubCategory:   "meta_validation",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(1 * time.Minute),
		})
		return issues
	}

	// Read and validate the file
	data, err := safeReadFile(path)
	if err != nil {
		issues = append(issues, Issue{
			File:          path,
			Severity:      SeverityHigh,
			Message:       fmt.Sprintf("Failed to read schema file: %v", err),
			Category:      CategorySchema,
			SubCategory:   "meta_validation",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(1 * time.Minute),
		})
		return issues
	}

	// Validate using the pkg/schema library
	result, err := validator.ValidateBytes(data)
	if err != nil {
		issues = append(issues, Issue{
			File:          path,
			Severity:      SeverityHigh,
			Message:       fmt.Sprintf("Meta-validation error: %v", err),
			Category:      CategorySchema,
			SubCategory:   "meta_validation",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(3 * time.Minute),
		})
		return issues
	}

	// Process validation errors with enhanced reporting
	if !result.Valid {
		for _, verr := range result.Errors {
			severity := SeverityMedium
			// Categorize errors by severity
			if strings.Contains(strings.ToLower(verr.Message), "required") {
				severity = SeverityHigh
			} else if strings.Contains(strings.ToLower(verr.Message), "additional") {
				severity = SeverityLow
			}

			issues = append(issues, Issue{
				File:          path,
				Line:          verr.Context.LineNumber,
				Severity:      severity,
				Message:       fmt.Sprintf("Schema validation error at %s: %s", verr.Path, verr.Message),
				Category:      CategorySchema,
				SubCategory:   "meta_validation",
				AutoFixable:   false,
				EstimatedTime: HumanReadableDuration(2 * time.Minute),
			})
		}
	}

	return issues
}

func (r *SchemaAssessmentRunner) checkJSONSyntax(path string) error {
	// Read file and attempt decode (sanitized and restricted to repo root)
	data, err := safeReadFile(path) // #nosec G304 -- path cleaned and restricted to working directory
	if err != nil {
		return err
	}
	var v interface{}
	dec := jsonNewDecoder(bytesNewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	return nil
}

func (r *SchemaAssessmentRunner) checkYAMLSyntax(path string) error {
	// Read file with sanitized path rules
	data, err := safeReadFile(path) // #nosec G304 -- path cleaned and restricted to working directory
	if err != nil {
		return err
	}
	var v interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

// checkJSONSchemaStructure performs minimal structural checks for JSON Schema documents in YAML/JSON
func (r *SchemaAssessmentRunner) checkJSONSchemaStructure(path string) error {
	data, err := safeReadFile(path) // #nosec G304 -- path cleaned and restricted to working directory
	if err != nil {
		return err
	}
	// Try YAML first
	var doc interface{}
	if yaml.Unmarshal(data, &doc) != nil {
		// Fallback to JSON
		var j interface{}
		dec := jsonNewDecoder(bytesNewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&j); err != nil {
			return fmt.Errorf("unable to parse as YAML or JSON: %v", err)
		}
		doc = j
	}
	// Determine draft from $schema (default to 2020-12)
	draft := "2020-12"
	if m, ok := doc.(map[string]interface{}); ok {
		if v, ok := m["$schema"].(string); ok {
			switch {
			case strings.Contains(v, "draft-07"):
				draft = "draft-07"
			case strings.Contains(v, "2020-12"):
				draft = "2020-12"
			}
		}
	}
	// Offline-first: run our structural sanity checks before library validation
	if err := sanityCheckJSONSchema(doc); err != nil {
		return err
	}

	// Skip library meta-schema validation for now (network-heavy and draft support varies).
	// Sanity checks above provide offline structural validation for key fields.
	_ = draft
	return nil
}

// checkJSONSchemaWithMeta attempts meta-schema validation via gojsonschema using embedded drafts.
// Note: Remote references are disabled to ensure offline operation.
func (r *SchemaAssessmentRunner) checkJSONSchemaWithMeta(path string) error {
	data, err := safeReadFile(path) // #nosec G304 -- path cleaned and restricted to working directory
	if err != nil {
		return err
	}
	var doc interface{}
	if yaml.Unmarshal(data, &doc) != nil {
		var j interface{}
		dec := jsonNewDecoder(bytesNewReader(data))
		dec.UseNumber()
		if err := dec.Decode(&j); err != nil {
			return fmt.Errorf("unable to parse as YAML or JSON: %v", err)
		}
		doc = j
	}

	// Conditionally remove $schema field to prevent remote fetching in offline mode
	if isOfflineMode() {
		if m, ok := doc.(map[string]interface{}); ok {
			delete(m, "$schema")
		}
	}

	// Determine draft
	draft := "2020-12"
	if m, ok := doc.(map[string]interface{}); ok {
		if v, ok := m["$schema"].(string); ok {
			switch {
			case strings.Contains(v, "draft-07"):
				draft = "draft-07"
			case strings.Contains(v, "2020-12"):
				draft = "2020-12"
			}
		}
	}
	meta, ok := assets.GetJSONSchemaMeta(draft)
	if !ok || len(meta) == 0 {
		return fmt.Errorf("embedded meta-schema not available for %s", draft)
	}

	// Conditionally remove $schema from meta-schema in offline mode
	if isOfflineMode() {
		var metaDoc interface{}
		if err := json.Unmarshal(meta, &metaDoc); err == nil {
			if m, ok := metaDoc.(map[string]interface{}); ok {
				delete(m, "$schema")
				if newMeta, err := json.Marshal(metaDoc); err == nil {
					meta = newMeta
				}
			}
		}
	}

	jsonDoc, err := toCanonicalJSON(doc)
	if err != nil {
		return fmt.Errorf("json conversion failed: %v", err)
	}
	schemaLoader := gojsonschema.NewBytesLoader(meta)
	docLoader := gojsonschema.NewBytesLoader(jsonDoc)
	res, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return fmt.Errorf("meta-validation error: %v", err)
	}
	if !res.Valid() {
		var b strings.Builder
		for _, e := range res.Errors() {
			b.WriteString(e.String())
			b.WriteString("\n")
		}
		return fmt.Errorf("meta-schema validation failed:\n%s", b.String())
	}
	return nil
}

func toCanonicalJSON(v interface{}) ([]byte, error) {
	// Marshal through encoding/json for validation loader
	// Ensure numbers are retained reasonably; here we allow defaults
	// to keep implementation simple.
	return json.Marshal(v)
}

// safeReadFile sanitizes the path, resolves to absolute, and restricts reads to the current working directory subtree.
func safeReadFile(p string) ([]byte, error) {
	cleaned := filepath.Clean(p)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return nil, err
	}
	// Determine repository root (presence of .git) to constrain reads within repo
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		// Fallback to working directory constraint
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repoRoot = wd
	}
	rootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	// Ensure abs is under repo root
	sep := string(os.PathSeparator)
	if abs != rootAbs && !strings.HasPrefix(abs+sep, rootAbs+sep) {
		return nil, fmt.Errorf("refusing to read outside repository root: %s", abs)
	}
	// Read file after validation: path is cleaned and constrained to repo subtree
	return os.ReadFile(abs) // #nosec G304 -- sanitized absolute path within repository root
}

// findRepoRoot walks up from the current working directory to locate a .git directory
func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root
			return ""
		}
		dir = parent
	}
}

// sanityCheckJSONSchema applies basic structural rules independent of draft support
func sanityCheckJSONSchema(doc interface{}) error {
	m, ok := doc.(map[string]interface{})
	if !ok {
		return fmt.Errorf("root must be an object")
	}
	if v, ok := m["type"]; ok {
		allowed := map[string]struct{}{"string": {}, "number": {}, "integer": {}, "object": {}, "array": {}, "boolean": {}, "null": {}}
		switch tv := v.(type) {
		case string:
			if _, ok := allowed[tv]; !ok {
				return fmt.Errorf("type must be one of string, number, integer, object, array, boolean, null")
			}
		case []interface{}:
			for _, it := range tv {
				s, ok := it.(string)
				if !ok {
					return fmt.Errorf("type array must contain strings")
				}
				if _, ok := allowed[s]; !ok {
					return fmt.Errorf("type contains invalid entry: %s", s)
				}
			}
		default:
			return fmt.Errorf("type must be string or array of strings")
		}
	}
	if v, ok := m["required"]; ok {
		arr, ok := v.([]interface{})
		if !ok {
			return fmt.Errorf("required must be an array of strings")
		}
		for _, it := range arr {
			if _, ok := it.(string); !ok {
				return fmt.Errorf("required must contain only strings")
			}
		}
	}
	if v, ok := m["additionalProperties"]; ok {
		switch v.(type) {
		case bool, map[string]interface{}:
			// ok
		default:
			return fmt.Errorf("additionalProperties must be boolean or object")
		}
	}
	return nil
}

// small indirections to avoid extra imports in tests without clashes
var (
	jsonNewDecoder = func(r *bytes.Reader) *json.Decoder { return json.NewDecoder(r) }
	bytesNewReader = func(b []byte) *bytes.Reader { return bytes.NewReader(b) }
)

// init registers the schema assessment runner
func init() {
	RegisterAssessmentRunner(CategorySchema, NewSchemaAssessmentRunner())
}
