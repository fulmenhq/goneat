package dependencies

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// CargoDenySecurityResult is deliberately separate from the license/bans API.
// Partial trustworthy findings and exact JSON records remain available on error.
type CargoDenySecurityResult struct {
	*CargoDenyResult
	Version  string
	Complete bool
	Records  []json.RawMessage
	Summary  map[string]json.RawMessage
}

type cargoDenySecurityReport struct {
	findings       []CargoDenyFinding
	records        []json.RawMessage
	summary        map[string]json.RawMessage
	advisoryErrors uint32
	sourceErrors   uint32
	err            error
}

// RunCargoDenySecurity runs only the existing advisories/sources invocation.
// RunCargoDeny and all license/bans/default dependency consumers are unchanged.
func RunCargoDenySecurity(ctx context.Context, target string, timeout time.Duration) (*CargoDenySecurityResult, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if !IsCargoAvailable() {
		return nil, errors.New("cargo is not available")
	}
	project := DetectRustProject(target)
	if project == nil || project.CargoTomlPath == "" {
		return nil, nil
	}
	root := project.EffectiveRoot()
	if root == "" {
		root = target
	}
	// Version identifies the legacy exit contract, not a recommendation ceiling.
	versionCmd := exec.CommandContext(ctx, "cargo", "deny", "--version")
	versionCmd.Dir = root
	versionOutput, versionErr := versionCmd.Output()
	version := parseVersionFromOutput(string(versionOutput))
	if version == "" {
		versionErr = errors.Join(versionErr, errors.New("cargo-deny version could not be identified for its exit contract"))
	}
	if version != "" && compareVersions(version, CargoDenyMinVersion) < 0 {
		logger.Warn(fmt.Sprintf("cargo-deny %s below minimum %s; results may be unreliable", version, CargoDenyMinVersion))
	}
	start := time.Now()
	cmd := exec.CommandContext(ctx, "cargo", "deny", "--format", "json", "check", "advisories", "sources")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	processErr := cmd.Run()
	exitCode := 0
	var exitErr *exec.ExitError
	if processErr != nil {
		exitCode = -1
		if errors.As(processErr, &exitErr) {
			exitCode = exitErr.ExitCode()
			if exitCode >= 0 && ctx.Err() == nil {
				processErr = nil
			}
		}
	}
	report := parseCargoDenySecurityReport(bytes.NewReader(stderr.Bytes()))
	completionErr := errors.Join(versionErr, processErr, ctx.Err(), cargoDenySecurityCompletionError(report, version, exitCode))
	result := &CargoDenySecurityResult{
		CargoDenyResult: &CargoDenyResult{Findings: report.findings, RootPath: root, ReportFile: rustIssueFile(project), Duration: time.Since(start), Failed: exitCode != 0 || completionErr != nil, ExitCode: exitCode},
		Version:         version, Complete: completionErr == nil, Records: report.records, Summary: report.summary,
	}
	return result, completionErr
}

