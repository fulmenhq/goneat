package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fulmenhq/goneat/internal/assess"
	"github.com/fulmenhq/goneat/pkg/buildinfo"
	"github.com/fulmenhq/goneat/pkg/safeio"
	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/fulmenhq/goneat/pkg/schema/mapping"
	"github.com/fulmenhq/goneat/pkg/work"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

var (
	validateSuiteDataRoot         string
	validateSuiteSchemasRoot      string
	validateSuiteManifestPath     string
	validateSuiteRefDirs          []string
	validateSuiteNoIgnore         bool
	validateSuiteForceInclude     []string
	validateSuiteExclude          []string
	validateSuiteSkip             []string
	validateSuiteExpectFail       []string
	validateSuiteStrict           bool
	validateSuiteEnableMeta       bool
	validateSuiteMaxWorkers       int
	validateSuiteFormat           string
	validateSuiteTimeout          time.Duration
	validateSuiteFailOnUnmapped   bool
	validateSuiteSchemaResolution string
)

var validateSuiteCmd = &cobra.Command{
	Use:          "suite --data DIR",
	Short:        "Validate a suite of data files in bulk",
	Long:         "Validate many JSON/YAML files against schemas in bulk, using schema-mappings and optional offline $ref resolution.",
	SilenceUsage: true,
	RunE:         runValidateSuite,
}

type validateSuiteMetadata struct {
	Tool             string   `json:"tool"`
	Version          string   `json:"version"`
	GeneratedAt      string   `json:"generated_at"`
	Duration         string   `json:"duration"`
	RepoRoot         string   `json:"repo_root"`
	DataRoot         string   `json:"data_root"`
	SchemasRoot      string   `json:"schemas_root,omitempty"`
	ManifestPath     string   `json:"manifest_path"`
	RefDirs          []string `json:"ref_dirs,omitempty"`
	SchemaResolution string   `json:"schema_resolution"`
	MaxWorkers       int      `json:"max_workers"`
	NoIgnore         bool     `json:"no_ignore"`
	ForceInclude     []string `json:"force_include,omitempty"`
	Exclude          []string `json:"exclude,omitempty"`
	Skip             []string `json:"skip,omitempty"`
	ExpectFail       []string `json:"expect_fail,omitempty"`
	EnableMeta       bool     `json:"enable_meta"`
	Strict           bool     `json:"strict"`
	FailOnUnmapped   bool     `json:"fail_on_unmapped"`
}

type validateSuiteSummary struct {
	Total          int `json:"total"`
	Validated      int `json:"validated"`
	Passed         int `json:"passed"`
	Failed         int `json:"failed"`
	ExpectedFail   int `json:"expected_fail"`
	UnexpectedPass int `json:"unexpected_pass"`
	Skipped        int `json:"skipped"`
	Unmapped       int `json:"unmapped"`
}

type validateSuiteSchemaRef struct {
	ID     string `json:"id,omitempty"`
	Source string `json:"source,omitempty"`
	Path   string `json:"path,omitempty"`
}

type validateSuiteFileResult struct {
	Path     string                   `json:"path"`
	Schema   *validateSuiteSchemaRef  `json:"schema,omitempty"`
	Status   string                   `json:"status"`
	Valid    bool                     `json:"valid"`
	Errors   []schema.ValidationError `json:"errors,omitempty"`
	Error    string                   `json:"error,omitempty"`
	Duration string                   `json:"duration,omitempty"`
}

type validateSuiteResult struct {
	Metadata       validateSuiteMetadata     `json:"metadata"`
	MetaValidation *assess.AssessmentResult  `json:"meta_validation,omitempty"`
	Mapping        *mapping.LoadResult       `json:"mapping,omitempty"`
	Summary        validateSuiteSummary      `json:"summary"`
	Files          []validateSuiteFileResult `json:"files"`
}

