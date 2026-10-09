package dependencies

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

const denySecurityDiagnostic = `{"type":"diagnostic","fields":{"severity":"error","message":"fixture advisory","code":"vulnerability","advisory":{"id":"RUSTSEC-2026-0001","url":"https://example.org/advisory"},"labels":[{"span":"fixture span","message":"fixture label"}],"future":9007199254740993}}`

func denySecuritySummary(advisories, sources uint32) string {
	return fmt.Sprintf(`{"type":"summary","fields":{"advisories":{"errors":%d,"warnings":0,"notes":0,"helps":0},"sources":{"errors":%d,"warnings":0,"notes":0,"helps":0},"future":{"value":9007199254740993}}}`, advisories, sources)
}

func TestCargoDenySecurityExitSummaryMatrix(t *testing.T) {
	for _, version := range []string{"0.14.0", "0.14.1", "0.20.2", "1.0.0"} {
		for _, counts := range [][2]uint32{{0, 0}, {1, 0}, {0, 1}, {2, 3}, {4294967295, 4294967295}} {
			for _, exit := range []int{0, 1, 8, 9, 2, 4, 16, 126, -1} {
				t.Run(fmt.Sprintf("%s-advisories%d-sources%d-exit%d", version, counts[0], counts[1], exit), func(t *testing.T) {
					// Summary counts, not invented diagnostic/advisory identities,
					// explain completed policy outcomes on both exit contracts.
					report := parseCargoDenySecurityReport(strings.NewReader(denySecuritySummary(counts[0], counts[1])))
					expected := 0
					if counts[0] > 0 {
						expected |= 1
					}
					if counts[1] > 0 {
						expected |= 8
					}
					if version == "0.14.0" && expected != 0 {
						expected = 1
					}
					err := cargoDenySecurityCompletionError(report, version, exit)
					if (err == nil) != (exit == expected) || len(report.findings) != 0 {
						t.Fatalf("contract mismatch or invented advisory: %+v err=%v expected=%d", report, err, expected)
					}
				})
			}
		}
	}
}

func TestCargoDenySecurityReportFailuresRetainFindings(t *testing.T) {
	clean := denySecuritySummary(0, 0)
	for _, tt := range []struct {
		name, report string
		failure      bool
		findings     int
	}{
		{"empty", "", true, 0},
		{"plain usage", "error: invalid configuration", true, 0},
		{"empty object", "{}", true, 0},
		{"diagnostic without summary", denySecurityDiagnostic, true, 1},
		{"missing sources", `{"type":"summary","fields":{"advisories":{"errors":0,"warnings":0,"notes":0,"helps":0}}}`, true, 0},
		{"missing advisories", `{"type":"summary","fields":{"sources":{"errors":1,"warnings":0,"notes":0,"helps":0}}}`, true, 0},
		{"null sources", strings.Replace(clean, `"sources":{"errors":0,"warnings":0,"notes":0,"helps":0}`, `"sources":null`, 1), true, 0},
		{"missing counter", strings.Replace(clean, `"notes":0,`, "", 1), true, 0},
		{"fractional counter", strings.Replace(clean, `"errors":0`, `"errors":0.5`, 1), true, 0},
		{"negative counter", strings.Replace(clean, `"errors":0`, `"errors":-1`, 1), true, 0},
		{"oversized counter", strings.Replace(clean, `"errors":0`, `"errors":4294967296`, 1), true, 0},
		{"string counter", strings.Replace(clean, `"errors":0`, `"errors":"0"`, 1), true, 0},
		{"boolean counter", strings.Replace(clean, `"errors":0`, `"errors":false`, 1), true, 0},
		{"duplicate summary", denySecurityDiagnostic + "\n" + clean + "\n" + clean, true, 1},
		{"late diagnostic", clean + "\n" + denySecurityDiagnostic, true, 1},
		{"truncated after finding", denySecurityDiagnostic + "\n" + `{"type":"summary","fields":`, true, 1},
		{"malformed before valid evidence", "invalid\n" + denySecurityDiagnostic + "\n" + clean, true, 1},
		{"duplicate statistic", strings.Replace(clean, `"errors":0`, `"errors":0,"errors":0`, 1), true, 0},
		{"uninvoked bans", strings.Replace(clean, `"future":`, `"bans":{"errors":0,"warnings":0,"notes":0,"helps":0},"future":`, 1), true, 0},
		{"valid future records", `{"type":"future","fields":{"value":9007199254740993}}` + "\n" + clean, false, 0},
		{"uncoded general diagnostic", `{"type":"diagnostic","fields":{"severity":"note","message":"general note"}}` + "\n" + clean, false, 1},
		{"long record without Scanner limit", strings.Replace(denySecurityDiagnostic, `fixture advisory`, strings.Repeat("x", 100000), 1) + "\n" + denySecuritySummary(1, 0), false, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report := parseCargoDenySecurityReport(strings.NewReader(tt.report))
			if (report.err != nil) != tt.failure || len(report.findings) != tt.findings {
				t.Fatalf("wrong completion/evidence: findings=%d records=%d err=%v", len(report.findings), len(report.records), report.err)
			}
			if len(report.findings) > 0 && tt.name != "uncoded general diagnostic" && (report.findings[0].ID != "RUSTSEC-2026-0001" || len(report.findings[0].Labels) != 1) {
				t.Fatalf("existing mapping lost: %+v", report.findings)
			}
		})
	}
}

type denySecurityFaultReader struct {
	input io.Reader
	err   error
}

func (r *denySecurityFaultReader) Read(p []byte) (int, error) {
	n, err := r.input.Read(p)
	if errors.Is(err, io.EOF) {
		return n, r.err
	}
	return n, err
}

func TestCargoDenySecurityReadErrorAndUnknownPrecision(t *testing.T) {
	cause := errors.New("fixture reader failure")
	report := parseCargoDenySecurityReport(&denySecurityFaultReader{input: strings.NewReader(denySecurityDiagnostic + "\n" + denySecuritySummary(1, 0)), err: cause})
	if len(report.findings) != 1 || !errors.Is(report.err, cause) {
		t.Fatalf("read failure must retain finding: %+v", report)
	}
	encoded, err := json.Marshal(report.records)
	if err != nil || !strings.Contains(string(encoded), "9007199254740993") {
		t.Fatalf("unknown/numeric evidence changed: %s err=%v", encoded, err)
	}
	if cargoDenySecurityCompletionError(parseCargoDenySecurityReport(strings.NewReader(denySecuritySummary(0, 0))), "", 0) == nil {
		t.Fatal("unknown version cannot identify exit contract")
	}
}
