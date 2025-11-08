/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dates

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fulmenhq/goneat/pkg/ignore"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/schema"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"gopkg.in/yaml.v3"
)

// DatePattern defines an order-aware date regex pattern
type DatePattern struct {
	Regex       string `json:"regex" yaml:"regex"`
	Order       string `json:"order,omitempty" yaml:"order,omitempty"` // YMD, MDY, DMY
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type FutureDates struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	MaxSkew  string `json:"max_skew" yaml:"max_skew"`
	Severity string `json:"severity" yaml:"severity"`
	AutoFix  bool   `json:"auto_fix" yaml:"auto_fix"`
}

type StaleEntries struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	WarnDays int    `json:"warn_days" yaml:"warn_days"`
	Severity string `json:"severity" yaml:"severity"`
}

type MonotonicOrder struct {
	Enabled     bool     `json:"enabled" yaml:"enabled"`
	Files       []string `json:"files" yaml:"files"`
	Severity    string   `json:"severity" yaml:"severity"`
	CheckTopN   int      `json:"check_top_n,omitempty" yaml:"check_top_n,omitempty"`
	IgnoreFiles []string `json:"ignore_files,omitempty" yaml:"ignore_files,omitempty"`
}

type CrossFileConsistency struct {
	Enabled  bool       `json:"enabled" yaml:"enabled"`
	Groups   [][]string `json:"groups" yaml:"groups"`
	Severity string     `json:"severity" yaml:"severity"`
}

type Rules struct {
	FutureDates          FutureDates          `json:"future_dates" yaml:"future_dates"`
	StaleEntries         StaleEntries         `json:"stale_entries" yaml:"stale_entries"`
	MonotonicOrder       MonotonicOrder       `json:"monotonic_order" yaml:"monotonic_order"`
	CrossFileConsistency CrossFileConsistency `json:"cross_file_consistency" yaml:"cross_file_consistency"`
}

type Files struct {
	Include          []string `json:"include" yaml:"include"`
	Exclude          []string `json:"exclude" yaml:"exclude"`
	TextExtensions   []string `json:"text_extensions" yaml:"text_extensions"`
	MaxFileSizeBytes int64    `json:"max_file_size_bytes" yaml:"max_file_size_bytes"`
}

type Output struct {
	Format string `json:"format" yaml:"format"`
	FailOn string `json:"fail_on" yaml:"fail_on"`
}

type AiSafety struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	DetectPlaceholders bool   `json:"detect_placeholders" yaml:"detect_placeholders"`
	DetectImpossible   bool   `json:"detect_impossible" yaml:"detect_impossible"`
	Severity           string `json:"severity" yaml:"severity"`
}

// DatesConfig contains configuration for dates validation
type DatesConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	Rules Rules `json:"rules" yaml:"rules"`
	Files Files `json:"files" yaml:"files"`

	DatePatterns []DatePattern `json:"date_patterns" yaml:"date_patterns"`
	Timezone     string        `json:"timezone" yaml:"timezone"`
	Now          *string       `json:"now,omitempty" yaml:"now,omitempty"`
	AiSafety     AiSafety      `json:"ai_safety" yaml:"ai_safety"`
	Output       Output        `json:"output" yaml:"output"`
}

// DatesIssue represents a single dates finding
type DatesIssue struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Category    string `json:"category"`
	AutoFixable bool   `json:"auto_fixable"`
}

// DatesResult for dates-specific output (consumed by assess runner)
type DatesResult struct {
	Success       bool                   `json:"success"`
	Issues        []DatesIssue           `json:"issues"`
	Metrics       map[string]interface{} `json:"metrics"`
	ExecutionTime string                 `json:"execution_time"`
	Error         string                 `json:"error,omitempty"`
}

// DatesRunner handles dates validation logic
type DatesRunner struct {
	config DatesConfig
}

