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
    "github.com/3leaps/goneat/internal/assets"
    "github.com/3leaps/goneat/pkg/work"
    "github.com/xeipuuv/gojsonschema"
    "gopkg.in/yaml.v3"
)

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
			ExecutionTime: time.Since(start),
			Error:         fmt.Sprintf("discovery failed: %v", err),
		}, nil
	}

    for _, f := range candidates {
		select {
		case <-ctx.Done():
			return &AssessmentResult{CommandName: r.commandName, Category: CategorySchema, Success: false, ExecutionTime: time.Since(start), Error: ctx.Err().Error()}, nil
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
					EstimatedTime: 2 * time.Minute,
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
					EstimatedTime: 2 * time.Minute,
				})
				continue
			}
        }
        // Minimal JSON Schema structural validation for repo schema files (preview)
        if r.isLikelyJSONSchema(f) {
            if err := r.checkJSONSchemaStructure(f); err != nil {
                issues = append(issues, Issue{
                    File:          f,
                    Severity:      SeverityHigh,
                    Message:       fmt.Sprintf("JSON Schema structural validation failed: %v", err),
                    Category:      CategorySchema,
                    SubCategory:   "jsonschema",
                    AutoFixable:   false,
                    EstimatedTime: 3 * time.Minute,
                })
            }
        }
    }

	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategorySchema,
		Success:       true,
		ExecutionTime: time.Since(start),
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

    // If include files provided, use them directly (subject to exclude and extension)
    if len(config.IncludeFiles) > 0 {
        for _, inc := range config.IncludeFiles {
            p := filepath.Clean(inc)
            info, err := os.Stat(p)
            if err == nil && info.IsDir() {
                // Use planner for this directory to respect .goneatignore
                planner := work.NewPlanner(work.PlannerConfig{
                    Command:           "schema",
                    Paths:             []string{p},
                    ExecutionStrategy: "sequential",
                    IgnoreFile:        ".goneatignore",
                    Verbose:           false,
                })
                manifest, perr := planner.GenerateManifest()
                if perr == nil {
                    for _, item := range manifest.WorkItems {
                        if !isUnderSchemas(item.Path) { continue }
                        low := strings.ToLower(item.Path)
                        if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".json") {
                            if !r.isExcluded(item.Path, config) {
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
                if !r.isExcluded(p, config) {
                    files = append(files, p)
                }
            }
        }
        return files, nil
    }

    // Use unified planner to respect .goneatignore
    planner := work.NewPlanner(work.PlannerConfig{
        Command:           "schema",
        Paths:             []string{target},
        ExecutionStrategy: "sequential",
        IgnoreFile:        ".goneatignore",
        Verbose:           false,
    })
    manifest, err := planner.GenerateManifest()
    if err != nil {
        return nil, fmt.Errorf("failed to discover files: %w", err)
    }
    for _, item := range manifest.WorkItems {
        p := item.Path
        if !isUnderSchemas(p) {
            continue
        }
        low := strings.ToLower(p)
        if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".json") {
            if !r.isExcluded(p, config) {
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

func isUnderSchemas(path string) bool {
    parts := strings.Split(filepath.ToSlash(path), "/")
    for _, seg := range parts {
        if seg == "schemas" {
            return true
        }
    }
    return false
}

func (r *SchemaAssessmentRunner) checkJSONSyntax(path string) error {
	// Read file and attempt decode
	data, err := os.ReadFile(path)
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
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var v interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

// isLikelyJSONSchema detects if a file appears to be a JSON Schema document we maintain
func (r *SchemaAssessmentRunner) isLikelyJSONSchema(path string) bool {
    low := strings.ToLower(path)
    if strings.Contains(low, "/schemas/") {
        return true
    }
    // Quick content sniff: presence of "$schema" or top-level "type" is handled in structural check (performed later)
    return false
}

// checkJSONSchemaStructure performs minimal structural checks for JSON Schema documents in YAML/JSON
func (r *SchemaAssessmentRunner) checkJSONSchemaStructure(path string) error {
    data, err := os.ReadFile(path)
    if err != nil { return err }
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
    // Load embedded meta-schema
    meta, ok := assets.GetJSONSchemaMeta(draft)
    if !ok || len(meta) == 0 {
        return fmt.Errorf("embedded meta-schema not available for %s", draft)
    }
    // Convert doc to canonical JSON bytes for validation
    jsonDoc, err := toCanonicalJSON(doc)
    if err != nil { return fmt.Errorf("json conversion failed: %v", err) }

    schemaLoader := gojsonschema.NewBytesLoader(meta)
    docLoader := gojsonschema.NewBytesLoader(jsonDoc)
    res, err := gojsonschema.Validate(schemaLoader, docLoader)
    if err != nil { return fmt.Errorf("meta-validation error: %v", err) }
    if !res.Valid() {
        var b strings.Builder
        for _, e := range res.Errors() {
            b.WriteString(e.String())
            b.WriteString("\n")
        }
        return fmt.Errorf("meta-schema validation failed:\n%s", b.String())
    }
    // Run additional structural sanity checks to catch issues not covered by the library/draft support.
    if err := sanityCheckJSONSchema(doc); err != nil {
        return err
    }
    return nil
}

func toCanonicalJSON(v interface{}) ([]byte, error) {
    // Marshal through encoding/json for validation loader
    // Ensure numbers are retained reasonably; here we allow defaults
    // to keep implementation simple.
    return json.Marshal(v)
}

// sanityCheckJSONSchema applies basic structural rules independent of draft support
func sanityCheckJSONSchema(doc interface{}) error {
    m, ok := doc.(map[string]interface{})
    if !ok {
        return fmt.Errorf("root must be an object")
    }
    if v, ok := m["type"]; ok {
        allowed := map[string]struct{}{"string":{},"number":{},"integer":{},"object":{},"array":{},"boolean":{},"null":{}}
        switch tv := v.(type) {
        case string:
            if _, ok := allowed[tv]; !ok {
                return fmt.Errorf("type must be one of string, number, integer, object, array, boolean, null")
            }
        case []interface{}:
            for _, it := range tv {
                s, ok := it.(string)
                if !ok { return fmt.Errorf("type array must contain strings") }
                if _, ok := allowed[s]; !ok { return fmt.Errorf("type contains invalid entry: %s", s) }
            }
        default:
            return fmt.Errorf("type must be string or array of strings")
        }
    }
    if v, ok := m["required"]; ok {
        arr, ok := v.([]interface{})
        if !ok { return fmt.Errorf("required must be an array of strings") }
        for _, it := range arr {
            if _, ok := it.(string); !ok { return fmt.Errorf("required must contain only strings") }
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
