package policy

import (
	"strings"
	"testing"
	"time"
)

// parseCoolingYAML decodes YAML the same way the analyzers do, so an unquoted
// until keeps its literal text exactly as it does from .goneat/dependencies.yaml.
func parseCoolingYAML(t *testing.T, doc string) map[string]interface{} {
	t.Helper()
	var data map[string]interface{}
	if err := UnmarshalPolicyYAML([]byte(doc), &data); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	return data
}

func TestParseCoolingConfig_UnquotedUntilKeepsDate(t *testing.T) {
	data := parseCoolingYAML(t, `
cooling:
  enabled: true
  exceptions:
    - pattern: temp-package
      until: 2020-01-01
`)
	if _, ok := data["cooling"].(map[string]interface{})["exceptions"].([]interface{})[0].(map[string]interface{})["until"].(string); !ok {
		t.Fatal("test premise: an unquoted until should keep its literal text")
	}

	cfg, err := ParseCoolingConfig(data)
	if err != nil {
		t.Fatalf("ParseCoolingConfig: %v", err)
	}
	if len(cfg.Exceptions) != 1 {
		t.Fatalf("expected 1 exception, got %d", len(cfg.Exceptions))
	}
	// Dropping the date here would make the exception permanent.
	if got := cfg.Exceptions[0].Until; got != "2020-01-01" {
		t.Fatalf("until = %q, want 2020-01-01", got)
	}
}

func TestParseCoolingConfig_QuotedFutureUntilKept(t *testing.T) {
	data := parseCoolingYAML(t, `
cooling:
  enabled: true
  exceptions:
    - pattern: temp-package
      until: "2099-12-31"
`)
	cfg, err := ParseCoolingConfig(data)
	if err != nil {
		t.Fatalf("ParseCoolingConfig: %v", err)
	}
	if len(cfg.Exceptions) != 1 || cfg.Exceptions[0].Until != "2099-12-31" {
		t.Fatalf("exceptions = %+v", cfg.Exceptions)
	}
}

func TestParseCoolingConfig_InvalidUntilIsErrorAndNotExempting(t *testing.T) {
	cases := map[string]string{
		"garbage":        `until: "next tuesday"`,
		"wrong format":   `until: "01/02/2030"`,
		"unpadded":       `until: "2030-1-5"`,
		"explicit empty": `until: ""`,
		"integer":        `until: 20301231`,
		"boolean":        `until: true`,
		"list":           `until: ["2030-01-01"]`,
		"datetime":       `until: 2030-01-01T12:00:00Z`,
	}
	for name, untilLine := range cases {
		t.Run(name, func(t *testing.T) {
			data := parseCoolingYAML(t, `
cooling:
  enabled: true
  exceptions:
    - pattern: bad-until
      `+untilLine+`
    - pattern: good-one
      until: "2099-12-31"
`)
			cfg, err := ParseCoolingConfig(data)
			if err == nil {
				t.Fatal("expected a config error for an invalid until")
			}
			if !strings.Contains(err.Error(), "bad-until") {
				t.Errorf("error should name the pattern, got: %v", err)
			}
			if cfg == nil || !cfg.Enabled {
				t.Fatalf("config must still be returned so other checks run, got %+v", cfg)
			}
			for _, exc := range cfg.Exceptions {
				if exc.Pattern == "bad-until" {
					t.Fatalf("exception with invalid until must not be kept: %+v", exc)
				}
			}
			if len(cfg.Exceptions) != 1 || cfg.Exceptions[0].Pattern != "good-one" {
				t.Errorf("valid sibling exception should survive, got %+v", cfg.Exceptions)
			}
		})
	}
}

func TestParseCoolingConfig_NumericThresholdTypes(t *testing.T) {
	data := map[string]interface{}{
		"cooling": map[string]interface{}{
			"enabled":              true,
			"min_age_days":         int64(14),
			"min_downloads":        uint64(500),
			"min_downloads_recent": float64(50),
			"grace_period_days":    float64(2),
		},
	}
	cfg, err := ParseCoolingConfig(data)
	if err != nil {
		t.Fatalf("ParseCoolingConfig: %v", err)
	}
	if cfg.MinAgeDays != 14 || cfg.MinDownloads != 500 || cfg.MinDownloadsRecent != 50 || cfg.GracePeriodDays != 2 {
		t.Fatalf("thresholds not applied: %+v", cfg)
	}
}