func init() {
	validateCmd.AddCommand(validateSuiteCmd)

	validateSuiteCmd.Flags().StringVar(&validateSuiteDataRoot, "data", "", "Root directory of data/examples to validate (required)")
	_ = validateSuiteCmd.MarkFlagRequired("data")

	validateSuiteCmd.Flags().StringVar(&validateSuiteSchemasRoot, "schemas", "", "Optional schemas directory to meta-validate before data validation")
	validateSuiteCmd.Flags().BoolVar(&validateSuiteEnableMeta, "enable-meta", false, "Meta-validate schemas in --schemas using embedded drafts")

	validateSuiteCmd.Flags().StringVar(&validateSuiteManifestPath, "manifest", mapping.DefaultManifestRelativePath, "Schema mapping manifest path (defaults to .goneat/schema-mappings.yaml)")
	validateSuiteCmd.Flags().StringSliceVar(&validateSuiteRefDirs, "ref-dir", []string{}, "Directory tree of schema files used to resolve absolute $ref URLs offline (repeatable)")

	validateSuiteCmd.Flags().BoolVar(&validateSuiteNoIgnore, "no-ignore", false, "Disable .goneatignore/.gitignore for discovery")
	validateSuiteCmd.Flags().StringSliceVar(&validateSuiteForceInclude, "force-include", []string{}, "Force-include paths or globs even if ignored (repeatable)")
	validateSuiteCmd.Flags().StringSliceVar(&validateSuiteExclude, "exclude", []string{}, "Exclude paths or globs (repeatable)")
	validateSuiteCmd.Flags().StringSliceVar(&validateSuiteSkip, "skip", []string{}, "Skip matching files (repeatable glob)")
	validateSuiteCmd.Flags().StringSliceVar(&validateSuiteExpectFail, "expect-fail", []string{}, "Treat matching files as expected failures (repeatable glob)")

	validateSuiteCmd.Flags().BoolVar(&validateSuiteStrict, "strict", false, "Fail if any files are unmapped or excluded")
	validateSuiteCmd.Flags().BoolVar(&validateSuiteFailOnUnmapped, "fail-on-unmapped", true, "Fail the suite if any files have no schema mapping")
	validateSuiteCmd.Flags().StringVar(&validateSuiteSchemaResolution, "schema-resolution", "prefer-id", "Schema resolution strategy (prefer-id, id-strict, path-only)")
	validateSuiteCmd.Flags().IntVar(&validateSuiteMaxWorkers, "workers", runtime.NumCPU(), "Max parallel workers")
	validateSuiteCmd.Flags().DurationVar(&validateSuiteTimeout, "timeout", 3*time.Minute, "Validation timeout")
	validateSuiteCmd.Flags().StringVar(&validateSuiteFormat, "format", "markdown", "Output format (markdown, json)")
}