// DefaultDatesConfig returns sensible defaults for scanning
func DefaultDatesConfig() DatesConfig {
	return DatesConfig{
		Enabled: true,
		Rules: Rules{
			FutureDates:          FutureDates{Enabled: true, MaxSkew: "24h", Severity: "error", AutoFix: false},
			StaleEntries:         StaleEntries{Enabled: true, WarnDays: 180, Severity: "warning"},
			MonotonicOrder:       MonotonicOrder{Enabled: true, Files: []string{"**/CHANGELOG*.md", "**/HISTORY*.md", "**/NEWS*.md", "**/*changelog*.md"}, Severity: "warning"},
			CrossFileConsistency: CrossFileConsistency{Enabled: false, Groups: [][]string{}, Severity: "warning"},
		},
		Files: Files{
			Include: []string{
				"CHANGELOG.md", "**/CHANGELOG*.md", "**/HISTORY.md", "RELEASE_NOTES.md", "**/RELEASE*.md", "**/VERSION",
			},
			Exclude:          []string{"**/node_modules/**", "**/.git/**", "**/dist/**", "**/build/**", "**/.scratchpad/**"},
			TextExtensions:   []string{".md", ".txt", ".yaml", ".yml", ".json", ".toml"},
			MaxFileSizeBytes: 4 * 1024 * 1024,
		},
		DatePatterns: []DatePattern{{Regex: `(\d{4})-(\d{2})-(\d{2})`, Order: "YMD", Description: "ISO 8601"}},
		Timezone:     "UTC",
		Now:          nil,
		AiSafety:     AiSafety{Enabled: true, DetectPlaceholders: true, DetectImpossible: true, Severity: "warning"},
		Output:       Output{Format: "text", FailOn: "error"},
	}
}

// LoadDatesConfig loads dates configuration (Pattern 1: user-extensible-from-default)
// Search: project/.goneat/dates.yaml -> GONEAT_HOME/config/dates.yaml -> built-in defaults
// For single files, uses the file's directory as working directory for config resolution
func LoadDatesConfig(target string) DatesConfig {
	cfg := DefaultDatesConfig()

	// Use standardized config resolver to find the appropriate config file
	resolver := NewConfigResolver(target)
	configPath, found := resolver.ResolveConfigFile("dates")

	if !found {
		return cfg
	}

	// #nosec G304 -- configPath from ResolveConfigFile with controlled paths
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg
	}

	// Parse raw data to interface{} for schema validation
	var doc interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// Fall back to defaults if parsing fails
		return cfg
	}

	// Validate against dates schema
	valResult, err := schema.Validate(doc, "dates-v1.0.0")
	if err != nil || !valResult.Valid {
		// Log validation errors but continue with defaults
		if err != nil {
			logger.Debug(fmt.Sprintf("dates config schema validation setup failed: %v", err))
		} else if !valResult.Valid {
			logger.Debug(fmt.Sprintf("dates config validation failed: %d errors", len(valResult.Errors)))
			for _, ve := range valResult.Errors {
				logger.Debug(fmt.Sprintf("- %s: %s", ve.Path, ve.Message))
			}
		}
		return cfg
	}

	// If validation passes, unmarshal to struct
	var fileCfg DatesConfig
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return cfg
	}

	// DEBUG: Log what we loaded
	logger.Debug(fmt.Sprintf("dates config loaded: AiSafety.Enabled=%v", fileCfg.AiSafety.Enabled))

	// Schema validated config can be trusted - merge defaults with user config
	// Includes: Opt-in approach - if user specifies any includes, use only theirs
	if len(fileCfg.Files.Include) == 0 {
		fileCfg.Files.Include = cfg.Files.Include
	}
	// Excludes: Always include defaults, plus any user-specified excludes
	fileCfg.Files.Exclude = append(fileCfg.Files.Exclude, cfg.Files.Exclude...)
	if len(fileCfg.Files.TextExtensions) == 0 {
		fileCfg.Files.TextExtensions = cfg.Files.TextExtensions
	}
	if fileCfg.Files.MaxFileSizeBytes == 0 {
		fileCfg.Files.MaxFileSizeBytes = cfg.Files.MaxFileSizeBytes
	}
	if len(fileCfg.DatePatterns) == 0 {
		fileCfg.DatePatterns = cfg.DatePatterns
	}
	if fileCfg.Timezone == "" {
		fileCfg.Timezone = cfg.Timezone
	}
	if fileCfg.Rules.FutureDates.MaxSkew == "" && fileCfg.Rules.FutureDates.Enabled {
		fileCfg.Rules.FutureDates.MaxSkew = cfg.Rules.FutureDates.MaxSkew
	}
	if fileCfg.Rules.FutureDates.Severity == "" {
		fileCfg.Rules.FutureDates.Severity = cfg.Rules.FutureDates.Severity
	}
	if fileCfg.Rules.StaleEntries.Severity == "" {
		fileCfg.Rules.StaleEntries.Severity = cfg.Rules.StaleEntries.Severity
	}
	if fileCfg.Rules.MonotonicOrder.Severity == "" {
		fileCfg.Rules.MonotonicOrder.Severity = cfg.Rules.MonotonicOrder.Severity
	}
	if fileCfg.Output.Format == "" {
		fileCfg.Output.Format = cfg.Output.Format
	}
	if fileCfg.Output.FailOn == "" {
		fileCfg.Output.FailOn = cfg.Output.FailOn
	}

	return fileCfg
}

