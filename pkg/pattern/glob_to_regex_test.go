/*
Copyright © 2026 3 Leaps <info@3leaps.net>
*/
package pattern

import (
	"regexp"
	"testing"
)

func TestGlobToRegexp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{"simple_dir", "vendor", "vendor", false},
		{"dir_with_slash", "node_modules/", "node_modules/", false},
		{"glob_star_prefix", "*.egg-info", `[^/]*\.egg-info`, false},
		{"glob_star_suffix", "dist*", `dist[^/]*`, false},
		{"glob_double_star", "**/dist", `(.*/)?dist`, false},
		{"glob_question", "test?", `test[^/]`, false},
		{"dot_escaped", ".cache", `\.cache`, false},
		{"brackets_escaped", "build-[0-9]+", `build-\[0-9\]\+`, false},
		{"complex_glob", "**/target/", `(.*/)?target/`, false},
		{"empty", "", "", true},
		{"negation", "!vendor", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GlobToRegexp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("GlobToRegexp(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("GlobToRegexp(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGlobToRegexp_GeneratedRegexCompiles(t *testing.T) {
	patterns := []string{
		"vendor",
		"node_modules/",
		"*.egg-info",
		"**/dist",
		"test?",
		".cache",
		"build-*",
		"**/target/",
	}

	for _, p := range patterns {
		t.Run(p, func(t *testing.T) {
			regex, err := GlobToRegexp(p)
			if err != nil {
				t.Fatalf("GlobToRegexp(%q) error: %v", p, err)
			}
			_, err = regexp.Compile(regex)
			if err != nil {
				t.Errorf("generated regex %q does not compile: %v", regex, err)
			}
		})
	}
}

func TestGlobToRegexp_PathMatching(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		path        string
		shouldMatch bool
	}{
		{"glob_prefix_match", "*.egg-info", "foo.egg-info", true},
		{"glob_prefix_no_match", "*.egg-info", "foo-egg-info", false},
		{"glob_star_match", "dist/*", "dist/build", true},
		{"double_star_match_subdir", "**/dist", "foo/bar/dist", true},
		{"double_star_match_root", "**/dist", "dist", true},
		{"question_single_char", "test?", "test1", true},
		{"question_wrong_length", "test?", "test12", false},
		{"simple_dir_exact", "vendor", "vendor", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex, err := GlobToRegexp(tt.pattern)
			if err != nil {
				t.Fatalf("GlobToRegexp(%q) error: %v", tt.pattern, err)
			}
			re := regexp.MustCompile("^" + regex + "$")
			matched := re.MatchString(tt.path)
			if matched != tt.shouldMatch {
				t.Errorf("pattern %q regex %q on path %q: got %v, want %v",
					tt.pattern, regex, tt.path, matched, tt.shouldMatch)
			}
		})
	}
}

func TestToGosecExcludeRegex(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOK     bool
		wantReason string
	}{
		{"simple_dir", "vendor", true, ""},
		{"trailing_slash", "node_modules/", true, ""},
		{"glob_pattern", "*.egg-info", true, ""},
		{"double_star", "**/dist", true, ""},
		{"empty", "", false, ReasonEmptyPattern},
		{"whitespace_only", "   ", false, ReasonEmptyPattern},
		{"negation", "!vendor", false, ReasonNegationNotSupported},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex, ok, reason := ToGosecExcludeRegex(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ToGosecExcludeRegex(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !tt.wantOK && reason != tt.wantReason {
				t.Errorf("ToGosecExcludeRegex(%q) reason = %q, want %q", tt.input, reason, tt.wantReason)
			}
			if tt.wantOK {
				if _, err := regexp.Compile(regex); err != nil {
					t.Errorf("ToGosecExcludeRegex(%q) regex %q does not compile: %v", tt.input, regex, err)
				}
			}
		})
	}
}

func TestToGosecExcludeRegexes(t *testing.T) {
	tests := []struct {
		name          string
		inputs        []string
		wantCount     int
		wantSkipCount int
	}{
		{"all_valid", []string{"vendor", "node_modules", "*.egg-info"}, 3, 0},
		{"with_empty", []string{"vendor", "", "node_modules"}, 2, 1},
		{"with_negation", []string{"vendor", "!ignored", "node_modules"}, 2, 1},
		{"with_duplicate", []string{"vendor", "vendor/", "node_modules"}, 2, 1},
		{"all_invalid", []string{"", "  ", "!foo"}, 0, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regexes, skipCount, _ := ToGosecExcludeRegexes(tt.inputs)
			if len(regexes) != tt.wantCount {
				t.Errorf("ToGosecExcludeRegexes(%v) count = %d, want %d", tt.inputs, len(regexes), tt.wantCount)
			}
			if skipCount != tt.wantSkipCount {
				t.Errorf("ToGosecExcludeRegexes(%v) skipCount = %d, want %d", tt.inputs, skipCount, tt.wantSkipCount)
			}
		})
	}
}

func TestToGosecExcludeRegexDecisions(t *testing.T) {
	inputs := []string{"*.egg-info/", "!vendor", "vendor", "vendor/"}
	regexes, decisions := ToGosecExcludeRegexDecisions(inputs)

	if len(regexes) != 2 {
		t.Fatalf("expected 2 regexes, got %d (%v)", len(regexes), regexes)
	}

	if len(decisions) != len(inputs) {
		t.Fatalf("expected %d decisions, got %d", len(inputs), len(decisions))
	}

	if !decisions[0].Accepted || decisions[0].Raw != "*.egg-info/" || decisions[0].Normalized != "*.egg-info" {
		t.Errorf("unexpected decision[0]: %+v", decisions[0])
	}

	if decisions[1].Accepted || decisions[1].Reason != ReasonNegationNotSupported {
		t.Errorf("unexpected decision[1]: %+v", decisions[1])
	}

	if !decisions[2].Accepted || decisions[2].Regex != "vendor" {
		t.Errorf("unexpected decision[2]: %+v", decisions[2])
	}

	if decisions[3].Accepted || decisions[3].Reason != ReasonDuplicatePattern {
		t.Errorf("unexpected decision[3]: %+v", decisions[3])
	}
}
