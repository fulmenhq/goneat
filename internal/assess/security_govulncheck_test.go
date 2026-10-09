package assess

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

const govulnConfigFixture = `{"config":{"protocol_version":"v1.0.0","scanner_name":"fixture"}}`
const govulnFindingFixture = `{"finding":{"osv":"GO-2026-0001","trace":[{"module":"example.org/module","package":"example.org/module/pkg","function":"Call"}]}}`

func TestGovulnStreamCompletion(t *testing.T) {
	for _, tt := range []struct {
		name     string
		stream   string
		findings int
		failure  bool
	}{
		{"clean", govulnConfigFixture, 0, false},
		{"finding exit zero report", govulnConfigFixture + "\n" + govulnFindingFixture, 1, false},
		{"pretty printed events", "{\n\"config\":{\n\"protocol_version\":\"v1.0.0\"}}\n{\n\"progress\":{\"message\":\"scan\"}}\n" + govulnFindingFixture, 1, false},
		{"known and future events", govulnConfigFixture + `{"osv":{"id":"GO-2026-0001"}}{"SBOM":{"modules":[]}}{"future":{"value":9007199254740993}}`, 0, false},
		{"absent report", "", 0, true},
		{"invalid report", "not JSON", 0, true},
		{"truncated after finding", govulnConfigFixture + govulnFindingFixture + `{"progress":`, 1, true},
		{"malformed after finding", govulnConfigFixture + govulnFindingFixture + "invalid", 1, true},
		{"missing configuration retains finding", govulnFindingFixture, 1, true},
		{"invalid finding", govulnConfigFixture + `{"finding":{"osv":"GO-2026-0001"}}`, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := parseGovulnStream("/fixture", strings.NewReader(tt.stream))
			if (err != nil) != tt.failure || len(issues) != tt.findings {
				t.Fatalf("findings or completion wrong: issues=%+v err=%v", issues, err)
			}
			if len(issues) > 0 && !strings.Contains(issues[0].Message, "GO-2026-0001 in example.org/module (example.org/module/pkg)") {
				t.Fatalf("real protocol finding lost identity: %+v", issues[0])
			}
		})
	}
}

const govulnScopedConfigFixture = `{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scanner_version":"v0.0.0","go_version":"go1.26.6","scan_mode":"source","scan_level":"symbol"}}`
const govulnInventoryFixture = `{"SBOM":{"go_version":"go1.26.6","roots":["example.org/root"],"modules":[{"path":"golang.org/x/crypto","version":"v0.56.0"}]}}`
const govulnModuleInventoryFixture = `{"finding":{"osv":"GO-2026-5932","trace":[{"module":"golang.org/x/crypto","version":"v0.56.0"}]}}`