func parseCargoDenySecurityReport(input io.Reader) cargoDenySecurityReport {
	reader := bufio.NewReader(input)
	var report cargoDenySecurityReport
	var errs []error
	summaries := 0
	for {
		line, readErr := reader.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			fields, err := cargoDenySecurityJSONObject(line)
			if err != nil {
				errs = append(errs, fmt.Errorf("cargo-deny report JSON: %w", err))
			} else {
				// Copy because the reader's buffers and unknown values must not
				// become an approximate reconstructed report or float64 values.
				report.records = append(report.records, append(json.RawMessage(nil), line...))
				if summaries > 0 {
					errs = append(errs, errors.New("cargo-deny report has records after its final summary"))
				}
				var recordType string
				if json.Unmarshal(fields["type"], &recordType) != nil || recordType == "" {
					errs = append(errs, errors.New("cargo-deny report record has no valid type"))
				} else {
					switch recordType {
					case "diagnostic":
						var diagnostic cargoDenyFields
						// The upstream serializer omits a code for uncoded general
						// diagnostics; absence is not itself a malformed report.
						if json.Unmarshal(fields["fields"], &diagnostic) != nil || !cargoDenySecuritySeverity(diagnostic.Severity) {
							errs = append(errs, errors.New("cargo-deny report has an invalid diagnostic"))
						} else {
							report.findings = append(report.findings, mapCargoDenySecurityFinding(&diagnostic))
						}
					case "summary":
						summaries++
						if summaries == 1 {
							report.summary, err = cargoDenySecurityJSONObject(fields["fields"])
							if err != nil {
								errs = append(errs, fmt.Errorf("cargo-deny summary: %w", err))
							}
						}
					default:
						// Future valid records are retained, not invented findings.
					}
				}
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				errs = append(errs, fmt.Errorf("cargo-deny report reader: %w", readErr))
			}
			break
		}
	}
	if summaries != 1 {
		errs = append(errs, errors.New("cargo-deny report requires one completed summary"))
	}
	for _, check := range []string{"bans", "licenses"} {
		if _, ok := report.summary[check]; ok {
			errs = append(errs, errors.New("cargo-deny security summary contains an uninvoked check"))
		}
	}
	for _, check := range []string{"advisories", "sources"} {
		stats, err := cargoDenySecurityJSONObject(report.summary[check])
		if err != nil {
			errs = append(errs, fmt.Errorf("cargo-deny summary is missing completed %s statistics", check))
			continue
		}
		for _, counter := range []string{"errors", "warnings", "notes", "helps"} {
			var count *uint32
			if json.Unmarshal(stats[counter], &count) != nil || count == nil {
				errs = append(errs, fmt.Errorf("cargo-deny %s summary has missing or invalid u32 %s", check, counter))
				continue
			}
			if counter == "errors" {
				if check == "advisories" {
					report.advisoryErrors = *count
				} else {
					report.sourceErrors = *count
				}
			}
		}
	}
	report.err = errors.Join(errs...)
	return report
}

func cargoDenySecurityJSONObject(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, errors.New("expected report object")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fields, err
		}
		key, ok := token.(string)
		if !ok {
			return fields, errors.New("invalid report member")
		}
		if _, exists := fields[key]; exists {
			return fields, errors.New("duplicate report member")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return fields, err
		}
		fields[key] = value
	}
	if token, err := decoder.Token(); err != nil {
		return fields, err
	} else if token != json.Delim('}') {
		return fields, errors.New("incomplete report object")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fields, errors.New("invalid trailing report data")
	}
	return fields, nil
}

func cargoDenySecurityCompletionError(report cargoDenySecurityReport, version string, exitCode int) error {
	if report.err != nil {
		return report.err
	}
	if version == "" {
		return errors.New("cargo-deny version is required to reconcile the observed exit contract")
	}
	expected := 0
	if report.advisoryErrors > 0 {
		expected |= 1
	}
	if report.sourceErrors > 0 {
		expected |= 8
	}
	if version == "0.14.0" && expected != 0 {
		expected = 1
	}
	if exitCode != expected {
		return fmt.Errorf("cargo-deny exited %d but completed advisory/source statistics require %d", exitCode, expected)
	}
	return nil
}

func cargoDenySecuritySeverity(severity string) bool {
	switch severity {
	case "error", "warning", "note", "help":
		return true
	default:
		return false
	}
}

// Reuse the existing code/type mapping, finding type, labels and presentation.
// This additive mapper does not change shared license/bans execution behavior.
func mapCargoDenySecurityFinding(fields *cargoDenyFields) CargoDenyFinding {
	finding := CargoDenyFinding{Type: mapCodeToType(fields.Code), Severity: fields.Severity, Message: strings.TrimSpace(fields.Message), Code: fields.Code}
	for _, label := range fields.Labels {
		finding.Labels = append(finding.Labels, CargoDenyLabel(label))
	}
	if finding.Message == "" && len(fields.Labels) > 0 {
		finding.Message = fields.Labels[0].Message
	}
	if fields.Advisory != nil {
		finding.ID = fields.Advisory.ID
		finding.URL = fields.Advisory.URL
		if finding.Message == "" {
			finding.Message = fields.Advisory.Title
		}
		if finding.Severity == "" {
			finding.Severity = fields.Advisory.Severity
		}
	}
	if finding.Message == "" {
		finding.Message = "cargo-deny finding"
	}
	return finding
}
