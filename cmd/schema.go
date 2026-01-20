package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/fulmenhq/goneat/pkg/safeio"
	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/fulmenhq/goneat/pkg/schema/signature"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	schemaValidateSchemaID        string
	schemaValidateFormat          string
	schemaValidateWorkers         int
	schemaValidateSchemaRecursive bool
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Schema validation utilities",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

var schemaValidateSchemaCmd = &cobra.Command{
	Use:   "validate-schema [files...]",
	Short: "Validate schema files against embedded meta-schemas",
	Long:  "Validate schema files (e.g., JSON Schema drafts) against embedded meta-schemas to ensure structural correctness.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSchemaValidateSchema,
}

func init() {
	rootCmd.AddCommand(schemaCmd)
	schemaCmd.AddCommand(schemaValidateSchemaCmd)

	schemaValidateSchemaCmd.Flags().StringVar(&schemaValidateSchemaID, "schema-id", "", "Schema signature id to validate against (e.g., json-schema-draft-07)")
	schemaValidateSchemaCmd.Flags().StringVar(&schemaValidateFormat, "format", "text", "Output format: text|json")
	schemaValidateSchemaCmd.Flags().IntVar(&schemaValidateWorkers, "workers", 0, "Number of parallel workers (0=auto)")
	schemaValidateSchemaCmd.Flags().BoolVar(&schemaValidateSchemaRecursive, "recursive", false, "If a directory is provided, recursively validate schema files within")
}