func govulnPackageWideFixture(t *testing.T) string {
	t.Helper()
	imports := make([]map[string]interface{}, 0, len(govulnLegacyPackages))
	for _, path := range govulnLegacyPackages {
		imports = append(imports, map[string]interface{}{"path": path})
	}
	value := map[string]interface{}{"osv": map[string]interface{}{
		"id": govulnInventoryAdvisory,
		"affected": []interface{}{map[string]interface{}{
			"package":            map[string]string{"name": "golang.org/x/crypto", "ecosystem": "Go"},
			"ranges":             []interface{}{map[string]interface{}{"type": "SEMVER", "events": []map[string]string{{"introduced": "0"}}}},
			"ecosystem_specific": map[string]interface{}{"imports": imports},
		}},
	}}
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func govulnInventoryStream(t *testing.T, finding string) string {
	t.Helper()
	return govulnScopedConfigFixture + govulnInventoryFixture + govulnPackageWideFixture(t) + finding
}

func govulnDecodeAndFinish(t *testing.T, stream string, completion error) ([]Issue, map[string]interface{}, error) {
	t.Helper()
	r, err := readGovulnStream("/fixture", strings.NewReader(stream))
	return r.finish(errors.Join(err, completion))
}

func TestGovulnModuleInventoryRetained(t *testing.T) {
	issues, metadata, err := govulnDecodeAndFinish(t, govulnInventoryStream(t, govulnModuleInventoryFixture), nil)
	if err != nil || len(issues) != 1 || issues[0].Severity != SeverityInfo || issues[0].File != filepath.Join("/fixture", "go.mod") {
		t.Fatalf("inventory: %+v %v", issues, err)
	}
	rows := metadata["findings"].([]govulnObservation)
	if len(rows) != 1 || rows[0].EventIndex != 3 || rows[0].IssueIndex != 0 || rows[0].Classification != "inventory" || metadata["execution_complete"] != true {
		t.Fatalf("provenance: %+v", metadata)
	}
}

func TestGovulnImportedPackageRemainsHigh(t *testing.T) {
	testGovulnPositiveGrade(t, "package", `,"package":"golang.org/x/crypto/openpgp"`)
}
func TestGovulnCalledSymbolRemainsHigh(t *testing.T) {
	testGovulnPositiveGrade(t, "symbol", `,"package":"golang.org/x/crypto/openpgp","function":"init","position":{"filename":"openpgp.go","line":1},"receiver":""`)
}
func TestGovulnPackageWideAdvisoryImportRemainsHigh(t *testing.T) {
	TestGovulnImportedPackageRemainsHigh(t)
}
func testGovulnPositiveGrade(t *testing.T, grade, fields string) {
	t.Helper()
	finding := strings.Replace(govulnModuleInventoryFixture, `"version":"v0.56.0"`, `"version":"v0.56.0"`+fields, 1)
	issues, metadata, err := govulnDecodeAndFinish(t, govulnInventoryStream(t, finding), nil)
	if err != nil || len(issues) != 1 || issues[0].Severity != SeverityHigh {
		t.Fatalf("positive: %+v %v", issues, err)
	}
	row := metadata["findings"].([]govulnObservation)[0]
	if row.Grade != grade || row.Classification != "actionable" || !strings.Contains(string(row.Finding), fields) {
		t.Fatalf("trace lost: %+v", row)
	}
}

func TestGovulnNoFixedVersionIsValidAdvisory(t *testing.T) {
	issues, _, err := govulnDecodeAndFinish(t, govulnInventoryStream(t, govulnModuleInventoryFixture), nil)
	if err != nil || len(issues) != 1 || issues[0].Severity != SeverityInfo {
		t.Fatalf("no fix is valid inventory: %+v %v", issues, err)
	}
}

func TestGovulnIncrementalGradesRetainAllFindings(t *testing.T) {
	pack := strings.Replace(govulnModuleInventoryFixture, `"version":"v0.56.0"`, `"version":"v0.56.0","package":"golang.org/x/crypto/openpgp"`, 1)
	sym := strings.Replace(pack, `"package":"golang.org/x/crypto/openpgp"`, `"package":"golang.org/x/crypto/openpgp","function":"init"`, 1)
	issues, metadata, err := govulnDecodeAndFinish(t, govulnInventoryStream(t, govulnModuleInventoryFixture+pack+sym+sym), nil)
	if err != nil || len(issues) != 4 {
		t.Fatalf("incremental: %+v %v", issues, err)
	}
	for i, issue := range issues {
		want := SeverityHigh
		if i == 0 {
			want = SeverityInfo
		}
		if issue.Severity != want {
			t.Fatalf("grade%d: %+v", i, issue)
		}
	}
	rows := metadata["findings"].([]govulnObservation)
	for i, row := range rows {
		if row.IssueIndex != i || row.EventIndex != i+3 {
			t.Fatalf("order: %+v", rows)
		}
	}
}

func TestGovulnModuleOnlyScopeIsNotPackageAbsence(t *testing.T) {
	stream := strings.Replace(govulnInventoryStream(t, govulnModuleInventoryFixture), `"scan_level":"symbol"`, `"scan_level":"module"`, 1)
	issues, _, err := govulnDecodeAndFinish(t, stream, nil)
	if err == nil || len(issues) != 1 || issues[0].Severity != SeverityHigh {
		t.Fatalf("module scope: %+v %v", issues, err)
	}
}

func TestGovulnUnknownScopeRetainsHighAndErrors(t *testing.T) {
	base := govulnInventoryStream(t, govulnModuleInventoryFixture)
	for _, tt := range []struct{ name, from, to string }{
		{"missing_scope", `,"scan_level":"symbol"`, ""}, {"future_scope", `"scan_level":"symbol"`, `"scan_level":"future"`},
		{"binary", `"scan_mode":"source"`, `"scan_mode":"binary"`}, {"unproven_version", `"scanner_version":"v0.0.0"`, `"scanner_version":"v9.0.0"`},
		{"wrong_producer", `"scanner_name":"govulncheck"`, `"scanner_name":"unknown"`}, {"missing_go", `,"go_version":"go1.26.6"`, ""},
		{"missing_advisory", govulnPackageWideFixture(t), ""}, {"missing_inventory", govulnInventoryFixture, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			issues, metadata, err := govulnDecodeAndFinish(t, strings.Replace(base, tt.from, tt.to, 1), nil)
			if err == nil || len(issues) != 1 || issues[0].Severity != SeverityHigh || metadata["classification_error"] != true {
				t.Fatalf("unknown became inventory: %+v %+v %v", issues, metadata, err)
			}
		})
	}
}

