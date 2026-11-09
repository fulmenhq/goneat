package schema

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/internal/schemaengine"
	"github.com/fulmenhq/goneat/internal/schemarun"
	"github.com/fulmenhq/goneat/pkg/safeio"
	"golang.org/x/sync/errgroup"
)

// AdmissionError reports a schema that was refused before validation: an
// unsupported dialect, an unknown format, a reference outside the admitted
// resources, a budget overrun or an invalid schema. It is never a
// validation result.
type AdmissionError = schemaengine.AdmissionError

// Result holds the validation result and the checking policy it was produced
// under.
type Result struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`

	Engine        string `json:"engine,omitempty"`
	Dialect       string `json:"dialect,omitempty"`
	DialectSource string `json:"dialect_source,omitempty"` // declared, defaulted or metaschema
	FormatMode    string `json:"format_mode,omitempty"`
	LeapPolicy    string `json:"leap_policy,omitempty"`
}

// ValidationContext provides additional context for validation errors.
type ValidationContext struct {
	SourceFile string `json:"source_file,omitempty"`
	SourceType string `json:"source_type,omitempty"` // "file", "bytes", "string"
	LineNumber int    `json:"line_number,omitempty"`
	Severity   string `json:"severity,omitempty"` // "error", "warning"
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

// registry caches compiled schemas (and compile failures) by name.
var (
	schemaRegistry map[string]registryEntry
	schemaPaths    map[string]string // name -> embed path
	regMu          sync.RWMutex
)

type registryEntry struct {
	compiled *schemaengine.Compiled
	err      error
}

// Validator wraps a compiled schema for repeated validation.
type Validator struct {
	schema *schemaengine.Compiled
}

func init() {
	initRegistry()
}

// initRegistry records embedded schema paths only. Compilation is deferred to
// GetEmbeddedValidator, which compiles and caches each schema on first use, so
// process startup does not pay to compile schemas a command never touches.
func initRegistry() {
	schemaRegistry = make(map[string]registryEntry)
	schemaPaths = make(map[string]string)
	for _, info := range assets.GetSchemaNames() {
		schemaPaths[info.Name] = info.Path
	}
}

func compileSchemaBytes(schemaBytes []byte, resources []schemaengine.Resource) (*schemaengine.Compiled, error) {
	c, err := schemarun.Compile(schemaBytes, resources)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}
	return c, nil
}

// NewValidatorFromBytes compiles schema bytes (JSON or YAML) into a reusable validator.
func NewValidatorFromBytes(schemaBytes []byte) (*Validator, error) {
	sch, err := compileSchemaBytes(schemaBytes, nil)
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
// A schema that fails to compile keeps failing with the same error.
func GetEmbeddedValidator(schemaName string) (*Validator, error) {
	regMu.RLock()
	if e, ok := schemaRegistry[schemaName]; ok {
		regMu.RUnlock()
		if e.err != nil {
			return nil, e.err
		}
		return &Validator{schema: e.compiled}, nil
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

	// Bundled copies of the JSON Schema metaschemas compile from the
	// engine's embedded metaschema; their json-schema.org identities are
	// reserved for it.
	var sch *schemaengine.Compiled
	var err error
	if draft, ok := embeddedMetaschemaDraft(data); ok {
		sch, err = schemarun.CompileMetaschema(draft)
	} else {
		sch, err = compileSchemaBytes(data, nil)
	}
	if err != nil {
		err = fmt.Errorf("embedded schema %s: %w", schemaName, err)
	}

	regMu.Lock()
	schemaRegistry[schemaName] = registryEntry{compiled: sch, err: err}
	if _, exists := schemaPaths[schemaName]; !exists {
		schemaPaths[schemaName] = path
	}
	regMu.Unlock()

	if err != nil {
		return nil, err
	}
	return &Validator{schema: sch}, nil
}

func embeddedMetaschemaDraft(data []byte) (string, bool) {
	jb, err := schemarun.SchemaJSON(data)
	if err != nil {
		return "", false
	}
	return schemarun.MetaschemaDraft(schemarun.DeclaredID(jb))
}

// NewValidatorFromMetaSchema returns a validator for a JSON Schema draft's
// meta-schema, compiled from the engine's embedded copy (no network).
func NewValidatorFromMetaSchema(draft string) (*Validator, error) {
	c, err := schemarun.CompileMetaschema(draft)
	if err != nil {
		return nil, err
	}
	return &Validator{schema: c}, nil
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
	data, _, err := schemarun.DecodeInstance(dataBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data bytes (JSON/YAML): %w", err)
	}
	return validateWithCompiled(v.schema, data)
}

func validateWithCompiled(sch *schemaengine.Compiled, data interface{}) (*Result, error) {
	valid, issues, err := schemarun.Validate(sch, data)
	if err != nil {
		return nil, err
	}
	cap := sch.Capability()
	res := &Result{
		Valid:         valid,
		Engine:        cap.Engine,
		Dialect:       cap.Dialect,
		DialectSource: cap.DialectSource,
		FormatMode:    cap.FormatMode,
		LeapPolicy:    cap.LeapPolicy,
	}
	for _, is := range issues {
		res.Errors = append(res.Errors, ValidationError{
			Path:    is.Path,
			Message: schemarun.ImproveMessage(is, ""),
		})
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
	validator, err := NewValidatorFromBytes(schemaBytes)
	if err != nil {
		return nil, err
	}
	return validator.Validate(data)
}

// ValidateFromBytesWithRefDirs validates data against schema bytes. Schema
// files under refDirs that declare an $id are admitted as resources, so
// absolute $ref URLs resolve offline; nothing else is reachable.
func ValidateFromBytesWithRefDirs(schemaBytes []byte, data interface{}, refDirs []string) (*Result, error) {
	if len(refDirs) == 0 {
		return ValidateFromBytes(schemaBytes, data)
	}
	idx, err := BuildIDIndexFromRefDirs(refDirs)
	if err != nil {
		return nil, err
	}
	return validateWithIndex(schemaBytes, data, idx, "ref-dirs")
}

// ValidateFromBytesWithIDIndex validates data against schema bytes, admitting
// the schemas in an offline $id index as resources.
func ValidateFromBytesWithIDIndex(schemaBytes []byte, data interface{}, idx *IDIndex) (*Result, error) {
	if idx == nil || idx.Len() == 0 {
		return ValidateFromBytes(schemaBytes, data)
	}
	return validateWithIndex(schemaBytes, data, idx, "id-index")
}

func validateWithIndex(schemaBytes []byte, data interface{}, idx *IDIndex, via string) (*Result, error) {
	rootID, root, err := extractAndNormalizeSchema(schemaBytes)
	if err != nil {
		return nil, err
	}
	resources, err := idx.resources(rootID, root)
	if err != nil {
		return nil, err
	}
	sch, err := schemarun.Compile(root, resources)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema with %s: %w", via, err)
	}
	return validateWithCompiled(sch, data)
}

// resources returns the index entries as admitted resources, leaving out the
// root's own $id. An entry that shares the root's $id must be the same
// document.
func (i *IDIndex) resources(rootID string, root []byte) ([]schemaengine.Resource, error) {
	ids := make([]string, 0, len(i.entries))
	for id := range i.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]schemaengine.Resource, 0, len(ids))
	for _, id := range ids {
		e := i.entries[id]
		if id == rootID {
			if !schemarun.SameDocument(e.Normalized, root) {
				return nil, fmt.Errorf("duplicate schema $id %q: %s differs from root schema", id, e.Path)
			}
			continue
		}
		out = append(out, schemaengine.Resource{URL: id, Bytes: e.Normalized})
	}
	return out, nil
}

// extractAndNormalizeSchema returns a schema's declared $id (or draft-04 id)
// and its JSON text. $schema is always preserved.
func extractAndNormalizeSchema(schemaBytes []byte) (string, []byte, error) {
	jb, err := schemarun.SchemaJSON(schemaBytes)
	if err != nil {
		return "", nil, err
	}
	return schemarun.DeclaredID(jb), jb, nil
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

	// JSON text is decoded strictly and exactly; anything else as YAML.
	data, format, err := schemarun.DecodeInstance(dataBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data bytes as %s: %w", strings.ToUpper(format), err)
	}
	options.Context.SourceType = format

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

	data, _, err := schemarun.DecodeInstance(dataBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data file %s: %w", dataClean, err)
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

	data, _, err := schemarun.DecodeInstance(dataBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse data bytes: %w", err)
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
		"goneat-config-v1.0.0":       "embedded_schemas/config/goneat-config-v1.0.0.yaml",
		"dates":                      "embedded_schemas/schemas/config/dates.yaml",
		"assess-config-v1.0.0":       "embedded_schemas/schemas/config/v1.0.0/assess-config.yaml",
		"dependencies-policy-v1.0.0": "embedded_schemas/schemas/config/v1.0.0/dependencies-policy.yaml",
		"hooks-manifest-v1.0.0":      "embedded_schemas/schemas/work/v1.0.0/hooks-manifest.yaml",

		"work-manifest-v1.0.0": "embedded_schemas/work/work-manifest-v1.0.0.yaml",
	}

	if path, ok := knownSchemas[schemaName]; ok {
		return path
	}
	// Fallback: assume .yaml extension in config directory
	return "embedded_schemas/config/" + schemaName + ".yaml"
}
