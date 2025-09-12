package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/pkg/safeio"
	"github.com/xeipuuv/gojsonschema"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

// Result holds the validation result.
type Result struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationContext provides additional context for validation errors.
type ValidationContext struct {
	SourceFile string `json:"source_file,omitempty"`
	SourceType string `json:"source_type,omitempty"` // "file", "bytes", "string"
	LineNumber int    `json:"line_number,omitempty"` // From gojsonschema location
	Severity   string `json:"severity,omitempty"`    // "error", "warning"
}

// ValidationError represents a single validation error.
type ValidationError struct {
	Path    string            `json:"path,omitempty"`
	Message string            `json:"message"`
	Context ValidationContext `json:"context,omitempty"`
}

// SecurityContext configures security constraints for validation operations.
type SecurityContext struct {
	AllowedDirs  []string `json:"allowed_dirs,omitempty"`        // Anchor paths to these dirs
	MaxFileSize  int64    `json:"max_file_size_bytes,omitempty"` // Default: 10MB
	EnforceDraft bool     `json:"enforce_draft_only,omitempty"`  // Limit to Draft-07/2020-12
}

// NewSecurityContext returns a SecurityContext with secure defaults.
func NewSecurityContext() SecurityContext {
	return SecurityContext{
		MaxFileSize:  10 * 1024 * 1024, // 10MB default
		EnforceDraft: true,             // Always enforce draft limits
		AllowedDirs:  []string{"."},    // Current dir only by default
	}
}

// ValidationOptions configures optional behaviors for validation.
type ValidationOptions struct {
	Context ValidationContext
	Audit   bool
}

// BatchOptions configures batch validation behavior.
type BatchOptions struct {
	MaxConcurrency int           `json:"max_concurrency,omitempty"` // Default: runtime.NumCPU()
	Timeout        time.Duration `json:"timeout,omitempty"`         // Default: 30s
	Security       SecurityContext
}

// BatchResult aggregates results from multiple validations.
type BatchResult struct {
	Valid           bool               `json:"valid"`
	TotalFiles      int                `json:"total_files"`
	ValidFiles      int                `json:"valid_files"`
	InvalidFiles    int                `json:"invalid_files"`
	OverallSeverity string             `json:"overall_severity,omitempty"` // "pass", "warn", "fail"
	Summary         []string           `json:"summary,omitempty"`
	FileResults     map[string]*Result `json:"file_results"`
}

// registry caches compiled schemas by name for reuse
var (
	schemaRegistry map[string]*gojsonschema.Schema
	schemaPaths    map[string]string // name -> embed path
	regMu          sync.RWMutex
)

func init() {
	initRegistry()
}

func initRegistry() {
	schemaRegistry = make(map[string]*gojsonschema.Schema)
	schemaPaths = make(map[string]string)
	compileKnownSchemas()
}

func compileKnownSchemas() {
	for _, info := range assets.GetSchemaNames() {
		if data, ok := assets.GetSchema(info.Path); ok {
			if sch, err := compileSchemaBytes(data); err == nil {
				regMu.Lock()
				schemaRegistry[info.Name] = sch
				schemaPaths[info.Name] = info.Path
				regMu.Unlock()
			}
		}
	}
}

func compileSchemaBytes(schemaBytes []byte) (*gojsonschema.Schema, error) {
	// Try YAML first; if it parses, convert to canonical JSON bytes for loader
	var tmp any
	if err := yaml.Unmarshal(schemaBytes, &tmp); err == nil {
		jb, jerr := json.Marshal(tmp)
		if jerr != nil {
			return nil, fmt.Errorf("failed to encode schema to JSON: %w", jerr)
		}
		loader := gojsonschema.NewBytesLoader(jb)
		sch, err := gojsonschema.NewSchema(loader)
		if err != nil {
			return nil, fmt.Errorf("failed to load schema: %w", err)
		}
		return sch, nil
	}
	// Fall back to JSON bytes directly
	loader := gojsonschema.NewBytesLoader(schemaBytes)
	sch, err := gojsonschema.NewSchema(loader)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}
	return sch, nil
}