func TestGovulnIncompleteModuleReportRetainsFindings(t *testing.T) {
	for _, tail := range []string{`{"progress":`, "not JSON"} {
		issues, _, err := govulnDecodeAndFinish(t, govulnInventoryStream(t, govulnModuleInventoryFixture)+tail, nil)
		if err == nil || len(issues) != 1 || issues[0].Severity != SeverityHigh {
			t.Fatalf("partial: %+v %v", issues, err)
		}
	}
	cause := errors.New("child completion failed")
	issues, _, err := govulnDecodeAndFinish(t, govulnInventoryStream(t, govulnModuleInventoryFixture), cause)
	if !errors.Is(err, cause) || issues[0].Severity != SeverityHigh {
		t.Fatalf("failed child: %+v %v", issues, err)
	}
}

func TestGovulnContradictoryFindingScopeFails(t *testing.T) {
	stream := govulnInventoryStream(t, govulnModuleInventoryFixture)
	for _, tail := range []string{govulnScopedConfigFixture, `{"future":{"level":"unknown"}}`, strings.Replace(govulnModuleInventoryFixture, `"version":"v0.56.0"`, `"version":"wrong"`, 1)} {
		issues, _, err := govulnDecodeAndFinish(t, stream+tail, nil)
		if err == nil || len(issues) == 0 || issues[0].Severity != SeverityHigh {
			t.Fatalf("contradiction: %+v %v", issues, err)
		}
	}
}

func TestGovulnMalformedTraceRetainsPriorFindings(t *testing.T) {
	for _, bad := range []string{`{"finding":{"osv":"GO-2026-5932","trace":[]}}`, `{"finding":{"osv":"GO-2026-5932","trace":[{"module":"golang.org/x/crypto","package":null}]}}`, `{"finding":{"osv":"GO-2026-5932","trace":[{"module":"golang.org/x/crypto","package":"p","position":{"line":"wrong"}}]}}`} {
		issues, _, err := govulnDecodeAndFinish(t, govulnInventoryStream(t, govulnModuleInventoryFixture)+bad, nil)
		if err == nil || len(issues) != 1 || issues[0].Severity != SeverityHigh {
			t.Fatalf("invalid trace: %+v %v", issues, err)
		}
	}
}

func TestGovulnUnknownFieldsAndNumbersPreserved(t *testing.T) {
	finding := strings.Replace(govulnModuleInventoryFixture, `"version":"v0.56.0"`, `"version":"v0.56.0","future":9007199254740993`, 1)
	osv := strings.Replace(govulnPackageWideFixture(t), `"id":"GO-2026-5932"`, `"id":"GO-2026-5932","future":9007199254740993`, 1)
	issues, metadata, err := govulnDecodeAndFinish(t, govulnScopedConfigFixture+govulnInventoryFixture+osv+finding, nil)
	if err != nil || len(issues) != 1 {
		t.Fatalf("future: %+v %v", issues, err)
	}
	b, e := json.Marshal(metadata)
	if e != nil || strings.Count(string(b), "9007199254740993") != 2 {
		t.Fatalf("precision lost: %s %v", b, e)
	}
}

func TestGovulnProducerIdentityIsReportedNotReconstructed(t *testing.T) {
	_, metadata, err := govulnDecodeAndFinish(t, govulnInventoryStream(t, govulnModuleInventoryFixture), nil)
	p := metadata["producer"].(govulnProducer)
	if err != nil || p.ScannerVersion != "v0.0.0" || p.GoVersion != "go1.26.6" || p.ScanMode != "source" || p.ScanLevel != "symbol" {
		t.Fatalf("reported identity: %+v %v", p, err)
	}
}