func runValidateSuite(cmd *cobra.Command, _ []string) error {
	start := time.Now()

	repoRoot, err := inferSuiteRepoRoot(validateSuiteDataRoot)
	if err != nil {
		return err
	}

	ctx, cancel := contextWithTimeout(cmd.Context(), validateSuiteTimeout)
	defer cancel()

	if err := validateSchemaResolution(validateSuiteSchemaResolution); err != nil {
		return err
	}

	resolver, loadResult, err := loadSuiteMapping(repoRoot, validateSuiteManifestPath)
	if err != nil {
		return err
	}

	files, err := discoverSuiteFiles(repoRoot, validateSuiteDataRoot)
	if err != nil {
		return err
	}

	var metaRes *assess.AssessmentResult
	if validateSuiteEnableMeta {
		if validateSuiteSchemasRoot == "" {
			return fmt.Errorf("--enable-meta requires --schemas")
		}
		metaRes = runSuiteMetaValidation(ctx, repoRoot, validateSuiteSchemasRoot)
		if metaRes != nil && metaRes.Error != "" {
			return fmt.Errorf("meta-validation failed: %s", metaRes.Error)
		}
	}

	var idIndex *schema.IDIndex
	if validateSuiteSchemaResolution != string(schemaResolutionPathOnly) && len(validateSuiteRefDirs) > 0 {
		idx, err := schema.BuildIDIndexFromRefDirs(validateSuiteRefDirs)
		if err != nil {
			return err
		}
		idIndex = idx
	}

	results := make([]validateSuiteFileResult, 0, len(files))
	resultsMu := &sync.Mutex{}

	summary := validateSuiteSummary{Total: len(files)}
	summaryMu := &sync.Mutex{}

	g, gctx := errgroup.WithContext(ctx)
	if validateSuiteMaxWorkers > 0 {
		g.SetLimit(validateSuiteMaxWorkers)
	}

	for _, file := range files {
		file := file
		g.Go(func() error {
			res := validateSuiteOne(gctx, repoRoot, file, loadResult, resolver, idIndex, validateSuiteSchemaResolution)

			resultsMu.Lock()
			results = append(results, res)
			resultsMu.Unlock()

			summaryMu.Lock()
			switch res.Status {
			case "pass":
				summary.Validated++
				summary.Passed++
			case "fail":
				summary.Validated++
				summary.Failed++
			case "expected_fail":
				summary.Validated++
				summary.ExpectedFail++
			case "unexpected_pass":
				summary.Validated++
				summary.UnexpectedPass++
			case "skipped":
				summary.Skipped++
			case "unmapped":
				summary.Unmapped++
			default:
				// no-op
			}
			summaryMu.Unlock()

			return nil
		})
	}

	_ = g.Wait()

	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })

	suiteRes := validateSuiteResult{
		Metadata: validateSuiteMetadata{
			Tool:             "goneat",
			Version:          buildinfo.BinaryVersion,
			GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
			Duration:         time.Since(start).String(),
			RepoRoot:         repoRoot,
			DataRoot:         validateSuiteDataRoot,
			SchemasRoot:      validateSuiteSchemasRoot,
			ManifestPath:     validateSuiteManifestPath,
			RefDirs:          append([]string(nil), validateSuiteRefDirs...),
			SchemaResolution: validateSuiteSchemaResolution,
			MaxWorkers:       validateSuiteMaxWorkers,
			NoIgnore:         validateSuiteNoIgnore,
			ForceInclude:     append([]string(nil), validateSuiteForceInclude...),
			Exclude:          append([]string(nil), validateSuiteExclude...),
			Skip:             append([]string(nil), validateSuiteSkip...),
			ExpectFail:       append([]string(nil), validateSuiteExpectFail...),
			EnableMeta:       validateSuiteEnableMeta,
			Strict:           validateSuiteStrict,
			FailOnUnmapped:   validateSuiteFailOnUnmapped,
		},
		MetaValidation: metaRes,
		Mapping:        loadResult,
		Summary:        summary,
		Files:          results,
	}

	shouldFail := summary.Failed > 0 || summary.UnexpectedPass > 0
	if validateSuiteFailOnUnmapped && summary.Unmapped > 0 {
		shouldFail = true
	}
	if validateSuiteStrict && (summary.Skipped > 0 || summary.Unmapped > 0) {
		shouldFail = true
	}

	switch strings.ToLower(validateSuiteFormat) {
	case "json":
		b, _ := json.MarshalIndent(suiteRes, "", "  ")
		cmd.Printf("%s\n", b)
	case "markdown":
		printSuiteMarkdown(cmd, suiteRes)
	default:
		return fmt.Errorf("invalid format: %s", validateSuiteFormat)
	}

	if shouldFail {
		return fmt.Errorf("validate suite failed")
	}
	return nil
}

func contextWithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

func inferSuiteRepoRoot(dataRoot string) (string, error) {
	if strings.TrimSpace(dataRoot) == "" {
		return "", fmt.Errorf("--data is required")
	}
	clean, err := safeio.CleanUserPath(dataRoot)
	if err != nil {
		return "", fmt.Errorf("invalid --data %q: %w", dataRoot, err)
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve --data: %w", err)
	}
	// Prefer a .git root if present; otherwise treat data root as repo root.
	dir := abs
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs, nil
		}
		dir = parent
	}
}

func loadSuiteMapping(repoRoot, manifestPath string) (*mapping.Resolver, *mapping.LoadResult, error) {
	mgr, err := mapping.NewManager()
	if err != nil {
		return nil, nil, fmt.Errorf("init schema mapping manager: %w", err)
	}
	res, err := mgr.Load(mapping.LoadOptions{RepoRoot: repoRoot, ManifestPath: manifestPath})
	if err != nil {
		return nil, nil, fmt.Errorf("load schema mapping manifest: %w", err)
	}
	resolver := mapping.NewResolver(res.Effective)
	return resolver, res, nil
}