// NewConfigResolver creates a config resolver - bridge to assess package
func NewConfigResolver(target string) ConfigResolver {
	workingDir := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		// Target is a file - use its directory for config resolution
		workingDir = filepath.Dir(target)
	}

	// Ensure we have an absolute path for consistent behavior
	if absDir, err := filepath.Abs(workingDir); err == nil {
		workingDir = absDir
	}

	return ConfigResolver{workingDir: workingDir}
}

// ConfigResolver provides config file resolution (local copy from assess package)
type ConfigResolver struct {
	workingDir string
}

// ResolveConfigFile finds category-specific config files using standardized search paths
func (cr *ConfigResolver) ResolveConfigFile(category string) (string, bool) {
	// 1. Project-level config
	projectConfig := filepath.Join(cr.workingDir, ".goneat", category+".yaml")
	if info, err := os.Stat(projectConfig); err == nil && !info.IsDir() {
		return projectConfig, true
	}

	// 2. User-level config (GONEAT_HOME)
	if homeDir := os.Getenv("GONEAT_HOME"); homeDir != "" {
		userConfig := filepath.Join(homeDir, "config", category+".yaml")
		if info, err := os.Stat(userConfig); err == nil && !info.IsDir() {
			return userConfig, true
		}
	} else if homeDir, err := os.UserHomeDir(); err == nil {
		// Fall back to ~/.goneat/config/{category}.yaml
		userConfig := filepath.Join(homeDir, ".goneat", "config", category+".yaml")
		if info, err := os.Stat(userConfig); err == nil && !info.IsDir() {
			return userConfig, true
		}
	}

	// 3. No config file found
	return "", false
}

// mapSeverityString normalizes severity strings to valid assess severity levels
func mapSeverityString(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "critical"
	case "high", "error": // Map "error" to "high" for backward compatibility
		return "high"
	case "medium", "warning": // Map "warning" to "medium" for backward compatibility
		return "medium"
	case "low":
		return "low"
	case "info":
		return "info"
	default:
		logger.Warn(fmt.Sprintf("Unknown severity level '%s', defaulting to 'medium'", severity))
		return "medium"
	}
}

// mapSeverityStringToAssessSeverity converts severity strings to assess IssueSeverity types
func mapSeverityStringToAssessSeverity(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "critical"
	case "high", "error": // Map "error" to "high" for backward compatibility
		return "high"
	case "medium", "warning": // Map "warning" to "medium" for backward compatibility
		return "medium"
	case "low":
		return "low"
	case "info":
		return "info"
	default:
		logger.Warn(fmt.Sprintf("Unknown severity level '%s', defaulting to 'medium'", severity))
		return "medium"
	}
}

// NewDatesRunner creates a runner
func NewDatesRunner() *DatesRunner { return &DatesRunner{config: DefaultDatesConfig()} }

// NewDatesRunnerWithConfig creates a new dates runner with the specified configuration
func NewDatesRunnerWithConfig(config DatesConfig) *DatesRunner {
	return &DatesRunner{config: config}
}