func TestGovulnPrivacyProvenanceNoSecretStderr(t *testing.T) {
	for _, tt := range []struct {
		name, fields string
		unknown      int
	}{
		{"known_private_db", `,"db":"https://user:secret@example.org","db_last_modified":"2026-01-01T00:00:00Z"`, 0},
		{"unknown_secret", `,"db":"https://user:secret@example.org","future_secret":"secret-value"`, 1},
		{"unknown_non_secret_scope", `,"future_scan_scope":"module"`, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stream := strings.Replace(govulnInventoryStream(t, govulnModuleInventoryFixture), `"protocol_version":"v1.0.0"`, `"protocol_version":"v1.0.0"`+tt.fields, 1)
			issues, metadata, err := govulnDecodeAndFinish(t, stream, nil)
			b, marshalErr := json.Marshal(metadata)
			unknown := tt.unknown != 0
			want := SeverityInfo
			if unknown {
				want = SeverityHigh
			}
			if (err != nil) != unknown || len(issues) != 1 || issues[0].Severity != want || marshalErr != nil || strings.Contains(string(b), "secret") || strings.Contains(string(b), "future_scan_scope") || metadata["unknown_config_fields"] != tt.unknown || metadata["classification_error"] != unknown || metadata["execution_complete"] == unknown {
				t.Fatalf("unknown scope or privacy incorrect: issues=%+v metadata=%s err=%v marshal=%v", issues, b, err, marshalErr)
			}
		})
	}
}

func TestGovulnOtherModuleAdvisoriesRemainHigh(t *testing.T) {
	stream := strings.ReplaceAll(govulnInventoryStream(t, govulnModuleInventoryFixture), govulnInventoryAdvisory, "GO-2026-0002")
	issues, _, err := govulnDecodeAndFinish(t, stream, nil)
	if err != nil || len(issues) != 1 || issues[0].Severity != SeverityHigh {
		t.Fatalf("blanket grading: %+v %v", issues, err)
	}
}

func TestGovulnAdvisoryDriftFailsClosed(t *testing.T) {
	base := govulnInventoryStream(t, govulnModuleInventoryFixture)
	for _, tt := range []struct{ from, to string }{{`"introduced":"0"`, `"fixed":"0.56.1"`}, {`"id":"GO-2026-5932"`, `"id":"GO-2026-5932","withdrawn":"2026-01-01T00:00:00Z"`}, {`"path":"golang.org/x/crypto/openpgp"`, `"path":"golang.org/x/crypto/openpgp","symbols":["Call"]`}} {
		issues, _, err := govulnDecodeAndFinish(t, strings.Replace(base, tt.from, tt.to, 1), nil)
		if err == nil || len(issues) != 1 || issues[0].Severity != SeverityHigh {
			t.Fatalf("advisory drift: %+v %v", issues, err)
		}
	}
}

func TestGovulnDuplicateFieldsFailClosed(t *testing.T) {
	stream := strings.Replace(govulnInventoryStream(t, govulnModuleInventoryFixture), `"scan_level":"symbol"`, `"scan_level":"module","scan_level":"symbol"`, 1)
	issues, _, err := govulnDecodeAndFinish(t, stream, nil)
	if err == nil || len(issues) != 1 || issues[0].Severity != SeverityHigh {
		t.Fatalf("duplicate field: %+v %v", issues, err)
	}
}

type securityFaultReader struct {
	input io.Reader
	err   error
}

func (r *securityFaultReader) Read(data []byte) (int, error) {
	n, err := r.input.Read(data)
	if errors.Is(err, io.EOF) {
		return n, r.err
	}
	return n, err
}

func TestGovulnStreamReadFailureRetainsFindings(t *testing.T) {
	cause := errors.New("fixture reader failed")
	reader := &securityFaultReader{input: strings.NewReader(govulnConfigFixture + govulnFindingFixture), err: cause}
	issues, err := parseGovulnStream("/fixture", reader)
	if len(issues) != 1 || !errors.Is(err, cause) {
		t.Fatalf("read failure must retain cause and prior finding: %+v err=%v", issues, err)
	}
}