func discoverSuiteFiles(repoRoot, dataRoot string) ([]string, error) {
	clean, err := safeio.CleanUserPath(dataRoot)
	if err != nil {
		return nil, fmt.Errorf("invalid data root %q: %w", dataRoot, err)
	}
	root := clean
	if !filepath.IsAbs(root) {
		root = filepath.Join(repoRoot, root)
	}
	root = filepath.Clean(root)

	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("data root %s: %w", root, err)
	}

	// Run discovery relative to repo root so ignore matching behaves.
	cwd, _ := os.Getwd()
	if repoRoot != "" {
		_ = os.Chdir(repoRoot)
		defer func() { _ = os.Chdir(cwd) }()
	}

	paths := []string{root}
	if repoRoot != "" {
		if rel, err := filepath.Rel(repoRoot, root); err == nil {
			paths = []string{rel}
		}
	}

	planner := work.NewPlanner(work.PlannerConfig{
		Command:              "validate-suite",
		Paths:                paths,
		ExecutionStrategy:    "parallel",
		IgnoreFile:           ".goneatignore",
		NoIgnore:             validateSuiteNoIgnore,
		ForceIncludePatterns: append([]string(nil), validateSuiteForceInclude...),
	})
	manifest, err := planner.GenerateManifest()
	if err != nil {
		return nil, err
	}

	var out []string
	for _, item := range manifest.WorkItems {
		p := item.Path
		low := strings.ToLower(p)
		if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") || strings.HasSuffix(low, ".json") {
			// Apply exclude patterns
			if matchAny(validateSuiteExclude, filepath.ToSlash(p)) {
				continue
			}
			out = append(out, p)
		}
	}
	return out, nil
}

func runSuiteMetaValidation(ctx context.Context, repoRoot, schemasRoot string) *assess.AssessmentResult {
	// Reuse the schema assessment runner for offline meta-validation.
	runner := assess.NewSchemaAssessmentRunner()
	cfg := assess.AssessmentConfig{
		Mode:               assess.AssessmentModeCheck,
		Timeout:            2 * time.Minute,
		IncludeFiles:       []string{schemasRoot},
		NoIgnore:           validateSuiteNoIgnore,
		ForceInclude:       append([]string(nil), validateSuiteForceInclude...),
		SchemaEnableMeta:   true,
		Concurrency:        validateSuiteMaxWorkers,
		SelectedCategories: []string{string(assess.CategorySchema)},
	}
	res, err := runner.Assess(ctx, repoRoot, cfg)
	if err != nil {
		return &assess.AssessmentResult{CommandName: "schema", Category: assess.CategorySchema, Success: false, Error: err.Error()}
	}
	return res
}

