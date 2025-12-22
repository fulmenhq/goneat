package schema

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
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

// isOfflineMode checks if offline schema validation is enabled via environment variable
func isOfflineMode() bool {
	return os.Getenv("GONEAT_OFFLINE_SCHEMA_VALIDATION") == "true"
}

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

// Validator wraps a compiled schema for repeated validation.
type Validator struct {
	schema *gojsonschema.Schema
}

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
		// Conditionally remove $schema field to prevent remote fetching in offline mode
		if isOfflineMode() {
			if m, ok := tmp.(map[string]interface{}); ok {
				delete(m, "$schema")
			}
		}
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
	// Fall back to JSON bytes directly - conditionally strip $schema in offline mode
	if isOfflineMode() {
		var jsonTmp any
		if err := json.Unmarshal(schemaBytes, &jsonTmp); err == nil {
			if m, ok := jsonTmp.(map[string]interface{}); ok {
				delete(m, "$schema")
				if jb, jerr := json.Marshal(jsonTmp); jerr == nil {
					schemaBytes = jb
				}
			}
		}
	}
	loader := gojsonschema.NewBytesLoader(schemaBytes)
	sch, err := gojsonschema.NewSchema(loader)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}
	return sch, nil
}

// NewValidatorFromBytes compiles schema bytes (JSON or YAML) into a reusable validator.
func NewValidatorFromBytes(schemaBytes []byte) (*Validator, error) {
	sch, err := compileSchemaBytes(schemaBytes)
	if err != nil {
		return nil, err
	}
	return &Validator{schema: sch}, nil
}

// NewValidatorFromFS loads a schema from the provided filesystem and path.
func NewValidatorFromFS(fsys fs.FS, schemaPath string) (*Validator, error) {
	data, err := fs.ReadFile(fsys, schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", schemaPath, err)
	}
	return NewValidatorFromBytes(data)
}

// NewValidatorFromEmbeddedPath loads a schema from goneat's embedded schema assets.
func NewValidatorFromEmbeddedPath(relPath string) (*Validator, error) {
	data, ok := assets.GetSchema(relPath)
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("embedded schema not found: %s", relPath)
	}
	return NewValidatorFromBytes(data)
}

// GetEmbeddedValidator returns a validator for a named embedded schema (e.g., goneat-config-v1.0.0).
func GetEmbeddedValidator(schemaName string) (*Validator, error) {
	regMu.RLock()
	if sch, ok := schemaRegistry[schemaName]; ok {
		regMu.RUnlock()
		return &Validator{schema: sch}, nil
	}
	path, hasPath := schemaPaths[schemaName]
	regMu.RUnlock()

	if !hasPath || path == "" {
		path = legacyMapSchemaNameToPath(schemaName)
	}
	if path == "" {
		return nil, fmt.Errorf("schema %s not found", schemaName)
	}

	data, ok := assets.GetSchema(path)
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("schema %s not found", schemaName)
	}

	sch, err := compileSchemaBytes(data)
	if err != nil {
		return nil, err
	}

	regMu.Lock()
	schemaRegistry[schemaName] = sch
	if _, exists := schemaPaths[schemaName]; !exists {
		schemaPaths[schemaName] = path
	}
	regMu.Unlock()

	return &Validator{schema: sch}, nil
}

// NewValidatorFromMetaSchema returns a validator for a bundled JSON Schema meta-schema draft.
func NewValidatorFromMetaSchema(draft string) (*Validator, error) {
	bytes, ok := assets.GetJSONSchemaMeta(draft)
	if !ok || len(bytes) == 0 {
		return nil, fmt.Errorf("meta-schema for %s not available", draft)
	}
	return NewValidatorFromBytes(bytes)
}

// Validate applies the compiled schema to the provided data structure.
func (v *Validator) Validate(data interface{}) (*Result, error) {
	if v == nil || v.schema == nil {
		return nil, fmt.Errorf("validator not initialised")
	}
	return validateWithCompiled(v.schema, data)
}

// ValidateBytes parses YAML/JSON bytes and validates them against the compiled schema.
func (v *Validator) ValidateBytes(dataBytes []byte) (*Result, error) {
	if v == nil || v.schema == nil {
		return nil, fmt.Errorf("validator not initialised")
	}
	var data interface{}
	if err := yaml.Unmarshal(dataBytes, &data); err != nil {
		if err := json.Unmarshal(dataBytes, &data); err != nil {
			return nil, fmt.Errorf("failed to parse data bytes (YAML/JSON): %w", err)
		}
	}
	return validateWithCompiled(v.schema, data)
}