func TestParseCoolingConfig_NumericThresholdFromYAML(t *testing.T) {
	// 1e2 resolves to float64 in YAML; a whole value must be accepted.
	data := parseCoolingYAML(t, `
cooling:
  enabled: true
  min_age_days: 1.4e1
  min_downloads: 250
`)
	cfg, err := ParseCoolingConfig(data)
	if err != nil {
		t.Fatalf("ParseCoolingConfig: %v", err)
	}
	if cfg.MinAgeDays != 14 || cfg.MinDownloads != 250 {
		t.Fatalf("thresholds not applied: %+v", cfg)
	}
}

func TestParseCoolingConfig_InvalidNumericThresholdIsError(t *testing.T) {
	cases := map[string]interface{}{
		"fractional float": 7.5,
		"string":           "7",
		"bool":             true,
		"list":             []interface{}{7},
		"uint64 overflow":  uint64(1 << 63),
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			data := map[string]interface{}{
				"cooling": map[string]interface{}{
					"enabled":      true,
					"min_age_days": value,
				},
			}
			cfg, err := ParseCoolingConfig(data)
			if err == nil {
				t.Fatalf("expected an error for min_age_days=%v", value)
			}
			if !strings.Contains(err.Error(), "min_age_days") {
				t.Errorf("error should name the field, got: %v", err)
			}
			if cfg == nil || cfg.MinAgeDays != 7 {
				t.Errorf("default must remain in place on error, got %+v", cfg)
			}
		})
	}
}

// An unquoted or quoted null until is a config error, never "no expiry".
func TestParseCoolingConfig_NullUntilIsErrorAndNotPermanent(t *testing.T) {
	cases := map[string]string{
		"null":      "until: null",
		"tilde":     "until: ~",
		"bare":      "until:",
		"null word": "until: Null",
	}
	for name, untilLine := range cases {
		t.Run(name, func(t *testing.T) {
			data := parseCoolingYAML(t, `
cooling:
  enabled: true
  exceptions:
    - pattern: null-until
      `+untilLine+`
`)
			cfg, err := ParseCoolingConfig(data)
			if err == nil {
				t.Fatal("a present null until must be a config error")
			}
			if cfg == nil || len(cfg.Exceptions) != 0 {
				t.Fatalf("the exception must be dropped, got %+v", cfg)
			}
		})
	}
}

// The parser must reject null even for a map that never went through YAML text.
func TestParseCoolingConfig_NullUntilFromMapIsError(t *testing.T) {
	data := map[string]interface{}{
		"cooling": map[string]interface{}{
			"enabled": true,
			"exceptions": []interface{}{
				map[string]interface{}{"pattern": "null-until", "until": nil},
			},
		},
	}
	cfg, err := ParseCoolingConfig(data)
	if err == nil || len(cfg.Exceptions) != 0 {
		t.Fatalf("expected an error and no exception, got %+v, %v", cfg, err)
	}
}

func TestParseCoolingConfig_AbsentUntilIsPermanent(t *testing.T) {
	data := parseCoolingYAML(t, `
cooling:
  enabled: true
  exceptions:
    - pattern: forever
`)
	cfg, err := ParseCoolingConfig(data)
	if err != nil {
		t.Fatalf("ParseCoolingConfig: %v", err)
	}
	if len(cfg.Exceptions) != 1 || cfg.Exceptions[0].Until != "" {
		t.Fatalf("an absent until is a permanent exception, got %+v", cfg.Exceptions)
	}
}

// Quoting must not change the outcome: spellings that are invalid when quoted
// are invalid when unquoted. YAML would otherwise turn them into time.Time.
func TestParseCoolingConfig_UnquotedUntilFollowsQuotedRules(t *testing.T) {
	invalid := []string{
		"2030-1-5",
		"2030-01-5",
		"2030-1-05",
		"2030-01-01T00:00:00Z",
		"2030-01-01t00:00:00z",
		"2030-01-01 00:00:00",
		"2030-01-01T01:00:00+01:00",
		"2030-01-01T12:00:00Z",
	}
	for _, until := range invalid {
		for _, quote := range []string{"", `"`} {
			t.Run(quote+until+quote, func(t *testing.T) {
				data := parseCoolingYAML(t, `
cooling:
  enabled: true
  exceptions:
    - pattern: bad-until
      until: `+quote+until+quote+`
`)
				cfg, err := ParseCoolingConfig(data)
				if err == nil {
					t.Fatal("expected a config error")
				}
				if cfg == nil || len(cfg.Exceptions) != 0 {
					t.Fatalf("the exception must not be kept, got %+v", cfg)
				}
			})
		}
	}
}

