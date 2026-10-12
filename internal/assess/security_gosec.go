package assess

import (
	"bytes"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"runtime/debug"
	"sort"
	"sync"

	"golang.org/x/mod/semver"
)

// The Go toolchain that compiled the packages writes export data newer than
// the reader linked into gosec. Only the version numbers are captured.
var (
	gosecExportDataSkewPattern = regexp.MustCompile(`export data version (\d+) is greater than maximum supported version (\d+)`)
	gosecVersionPattern        = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)
)

type gosecProcessingError struct {
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Error  string `json:"error"`
}

type gosecScanReport struct {
	issues       []Issue
	suppressions []Suppression
	complete     bool
	err          error
}

// Decode members independently so a truncated tail cannot erase a complete
// Issues member. Raw members preserve unknown fields and numeric precision.
func (r *SecurityAssessmentRunner) parseGosecScanReport(output []byte) gosecScanReport {
	decoder := json.NewDecoder(bytes.NewReader(output))
	fields := make(map[string]json.RawMessage)
	var errs []error
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return gosecScanReport{err: errors.New("gosec report is missing a JSON object")}
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			errs = append(errs, fmt.Errorf("gosec report member: %w", err))
			break
		}
		key, ok := token.(string)
		if !ok {
			errs = append(errs, errors.New("gosec report member is not named"))
			break
		}
		if _, exists := fields[key]; exists {
			var duplicate json.RawMessage
			if err := decoder.Decode(&duplicate); err != nil {
				errs = append(errs, err)
				break
			}
			errs = append(errs, errors.New("gosec report contains a duplicate member"))
			continue
		}
		if key == "Issues" || key == "Suppressions" {
			// Complete earlier entries survive a truncated later array entry.
			var entries []json.RawMessage
			token, arrayErr := decoder.Token()
			if arrayErr == nil && token == json.Delim('[') {
				for decoder.More() {
					var entry json.RawMessage
					if arrayErr = decoder.Decode(&entry); arrayErr != nil {
						break
					}
					entries = append(entries, entry)
				}
				if arrayErr == nil {
					token, arrayErr = decoder.Token()
					if arrayErr == nil && token != json.Delim(']') {
						arrayErr = errors.New("invalid report array end")
					}
				}
			} else if arrayErr == nil && token != nil {
				arrayErr = errors.New("invalid report array")
			}
			fields[key], _ = json.Marshal(entries)
			if arrayErr != nil {
				errs = append(errs, fmt.Errorf("gosec report %s: %w", key, arrayErr))
				break
			}
			continue
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			errs = append(errs, fmt.Errorf("gosec report member value: %w", err))
			break
		}
		fields[key] = raw
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		errs = append(errs, errors.New("gosec report is truncated"))
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		errs = append(errs, errors.New("gosec report has invalid trailing data"))
	}
	for _, key := range []string{"Issues", "Golang errors", "Stats", "GosecVersion"} {
		if _, ok := fields[key]; !ok {
			errs = append(errs, fmt.Errorf("gosec report is missing %s", key))
		}
	}
	// Map each complete entry independently. A malformed later entry must not
	// erase earlier findings or suppressed findings through whole-array decoding.
	var issues []Issue
	var suppressions []Suppression
	for _, key := range []string{"Issues", "Suppressions"} {
		if _, exists := fields[key]; !exists {
			continue
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(fields[key], &entries); err != nil {
			errs = append(errs, err)
			continue
		}
		for _, entry := range entries {
			var identity struct {
				File     string `json:"file"`
				RuleID   string `json:"rule_id"`
				Severity string `json:"severity"`
			}
			if err := json.Unmarshal(entry, &identity); err != nil || identity.File == "" || identity.RuleID == "" || (key == "Issues" && identity.Severity == "") {
				errs = append(errs, errors.New("gosec report has an invalid finding or suppression"))
				continue
			}
			encoded, _ := json.Marshal(map[string][]json.RawMessage{key: {entry}})
			mappedIssues, mappedSuppressions, mappingErr := r.parseGosecOutputWithSuppressions(encoded)
			issues = append(issues, mappedIssues...)
			suppressions = append(suppressions, mappedSuppressions...)
			if mappingErr != nil {
				errs = append(errs, errors.New("gosec report finding or suppression mapping failed"))
			}
		}
	}
	var stats map[string]json.RawMessage
	if err := json.Unmarshal(fields["Stats"], &stats); err != nil || stats == nil {
		errs = append(errs, errors.New("gosec report has invalid scan statistics"))
	} else {
		for _, key := range []string{"files", "lines", "nosec", "found"} {
			var value *int64
			if err := json.Unmarshal(stats[key], &value); err != nil || value == nil || *value < 0 {
				errs = append(errs, errors.New("gosec report has incomplete or invalid scan statistics"))
				break
			}
		}
	}
	var version string
	if err := json.Unmarshal(fields["GosecVersion"], &version); err != nil || version == "" {
		errs = append(errs, errors.New("gosec report has no scanner version"))
	}
	var processing map[string][]gosecProcessingError
	if err := json.Unmarshal(fields["Golang errors"], &processing); err != nil {
		errs = append(errs, errors.New("gosec report has invalid processing errors"))
	}
	reportErr := errors.Join(errs...)
	complete := reportErr == nil
	if len(processing) > 0 {
		// Error bodies may contain source or credentials; surface completion
		// failure without echoing raw processing-error text.
		reportErr = errors.Join(reportErr, fmt.Errorf("gosec reported processing errors in %d package(s)", len(processing)))
		if skew := newGosecExportDataSkewError(processing, version); skew != nil {
			reportErr = errors.Join(reportErr, skew)
		}
	}
	return gosecScanReport{issues: issues, suppressions: suppressions, complete: complete, err: reportErr}
}