// Assess scans target for date issues according to config
func (r *DatesRunner) Assess(ctx context.Context, target string, _ interface{}) (*DatesResult, error) {
	start := time.Now()
	cfg := r.config
	if !cfg.Enabled {
		return &DatesResult{Success: true, Issues: []DatesIssue{}, Metrics: map[string]interface{}{"enabled": false}, ExecutionTime: time.Since(start).String()}, nil
	}
	now := time.Now()
	if cfg.Now != nil {
		if t, err := time.Parse(time.RFC3339, *cfg.Now); err == nil {
			now = t
		}
	}
	loc := time.UTC
	if cfg.Timezone != "" {
		if l, err := time.LoadLocation(cfg.Timezone); err == nil {
			loc = l
		}
	}
	now = now.In(loc)

	// Debug: Configuration summary
	logger.Debug(fmt.Sprintf("Dates assessment config: MonotonicOrder.Enabled=%v, target=%s",
		cfg.Rules.MonotonicOrder.Enabled, target))

	// compile patterns
	var pats []patView
	for _, p := range cfg.DatePatterns {
		if re, err := regexp.Compile(p.Regex); err == nil {
			pats = append(pats, patView{re: re, order: p.Order})
		}
	}

	// collect files (with incremental optimization if repo present)
	var files []string
	var changedOnly []string
	var repoPtr *git.Repository
	if os.Getenv("GONEAT_DATES_NO_INC") == "" {
		if repo, err := git.PlainOpen(target); err == nil {
			repoPtr = repo
			if wt, err2 := repo.Worktree(); err2 == nil {
				if st, err3 := wt.Status(); err3 == nil {
					for rel, s := range st {
						// ACMR-like: Added/Modified/Renamed (ignore deleted/unmodified)
						if s.Worktree == git.Unmodified && s.Staging == git.Unmodified {
							continue
						}
						if s.Worktree == git.Deleted || s.Staging == git.Deleted {
							continue
						}
						changedOnly = append(changedOnly, filepath.ToSlash(rel))
					}
				}
			}
		}
	}

	// Determine repository birth time (earliest commit)
	var repoBirth time.Time
	hasRepoBirth := false
	if repoPtr != nil {
		if t, ok := repoFirstCommitTime(repoPtr); ok {
			repoBirth = t
			hasRepoBirth = true
		}
	}

	// Create ignore matcher to respect .gitignore and .goneatignore
	var ignoreMatcher *ignore.Matcher
	if repoRoot, err := findRepoRoot(target); err == nil {
		if matcher, err := ignore.NewMatcher(repoRoot); err == nil {
			ignoreMatcher = matcher
		}
	}

	includeFile := func(rel string, info fs.FileInfo) bool {
		// Check ignore patterns first (highest priority)
		if ignoreMatcher != nil {
			fullPath := filepath.Join(target, rel)
			if ignoreMatcher.IsIgnored(fullPath) {
				return false
			}
		}

		// Fallback to legacy include/exclude patterns for backward compatibility
		if rel == "." || strings.HasPrefix(rel, ".git/") {
			return false
		}
		if !matchInclude(rel, cfg.Files.Include) {
			return false
		}
		if matchExclude(rel, cfg.Files.Exclude) {
			return false
		}
		if info != nil && cfg.Files.MaxFileSizeBytes > 0 && info.Size() > cfg.Files.MaxFileSizeBytes {
			return false
		}
		return true
	}

	// --- File Discovery ---
	st, err := os.Stat(target)
	isSingleFile := err == nil && !st.IsDir()

	logger.Debug(fmt.Sprintf("Dates file discovery: target=%s, isSingleFile=%v", target, isSingleFile))

	if isSingleFile {
		// Target is a single file. Check if it should be included.
		// For single files, we need to check the full relative path from cwd or the filename
		rel := filepath.Base(target)

		// First try matching the base filename against patterns
		included := includeFile(rel, st)

		// If that fails, try the full relative path (useful for patterns like "**/CHANGELOG*.md")
		if !included {
			if cwd, err := os.Getwd(); err == nil {
				if relPath, err := filepath.Rel(cwd, target); err == nil {
					relPath = filepath.ToSlash(relPath)
					included = includeFile(relPath, st)
					if included {
						rel = relPath // Use the relative path that matched
					}
				}
			}
		}

		logger.Debug(fmt.Sprintf("Dates single file: rel=%s, included=%v (patterns: %v)", rel, included, cfg.Files.Include))
		if included {
			files = append(files, rel) // Use the relative path
		}
	} else {
		// Target is a directory, proceed with discovery.
		if len(changedOnly) > 0 {
			// Incremental: filter changedOnly by include/exclude
			for _, rel := range changedOnly {
				abs := filepath.Join(target, rel)
				if st, err := os.Stat(abs); err == nil && !st.IsDir() {
					if includeFile(filepath.ToSlash(rel), st) {
						files = append(files, filepath.ToSlash(rel))
					}
				}
			}
		}
		if len(files) == 0 {
			_ = filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(target, path)
				rel = filepath.ToSlash(rel)
				if includeFile(rel, nil) {
					files = append(files, rel)
				}
				return nil
			})
		}
	}

	logger.Debug(fmt.Sprintf("Dates processing: found %d files to analyze", len(files)))
	for i, f := range files {
		logger.Debug(fmt.Sprintf("Dates file %d: %s", i+1, f))
	}

	issues := make([]DatesIssue, 0, 16)
	var mu sync.Mutex
	ch := make(chan string, len(files))
	for _, f := range files {
		ch <- f
	}
	close(ch)
	var wg sync.WaitGroup
	workers := 4
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range ch {
				var abs, rel string
				if isSingleFile {
					// For a single file, use the original target path as absolute, file as relative
					abs = target
					rel = file
				} else {
					abs = filepath.Join(target, file)
					rel = file
				}

				// #nosec G304 -- abs path constructed from validated target and file inputs
				data, err := os.ReadFile(abs)
				if err != nil {
					logger.Debug(fmt.Sprintf("Dates failed to read file %s: %v", abs, err))
					continue
				}
				content := string(data)
				if cfg.AiSafety.Enabled && cfg.AiSafety.DetectPlaceholders {
					// Check for obvious placeholder patterns
					hasPlaceholder := false
					placeholderMsg := ""

					// Check for [DATE] placeholder
					if strings.Contains(content, "[DATE]") {
						hasPlaceholder = true
						placeholderMsg = "Placeholder date detected: [DATE]"
					}

					// Check for YYYY-MM-DD in contexts that suggest placeholders (not documentation)
					if strings.Contains(content, "YYYY-MM-DD") {
						lines := strings.Split(content, "\n")
						for _, line := range lines {
							trimmed := strings.TrimSpace(line)
							// Check for template-like patterns (headings with placeholders)
							if strings.Contains(trimmed, "YYYY-MM-DD") &&
								(strings.HasPrefix(trimmed, "##") ||
									strings.Contains(trimmed, "x.y.z") ||
									strings.Contains(trimmed, "YYYY-MM-DD") && strings.Count(trimmed, "YYYY-MM-DD") == 1 && len(trimmed) < 50) {
								// Skip if it's clearly documentation about date formats
								if !strings.Contains(strings.ToLower(line), "format") &&
									!strings.Contains(strings.ToLower(line), "support") &&
									!strings.Contains(strings.ToLower(line), "pattern") {
									hasPlaceholder = true
									placeholderMsg = fmt.Sprintf("Template placeholder detected: %s", strings.TrimSpace(line))
									break
								}
							}
						}
					}

					if hasPlaceholder {
						mu.Lock()
						severity := mapSeverityStringToAssessSeverity(cfg.AiSafety.Severity)
						issues = append(issues, DatesIssue{File: rel, Line: 0, Column: 0, Severity: severity, Message: placeholderMsg, Category: "dates", AutoFixable: true})
						mu.Unlock()
					}
				}
				// Extract all dates once for file-level analyses (monotonic, stale, plausibility)
				dates := extractDates(content, pats, now.Location())
				for _, cp := range pats {
					all := cp.re.FindAllStringSubmatchIndex(content, -1)
					if len(all) == 0 {
						continue
					}
					for _, idx := range all {
						if len(idx) < 8 {
							continue
						}
						g1 := content[idx[2]:idx[3]]
						g2 := content[idx[4]:idx[5]]
						g3 := content[idx[6]:idx[7]]
						y, m, d := ParseDateParts(g1, g2, g3, cp.order)
						if !isValidDate(y, m, d) {
							continue
						}
						if cfg.Rules.FutureDates.Enabled && isFuture(y, m, d, now, cfg.Rules.FutureDates.MaxSkew) {
							line := findLineNumber(content, content[idx[0]:idx[1]])
							mu.Lock()
							severity := mapSeverityString(cfg.Rules.FutureDates.Severity)
							issues = append(issues, DatesIssue{File: rel, Line: line, Column: 0, Severity: severity, Message: fmt.Sprintf("Future date found: %04d-%02d-%02d", y, m, d), Category: "dates", AutoFixable: cfg.Rules.FutureDates.AutoFix})
							mu.Unlock()
						}
					}
				}
				// Enhanced changelog analysis with better error messages
				if cfg.Rules.MonotonicOrder.Enabled && matchesAny(rel, cfg.Rules.MonotonicOrder.Files) && !matchesAny(rel, cfg.Rules.MonotonicOrder.IgnoreFiles) {
					entries := extractChangelogEntries(content, now.Location())

					// Check for multiple "Unreleased" sections
					unreleasedCount := 0
					for _, entry := range entries {
						if strings.EqualFold(entry.Version, "unreleased") {
							unreleasedCount++
						}
					}
					if unreleasedCount > 1 {
						mu.Lock()
						severity := mapSeverityStringToAssessSeverity(cfg.Rules.MonotonicOrder.Severity)
						issues = append(issues, DatesIssue{
							File: rel, Line: 0, Column: 0, Severity: severity,
							Message:  fmt.Sprintf("Multiple 'Unreleased' sections found (%d) - should have only one at the top", unreleasedCount),
							Category: "dates", AutoFixable: false,
						})
						mu.Unlock()
					}

					// Check for "In Development" entries in wrong places
					for i, entry := range entries {
						if strings.Contains(strings.ToLower(entry.Text), "development") && i > 0 {
							mu.Lock()
							severity := mapSeverityStringToAssessSeverity(cfg.Rules.MonotonicOrder.Severity)
							issues = append(issues, DatesIssue{
								File: rel, Line: 0, Column: 0, Severity: severity,
								Message:  fmt.Sprintf("'In Development' entry '%s' appears after other releases - should be at top or have a proper date", entry.Version),
								Category: "dates", AutoFixable: false,
							})
							mu.Unlock()
						}
					}

					// Extract dated entries for monotonic order checking
					var datedEntries []ChangelogEntry
					for _, entry := range entries {
						if entry.Date != nil {
							datedEntries = append(datedEntries, entry)
						}
					}

					// Check monotonic order of dated releases
					if len(datedEntries) > 1 {
						var dates []time.Time
						for _, entry := range datedEntries {
							dates = append(dates, *entry.Date)
						}

						topN := cfg.Rules.MonotonicOrder.CheckTopN
						checkedDates := dates
						if topN > 0 && len(checkedDates) > topN {
							checkedDates = checkedDates[:topN]
						}

						if !isMonotonicDescending(checkedDates) {
							mu.Lock()
							severity := mapSeverityStringToAssessSeverity(cfg.Rules.MonotonicOrder.Severity)

							// Find the specific violation
							violationMsg := "Changelog release dates not in descending order"
							for i := 0; i < len(checkedDates)-1; i++ {
								if checkedDates[i].Before(checkedDates[i+1]) {
									older := checkedDates[i].Format("2006-01-02")
									newer := checkedDates[i+1].Format("2006-01-02")
									violationMsg = fmt.Sprintf("Changelog date order violation: %s appears before %s (should be after)", older, newer)
									break
								}
							}

							issues = append(issues, DatesIssue{
								File: rel, Line: 0, Column: 0, Severity: severity,
								Message: violationMsg, Category: "dates", AutoFixable: false,
							})
							mu.Unlock()
						}
					}

					// Debug logging for scan results (no informational issues - only report actual problems)
					sample := sampleDates(func() []time.Time {
						var dates []time.Time
						for _, entry := range datedEntries {
							dates = append(dates, *entry.Date)
						}
						return dates
					}(), 5)

					logger.Debug(fmt.Sprintf("Changelog scan: %s - found %d total entries (%d dated, %d undated); sample dates: %s",
						rel, len(entries), len(datedEntries), len(entries)-len(datedEntries), sample))
				} else {
					logger.Debug(fmt.Sprintf("Dates monotonic check skipped for %s: enabled=%v, matches=%v", rel, cfg.Rules.MonotonicOrder.Enabled, matchesAny(rel, cfg.Rules.MonotonicOrder.Files)))
				}

				// Stale (latest-only) detection
				if cfg.Rules.StaleEntries.Enabled && len(dates) > 0 && cfg.Rules.StaleEntries.WarnDays > 0 {
					latest := dates[0]
					for _, dt := range dates {
						if dt.After(latest) {
							latest = dt
						}
					}
					warnDur := time.Duration(cfg.Rules.StaleEntries.WarnDays) * 24 * time.Hour
					if now.Sub(latest) > warnDur {
						mu.Lock()
						severity := mapSeverityStringToAssessSeverity(cfg.Rules.StaleEntries.Severity)
						issues = append(issues, DatesIssue{File: rel, Line: 0, Column: 0, Severity: severity, Message: fmt.Sprintf("Stale entry: latest date %04d-%02d-%02d is older than %d days", latest.Year(), int(latest.Month()), latest.Day(), cfg.Rules.StaleEntries.WarnDays), Category: "dates", AutoFixable: false})
						mu.Unlock()
					}
				}

				// Repo-time plausibility (error): any date predating repo creation by > grace
				if hasRepoBirth && len(dates) > 0 {
					grace := 48 * time.Hour
					baseline := repoBirth.Add(-grace)
					for _, dt := range dates {
						if dt.Before(baseline) {
							mu.Lock()
							issues = append(issues, DatesIssue{File: rel, Line: 0, Column: 0, Severity: "high", Message: fmt.Sprintf("Impossible chronology: date %04d-%02d-%02d predates repository creation %s", dt.Year(), int(dt.Month()), dt.Day(), repoBirth.Format("2006-01-02")), Category: "dates", AutoFixable: false})
							mu.Unlock()
							break
						}
					}
				}
			}
		}()
	}
	wg.Wait()

	success := true // Always successful unless there's a critical error - issues are expected
	logger.Debug(fmt.Sprintf("Dates assessment complete: %d issues found", len(issues)))
	return &DatesResult{Success: success, Issues: issues, Metrics: map[string]interface{}{"enabled": cfg.Enabled}, ExecutionTime: time.Since(start).String()}, nil
}