// improveErrorMessage translates cryptic JSON Schema validator messages into more actionable ones.
func improveErrorMessage(path, message string) string {
	// Detect mutual exclusivity violations in tools-config schemas
	// Match any path under tools (e.g., "tools.badtool", "tools.goneat")
	if len(path) >= 6 && path[:6] == "tools." {
		if message == "Must not validate the schema (not)" {
			return "Both 'install' and 'install_commands' cannot be present (mutually exclusive). Use only 'install' for v1.1.0+ package managers, or only 'install_commands' for legacy scripts."
		}
		if message == "Additional property install is not allowed" {
			return "The 'install' property requires schema v1.1.0+. Either upgrade to v1.1.0 schema or use 'install_commands' instead."
		}
	}

	// Generic improvement for "not" schema failures
	if message == "Must not validate the schema (not)" {
		return message + " (Schema constraint violation - check for mutually exclusive properties or invalid combinations)"
	}

	// Return original message if no improvement available
	return message
}

func validateWithCompiled(sch *gojsonschema.Schema, data interface{}) (*Result, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encode data to JSON: %w", err)
	}
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
			originalMsg := verr.Description()
			improvedMsg := improveErrorMessage(field, originalMsg)
			res.Errors = append(res.Errors, ValidationError{
				Path:    field,
				Message: improvedMsg,
			})
		}
	}
	return res, nil
}

// Validate validates data (interface{}) against the named schema (e.g., "goneat-config-v1.0.0").
func Validate(data interface{}, schemaName string) (*Result, error) {
	validator, err := GetEmbeddedValidator(schemaName)
	if err != nil {
		return nil, err
	}
	return validator.Validate(data)
}

// ValidateFromBytes validates data against schema bytes (JSON or YAML).
func ValidateFromBytes(schemaBytes []byte, data interface{}) (*Result, error) {
	if err := ensureSupportedDraft(schemaBytes); err != nil {
		return nil, err
	}
	validator, err := NewValidatorFromBytes(schemaBytes)
	if err != nil {
		return nil, err
	}
	return validator.Validate(data)
}

// ValidateFromBytesWithRefDirs validates data against schema bytes, preloading additional
// schemas from the provided directories to resolve remote $ref URLs offline.
//
// This is intended for early-stage ecosystems where schemas use absolute HTTP(S) $id/$ref
// URIs before a schema registry host is live.
func ValidateFromBytesWithRefDirs(schemaBytes []byte, data interface{}, refDirs []string) (*Result, error) {
	if len(refDirs) == 0 {
		return ValidateFromBytes(schemaBytes, data)
	}
	if err := ensureSupportedDraft(schemaBytes); err != nil {
		return nil, err
	}

	sch, err := compileSchemaBytesWithRefDirs(schemaBytes, refDirs)
	if err != nil {
		return nil, err
	}
	return validateWithCompiled(sch, data)
}