// Validate validates data (interface{}) against the named schema (e.g., "goneat-config-v1.0.0").
func Validate(data interface{}, schemaName string) (*Result, error) {
	// Fast path: use compiled registry
	regMu.RLock()
	sch, ok := schemaRegistry[schemaName]
	regMu.RUnlock()
	if !ok {
		// Attempt to compile on-demand from known paths
		regMu.RLock()
		path, hasPath := schemaPaths[schemaName]
		regMu.RUnlock()
		if !hasPath {
			// As a last resort, attempt legacy mapping (to avoid breakage)
			path = legacyMapSchemaNameToPath(schemaName)
		}
		if dataBytes, ok := assets.GetSchema(path); ok && len(dataBytes) > 0 {
			var err error
			sch, err = compileSchemaBytes(dataBytes)
			if err != nil {
				return nil, err
			}
			regMu.Lock()
			schemaRegistry[schemaName] = sch
			if _, exists := schemaPaths[schemaName]; !exists {
				schemaPaths[schemaName] = path
			}
			regMu.Unlock()
		} else {
			return nil, fmt.Errorf("schema %s not found", schemaName)
		}
	}

	dataJSON, _ := json.Marshal(data)
	docLoader := gojsonschema.NewBytesLoader(dataJSON)
	result, err := sch.Validate(docLoader)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}
	res := &Result{Valid: result.Valid()}
	if !result.Valid() {
		for _, verr := range result.Errors() {
			field := verr.Field()
			if field == "" {
				field = "root"
			}
			res.Errors = append(res.Errors, ValidationError{
				Path:    field,
				Message: verr.Description(),
			})
		}
	}
	return res, nil
}

// ValidateFromBytes validates data against schema bytes (JSON or YAML).
func ValidateFromBytes(schemaBytes []byte, data interface{}) (*Result, error) {
	var loader gojsonschema.JSONLoader

	// Try YAML first; if it parses, convert to JSON bytes for loader
	var tmp any
	if err := yaml.Unmarshal(schemaBytes, &tmp); err == nil {
		// Check draft version from the parsed YAML/JSON
		if schemaMap, ok := tmp.(map[string]interface{}); ok {
			if v, ok := schemaMap["$schema"].(string); ok {
				if !strings.Contains(v, "draft-07") && !strings.Contains(v, "2020-12") {
					return nil, fmt.Errorf("unsupported $schema: only Draft-07 and Draft-2020-12 supported")
				}
			}
		}
		jb, jerr := json.Marshal(tmp)
		if jerr != nil {
			return nil, fmt.Errorf("failed to encode schema to JSON: %w", jerr)
		}
		// For YAML input, always use the marshaled JSON bytes
		// For JSON input that was parsed as YAML (since JSON is valid YAML),
		// the marshaled version should be identical to the original
		loader = gojsonschema.NewBytesLoader(jb)
	} else {
		// JSON path - parse directly as JSON
		var schemaDoc map[string]interface{}
		if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
			return nil, fmt.Errorf("invalid schema format (must be valid YAML or JSON): %w", err)
		}
		if v, ok := schemaDoc["$schema"].(string); ok {
			if !strings.Contains(v, "draft-07") && !strings.Contains(v, "2020-12") {
				return nil, fmt.Errorf("unsupported $schema: only Draft-07 and Draft-2020-12 supported")
			}
		}
		loader = gojsonschema.NewBytesLoader(schemaBytes)
	}

	dataJSON, _ := json.Marshal(data)
	docLoader := gojsonschema.NewBytesLoader(dataJSON)
	result, err := gojsonschema.Validate(loader, docLoader)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	res := &Result{Valid: result.Valid()}
	if !result.Valid() {
		for _, verr := range result.Errors() {
			field := verr.Field()
			if field == "" {
				field = "root"
			}
			res.Errors = append(res.Errors, ValidationError{
				Path:    field,
				Message: verr.Description(),
			})
		}
	}
	return res, nil
}