func validateSuiteOne(ctx context.Context, repoRoot, file string, loadResult *mapping.LoadResult, resolver *mapping.Resolver, idIndex *schema.IDIndex, schemaResolution string) validateSuiteFileResult {
	start := time.Now()

	normPath := filepath.Clean(file)
	fullPath := normPath
	if repoRoot != "" && !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(repoRoot, fullPath)
	}
	fullPath = filepath.Clean(fullPath)

	rel := filepath.ToSlash(strings.TrimPrefix(normPath, "./"))
	if repoRoot != "" {
		if r, err := filepath.Rel(repoRoot, fullPath); err == nil {
			rel = filepath.ToSlash(strings.TrimPrefix(r, "./"))
		}
	}

	if matchAny(validateSuiteSkip, rel) {
		return validateSuiteFileResult{Path: rel, Status: "skipped", Valid: true, Duration: time.Since(start).String()}
	}

	resolution, ok := resolver.Resolve(rel)
	if ok && resolution.Excluded {
		return validateSuiteFileResult{Path: rel, Status: "skipped", Valid: true, Duration: time.Since(start).String()}
	}
	if !ok || strings.TrimSpace(resolution.SchemaID) == "" {
		return validateSuiteFileResult{Path: rel, Status: "unmapped", Valid: false, Error: "no schema mapping", Duration: time.Since(start).String()}
	}

	isExpectedFail := matchAny(validateSuiteExpectFail, rel)

	source := resolution.Source
	if source == "" {
		source = mapping.SourceEmbedded
	}

	schemaPath := ""
	if source == mapping.SourceLocal {
		schemaPath = resolveSuiteOverridePath(loadResult, resolution.SchemaID)
		if schemaPath == "" {
			schemaPath = resolution.SchemaID
		}
		if !looksLikeSchemaPath(schemaPath) {
			return validateSuiteFileResult{Path: rel, Schema: &validateSuiteSchemaRef{ID: resolution.SchemaID, Source: string(source)}, Status: "fail", Valid: false, Error: fmt.Sprintf("local schema mapping for %q requires overrides.path or schema_path", resolution.SchemaID), Duration: time.Since(start).String()}
		}
		clean, err := safeio.CleanUserPath(schemaPath)
		if err != nil {
			return validateSuiteFileResult{Path: rel, Schema: &validateSuiteSchemaRef{ID: resolution.SchemaID, Source: string(source)}, Status: "fail", Valid: false, Error: err.Error(), Duration: time.Since(start).String()}
		}
		schemaPath = clean
		if repoRoot != "" && !filepath.IsAbs(schemaPath) {
			schemaPath = filepath.Join(repoRoot, schemaPath)
		}
		schemaPath = filepath.Clean(schemaPath)
	}

	schemaRef := &validateSuiteSchemaRef{ID: resolution.SchemaID, Source: string(source), Path: schemaPath}

	dataBytes, err := os.ReadFile(fullPath) // #nosec G304 -- path comes from work planner output
	if err != nil {
		return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: err.Error(), Duration: time.Since(start).String()}
	}

	var result *schema.Result

	switch source {
	case mapping.SourceEmbedded:
		validator, err := schema.GetEmbeddedValidator(resolution.SchemaID)
		if err != nil {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: err.Error(), Duration: time.Since(start).String()}
		}
		result, err = validator.ValidateBytes(dataBytes)
		if err != nil {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: err.Error(), Duration: time.Since(start).String()}
		}
	case mapping.SourceLocal:
		if schemaPath == "" {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: fmt.Sprintf("local schema path not resolved for %q", resolution.SchemaID), Duration: time.Since(start).String()}
		}
		schemaBytes, err := os.ReadFile(filepath.Clean(schemaPath)) // #nosec G304 -- schemaPath sanitized
		if err != nil {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: err.Error(), Duration: time.Since(start).String()}
		}

		doc, err := parseDataBytes(fullPath, dataBytes)
		if err != nil {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: err.Error(), Duration: time.Since(start).String()}
		}
		if idIndex != nil {
			result, err = schema.ValidateFromBytesWithIDIndex(schemaBytes, doc, idIndex)
		} else {
			result, err = schema.ValidateFromBytesWithRefDirs(schemaBytes, doc, validateSuiteRefDirs)
		}
		if err != nil {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: err.Error(), Duration: time.Since(start).String()}
		}
	case mapping.SourceExternal:
		schemaID := strings.TrimSpace(resolution.SchemaID)
		if !isSchemaIDURL(schemaID) {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: fmt.Sprintf("external schema_id must be an absolute URL: %q", schemaID), Duration: time.Since(start).String()}
		}
		if schemaResolutionMode(strings.ToLower(schemaResolution)) == schemaResolutionPathOnly {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: fmt.Sprintf("schema-resolution=path-only cannot resolve external schema_id %q", schemaID), Duration: time.Since(start).String()}
		}
		if idIndex == nil {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: fmt.Sprintf("cannot resolve external schema_id %q without --ref-dir", schemaID), Duration: time.Since(start).String()}
		}
		entry, ok := idIndex.Get(schemaID)
		if !ok {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: fmt.Sprintf("schema_id not found in --ref-dir index: %q", schemaID), Duration: time.Since(start).String()}
		}

		schemaRef.Path = entry.Path

		doc, err := parseDataBytes(fullPath, dataBytes)
		if err != nil {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: err.Error(), Duration: time.Since(start).String()}
		}
		result, err = schema.ValidateFromBytesWithIDIndex(entry.Normalized, doc, idIndex)
		if err != nil {
			return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: err.Error(), Duration: time.Since(start).String()}
		}
	default:
		return validateSuiteFileResult{Path: rel, Schema: schemaRef, Status: "fail", Valid: false, Error: fmt.Sprintf("unsupported schema source: %s", source), Duration: time.Since(start).String()}
	}

	fileRes := validateSuiteFileResult{Path: rel, Schema: schemaRef, Valid: result.Valid, Errors: result.Errors, Duration: time.Since(start).String()}
	if result.Valid {
		if isExpectedFail {
			fileRes.Status = "unexpected_pass"
			fileRes.Valid = true
			fileRes.Error = "file was expected to fail but passed"
			return fileRes
		}
		fileRes.Status = "pass"
		return fileRes
	}
	if isExpectedFail {
		fileRes.Status = "expected_fail"
		fileRes.Valid = false
		return fileRes
	}
	fileRes.Status = "fail"
	return fileRes
}

