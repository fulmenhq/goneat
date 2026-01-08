package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/fulmenhq/goneat/pkg/safeio"
	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/fulmenhq/goneat/pkg/schema/signature"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	schemaValidateSchemaID string
	schemaValidateFormat   string
	schemaValidateWorkers  int
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
}

type schemaValidateResult struct {
	File     string   `json:"file"`
	SchemaID string   `json:"schema_id"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
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

	results := make([]schemaValidateResult, len(args))
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

	for idx, input := range args {
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
	case "json-schema-draft-07":
		return validateJSONSchemaDraftCached("draft-07", schemaBytes, cache)
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