// ValidateDataFromBytes validates raw data bytes against schema bytes with optional behaviors.
func ValidateDataFromBytes(schemaBytes, dataBytes []byte, opts ...func(*ValidationOptions)) (*Result, error) {
	// Parse options
	options := &ValidationOptions{}
	for _, o := range opts {
		o(options)
	}

	// Audit logging disabled for initial implementation
	// TODO: Re-enable with proper logger integration when audit feature is needed

	// Parse dataBytes to interface{} (try YAML first, then JSON)
	var data interface{}
	var parseErr error
	// Try YAML
	if err := yaml.Unmarshal(dataBytes, &data); err == nil {
		options.Context.SourceType = "yaml"
	} else {
		// Store YAML error before trying JSON
		parseErr = err
		// Fall back to JSON
		if err := json.Unmarshal(dataBytes, &data); err != nil {
			return nil, fmt.Errorf("failed to parse data bytes (tried YAML then JSON): YAML err: %v, JSON err: %w", parseErr, err)
		}
		options.Context.SourceType = "json"
	}

	// Validate using existing logic
	res, err := ValidateFromBytes(schemaBytes, data)
	if err != nil {
		// Audit logging disabled for initial implementation
		return nil, err
	}

	// Enhance errors with context if provided
	if !res.Valid && options.Context.SourceFile != "" {
		for i := range res.Errors {
			res.Errors[i].Context = options.Context
		}
	}

	// Audit logging disabled for initial implementation

	return res, nil
}

// ValidateFile validates a file against schema bytes, sanitizing path with safeio.CleanUserPath.
func ValidateFile(schemaBytes []byte, dataFilePath string) (*Result, error) {
	cleanPath, err := safeio.CleanUserPath(dataFilePath)
	if err != nil {
		return nil, fmt.Errorf("path sanitization failed: %w", err)
	}

	dataBytes, err := os.ReadFile(cleanPath) // #nosec G304 -- cleanPath sanitized with safeio.CleanUserPath
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", cleanPath, err)
	}

	return ValidateDataFromBytes(schemaBytes, dataBytes)
}

// ValidateFileFromSchemaFile validates a data file against a schema file, sanitizing both paths.
func ValidateFileFromSchemaFile(schemaFilePath string, dataFilePath string) (*Result, error) {
	schemaClean, err := safeio.CleanUserPath(schemaFilePath)
	if err != nil {
		return nil, fmt.Errorf("schema path sanitization failed: %w", err)
	}

	dataClean, err := safeio.CleanUserPath(dataFilePath)
	if err != nil {
		return nil, fmt.Errorf("data path sanitization failed: %w", err)
	}

	schemaBytes, err := os.ReadFile(schemaClean) // #nosec G304 -- schemaClean sanitized with safeio.CleanUserPath
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", schemaClean, err)
	}

	dataBytes, err := os.ReadFile(dataClean) // #nosec G304 -- dataClean sanitized with safeio.CleanUserPath
	if err != nil {
		return nil, fmt.Errorf("failed to read data file %s: %w", dataClean, err)
	}

	return ValidateDataFromBytes(schemaBytes, dataBytes)
}