func resolveSuiteOverridePath(loadResult *mapping.LoadResult, schemaID string) string {
	if loadResult == nil {
		return ""
	}
	id := strings.TrimSpace(schemaID)
	if id == "" {
		return ""
	}
	for _, ov := range loadResult.Effective.Overrides {
		if strings.TrimSpace(ov.SchemaID) != id {
			continue
		}
		p := strings.TrimSpace(ov.Path)
		if p != "" {
			return p
		}
	}
	return ""
}

func looksLikeSchemaPath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return false
	}
	if strings.ContainsAny(p, "/\\") {
		return true
	}
	low := strings.ToLower(p)
	return strings.HasSuffix(low, ".json") || strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml")
}

func parseDataBytes(path string, data []byte) (any, error) {
	ext := strings.ToLower(filepath.Ext(path))
	var doc any
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("failed to parse %s as YAML: %w", path, err)
		}
	case ".json":
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("failed to parse %s as JSON: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported data format: %s", ext)
	}
	return doc, nil
}

func matchAny(patterns []string, path string) bool {
	if len(patterns) == 0 {
		return false
	}
	p := filepath.ToSlash(strings.TrimPrefix(path, "./"))
	for _, raw := range patterns {
		pat := filepath.ToSlash(strings.TrimSpace(raw))
		if pat == "" {
			continue
		}
		ok, err := doublestar.Match(pat, p)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func printSuiteMarkdown(cmd *cobra.Command, res validateSuiteResult) {
	cmd.Printf("# Validation Suite Results\n\n")
	cmd.Printf("- Total: %d\n", res.Summary.Total)
	cmd.Printf("- Passed: %d\n", res.Summary.Passed)
	cmd.Printf("- Failed: %d\n", res.Summary.Failed)
	cmd.Printf("- Expected fail: %d\n", res.Summary.ExpectedFail)
	cmd.Printf("- Unexpected pass: %d\n", res.Summary.UnexpectedPass)
	cmd.Printf("- Skipped: %d\n", res.Summary.Skipped)
	cmd.Printf("- Unmapped: %d\n", res.Summary.Unmapped)
	cmd.Printf("\n")

	if res.Summary.Failed == 0 && res.Summary.UnexpectedPass == 0 && res.Summary.Unmapped == 0 {
		cmd.Println("✅ Suite passed")
		return
	}
	cmd.Println("❌ Suite has failures")

	for _, f := range res.Files {
		switch f.Status {
		case "fail", "unexpected_pass", "unmapped":
			cmd.Printf("\n## %s (%s)\n", f.Path, f.Status)
			if f.Schema != nil {
				cmd.Printf("- Schema: %s (%s)\n", f.Schema.ID, f.Schema.Source)
			}
			if f.Error != "" {
				cmd.Printf("- Error: %s\n", f.Error)
			}
			for _, e := range f.Errors {
				cmd.Printf("- %s: %s\n", e.Path, e.Message)
			}
		}
	}
}