// Helper functions

// parseInt extracts digits from s and parses to int (non-digits ignored)
func parseInt(s string) int {
	b := strings.Builder{}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return 0
	}
	n := 0
	for _, r := range b.String() {
		n = n*10 + int(r-'0')
	}
	return n
}

// ParseDateParts maps captured strings based on order to Y,M,D ints
func ParseDateParts(a, b, c, order string) (int, int, int) {
	switch order {
	case "YMD":
		return parseInt(a), parseInt(b), parseInt(c)
	case "MDY":
		return parseInt(c), parseInt(a), parseInt(b)
	case "DMY":
		return parseInt(c), parseInt(b), parseInt(a)
	default:
		return parseInt(a), parseInt(b), parseInt(c)
	}
}

func isValidDate(year, month, day int) bool {
	if month < 1 || month > 12 {
		return false
	}
	if day < 1 || day > 31 {
		return false
	}
	return true
}

func isFuture(y, m, d int, now time.Time, maxSkew string) bool {
	date := time.Date(y, time.Month(m), d, 0, 0, 0, 0, now.Location())
	dur, err := parseFlexibleDuration(maxSkew)
	if err != nil {
		dur = 0
	}
	return date.After(now.Add(dur))
}

// parseFlexibleDuration supports Go durations and day shorthand like "5d"
func parseFlexibleDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		// simple day suffix support
		n := strings.TrimSuffix(s, "d")
		if n == "" {
			return 0, fmt.Errorf("invalid days duration: %s", s)
		}
		// extract digits
		days := parseInt(n)
		if days <= 0 && n != "0" {
			return 0, fmt.Errorf("invalid days duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func findLineNumber(content, substr string) int {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, substr) {
			return i + 1
		}
	}
	return 0
}

