package assess

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bytes"
	"encoding/json"
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
		// Placeholder for semantic validators (jsonschema/openapi/asyncapi/protobuf) in future passes
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
	include := map[string]struct{}{}
	for _, inc := range config.IncludeFiles {
		include[filepath.Clean(inc)] = struct{}{}
	}
	// If include list provided, prefer those paths directly
	if len(include) > 0 {
		for inc := range include {
			// Accept only yaml/json files for this preview
			low := strings.ToLower(inc)
			if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".json") {
				files = append(files, inc)
			}
		}
		return files, nil
	}

	// Otherwise walk target and collect yaml/json
	err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		low := strings.ToLower(path)
		if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".json") {
			// Respect exclude filters
			for _, ex := range config.ExcludeFiles {
				if strings.Contains(path, ex) {
					return nil
				}
			}
			files = append(files, path)
		}
		return nil
	})
	return files, err
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

// small indirections to avoid extra imports in tests without clashes
var (
	jsonNewDecoder = func(r *bytes.Reader) *json.Decoder { return json.NewDecoder(r) }
	bytesNewReader = func(b []byte) *bytes.Reader { return bytes.NewReader(b) }
)

// init registers the schema assessment runner
func init() {
	RegisterAssessmentRunner(CategorySchema, NewSchemaAssessmentRunner())
}
