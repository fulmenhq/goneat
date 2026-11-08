package assess

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"golang.org/x/mod/semver"
)

// Findings remain HIGH unless explicitly allowed by repository policy after
// successful protocol and process completion.

// Calibrated govulncheck releases: v1.1.4 is the first release that emits the
// SBOM inventory event, and the protocol definition is unchanged through
// v1.9.0. Raise the ceiling only after checking a newer release's protocol.
const (
	govulnScannerVersionFloor   = "v1.1.4"
	govulnScannerVersionCeiling = "v1.10.0"
)

type govulnProducer struct {
	ProtocolVersion string `json:"protocol_version"`
	ScannerName     string `json:"scanner_name"`
	ScannerVersion  string `json:"scanner_version"`
	GoVersion       string `json:"go_version"`
	ScanMode        string `json:"scan_mode"`
	ScanLevel       string `json:"scan_level"`
}

type govulnFrame struct {
	Module   string `json:"module"`
	Version  string `json:"version"`
	Package  string `json:"package"`
	Function string `json:"function"`
	Receiver string `json:"receiver"`
	Position *struct {
		Filename string `json:"filename"`
		Offset   uint64 `json:"offset"`
		Line     uint64 `json:"line"`
		Column   uint64 `json:"column"`
	} `json:"position"`
}

type govulnFinding struct {
	OSV          string        `json:"osv"`
	FixedVersion string        `json:"fixed_version"`
	Trace        []govulnFrame `json:"trace"`
}

type govulnInventory struct {
	GoVersion string   `json:"go_version"`
	Roots     []string `json:"roots"`
	Modules   []struct {
		Path    string `json:"path"`
		Version string `json:"version"`
	} `json:"modules"`
}

type govulnObservation struct {
	EventIndex     int             `json:"event_index"`
	IssueIndex     int             `json:"issue_index"`
	Grade          string          `json:"observed_grade"`
	Classification string          `json:"classification"`
	Finding        json.RawMessage `json:"finding"`
	decoded        govulnFinding
}

type govulnReport struct {
	producer            govulnProducer
	issues              []Issue
	findings            []govulnObservation
	osvs                []json.RawMessage
	advisories          map[string]json.RawMessage
	events              int
	unknownEvents       int
	unknownConfigFields int
	configDigest        string
	inventory           govulnInventory
	sbomSeen            bool
	configured          bool
	complete            bool
}

// govulncheck emits consecutive JSON objects, often pretty-printed across
// multiple lines. A line scanner cannot implement this protocol.
func parseGovulnStream(root string, input io.Reader) ([]Issue, error) {
	report, err := readGovulnStream(root, input)
	issues, _, err := report.finish(err)
	return issues, err
}

func readGovulnStream(root string, input io.Reader) (*govulnReport, error) {
	decoder := json.NewDecoder(input)
	report := &govulnReport{advisories: make(map[string]json.RawMessage)}
	var protocolErrors []error
	for {
		var rawEvent json.RawMessage
		var event map[string]json.RawMessage
		err := decoder.Decode(&rawEvent)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return report, errors.Join(append(protocolErrors, fmt.Errorf("govulncheck JSON stream: %w", err))...)
		}
		report.events++
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			protocolErrors = append(protocolErrors, errors.New("govulncheck event must be an object"))
			continue
		}
		if err := validateGovulnJSON(rawEvent); err != nil {
			protocolErrors = append(protocolErrors, err)
		}
		if len(event) != 1 {
			protocolErrors = append(protocolErrors, errors.New("govulncheck event must contain one message field"))
		}
		if raw, ok := event["config"]; ok {
			var config govulnProducer
			if err := json.Unmarshal(raw, &config); err != nil || config.ProtocolVersion == "" || report.configured || report.events != 1 {
				protocolErrors = append(protocolErrors, errors.New("govulncheck stream has invalid initial configuration"))
			} else {
				report.configured = true
				report.producer = config
				digest := sha256.Sum256(raw)
				report.configDigest = fmt.Sprintf("%x", digest)
				var fields map[string]json.RawMessage
				_ = json.Unmarshal(raw, &fields)
				for key := range fields {
					switch key {
					case "protocol_version", "scanner_name", "scanner_version", "go_version", "scan_mode", "scan_level", "db", "db_last_modified":
					default:
						report.unknownConfigFields++
					}
				}
			}
		}
		if raw, ok := event["SBOM"]; ok {
			var sbom govulnInventory
			if err := json.Unmarshal(raw, &sbom); err != nil || report.sbomSeen {
				protocolErrors = append(protocolErrors, errors.New("govulncheck inventory is invalid or repeated"))
			} else {
				report.sbomSeen, report.inventory = true, sbom
			}
		}
		if raw, ok := event["osv"]; ok {
			var osv struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &osv); err != nil || osv.ID == "" {
				protocolErrors = append(protocolErrors, errors.New("govulncheck advisory has no valid identity"))
			} else {
				report.osvs = append(report.osvs, raw)
				if prior, exists := report.advisories[osv.ID]; exists && !bytes.Equal(prior, raw) {
					protocolErrors = append(protocolErrors, errors.New("govulncheck advisory is contradictory"))
				}
				report.advisories[osv.ID] = raw
			}
		}
		if raw, ok := event["finding"]; ok {
			issue, err := parseGovulnFinding(root, raw)
			if err != nil {
				protocolErrors = append(protocolErrors, err)
			} else if issue != nil {
				var finding govulnFinding
				_ = json.Unmarshal(raw, &finding)
				grade := "module"
				for _, frame := range finding.Trace {
					if frame.Function != "" || frame.Receiver != "" {
						grade = "symbol"
						break
					} else if frame.Package != "" {
						grade = "package"
					}
				}
				report.findings = append(report.findings, govulnObservation{EventIndex: report.events - 1, IssueIndex: len(report.issues), Grade: grade, Classification: "actionable_or_unverified", Finding: raw, decoded: finding})
				report.issues = append(report.issues, *issue)
			}
		}
		// Config, progress, OSV and SBOM are not findings. Unknown future valid
		// message objects are likewise not invented vulnerabilities.
		for key, raw := range event {
			switch key {
			case "config", "progress", "osv", "SBOM", "finding":
			default:
				report.unknownEvents++
			}
			if key != "finding" && !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
				protocolErrors = append(protocolErrors, errors.New("govulncheck message payload is not an object"))
			}
		}
	}
	if !report.configured {
		protocolErrors = append(protocolErrors, errors.New("govulncheck stream is missing its configuration message"))
	}
	err := errors.Join(protocolErrors...)
	report.complete = err == nil
	return report, err
}

