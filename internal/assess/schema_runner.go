package assess

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"bytes"
	"encoding/json"

	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/fulmenhq/goneat/pkg/schema/mapping"
	"github.com/fulmenhq/goneat/pkg/work"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

// isOfflineMode checks if offline schema validation is enabled via environment variable
//
//nolint:unused
func isOfflineMode() bool {
	return os.Getenv("GONEAT_OFFLINE_SCHEMA_VALIDATION") == "true"
}

// SchemaAssessmentRunner implements AssessmentRunner for schema-aware validation (preview)
type SchemaAssessmentRunner struct {
	commandName string
}

type schemaMappingContext struct {
	repoRoot     string
	manifestPath string
	loadResult   *mapping.LoadResult
	resolver     *mapping.Resolver
	threshold    float64
	strict       bool
	diagnostics  []mapping.Diagnostic
}

func NewSchemaAssessmentRunner() *SchemaAssessmentRunner {
	return &SchemaAssessmentRunner{commandName: "schema"}
}

func (r *SchemaAssessmentRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	start := time.Now()
	var issues []Issue
	metrics := make(map[string]interface{})

	repoRoot := r.determineRepoRoot(target)

	var mappingCtx *schemaMappingContext
	if config.SchemaMapping.Enabled {
		var err error
		mappingCtx, err = r.prepareMappingContext(repoRoot, config)
		if err != nil {
			return &AssessmentResult{
				CommandName:   r.commandName,
				Category:      CategorySchema,
				Success:       false,
				ExecutionTime: HumanReadableDuration(time.Since(start)),
				Error:         fmt.Sprintf("schema mapping initialisation failed: %v", err),
			}, nil
		}
		if len(mappingCtx.diagnostics) > 0 {
			diags := make([]string, 0, len(mappingCtx.diagnostics))
			for _, d := range mappingCtx.diagnostics {
				diags = append(diags, d.Message)
			}
			metrics["schema_mapping_diagnostics"] = diags
		}
	}

	schemaCandidates, err := r.findCandidates(target, config)
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategorySchema,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(start)),
			Error:         fmt.Sprintf("discovery failed: %v", err),
		}, nil
	}
	metrics["schema_candidate_files"] = len(schemaCandidates)

	candidateStart := time.Now()

	workerCount := r.resolveSchemaWorkerCount(config)
	if r.shouldAvoidNestedParallelism(config) {
		workerCount = 1
	}
	if workerCount < 1 {
		workerCount = 1
	}
	if len(schemaCandidates) < 2 {
		workerCount = 1
	}

	metaCache := newMetaSchemaValidatorCache()

	var schemaValidated int
	var mu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(workerCount)

	for _, f := range schemaCandidates {
		f := f
		g.Go(func() error {
			fileIssues, validated := r.validateSchemaCandidate(gctx, f, config, metaCache)
			mu.Lock()
			if validated {
				schemaValidated++
			}
			issues = append(issues, fileIssues...)
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	if ctx.Err() != nil {
		return &AssessmentResult{CommandName: r.commandName, Category: CategorySchema, Success: false, ExecutionTime: HumanReadableDuration(time.Since(start)), Error: ctx.Err().Error()}, nil
	}

	metrics["schema_validated_files"] = schemaValidated
	metrics["schema_validation_workers"] = workerCount
	metrics["schema_meta_validation_enabled"] = config.SchemaEnableMeta
	metrics["schema_meta_validators_compiled"] = metaCache.Count()
	candidateDuration := time.Since(candidateStart)
	metrics["schema_validation_duration"] = HumanReadableDuration(candidateDuration)
	if secs := candidateDuration.Seconds(); secs > 0 {
		metrics["schema_validation_files_per_sec"] = float64(schemaValidated) / secs
	}

	if mappingCtx != nil {
		configCandidates, err := r.findConfigCandidates(target, config)
		if err != nil {
			return &AssessmentResult{
				CommandName:   r.commandName,
				Category:      CategorySchema,
				Success:       false,
				ExecutionTime: HumanReadableDuration(time.Since(start)),
				Error:         fmt.Sprintf("config discovery failed: %v", err),
			}, nil
		}
		metrics["schema_mapping_candidate_files"] = len(configCandidates)

		mappingIssues, mappingMetrics := r.processConfigMappings(ctx, configCandidates, mappingCtx)
		issues = append(issues, mappingIssues...)
		for k, v := range mappingMetrics {
			metrics[k] = v
		}
	}

	if len(issues) > 1 {
		sortIssuesDeterministic(issues)
	}

	var finalMetrics map[string]interface{}
	if len(metrics) > 0 {
		finalMetrics = metrics
	}

	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategorySchema,
		Success:       true,
		ExecutionTime: HumanReadableDuration(time.Since(start)),
		Issues:        issues,
		Metrics:       finalMetrics,
	}, nil
}

func (r *SchemaAssessmentRunner) CanRunInParallel() bool          { return true }
func (r *SchemaAssessmentRunner) GetCategory() AssessmentCategory { return CategorySchema }
func (r *SchemaAssessmentRunner) GetEstimatedTime(target string) time.Duration {
	return 2 * time.Second
}
func (r *SchemaAssessmentRunner) IsAvailable() bool { return true }

type metaSchemaValidatorCache struct {
	mu      sync.Mutex
	byDraft map[string]*schema.Validator
}

func newMetaSchemaValidatorCache() *metaSchemaValidatorCache {
	return &metaSchemaValidatorCache{byDraft: make(map[string]*schema.Validator)}
}

func (c *metaSchemaValidatorCache) Get(draft string) (*schema.Validator, error) {
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

func (c *metaSchemaValidatorCache) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.byDraft)
}