// ValidateFileWithSecurity validates a file against schema bytes with security constraints.
func ValidateFileWithSecurity(schemaBytes []byte, dataFilePath string, sec SecurityContext) (*Result, error) {
	cleanPath, err := safeio.CleanUserPath(dataFilePath)
	if err != nil {
		return nil, fmt.Errorf("path sanitization failed: %w", err)
	}

	// Check if path is within allowed dirs (secure containment check)
	inAllowed := false
	for _, dir := range sec.AllowedDirs {
		// Convert both to absolute paths
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		absClean, err := filepath.Abs(cleanPath)
		if err != nil {
			continue
		}

		// Check if clean path is within allowed directory
		rel, err := filepath.Rel(absDir, absClean)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(rel, "..") && !strings.Contains(rel, ".."+string(filepath.Separator)) {
			inAllowed = true
			break
		}
	}
	if !inAllowed {
		return nil, fmt.Errorf("path %s not in allowed directories: %v", cleanPath, sec.AllowedDirs)
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", cleanPath, err)
	}
	if info.Size() > sec.MaxFileSize {
		return nil, fmt.Errorf("file %s exceeds max size %d bytes (actual: %d)", cleanPath, sec.MaxFileSize, info.Size())
	}

	dataBytes, err := os.ReadFile(cleanPath) // #nosec G304 -- cleanPath validated against allowed directories with size limits
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", cleanPath, err)
	}

	// Draft enforcement is handled in ValidateFromBytes

	return ValidateDataFromBytes(schemaBytes, dataBytes, func(o *ValidationOptions) {
		o.Context.SourceFile = cleanPath
		o.Context.SourceType = "file"
	})
}

// ValidateFiles validates multiple files against schema bytes.
func ValidateFiles(schemaBytes []byte, dataFilePaths []string) (*BatchResult, error) {
	return ValidateFilesWithOptions(schemaBytes, dataFilePaths, BatchOptions{})
}