func compileSchemaBytesWithRefDirs(rootSchemaBytes []byte, refDirs []string) (*gojsonschema.Schema, error) {
	stripSchema := isOfflineMode() || len(refDirs) > 0

	schemaLoader := gojsonschema.NewSchemaLoader()
	// Ensure we only resolve via preloaded schemas (no implicit guessing)
	schemaLoader.AutoDetect = false

	rootID, normalizedRoot, err := extractAndNormalizeSchema(rootSchemaBytes, stripSchema)
	if err != nil {
		return nil, err
	}

	type registeredSchema struct {
		normalized []byte
		source     string
	}

	registered := make(map[string]registeredSchema)
	if rootID != "" {
		registered[rootID] = registeredSchema{normalized: normalizedRoot, source: "root schema"}
	}

	for _, dir := range refDirs {
		cleanDir, err := safeio.CleanUserPath(dir)
		if err != nil {
			return nil, fmt.Errorf("invalid ref-dir %s: %w", dir, err)
		}
		info, err := os.Stat(cleanDir)
		if err != nil {
			return nil, fmt.Errorf("ref-dir %s: %w", cleanDir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("ref-dir %s is not a directory", cleanDir)
		}

		err = filepath.WalkDir(cleanDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			switch ext {
			case ".json", ".yaml", ".yml":
				// ok
			default:
				return nil
			}

			fileBytes, err := os.ReadFile(path) // #nosec G304 -- path is discovered by walking a sanitized directory
			if err != nil {
				return fmt.Errorf("read ref schema %s: %w", path, err)
			}

			id, normalized, err := extractAndNormalizeSchema(fileBytes, stripSchema)
			if err != nil {
				// Ignore non-schema files in the directory tree (e.g., *.data.json) by treating parse errors as non-fatal.
				return nil
			}
			if id == "" {
				return nil
			}

			if existing, ok := registered[id]; ok {
				if bytes.Equal(existing.normalized, normalized) {
					return nil
				}
				return fmt.Errorf(
					"duplicate schema $id %q: %s differs from %s",
					id,
					path,
					existing.source,
				)
			}

			if err := schemaLoader.AddSchema(id, gojsonschema.NewBytesLoader(normalized)); err != nil {
				return fmt.Errorf("register schema %s (%s): %w", path, id, err)
			}
			registered[id] = registeredSchema{normalized: normalized, source: path}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	schema, err := schemaLoader.Compile(gojsonschema.NewBytesLoader(normalizedRoot))
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema with ref-dirs: %w", err)
	}
	return schema, nil
}

func extractAndNormalizeSchema(schemaBytes []byte, stripSchema bool) (string, []byte, error) {
	var tmp any
	if err := yaml.Unmarshal(schemaBytes, &tmp); err != nil {
		if err := json.Unmarshal(schemaBytes, &tmp); err != nil {
			return "", nil, fmt.Errorf("invalid schema format (must be valid YAML or JSON): %w", err)
		}
	}

	if m, ok := tmp.(map[string]any); ok {
		if stripSchema {
			delete(m, "$schema")
		}
		id, _ := m["$id"].(string)
		if id == "" {
			id, _ = m["id"].(string)
		}
		id = strings.TrimSpace(id)
		jb, err := json.Marshal(m)
		if err != nil {
			return "", nil, fmt.Errorf("failed to encode schema to JSON: %w", err)
		}
		return id, jb, nil
	}

	jb, err := json.Marshal(tmp)
	if err != nil {
		return "", nil, fmt.Errorf("failed to encode schema to JSON: %w", err)
	}
	return "", jb, nil
}

func ensureSupportedDraft(schemaBytes []byte) error {
	var schemaDoc map[string]interface{}
	if err := yaml.Unmarshal(schemaBytes, &schemaDoc); err != nil {
		if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
			return fmt.Errorf("invalid schema format (must be valid YAML or JSON): %w", err)
		}
	}
	if schemaDoc != nil {
		if v, ok := schemaDoc["$schema"].(string); ok {
			if !strings.Contains(v, "draft-07") && !strings.Contains(v, "2020-12") {
				return fmt.Errorf("unsupported $schema: only Draft-07 and Draft-2020-12 supported")
			}
		}
	}
	return nil
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

// ValidateFileWithSchemaPath validates a data file against a schema file path.
// This is an ergonomic helper that reads the schema file and auto-detects data format.
func ValidateFileWithSchemaPath(schemaPath string, dataPath string) (*Result, error) {
	// Read schema file
	schemaClean, err := safeio.CleanUserPath(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("schema path sanitization failed: %w", err)
	}

	schemaBytes, err := os.ReadFile(schemaClean) // #nosec G304 -- schemaClean sanitized with safeio.CleanUserPath
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", schemaClean, err)
	}

	// Read data file
	dataClean, err := safeio.CleanUserPath(dataPath)
	if err != nil {
		return nil, fmt.Errorf("data path sanitization failed: %w", err)
	}

	dataBytes, err := os.ReadFile(dataClean) // #nosec G304 -- dataClean sanitized with safeio.CleanUserPath
	if err != nil {
		return nil, fmt.Errorf("failed to read data file %s: %w", dataClean, err)
	}

	// Parse data (try YAML first, then JSON)
	var data interface{}
	if err := yaml.Unmarshal(dataBytes, &data); err != nil {
		if err := json.Unmarshal(dataBytes, &data); err != nil {
			return nil, fmt.Errorf("failed to parse data file %s: %w", dataClean, err)
		}
	}

	// Validate using existing logic
	return ValidateFromBytes(schemaBytes, data)
}

// ValidateFromFileWithBytes validates raw data bytes against a schema file path.
func ValidateFromFileWithBytes(schemaPath string, dataBytes []byte) (*Result, error) {
	// Read schema file
	schemaClean, err := safeio.CleanUserPath(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("schema path sanitization failed: %w", err)
	}

	schemaBytes, err := os.ReadFile(schemaClean) // #nosec G304 -- schemaClean sanitized with safeio.CleanUserPath
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", schemaClean, err)
	}

	// Parse data (try YAML first, then JSON)
	var data interface{}
	if err := yaml.Unmarshal(dataBytes, &data); err != nil {
		if err := json.Unmarshal(dataBytes, &data); err != nil {
			return nil, fmt.Errorf("failed to parse data bytes: %w", err)
		}
	}

	// Validate using existing logic
	return ValidateFromBytes(schemaBytes, data)
}

// ValidateWithOptions validates data against schema bytes with advanced options.
func ValidateWithOptions(schemaBytes []byte, data interface{}, opts ValidationOptions) (*Result, error) {
	// For now, this is a thin wrapper that applies options to ValidateFromBytes
	// Future enhancement could add SecurityContext and other advanced features
	res, err := ValidateFromBytes(schemaBytes, data)
	if err != nil {
		return nil, err
	}

	// Apply context to errors if provided
	if !res.Valid && opts.Context.SourceFile != "" {
		for i := range res.Errors {
			res.Errors[i].Context = opts.Context
		}
	}

	return res, nil
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