func TestParseCoolingConfig_UnquotedPaddedDateIsValid(t *testing.T) {
	data := parseCoolingYAML(t, `
cooling:
  enabled: true
  exceptions:
    - pattern: temp-package
      until: 2030-01-05
`)
	cfg, err := ParseCoolingConfig(data)
	if err != nil {
		t.Fatalf("ParseCoolingConfig: %v", err)
	}
	if len(cfg.Exceptions) != 1 || cfg.Exceptions[0].Until != "2030-01-05" {
		t.Fatalf("exceptions = %+v", cfg.Exceptions)
	}
}

// An until inherited through a YAML merge key gets the same literal handling.
func TestParseCoolingConfig_MergedUntilIsLiteral(t *testing.T) {
	data := parseCoolingYAML(t, `
defaults: &d
  until: 2030-1-5
cooling:
  enabled: true
  exceptions:
    - <<: *d
      pattern: merged
`)
	cfg, err := ParseCoolingConfig(data)
	if err == nil || len(cfg.Exceptions) != 0 {
		t.Fatalf("expected an error and no exception, got %+v, %v", cfg, err)
	}
}

func TestReadUntil_TimeValueMustBeUTCDate(t *testing.T) {
	utcDate := time.Date(2030, 1, 5, 0, 0, 0, 0, time.UTC)
	if got, err := readUntil(utcDate); err != nil || got != "2030-01-05" {
		t.Fatalf("UTC date: got %q, %v", got, err)
	}
	// Midnight UTC expressed in another zone has lost its spelling; reject it.
	offset := time.Date(2030, 1, 5, 1, 0, 0, 0, time.FixedZone("x", 3600))
	if _, err := readUntil(offset); err == nil {
		t.Fatal("a non-UTC time must be rejected")
	}
}

// Literal handling must also apply when cooling or its exceptions come in
// through YAML merge keys at the root or cooling level.
func TestUnmarshalPolicyYAML_MergeKeysAtRootAndCoolingLevel(t *testing.T) {
	cases := map[string]string{
		"root merge of cooling": `
base: &base
  cooling:
    enabled: true
    exceptions:
      - pattern: merged
        until: 2099-1-5
<<: *base
`,
		"root merge, rfc3339": `
base: &base
  cooling:
    enabled: true
    exceptions:
      - pattern: merged
        until: 2099-01-01T00:00:00Z
<<: *base
`,
		"cooling merge of exceptions": `
shared: &shared
  exceptions:
    - pattern: merged
      until: 2099-1-5
cooling:
  <<: *shared
  enabled: true
`,
		"merge list at root": `
a: &a
  other: 1
b: &b
  cooling:
    enabled: true
    exceptions:
      - pattern: merged
        until: 2099-1-5
<<: [*a, *b]
`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := ParseCoolingConfig(parseCoolingYAML(t, doc))
			if err == nil {
				t.Fatal("expected a config error for a non-canonical unquoted until")
			}
			if cfg == nil || len(cfg.Exceptions) != 0 {
				t.Fatalf("the exception must not be kept, got %+v", cfg)
			}
		})
	}
}

func TestUnmarshalPolicyYAML_MergePrecedenceAndValidDates(t *testing.T) {
	// Keys written in the mapping win over merged keys, as in plain YAML decoding.
	data := parseCoolingYAML(t, `
base: &base
  cooling:
    enabled: false
cooling:
  enabled: true
  exceptions:
    - pattern: kept
      until: 2099-01-05
<<: *base
`)
	cfg, err := ParseCoolingConfig(data)
	if err != nil {
		t.Fatalf("ParseCoolingConfig: %v", err)
	}
	if !cfg.Enabled || len(cfg.Exceptions) != 1 || cfg.Exceptions[0].Until != "2099-01-05" {
		t.Fatalf("explicit cooling must win and keep its valid date, got %+v", cfg)
	}
}

func TestUnmarshalPolicyYAML_CyclicMergeDoesNotHang(t *testing.T) {
	var data map[string]interface{}
	// yaml.v3 may reject this itself; either outcome is fine as long as it returns.
	_ = UnmarshalPolicyYAML([]byte("a: &a\n  <<: *a\ncooling:\n  enabled: true\n"), &data)
}