// ValidateFilesWithOptions validates multiple files with configurable options.
func ValidateFilesWithOptions(schemaBytes []byte, dataFilePaths []string, opts BatchOptions) (*BatchResult, error) {
	if opts.MaxConcurrency == 0 {
		opts.MaxConcurrency = runtime.NumCPU()
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Security.MaxFileSize == 0 {
		opts.Security = NewSecurityContext()
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(opts.MaxConcurrency)

	results := make(map[string]*Result, len(dataFilePaths))
	validCount, invalidCount, total := 0, 0, len(dataFilePaths)
	var mu sync.Mutex

	for _, path := range dataFilePaths {
		path := path // Capture for closure
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				cleanPath, err := safeio.CleanUserPath(path)
				if err != nil {
					return err
				}
				// Allowed dirs check (secure containment check)
				inAllowed := false
				for _, dir := range opts.Security.AllowedDirs {
					// Convert both to absolute paths
					absDir, err := filepath.Abs(dir)
					if err != nil {
						continue
					}
					absClean, err := filepath.Abs(cleanPath)
					if err != nil {
						continue
					}

					// Check if clean path is within allowed directory
					rel, err := filepath.Rel(absDir, absClean)
					if err != nil {
						continue
					}
					if !strings.HasPrefix(rel, "..") && !strings.Contains(rel, ".."+string(filepath.Separator)) {
						inAllowed = true
						break
					}
				}
				if !inAllowed {
					res := &Result{Valid: false}
					res.Errors = append(res.Errors, ValidationError{
						Path:    cleanPath,
						Message: "path not in allowed directories",
						Context: ValidationContext{SourceFile: cleanPath, Severity: "error"},
					})
					mu.Lock()
					results[cleanPath] = res
					invalidCount++
					mu.Unlock()
					return nil
				}
				info, err := os.Stat(cleanPath)
				if err != nil {
					res := &Result{Valid: false}
					res.Errors = append(res.Errors, ValidationError{
						Path:    cleanPath,
						Message: fmt.Sprintf("failed to stat: %v", err),
						Context: ValidationContext{SourceFile: cleanPath, Severity: "error"},
					})
					mu.Lock()
					results[cleanPath] = res
					invalidCount++
					mu.Unlock()
					return nil
				}
				if info.Size() > opts.Security.MaxFileSize {
					res := &Result{Valid: false}
					res.Errors = append(res.Errors, ValidationError{
						Path:    cleanPath,
						Message: fmt.Sprintf("file exceeds max size %d bytes (actual: %d)", opts.Security.MaxFileSize, info.Size()),
						Context: ValidationContext{SourceFile: cleanPath, Severity: "error"},
					})
					mu.Lock()
					results[cleanPath] = res
					invalidCount++
					mu.Unlock()
					return nil
				}
				dataBytes, err := os.ReadFile(cleanPath) // #nosec G304 -- cleanPath validated against security constraints
				if err != nil {
					res := &Result{Valid: false}
					res.Errors = append(res.Errors, ValidationError{
						Path:    cleanPath,
						Message: fmt.Sprintf("failed to read file: %v", err),
						Context: ValidationContext{SourceFile: cleanPath, Severity: "error"},
					})
					mu.Lock()
					results[cleanPath] = res
					invalidCount++
					mu.Unlock()
					return nil
				}
				res, err := ValidateDataFromBytes(schemaBytes, dataBytes, func(o *ValidationOptions) {
					o.Context.SourceFile = cleanPath
					o.Context.SourceType = "file"
				})
				if err != nil {
					res = &Result{Valid: false}
					res.Errors = append(res.Errors, ValidationError{
						Path:    cleanPath,
						Message: fmt.Sprintf("validation setup error: %v", err),
						Context: ValidationContext{SourceFile: cleanPath, Severity: "error"},
					})
					mu.Lock()
					results[cleanPath] = res
					invalidCount++
					mu.Unlock()
					return nil
				}
				mu.Lock()
				results[cleanPath] = res
				if res.Valid {
					validCount++
				} else {
					invalidCount++
				}
				mu.Unlock()
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	valid := validCount > 0 && invalidCount == 0
	overallSeverity := "pass"
	if invalidCount > 0 {
		overallSeverity = "fail"
	}
	summary := []string{fmt.Sprintf("Total: %d, Valid: %d, Invalid: %d", total, validCount, invalidCount)}

	return &BatchResult{
		Valid:           valid,
		TotalFiles:      total,
		ValidFiles:      validCount,
		InvalidFiles:    invalidCount,
		OverallSeverity: overallSeverity,
		Summary:         summary,
		FileResults:     results,
	}, nil
}

// ValidateDirectory validates all matching files in a directory against schema bytes.
func ValidateDirectory(schemaBytes []byte, dirPath string, pattern string) (*BatchResult, error) {
	return ValidateDirectoryWithOptions(schemaBytes, dirPath, pattern, BatchOptions{})
}

// ValidateDirectoryWithOptions validates directory files with options.
func ValidateDirectoryWithOptions(schemaBytes []byte, dirPath string, pattern string, opts BatchOptions) (*BatchResult, error) {
	if opts.MaxConcurrency == 0 {
		opts.MaxConcurrency = runtime.NumCPU()
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Security.MaxFileSize == 0 {
		opts.Security = NewSecurityContext()
	}

	var files []string
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return ValidateFilesWithOptions(schemaBytes, files, opts)
}

// legacyMapSchemaNameToPath maps schema names to their embedded filesystem paths.
// NOTE: Retained only as a fallback for names not present in assets.GetSchemaNames().
func legacyMapSchemaNameToPath(schemaName string) string {
	// Map known schema names to their paths
	knownSchemas := map[string]string{
		"goneat-config-v1.0.0":      "embedded_schemas/config/goneat-config-v1.0.0.yaml",
		"dates":                     "embedded_schemas/schemas/config/dates.yaml",
		"lifecycle-phase-v1.0.0":    "embedded_schemas/config/lifecycle-phase-v1.0.0.json",
		"release-phase-v1.0.0":      "embedded_schemas/config/release-phase-v1.0.0.json",
		"security-policy-v1.0.0":    "embedded_schemas/config/security-policy-v1.0.0.yaml",
		"suppression-report-v1.0.0": "embedded_schemas/output/suppression-report-v1.0.0.yaml",
		"hooks-manifest-v1.0.0":     "embedded_schemas/work/hooks-manifest-v1.0.0.yaml",
		"work-manifest-v1.0.0":      "embedded_schemas/work/work-manifest-v1.0.0.yaml",
	}

	if path, ok := knownSchemas[schemaName]; ok {
		return path
	}
	// Fallback: assume .yaml extension in config directory
	return "embedded_schemas/config/" + schemaName + ".yaml"
}
