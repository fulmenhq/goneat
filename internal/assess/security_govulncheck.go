package assess

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
)

// Findings are provisional HIGH until both protocol and process completion are
// known. Only the calibrated package-wide advisory has inventory semantics.
const govulnInventoryAdvisory = "GO-2026-5932"

var govulnLegacyPackages = []string{
	"golang.org/x/crypto/openpgp", "golang.org/x/crypto/openpgp/packet",
	"golang.org/x/crypto/openpgp/armor", "golang.org/x/crypto/openpgp/clearsign",
	"golang.org/x/crypto/openpgp/errors", "golang.org/x/crypto/openpgp/elgamal",
	"golang.org/x/crypto/openpgp/s2k",
}

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
	var classificationErrors []error
	for i := range r.findings {
		observation := &r.findings[i]
		finding := observation.decoded
		if finding.OSV != govulnInventoryAdvisory {
			continue // No generic advisory grading or severity policy change.
		}
		p := r.producer
		// v0.0.0 is the reported version of the calibrated producer, not an
		// immutable process image identity. Never substitute a formula version.
		supported := p.ProtocolVersion == "v1.0.0" && p.ScannerName == "govulncheck" && p.ScannerVersion == "v0.0.0" && p.ScanMode == "source" && p.ScanLevel == "symbol" && p.GoVersion != "" && r.sbomSeen && r.inventory.GoVersion == p.GoVersion && len(r.inventory.Roots) != 0 && r.unknownEvents == 0 && r.unknownConfigFields == 0
		moduleMatched := false
		for _, module := range r.inventory.Modules {
			if module.Path == finding.Trace[0].Module && module.Version == finding.Trace[0].Version {
				moduleMatched = true
			}
		}
		for _, root := range r.inventory.Roots {
			if root == "" {
				supported = false
			}
		}
		supported = supported && moduleMatched
		if !supported || !validGovulnInventoryAdvisory(r.advisories[finding.OSV]) || finding.Trace[0].Module != "golang.org/x/crypto" || finding.Trace[0].Version == "" || finding.FixedVersion != "" {
			observation.Classification = "unknown"
			classificationErrors = append(classificationErrors, errors.New("govulncheck inventory classification requires verified source/symbol scope and package-wide advisory evidence"))
			continue
		}
		if observation.Grade == "module" {
			if completionErr != nil || !r.complete {
				observation.Classification = "incomplete"
				classificationErrors = append(classificationErrors, errors.New("govulncheck inventory classification requires complete successful execution"))
				continue
			}
			observation.Classification = "inventory"
			r.issues[observation.IssueIndex].Severity = SeverityInfo
		} else {
			observation.Classification = "actionable"
		}
	}
	err := errors.Join(completionErr, errors.Join(classificationErrors...))
	// A contradictory event invalidates absence-based finalization for the
	// entire report, including module observations encountered before it.
	if err != nil {
		for i := range r.findings {
			if r.findings[i].Classification == "inventory" {
				r.findings[i].Classification = "incomplete"
				r.issues[r.findings[i].IssueIndex].Severity = SeverityHigh
			}
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

func validGovulnInventoryAdvisory(raw json.RawMessage) bool {
	var osv struct {
		ID        string `json:"id"`
		Withdrawn string `json:"withdrawn"`
		Affected  []struct {
			Package struct {
				Name      string `json:"name"`
				Ecosystem string `json:"ecosystem"`
			} `json:"package"`
			Ranges []struct {
				Type   string              `json:"type"`
				Events []map[string]string `json:"events"`
			} `json:"ranges"`
			EcosystemSpecific struct {
				Imports []struct {
					Path    string   `json:"path"`
					Symbols []string `json:"symbols"`
					GOOS    []string `json:"goos"`
					GOARCH  []string `json:"goarch"`
				} `json:"imports"`
			} `json:"ecosystem_specific"`
		} `json:"affected"`
	}
	if json.Unmarshal(raw, &osv) != nil || osv.ID != govulnInventoryAdvisory || osv.Withdrawn != "" || len(osv.Affected) != 1 {
		return false
	}
	a := osv.Affected[0]
	if a.Package.Name != "golang.org/x/crypto" || a.Package.Ecosystem != "Go" || len(a.Ranges) != 1 || a.Ranges[0].Type != "SEMVER" || len(a.Ranges[0].Events) != 1 || len(a.Ranges[0].Events[0]) != 1 || a.Ranges[0].Events[0]["introduced"] != "0" || len(a.EcosystemSpecific.Imports) != len(govulnLegacyPackages) {
		return false
	}
	seen := make(map[string]bool)
	for _, imp := range a.EcosystemSpecific.Imports {
		if seen[imp.Path] || len(imp.Symbols) != 0 || len(imp.GOOS) != 0 || len(imp.GOARCH) != 0 {
			return false
		}
		seen[imp.Path] = true
	}
	for _, path := range govulnLegacyPackages {
		if !seen[path] {
			return false
		}
	}
	return true
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