func matchInclude(rel string, patterns []string) bool {
	rel = filepath.ToSlash(rel)
	for _, p := range patterns {
		p = filepath.ToSlash(p)
		if strings.ContainsAny(p, "*?[") {
			if ok, _ := doublestar.Match(p, rel); ok {
				return true
			}
		} else if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(rel+"/", p) {
				return true
			}
		} else {
			if filepath.Base(rel) == p {
				return true
			}
		}
	}
	return false
}

func matchExclude(rel string, patterns []string) bool {
	rel = filepath.ToSlash(rel)
	for _, p := range patterns {
		p = filepath.ToSlash(p)
		if strings.ContainsAny(p, "*?[") {
			if ok, _ := doublestar.Match(p, rel); ok {
				return true
			}
		} else if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(rel+"/", p) {
				return true
			}
		} else {
			if filepath.Base(rel) == p {
				return true
			}
		}
	}
	return false
}

// matchesAny returns true if rel matches any glob in globs
func matchesAny(rel string, globs []string) bool {
	rel = filepath.ToSlash(rel)
	for _, g := range globs {
		g = filepath.ToSlash(g)
		if strings.ContainsAny(g, "*?[") {
			if ok, _ := doublestar.Match(g, rel); ok {
				return true
			}
		} else if strings.HasSuffix(g, "/") {
			if strings.HasPrefix(rel+"/", g) {
				return true
			}
		} else {
			if filepath.Base(rel) == g {
				return true
			}
		}
	}
	return false
}