type schemaValidateResult struct {
	File     string   `json:"file"`
	SchemaID string   `json:"schema_id"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
}

func expandSchemaValidateInputs(args []string, recursive bool) ([]string, error) {
	var expanded []string
	for _, arg := range args {
		if containsGlob(arg) {
			matches, err := filepath.Glob(arg)
			if err != nil {
				return nil, fmt.Errorf("invalid glob %s: %w", arg, err)
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("glob pattern matched no files: %s", arg)
			}
			expanded = append(expanded, matches...)
			continue
		}

		info, err := os.Stat(arg)
		if err == nil && info.IsDir() {
			if !recursive {
				return nil, fmt.Errorf("%s is a directory (use --recursive)", arg)
			}
			files, err := collectSchemaFiles(arg)
			if err != nil {
				return nil, err
			}
			if len(files) == 0 {
				return nil, fmt.Errorf("no schema files found under %s", arg)
			}
			expanded = append(expanded, files...)
			continue
		}

		expanded = append(expanded, arg)
	}

	// Ensure deterministic order
	sort.Strings(expanded)
	return expanded, nil
}

func containsGlob(value string) bool {
	return strings.ContainsAny(value, "*?[")
}

func collectSchemaFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".json", ".yaml", ".yml":
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func runSchemaValidateSchema(cmd *cobra.Command, args []string) error {
	format := strings.ToLower(schemaValidateFormat)
	if format != "text" && format != "json" {
		return fmt.Errorf("unsupported format: %s", schemaValidateFormat)
	}

	manifest, err := signature.LoadDefaultManifest()
	if err != nil {
		return fmt.Errorf("load schema manifest: %w", err)
	}

	var providedID string
	if schemaValidateSchemaID != "" {
		sig, ok := findSignature(manifest, schemaValidateSchemaID)
		if !ok {
			return fmt.Errorf("schema-id %s not found in signature manifest", schemaValidateSchemaID)
		}
		providedID = sig.ID
	}

	var detector *signature.Detector
	if providedID == "" {
		var derr error
		detector, derr = signature.NewDetector(manifest)
		if derr != nil {
			return fmt.Errorf("prepare schema detector: %w", derr)
		}
	}

	inputs, err := expandSchemaValidateInputs(args, schemaValidateSchemaRecursive)
	if err != nil {
		return err
	}

	results := make([]schemaValidateResult, len(inputs))
	var failures int
	var mu sync.Mutex

	workers := schemaValidateWorkers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}
	if len(args) < 2 {
		workers = 1
	}

	cache := newMetaSchemaDraftCache()
	var detectMu sync.Mutex

	g, gctx := errgroup.WithContext(cmd.Context())
	g.SetLimit(workers)

	for idx, input := range inputs {
		idx := idx
		input := input
		g.Go(func() error {
			select {
			case <-gctx.Done():
				return nil
			default:
			}

			cleanPath, err := safeio.CleanUserPath(input)
			if err != nil {
				mu.Lock()
				failures++
				results[idx] = schemaValidateResult{File: input, Valid: false, Errors: []string{fmt.Sprintf("invalid path: %v", err)}}
				mu.Unlock()
				return nil
			}

			schemaBytes, err := os.ReadFile(cleanPath) // #nosec G304 -- cleanPath sanitized with safeio.CleanUserPath
			if err != nil {
				mu.Lock()
				failures++
				results[idx] = schemaValidateResult{File: cleanPath, Valid: false, Errors: []string{fmt.Sprintf("failed to read schema: %v", err)}}
				mu.Unlock()
				return nil
			}

			schemaID := providedID
			if schemaID == "" {
				detectMu.Lock()
				match, ok := detector.Detect(cleanPath, schemaBytes, signature.DetectOptions{})
				detectMu.Unlock()
				if !ok {
					mu.Lock()
					failures++
					results[idx] = schemaValidateResult{File: cleanPath, Valid: false, Errors: []string{"unable to detect schema signature"}}
					mu.Unlock()
					return nil
				}
				schemaID = match.Signature.ID
			}

			valid, validationErrs, verr := validateSchemaByIDCached(schemaID, schemaBytes, cache)
			if verr != nil {
				mu.Lock()
				failures++
				results[idx] = schemaValidateResult{File: cleanPath, SchemaID: schemaID, Valid: false, Errors: []string{verr.Error()}}
				mu.Unlock()
				return nil
			}
			if !valid {
				mu.Lock()
				failures++
				results[idx] = schemaValidateResult{File: cleanPath, SchemaID: schemaID, Valid: false, Errors: validationErrs}
				mu.Unlock()
				return nil
			}

			mu.Lock()
			results[idx] = schemaValidateResult{File: cleanPath, SchemaID: schemaID, Valid: true}
			mu.Unlock()
			return nil
		})
	}

	_ = g.Wait()

	if format == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			return fmt.Errorf("encode JSON output: %w", err)
		}
	} else {
		for _, res := range results {
			if res.Valid {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ %s", res.File)
				if res.SchemaID != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), " (%s)", res.SchemaID)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
				continue
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "❌ %s", res.File)
			if res.SchemaID != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), " (%s)", res.SchemaID)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			for _, msg := range res.Errors {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", msg)
			}
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d schema file(s) failed validation", failures)
	}
	return nil
}

func findSignature(manifest *signature.Manifest, id string) (signature.Signature, bool) {
	lower := strings.ToLower(id)
	for _, sig := range manifest.Signatures {
		if strings.ToLower(sig.ID) == lower {
			return sig, true
		}
		for _, alias := range sig.Aliases {
			if strings.ToLower(alias) == lower {
				return sig, true
			}
		}
	}
	return signature.Signature{}, false
}

type metaSchemaDraftCache struct {
	mu      sync.Mutex
	byDraft map[string]*schema.Validator
}

func newMetaSchemaDraftCache() *metaSchemaDraftCache {
	return &metaSchemaDraftCache{byDraft: make(map[string]*schema.Validator)}
}

func (c *metaSchemaDraftCache) Get(draft string) (*schema.Validator, error) {
	draft = strings.TrimSpace(draft)
	c.mu.Lock()
	v, ok := c.byDraft[draft]
	c.mu.Unlock()
	if ok {
		return v, nil
	}

	validator, err := schema.NewValidatorFromMetaSchema(draft)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.byDraft[draft] = validator
	c.mu.Unlock()
	return validator, nil
}

func validateSchemaByIDCached(schemaID string, schemaBytes []byte, cache *metaSchemaDraftCache) (bool, []string, error) {
	switch schemaID {
	case "json-schema-draft-04":
		return validateJSONSchemaDraftCached("draft-04", schemaBytes, cache)
	case "json-schema-draft-06":
		return validateJSONSchemaDraftCached("draft-06", schemaBytes, cache)
	case "json-schema-draft-07":
		return validateJSONSchemaDraftCached("draft-07", schemaBytes, cache)
	case "json-schema-2019-09":
		return validateJSONSchemaDraftCached("2019-09", schemaBytes, cache)
	case "json-schema-2020-12":
		return validateJSONSchemaDraftCached("2020-12", schemaBytes, cache)
	default:
		return false, nil, fmt.Errorf("schema-id %s not supported for validation", schemaID)
	}
}

func validateJSONSchemaDraftCached(draft string, schemaBytes []byte, cache *metaSchemaDraftCache) (bool, []string, error) {
	validator, err := cache.Get(draft)
	if err != nil {
		return false, nil, err
	}

	res, err := validator.ValidateBytes(schemaBytes)
	if err != nil {
		return false, nil, err
	}
	if res.Valid {
		return true, nil, nil
	}

	errs := make([]string, 0, len(res.Errors))
	for _, e := range res.Errors {
		path := strings.TrimSpace(e.Path)
		if path == "" || path == "root" {
			errs = append(errs, e.Message)
		} else {
			errs = append(errs, fmt.Sprintf("%s: %s", path, e.Message))
		}
	}
	return false, errs, nil
}
