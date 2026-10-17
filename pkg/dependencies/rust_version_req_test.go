package dependencies

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCargoVersionMatches(t *testing.T) {
	for _, tc := range []struct {
		req, version string
		want         bool
	}{
		// Caret (default) at each leading-zero level.
		{"^1.2.3", "1.9.0", true},
		{"1.2.3", "2.0.0", false},
		{"^0.2.3", "0.2.9", true},
		{"^0.2.3", "0.3.0", false},
		{"^0.0.3", "0.0.4", false},
		{"^1", "1.0.4+wasi-0.2.12", true}, // build metadata ignored (estate: wasip2)
		{"^0", "0.9.0", true},
		{"^0.0", "0.1.0", false},
		// Tilde, exact, comparisons and partial upper bounds.
		{"~1.2.3", "1.2.9", true},
		{"~1.2.3", "1.3.0", false},
		{"~1", "1.9.0", true},
		{"=1.2.3", "1.2.3", true},
		{"=1.2.3", "1.2.4", false},
		{"=1.2", "1.2.7", true},
		{"=0.8.62", "0.8.62", true},
		{">=0.48.0, <=0.61", "0.61.2", true}, // estate: winapi-util -> windows-sys
		{">=0.48.0, <=0.61", "0.62.0", false},
		{">1.2", "1.2.9", false},
		{">1.2", "1.3.0", true},
		{"<1.2.3", "1.2.2", true},
		{"<1.2.3", "1.2.3", false},
		// Wildcards and empty requirements.
		{"*", "3.4.5", true},
		{"1.*", "1.4.0", true},
		{"1.*", "2.0.0", false},
		{"1.2.x", "1.2.7", true},
		{"*", "0.1.0", true},
		// Pre-releases only match requirements naming one; bounds exclude them.
		{"^1.0.0", "1.1.0-alpha", false},
		{"^1.0.0-alpha", "1.0.0-beta", true},
		{"<2.0.0, ^1.0.0-rc.1", "2.0.0-alpha", false},
		{"*", "0.1.0-dev", false},
		{"^1.0.0-alpha", "1.1.0-alpha", false},      // different core: not admitted
		{"^1.0.0-alpha", "1.0.1", true},             // releases still match normally
		{">=1.0.0-alpha.2", "1.0.0-alpha.10", true}, // numeric identifier order
		{"<1.0.0-alpha.10", "1.0.0-alpha.2", true},
		{"<1.0.0-alpha.10", "1.0.0-alpha.11", false},
		{"=1.0.0-rc.1", "1.0.0-rc.1", true},
		{">=1.0.0-alpha, <2.0.0", "1.0.0-beta.3", true},
		{">=1.0.0-alpha, <2.0.0", "1.5.0-beta", false},
	} {
		got, err := cargoVersionMatches(tc.req, tc.version)
		if err != nil || got != tc.want {
			t.Errorf("cargoVersionMatches(%q, %q) = %t, %v; want %t", tc.req, tc.version, got, err, tc.want)
		}
	}
	for _, tc := range []struct{ req, version string }{
		{"^1.2.3.4", "1.2.3"},
		{">= one", "1.0.0"},
		{"^1.2-alpha", "1.2.0"},
		{"1.*-pre", "1.0.0"},
		{"^1", "not-a-version"},
		{"^1", "01.0.0"},
		{"", "1.0.0"},
	} {
		if _, err := cargoVersionMatches(tc.req, tc.version); err == nil {
			t.Errorf("cargoVersionMatches(%q, %q) accepted unsupported input", tc.req, tc.version)
		}
	}
}

// TestCargoVersionMatchesRustOracle checks the Go matcher against expected
// results produced by the Rust semver crate (1.0.28, the version krates and
// cargo-deny build against) for a combinatorial table of operators, partial
// and wildcard comparators, pre-releases and conjunctions. Regenerate with
// the semver-oracle program described in the file header when changing rules.
func TestCargoVersionMatchesRustOracle(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "cargo_semver_oracle.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cases, mismatches := 0, 0
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("malformed oracle line %q", line)
		}
		req, version, want := fields[0], fields[1], fields[2]
		got, err := cargoVersionMatches(req, version)
		result := fmt.Sprint(got)
		if err != nil {
			result = "ERR"
		}
		cases++
		if result != want {
			mismatches++
			if mismatches <= 20 {
				t.Errorf("cargoVersionMatches(%q, %q) = %s, Rust semver = %s", req, version, result, want)
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d of %d cases differ from Rust semver", mismatches, cases)
	}
	if cases < 4000 {
		t.Fatalf("oracle table unexpectedly small: %d cases", cases)
	}
}