// Token validation rejects duplicate object members at every depth while
// leaving unknown values and numbers untouched in the original RawMessages.
func validateGovulnJSON(raw json.RawMessage) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func() error
	value = func() error {
		token, err := d.Token()
		if err != nil {
			return errors.New("govulncheck message contains invalid JSON")
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]bool)
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return errors.New("govulncheck message contains invalid JSON")
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return errors.New("govulncheck message contains duplicate or invalid fields")
				}
				seen[name] = true
				if err := value(); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := value(); err != nil {
					return err
				}
			}
		default:
			return errors.New("govulncheck message contains invalid JSON")
		}
		_, err = d.Token()
		return err
	}
	return value()
}

func (r *govulnReport) finish(completionErr error) ([]Issue, map[string]interface{}, error) {
	return r.finishWithAllowlist(completionErr, nil)
}

func (r *govulnReport) finishWithAllowlist(completionErr error, allowed map[string]bool) ([]Issue, map[string]interface{}, error) {
	var classificationErrors []error
	for i := range r.findings {
		observation := &r.findings[i]
		finding := observation.decoded
		if !allowed[finding.OSV] {
			continue
		}
		if reason := r.govulnAllowlistUnsupported(finding); reason != "" {
			observation.Classification = "unknown"
			classificationErrors = append(classificationErrors, fmt.Errorf("govulncheck allowlist requires verified report evidence: %s", reason))
			continue
		}
		if completionErr != nil || !r.complete {
			observation.Classification = "incomplete"
			classificationErrors = append(classificationErrors, errors.New("govulncheck allowlist requires complete successful execution"))
			continue
		}
		observation.Classification = "allowlisted"
		r.issues[observation.IssueIndex].Severity = SeverityInfo
	}
	err := errors.Join(completionErr, errors.Join(classificationErrors...))
	// A contradictory event invalidates absence-based finalization for the
	// entire report, including module observations encountered before it.
	if err != nil {
		for i := range r.findings {
			if r.findings[i].Classification == "allowlisted" {
				r.findings[i].Classification = "incomplete"
				r.issues[r.findings[i].IssueIndex].Severity = SeverityHigh
			}
		}
	}
	// Offline bump hint: once the advisory's fix is a tagged release, the
	// allowlisted finding can be cleared by upgrading. No network lookup.
	for _, row := range r.findings {
		if row.Classification != "allowlisted" {
			continue
		}
		if fix, ok := govulnTaggedFix(row.decoded.FixedVersion); ok {
			module := row.decoded.Trace[0]
			r.issues = append(r.issues, Issue{
				File: r.issues[row.IssueIndex].File, Severity: SeverityInfo,
				Message:  fmt.Sprintf("govulncheck: %s is fixed in tagged release %s %s; upgrade from %s to clear this allowlisted finding", row.decoded.OSV, module.Module, fix, module.Version),
				Category: CategorySecurity, SubCategory: "dependency-update",
			})
		}
	}
	return r.issues, map[string]interface{}{
		"producer": r.producer, "config_sha256": r.configDigest,
		"inventory":             r.inventory,
		"unknown_config_fields": r.unknownConfigFields, "unknown_events": r.unknownEvents,
		"protocol_complete": r.complete, "execution_complete": err == nil,
		"classification_error": len(classificationErrors) != 0,
		"event_count":          r.events, "findings": r.findings, "advisories": r.osvs,
	}, err
}

