package assess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Warning evidence is additive security metadata, not a severity-policy change
// or a shared assessment-engine API change.
type securityToolWithWarnings interface {
	RunWithWarnings(context.Context) ([]Issue, map[string][]json.RawMessage, error)
}

type cargoAuditReport struct {
	vulnerabilities []cargoAuditVuln
	warnings        map[string][]json.RawMessage
	err             error
}

func parseCargoAuditReport(input io.Reader) cargoAuditReport {
	decoder := json.NewDecoder(input)
	fields := make(map[string]json.RawMessage)
	vulnerabilityFields := make(map[string]json.RawMessage)
	report := cargoAuditReport{warnings: make(map[string][]json.RawMessage)}
	var errs []error
	listSeen, warningsSeen := false, false
	parseErr := readSecurityReportObject(decoder, func(key string, decoder *json.Decoder) error {
		switch key {
		case "vulnerabilities":
			return readSecurityReportObject(decoder, func(key string, decoder *json.Decoder) error {
				if key != "list" {
					var raw json.RawMessage
					if err := decoder.Decode(&raw); err != nil {
						return err
					}
					vulnerabilityFields[key] = raw
					return nil
				}
				listSeen = true
				return readSecurityReportArray(decoder, func(raw json.RawMessage) {
					var vuln cargoAuditVuln
					if err := json.Unmarshal(raw, &vuln); err != nil || vuln.Advisory.ID == "" || vuln.Package.Name == "" {
						errs = append(errs, errors.New("cargo-audit report contains an invalid vulnerability"))
						return
					}
					report.vulnerabilities = append(report.vulnerabilities, vuln)
				})
			})
		case "warnings":
			warningsSeen = true
			return readSecurityReportObject(decoder, func(kind string, decoder *json.Decoder) error {
				report.warnings[kind] = []json.RawMessage{}
				return readSecurityReportArray(decoder, func(raw json.RawMessage) {
					// Keep every safely decoded warning intact, including future
					// fields/kinds and integers beyond floating-point precision.
					report.warnings[kind] = append(report.warnings[kind], raw)
					var warning struct {
						Kind    string `json:"kind"`
						Package struct {
							Name string `json:"name"`
						} `json:"package"`
					}
					if err := json.Unmarshal(raw, &warning); err != nil || warning.Kind != kind || warning.Package.Name == "" {
						errs = append(errs, errors.New("cargo-audit report contains an invalid warning"))
					}
				})
			})
		default:
			var raw json.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				return err
			}
			fields[key] = raw
			return nil
		}
	})
	errs = append(errs, parseErr)
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("cargo-audit report has trailing data")
		}
		errs = append(errs, err)
	}
	for _, key := range []string{"database", "lockfile", "settings"} {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(fields[key], &object); err != nil || object == nil {
			errs = append(errs, fmt.Errorf("cargo-audit report is missing a valid %s object", key))
			continue
		}
		switch key {
		case "database":
			errs = append(errs, requireSecurityReportCount(object["advisory-count"], "database advisory"))
			for _, field := range []string{"last-commit", "last-updated"} {
				var optional *string
				if raw, ok := object[field]; !ok || json.Unmarshal(raw, &optional) != nil {
					errs = append(errs, errors.New("cargo-audit database metadata is incomplete or invalid"))
				}
			}
		case "lockfile":
			errs = append(errs, requireSecurityReportCount(object["dependency-count"], "lockfile dependency"))
		case "settings":
			// CPU/OS settings changed from optional scalars to lists between
			// supported source endpoints. Preserve that version compatibility.
			for _, field := range []string{"target_arch", "target_os", "severity", "ignore", "informational_warnings"} {
				if _, ok := object[field]; !ok {
					errs = append(errs, errors.New("cargo-audit report settings are incomplete"))
					break
				}
			}
			for _, field := range []string{"ignore", "informational_warnings"} {
				var values []string
				if json.Unmarshal(object[field], &values) != nil || values == nil {
					errs = append(errs, errors.New("cargo-audit report warning/ignore settings are invalid"))
				}
			}
			for _, field := range []string{"target_arch", "target_os", "severity"} {
				var optional *string
				if json.Unmarshal(object[field], &optional) != nil {
					var values []string
					if field == "severity" || json.Unmarshal(object[field], &values) != nil || values == nil {
						errs = append(errs, errors.New("cargo-audit report target/severity settings are invalid"))
					}
				}
			}
		}
	}
	var found *bool
	var count *uint64
	if json.Unmarshal(vulnerabilityFields["found"], &found) != nil || found == nil || json.Unmarshal(vulnerabilityFields["count"], &count) != nil || count == nil || !listSeen {
		errs = append(errs, errors.New("cargo-audit is without a vulnerability report with complete found/count/list evidence"))
	} else if *count != uint64(len(report.vulnerabilities)) || *found != (len(report.vulnerabilities) > 0) {
		errs = append(errs, errors.New("cargo-audit report vulnerability counts do not match its parsed list"))
	}
	if !warningsSeen {
		errs = append(errs, errors.New("cargo-audit report warnings are missing"))
	}
	report.err = errors.Join(errs...)
	return report
}

func cargoAuditCompletionError(report cargoAuditReport, exitCode int, processErr error) error {
	if processErr != nil || report.err != nil {
		return errors.Join(report.err, processErr)
	}
	if exitCode == 0 {
		return nil
	}
	if exitCode != 1 {
		return fmt.Errorf("cargo audit execution exited %d", exitCode)
	}
	warnings := 0
	for _, entries := range report.warnings {
		warnings += len(entries)
	}
	if len(report.vulnerabilities) == 0 && warnings == 0 {
		return errors.New("cargo audit exit 1 is an unaccounted tool failure: no project advisories or warnings explain the denial")
	}
	// Denied warning identities and self-advisories cannot be inferred from
	// this project report. Retain all warnings, without changing severity policy.
	return nil
}