func (r *SchemaAssessmentRunner) resolveSchemaWorkerCount(config AssessmentConfig) int {
	if config.Concurrency > 0 {
		return config.Concurrency
	}
	percent := config.ConcurrencyPercent
	if percent <= 0 {
		percent = 80
	}
	cores := runtime.NumCPU()
	workers := (cores * percent) / 100
	if workers < 1 {
		workers = 1
	}
	return workers
}

func (r *SchemaAssessmentRunner) shouldAvoidNestedParallelism(config AssessmentConfig) bool {
	// When schema is executed as part of a full assess run (i.e., no explicit category
	// selection), the assess engine already provides category-level parallelism.
	// Avoid per-file parallelism here to prevent CPU oversubscription.
	if len(config.SelectedCategories) == 0 {
		return true
	}
	if len(config.SelectedCategories) != 1 {
		return true
	}
	return strings.TrimSpace(config.SelectedCategories[0]) != string(CategorySchema)
}

func (r *SchemaAssessmentRunner) validateSchemaCandidate(ctx context.Context, path string, config AssessmentConfig, metaCache *metaSchemaValidatorCache) ([]Issue, bool) {
	select {
	case <-ctx.Done():
		return nil, false
	default:
	}

	data, err := safeReadFile(path)
	if err != nil {
		return []Issue{{
			File:          path,
			Severity:      SeverityHigh,
			Message:       fmt.Sprintf("Failed to read schema file: %v", err),
			Category:      CategorySchema,
			SubCategory:   "read",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(1 * time.Minute),
		}}, false
	}

	lower := strings.ToLower(path)

	// Syntax validation first
	if strings.HasSuffix(lower, ".json") {
		if err := r.checkJSONSyntaxBytes(data); err != nil {
			return []Issue{{
				File:          path,
				Severity:      SeverityHigh,
				Message:       fmt.Sprintf("JSON syntax error: %v", err),
				Category:      CategorySchema,
				SubCategory:   "json_syntax",
				AutoFixable:   false,
				EstimatedTime: HumanReadableDuration(2 * time.Minute),
			}}, false
		}
	}
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		if err := r.checkYAMLSyntaxBytes(data); err != nil {
			return []Issue{{
				File:          path,
				Severity:      SeverityHigh,
				Message:       fmt.Sprintf("YAML syntax error: %v", err),
				Category:      CategorySchema,
				SubCategory:   "yaml_syntax",
				AutoFixable:   false,
				EstimatedTime: HumanReadableDuration(2 * time.Minute),
			}}, false
		}
	}

	isSchema, draft, err := r.detectSchemaInfoFromBytes(data)
	if err != nil {
		return []Issue{{
			File:          path,
			Severity:      SeverityMedium,
			Message:       fmt.Sprintf("Failed to analyze schema info: %v", err),
			Category:      CategorySchema,
			SubCategory:   "schema_analysis",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(1 * time.Minute),
		}}, false
	}
	if !isSchema {
		return nil, false
	}

	// Check if draft is allowed by configuration
	if !r.isDraftAllowed(draft, config) {
		return []Issue{{
			File:          path,
			Severity:      SeverityLow,
			Message:       fmt.Sprintf("Schema draft '%s' not in allowed drafts list", draft),
			Category:      CategorySchema,
			SubCategory:   "draft_filter",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(1 * time.Minute),
		}}, true
	}

	// Basic structural validation
	if err := r.checkJSONSchemaStructureBytes(data); err != nil {
		return []Issue{{
			File:          path,
			Severity:      SeverityHigh,
			Message:       fmt.Sprintf("Schema structural validation failed: %v", err),
			Category:      CategorySchema,
			SubCategory:   "schema_structure",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(3 * time.Minute),
		}}, true
	}

	if !config.SchemaEnableMeta {
		return nil, true
	}

	metaIssues := r.validateSchemaMetaBytes(path, draft, data, metaCache)
	return metaIssues, true
}