// gosecExportDataSkewError reports that the active Go toolchain writes export data
// newer than the golang.org/x/tools reader linked into gosec, so no package was
// analyzed. The scan still fails.
type gosecExportDataSkewError struct {
	gosecVersion  string
	dataVersion   string
	maxVersion    string
	xtoolsVersion string
}

func (e *gosecExportDataSkewError) Error() string {
	built := ""
	if e.xtoolsVersion != "" {
		built = fmt.Sprintf(" (built with golang.org/x/tools %s)", e.xtoolsVersion)
	}
	return fmt.Sprintf("gosec %s%s cannot read export data version %s from the active Go toolchain (supports up to %s); install a gosec built with a newer golang.org/x/tools, or run with GOTOOLCHAIN set to the reference Go toolchain", e.gosecVersion, built, e.dataVersion, e.maxVersion)
}

func newGosecExportDataSkewError(processing map[string][]gosecProcessingError, version string) *gosecExportDataSkewError {
	packages := make([]string, 0, len(processing))
	for pkg := range processing {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	for _, pkg := range packages {
		for _, entry := range processing[pkg] {
			if m := gosecExportDataSkewPattern.FindStringSubmatch(entry.Error); m != nil {
				if !gosecVersionPattern.MatchString(version) {
					version = "unknown"
				}
				return &gosecExportDataSkewError{gosecVersion: version, dataVersion: m[1], maxVersion: m[2]}
			}
		}
	}
	return nil
}

// gosecXToolsVersion reads the golang.org/x/tools version recorded in a gosec
// binary's build information. It returns "" when the binary cannot be read.
func gosecXToolsVersion(binaryPath string) string {
	info, err := buildinfo.ReadFile(binaryPath)
	if err != nil {
		return ""
	}
	return xtoolsVersionFromBuildInfo(info)
}

// xtoolsVersionFromBuildInfo returns the golang.org/x/tools version from the
// main module or a dependency, following a replacement. It returns "" when
// the module is absent or the version is not a canonical module version.
func xtoolsVersionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return ""
	}
	modules := append([]*debug.Module{&info.Main}, info.Deps...)
	for _, m := range modules {
		if m == nil || m.Path != "golang.org/x/tools" {
			continue
		}
		if m.Replace != nil {
			m = m.Replace
		}
		if semver.Canonical(m.Version) != m.Version {
			return ""
		}
		return m.Version
	}
	return ""
}

// gosecXToolsVersionOnPath reads the gosec binary on PATH once per process.
var gosecXToolsVersionOnPath = sync.OnceValue(func() string {
	path, err := exec.LookPath("gosec")
	if err != nil {
		return ""
	}
	return gosecXToolsVersion(path)
})

// annotateGosecExportDataSkew adds the x/tools version of the gosec binary on
// PATH to the first export-data skew error in err. A shard reports at most one
// skew in practice: a skew report is complete, so it is not retried. Other
// errors are returned unchanged.
func annotateGosecExportDataSkew(err error, xtoolsVersion func() string) error {
	var skew *gosecExportDataSkewError
	if err == nil || !errors.As(err, &skew) || skew.xtoolsVersion != "" {
		return err
	}
	skew.xtoolsVersion = xtoolsVersion()
	return err
}

func gosecExecutionError(report gosecScanReport, processErr, contextErr error) error {
	if processErr != nil && contextErr == nil && report.complete && report.err == nil && len(report.issues) > 0 {
		var exitErr *exec.ExitError
		if errors.As(processErr, &exitErr) && exitErr.ExitCode() == 1 {
			// Only a complete issue-bearing report explains the documented
			// findings exit. An empty report or processing errors never do.
			processErr = nil
		}
	}
	return errors.Join(report.err, processErr, contextErr)
}

// Retried identical evidence is one finding, not additional vulnerabilities.
// Distinct findings/suppression justifications are retained in attempt order.
func uniqueGosecEvidence(issues []Issue, suppressions []Suppression) ([]Issue, []Suppression) {
	seen := make(map[string]bool)
	var uniqueIssues []Issue
	for _, issue := range issues {
		data, _ := json.Marshal(issue)
		key := string(data)
		if !seen[key] {
			seen[key] = true
			uniqueIssues = append(uniqueIssues, issue)
		}
	}
	seen = make(map[string]bool)
	var uniqueSuppressions []Suppression
	for _, suppression := range suppressions {
		data, err := json.Marshal(suppression)
		// If unforeseen metadata cannot be encoded, do not discard evidence.
		if err != nil {
			uniqueSuppressions = append(uniqueSuppressions, suppression)
			continue
		}
		key := string(data)
		if !seen[key] {
			seen[key] = true
			uniqueSuppressions = append(uniqueSuppressions, suppression)
		}
	}
	return uniqueIssues, uniqueSuppressions
}