// govulnAllowlistUnsupported guards policy application without inferring safety
// from module-only findings or baking an advisory identity into the scanner.
func (r *govulnReport) govulnAllowlistUnsupported(finding govulnFinding) string {
	p := r.producer
	switch {
	case p.ProtocolVersion != "v1.0.0":
		return fmt.Sprintf("protocol_version %q is not v1.0.0", p.ProtocolVersion)
	case p.ScannerName != "govulncheck":
		return fmt.Sprintf("scanner_name %q is not govulncheck", p.ScannerName)
	case !govulnScannerVersionCalibrated(p.ScannerVersion):
		return fmt.Sprintf("scanner_version %q is not v0.0.0 or a release at or above %s and below %s", p.ScannerVersion, govulnScannerVersionFloor, govulnScannerVersionCeiling)
	case p.ScanMode != "source":
		return fmt.Sprintf("scan_mode %q is not source", p.ScanMode)
	case p.ScanLevel != "symbol":
		return fmt.Sprintf("scan_level %q is not symbol", p.ScanLevel)
	case p.GoVersion == "":
		return "go_version is missing"
	case r.unknownEvents != 0:
		return fmt.Sprintf("%d unknown event(s)", r.unknownEvents)
	case r.unknownConfigFields != 0:
		return fmt.Sprintf("%d unknown config field(s)", r.unknownConfigFields)
	case !r.sbomSeen:
		return "SBOM inventory is missing"
	case r.inventory.GoVersion != p.GoVersion:
		return fmt.Sprintf("SBOM go_version %q does not match config go_version %q", r.inventory.GoVersion, p.GoVersion)
	case len(r.inventory.Roots) == 0:
		return "SBOM roots are missing"
	}
	for _, root := range r.inventory.Roots {
		if root == "" {
			return "SBOM contains an empty root"
		}
	}
	module := finding.Trace[0]
	if module.Version == "" {
		return fmt.Sprintf("finding module %q has no version", module.Module)
	}
	matched := false
	for _, m := range r.inventory.Modules {
		if m.Path == module.Module && m.Version == module.Version {
			matched = true
		}
	}
	if !matched {
		return fmt.Sprintf("finding module %q version %q is not in the SBOM inventory", module.Module, module.Version)
	}
	if len(r.advisories[finding.OSV]) == 0 {
		return "advisory evidence is missing"
	}
	return ""
}

// govulnTaggedFix returns the fixed version when it is a tagged release
// rather than a pseudo-version or prerelease.
func govulnTaggedFix(fixed string) (string, bool) {
	if fixed == "" || !semver.IsValid(fixed) || semver.Prerelease(fixed) != "" || semver.Build(fixed) != "" {
		return "", false
	}
	return fixed, true
}

// v0.0.0 is what an unstamped calibrated build reports. Stamped builds report
// their release version; only canonical releases in the calibrated range are
// accepted. Pseudo-versions and prereleases are not releases.
func govulnScannerVersionCalibrated(version string) bool {
	if version == "v0.0.0" {
		return true
	}
	return semver.Canonical(version) == version && semver.Prerelease(version) == "" && semver.Compare(version, govulnScannerVersionFloor) >= 0 && semver.Compare(version, govulnScannerVersionCeiling) < 0
}

func parseGovulnFinding(root string, raw json.RawMessage) (*Issue, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var finding govulnFinding
	if err := json.Unmarshal(raw, &finding); err != nil || finding.OSV == "" || len(finding.Trace) == 0 || finding.Trace[0].Module == "" {
		return nil, errors.New("govulncheck finding is missing a valid OSV or module trace")
	}
	for _, frame := range finding.Trace {
		if frame.Module == "" || (frame.Function != "" || frame.Receiver != "") && frame.Package == "" {
			return nil, errors.New("govulncheck finding contains an invalid trace frame")
		}
	}
	var fields struct {
		Trace []map[string]json.RawMessage `json:"trace"`
	}
	_ = json.Unmarshal(raw, &fields)
	for _, frame := range fields.Trace {
		for _, key := range []string{"module", "version", "package", "function", "receiver"} {
			if value, ok := frame[key]; ok && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, errors.New("govulncheck trace string is null")
			}
		}
	}
	return &Issue{
		File: filepath.Join(root, "go.mod"), Severity: SeverityHigh,
		Message:  fmt.Sprintf("govulncheck: %s in %s (%s)", finding.OSV, finding.Trace[0].Module, finding.Trace[0].Package),
		Category: CategorySecurity, SubCategory: "vulnerability",
	}, nil
}