// extractDates collects all dates found by patterns and returns time instances
type patView struct {
	re    *regexp.Regexp
	order string
}

func extractDates(content string, pats []patView, loc *time.Location) []time.Time {
	var out []time.Time
	for _, cp := range pats {
		all := cp.re.FindAllStringSubmatch(content, -1)
		if len(all) == 0 {
			continue
		}
		for _, m := range all {
			if len(m) < 4 {
				continue
			}
			y, mo, d := ParseDateParts(m[1], m[2], m[3], cp.order)
			if !isValidDate(y, mo, d) {
				continue
			}
			out = append(out, time.Date(y, time.Month(mo), d, 0, 0, 0, 0, loc))
		}
	}
	return out
}

// isMonotonicDescending checks that the first date is the newest and dates are non-increasing
func isMonotonicDescending(dates []time.Time) bool {
	if len(dates) < 2 {
		return true
	}
	for i := 0; i < len(dates)-1; i++ {
		if dates[i].Before(dates[i+1]) {
			return false
		}
	}
	return true
}

// ChangelogEntry represents a parsed changelog heading
type ChangelogEntry struct {
	Line      string
	Date      *time.Time
	Version   string
	IsRelease bool   // true for dated releases, false for Unreleased/In Development
	Text      string // The non-date text portion
}