func (r *SchemaAssessmentRunner) checkJSONSyntaxBytes(data []byte) error {
	var v interface{}
	dec := jsonNewDecoder(bytesNewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	return nil
}

func (r *SchemaAssessmentRunner) checkYAMLSyntaxBytes(data []byte) error {
	var v interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

func (r *SchemaAssessmentRunner) detectSchemaInfoFromBytes(data []byte) (isSchema bool, draft string, err error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		if err := json.Unmarshal(data, &doc); err != nil {
			return false, "", nil
		}
	}
	if schemaURL, ok := doc["$schema"].(string); ok {
		draft := r.extractDraftFromURL(schemaURL)
		return true, draft, nil
	}
	return false, "", nil
}

func (r *SchemaAssessmentRunner) checkJSONSchemaStructureBytes(data []byte) error {
	// Try YAML first
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
	if err := sanityCheckJSONSchema(doc); err != nil {
		return err
	}
	return nil
}

func (r *SchemaAssessmentRunner) validateSchemaMetaBytes(path string, draft string, schemaBytes []byte, metaCache *metaSchemaValidatorCache) []Issue {
	validator, err := metaCache.Get(draft)
	if err != nil {
		return []Issue{{
			File:          path,
			Severity:      SeverityMedium,
			Message:       fmt.Sprintf("Unsupported schema draft '%s': %v", draft, err),
			Category:      CategorySchema,
			SubCategory:   "meta_validation",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(1 * time.Minute),
		}}
	}

	result, err := validator.ValidateBytes(schemaBytes)
	if err != nil {
		return []Issue{{
			File:          path,
			Severity:      SeverityHigh,
			Message:       fmt.Sprintf("Meta-validation error: %v", err),
			Category:      CategorySchema,
			SubCategory:   "meta_validation",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(3 * time.Minute),
		}}
	}

	if result.Valid {
		return nil
	}

	issues := make([]Issue, 0, len(result.Errors))
	for _, verr := range result.Errors {
		severity := SeverityMedium
		msgLower := strings.ToLower(verr.Message)
		if strings.Contains(msgLower, "required") {
			severity = SeverityHigh
		} else if strings.Contains(msgLower, "additional") {
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
	return issues
}

func sortIssuesDeterministic(issues []Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		a := issues[i]
		b := issues[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.SubCategory != b.SubCategory {
			return a.SubCategory < b.SubCategory
		}
		return a.Message < b.Message
	})
}

func (r *SchemaAssessmentRunner) findCandidates(target string, config AssessmentConfig) ([]string, error) {
	return r.findFilteredCandidates(target, config, func(path string) bool {
		return r.isSchemaCandidate(path, config)
	})
}

func (r *SchemaAssessmentRunner) findConfigCandidates(target string, config AssessmentConfig) ([]string, error) {
	return r.findFilteredCandidates(target, config, nil)
}

func (r *SchemaAssessmentRunner) findFilteredCandidates(target string, config AssessmentConfig, candidateFn func(string) bool) ([]string, error) {
	var files []string

	matchesCandidate := func(path string) bool {
		if candidateFn == nil {
			return true
		}
		return candidateFn(path)
	}

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
							if !r.isExcluded(path, config) && matchesCandidate(path) {
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
							if !r.isExcluded(item.Path, config) && matchesCandidate(item.Path) {
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
				if !r.isExcluded(p, config) && matchesCandidate(p) {
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
			if !r.isExcluded(p, config) && matchesCandidate(p) {
				files = append(files, p)
			}
		}
	}
	return files, nil
}

func (r *SchemaAssessmentRunner) prepareMappingContext(repoRoot string, config AssessmentConfig) (*schemaMappingContext, error) {
	mgr, err := mapping.NewManager()
	if err != nil {
		return nil, fmt.Errorf("initialise schema mapping manager: %w", err)
	}
	loadResult, err := mgr.Load(mapping.LoadOptions{
		RepoRoot:     repoRoot,
		ManifestPath: config.SchemaMapping.ManifestPath,
	})
	if err != nil {
		return nil, fmt.Errorf("load schema mapping manifest: %w", err)
	}
	resolver := mapping.NewResolver(loadResult.Effective)
	manifestPath := loadResult.RepositoryPath
	if manifestPath == "" {
		manifestPath = filepath.Join(repoRoot, mapping.DefaultManifestRelativePath)
	}
	threshold := 0.0
	if loadResult.Effective.Config.MinConfidence != nil {
		threshold = *loadResult.Effective.Config.MinConfidence
	}
	if config.SchemaMapping.MinConfidence > 0 {
		threshold = config.SchemaMapping.MinConfidence
	}
	strict := config.SchemaMapping.Strict
	if !strict && loadResult.Effective.Config.StrictMode != nil {
		strict = *loadResult.Effective.Config.StrictMode
	}
	return &schemaMappingContext{
		repoRoot:     filepath.Clean(repoRoot),
		manifestPath: manifestPath,
		loadResult:   loadResult,
		resolver:     resolver,
		threshold:    threshold,
		strict:       strict,
		diagnostics:  loadResult.Diagnostics,
	}, nil
}

func (r *SchemaAssessmentRunner) processConfigMappings(ctx context.Context, files []string, mappingCtx *schemaMappingContext) ([]Issue, map[string]interface{}) {
	var issues []Issue
	metrics := make(map[string]interface{})

	seen := make(map[string]struct{})
	validationSuccess := 0
	validationFailures := 0
	strictFailures := 0

	for _, file := range files {
		select {
		case <-ctx.Done():
			return issues, metrics
		default:
		}

		abs := file
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(mappingCtx.repoRoot, file)
		}
		abs = filepath.Clean(abs)
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}

		rel, err := filepath.Rel(mappingCtx.repoRoot, abs)
		if err != nil {
			rel = filepath.Base(abs)
		}
		rel = filepath.ToSlash(strings.TrimPrefix(rel, "./"))

		resolution, ok := mappingCtx.resolver.Resolve(rel)
		if ok && resolution.Excluded {
			continue
		}
		if !ok || resolution.SchemaID == "" {
			if mappingCtx.strict {
				issues = append(issues, Issue{
					File:          abs,
					Severity:      SeverityMedium,
					Message:       fmt.Sprintf("No schema mapping found for %s", rel),
					Category:      CategorySchema,
					SubCategory:   "schema_mapping_missing",
					AutoFixable:   false,
					EstimatedTime: HumanReadableDuration(1 * time.Minute),
				})
				strictFailures++
			}
			continue
		}

		if resolution.Confidence < mappingCtx.threshold {
			if mappingCtx.strict {
				issues = append(issues, Issue{
					File:          abs,
					Severity:      SeverityMedium,
					Message:       fmt.Sprintf("Schema mapping confidence %.2f below threshold %.2f", resolution.Confidence, mappingCtx.threshold),
					Category:      CategorySchema,
					SubCategory:   "schema_mapping_confidence",
					AutoFixable:   false,
					EstimatedTime: HumanReadableDuration(1 * time.Minute),
				})
				strictFailures++
			}
			continue
		}

		configIssues := r.validateConfigAgainstSchema(abs, resolution.SchemaID)
		if len(configIssues) == 0 {
			validationSuccess++
		} else {
			validationFailures += len(configIssues)
			issues = append(issues, configIssues...)
		}
	}

	resMetrics := mappingCtx.resolver.Metrics()
	metrics["schema_mapping_files_evaluated"] = resMetrics.FilesEvaluated
	metrics["schema_mapping_mapped"] = resMetrics.Mapped
	metrics["schema_mapping_unmapped"] = resMetrics.Unmapped
	metrics["schema_mapping_exclusions"] = resMetrics.Excluded
	metrics["schema_mapping_confidence_threshold"] = mappingCtx.threshold
	metrics["schema_mapping_strict"] = mappingCtx.strict
	metrics["schema_mapping_manifest_path"] = mappingCtx.manifestPath
	metrics["schema_mapping_validation_success"] = validationSuccess
	metrics["schema_mapping_validation_failures"] = validationFailures
	metrics["schema_mapping_strict_failures"] = strictFailures
	if resMetrics.FilesEvaluated > 0 {
		den := float64(resMetrics.FilesEvaluated)
		metrics["schema_mapping_detection_rate"] = float64(resMetrics.Mapped) / den
		metrics["schema_mapping_unmapped_rate"] = float64(resMetrics.Unmapped) / den
	}

	return issues, metrics
}

func (r *SchemaAssessmentRunner) validateConfigAgainstSchema(path string, schemaID string) []Issue {
	var issues []Issue
	validator, err := schema.GetEmbeddedValidator(schemaID)
	if err != nil {
		issues = append(issues, Issue{
			File:          path,
			Severity:      SeverityHigh,
			Message:       fmt.Sprintf("Schema %s not available: %v", schemaID, err),
			Category:      CategorySchema,
			SubCategory:   "schema_mapping_validator",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(2 * time.Minute),
		})
		return issues
	}

	data, err := safeReadFile(path)
	if err != nil {
		issues = append(issues, Issue{
			File:          path,
			Severity:      SeverityHigh,
			Message:       fmt.Sprintf("Failed to read config for schema validation: %v", err),
			Category:      CategorySchema,
			SubCategory:   "schema_mapping_read",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(2 * time.Minute),
		})
		return issues
	}

	result, err := validator.ValidateBytes(data)
	if err != nil {
		issues = append(issues, Issue{
			File:          path,
			Severity:      SeverityHigh,
			Message:       fmt.Sprintf("Schema validation error (%s): %v", schemaID, err),
			Category:      CategorySchema,
			SubCategory:   "schema_mapping_validation",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(3 * time.Minute),
		})
		return issues
	}

	if !result.Valid {
		for _, verr := range result.Errors {
			issues = append(issues, Issue{
				File:          path,
				Line:          verr.Context.LineNumber,
				Severity:      SeverityHigh,
				Message:       fmt.Sprintf("Schema mapping violation (%s): %s", schemaID, formatValidationMessage(verr)),
				Category:      CategorySchema,
				SubCategory:   "schema_mapping_validation",
				AutoFixable:   false,
				EstimatedTime: HumanReadableDuration(3 * time.Minute),
			})
		}
	}
	return issues
}

func formatValidationMessage(verr schema.ValidationError) string {
	if verr.Path != "" {
		return fmt.Sprintf("%s: %s", verr.Path, verr.Message)
	}
	return verr.Message
}

func (r *SchemaAssessmentRunner) determineRepoRoot(start string) string {
	if start == "" {
		start = "."
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		if root := findRepoRoot(); root != "" {
			return root
		}
		return "."
	}
	dir := abs
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if root := findRepoRoot(); root != "" {
		return root
	}
	return abs
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
	if strings.Contains(schemaURL, "draft-06") {
		return "draft-06"
	}

	// Default to 2020-12 if we can't determine
	return "2020-12"
}

// isDraftAllowed checks if the detected draft is in the allowed list
func (r *SchemaAssessmentRunner) isDraftAllowed(draft string, config AssessmentConfig) bool {
	if len(config.SchemaDrafts) == 0 {
		// No filter specified, allow all supported drafts
		return draft == "draft-04" || draft == "draft-06" || draft == "draft-07" || draft == "2019-09" || draft == "2020-12"
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

//nolint:unused
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