// extractChangelogEntries parses H2 changelog headings and returns structured entries
// Handles: "## [v1.2.3] - YYYY-MM-DD", "## [Unreleased]", "## [0.1.6] - In Development"
func extractChangelogEntries(content string, loc *time.Location) []ChangelogEntry {
	lines := strings.Split(content, "\n")

	// More flexible regex that captures version/text and optional date
	// Supports both bracketed [v1.2.3] and unbracketed v1.2.3 formats
	dateRe := regexp.MustCompile(`^##\s+(?:\[([^\]]+)\]|([^\s-]+))(?:\s*-\s*(.+?))?\s*$`)

	var entries []ChangelogEntry
	for _, line := range lines {
		m := dateRe.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}

		// Handle both bracketed and unbracketed version formats
		version := ""
		if m[1] != "" {
			// Bracketed format: [v1.2.3]
			version = strings.TrimSpace(m[1])
		} else if m[2] != "" {
			// Unbracketed format: v1.2.3
			version = strings.TrimSpace(m[2])
		}

		dateText := ""
		if len(m) > 3 {
			dateText = strings.TrimSpace(m[3])
		}

		entry := ChangelogEntry{
			Line:    line,
			Version: version,
			Text:    dateText,
		}

		// Try to parse the date
		if dateText != "" {
			// Try ISO date format first
			if dateMatch := regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`).FindStringSubmatch(dateText); dateMatch != nil {
				y, mo, d := parseInt(dateMatch[1]), parseInt(dateMatch[2]), parseInt(dateMatch[3])
				if isValidDate(y, mo, d) {
					date := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, loc)
					entry.Date = &date
					entry.IsRelease = true
				}
			}
		}

		// Special handling for known non-date entries
		if strings.EqualFold(version, "unreleased") || strings.Contains(strings.ToLower(dateText), "development") {
			entry.IsRelease = false
		}

		entries = append(entries, entry)
	}
	return entries
}

// sampleDates formats up to k ISO dates from a slice for info output
func sampleDates(dts []time.Time, k int) string {
	if k <= 0 || len(dts) == 0 {
		return ""
	}
	if k > len(dts) {
		k = len(dts)
	}
	parts := make([]string, 0, k)
	for i := 0; i < k; i++ {
		parts = append(parts, dts[i].Format("2006-01-02"))
	}
	return strings.Join(parts, ", ")
}

// findRepoRoot finds the git repository root directory from a given path
func findRepoRoot(target string) (string, error) {
	// Get absolute path to start searching from
	absPath, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// If target is a file, start from its directory
	if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	// Walk up the directory tree looking for .git
	current := absPath
	for {
		gitDir := filepath.Join(current, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return current, nil
		}

		// Move up one directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding .git
			break
		}
		current = parent
	}

	return "", fmt.Errorf("no git repository found in %s or its parent directories", target)
}

// repoFirstCommitTime returns the earliest commit timestamp across all refs
func repoFirstCommitTime(repo *git.Repository) (time.Time, bool) {
	iter, err := repo.Log(&git.LogOptions{All: true})
	if err != nil {
		return time.Time{}, false
	}
	defer iter.Close()
	earliest := time.Time{}
	_ = iter.ForEach(func(c *object.Commit) error {
		t := c.Author.When
		if c.Committer.When.Before(t) {
			t = c.Committer.When
		}
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
		return nil
	})
	if earliest.IsZero() {
		return time.Time{}, false
	}
	return earliest, true
}
