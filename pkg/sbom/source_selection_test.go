package sbom

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSourcePatternGrammar(t *testing.T) {
	for _, tc := range []struct {
		pattern, name   string
		directory, want bool
	}{
		{"bin/**", "bin/stale", false, true},
		{"bin/**", "nested/bin/stale", false, false},
		{"**/bin/**", "nested/bin/stale", false, true},
		{"/bin/**", "bin/stale", false, true},
		{"/bin/**", "nested/bin/stale", false, false},
		{"bin/", "nested/bin/stale", false, true},
		{"bin/", "bin", false, false},
		{"bin/", "bin", true, true},
		{"/bin/", "nested/bin/stale", false, false},
		{"*.tmp", "a/thing.tmp", false, true},
		{"/thing.tmp", "a/thing.tmp", false, false},
		{"a/?.[ch]", "a/x.c", false, true},
		{"a/?.[ch]", "a/x.go", false, false},
		{`\#artifact`, "a/#artifact", false, true},
		{`\!artifact`, "!artifact", false, true},
		{`a/\*.tmp`, "a/*.tmp", false, true},
		{`a/\*.tmp`, "a/x.tmp", false, false},
		{`a/name\ `, "a/name ", false, true},
		{`\ name`, " name", false, true},
	} {
		t.Run(tc.pattern+"/"+tc.name, func(t *testing.T) {
			p, err := parseSourcePattern(tc.pattern)
			if err != nil || p == nil {
				t.Fatalf("parse: %v", err)
			}
			if got := p.excludes(sourceEntry{name: tc.name, directory: tc.directory}); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	for _, pattern := range []string{"!", "![broken", "!../escape", "!a/**b", "//server/path", "C:/bin/**", "../bin", "a/../b", "a//b", "a/**b", "***", "[broken", "{a,b}", `a\q`, "a\x00b"} {
		if _, err := parseSourcePattern(pattern); err == nil {
			t.Errorf("invalid pattern accepted: %q", pattern)
		}
	}
	for _, pattern := range []string{"", "  ", "# comment"} {
		p, err := parseSourcePattern(pattern)
		if err != nil || p != nil {
			t.Errorf("inert line parsed: %q", pattern)
		}
	}
}

func TestSourceSelectionNamedOnly(t *testing.T) {
	entries := []sourceEntry{
		{name: "bin", directory: true}, {name: "bin/named"}, {name: "bin/stale"},
		{name: "dist", directory: true}, {name: "dist/unrelated"}, {name: "custom.tmp"}, {name: "keep"},
	}
	fsys := fstest.MapFS{".goneatignore": {Data: []byte("*.tmp\n")}}
	for _, tc := range []struct {
		name string
		opts SourceOptions
		want []string
	}{
		{"default", SourceOptions{}, []string{"./bin/named", "./bin/stale", "./custom.tmp", "./dist/unrelated"}},
		{"named", SourceOptions{ForceInclude: []string{"bin/named"}}, []string{"./bin/stale", "./custom.tmp", "./dist/unrelated"}},
		{"directory", SourceOptions{ForceInclude: []string{"bin"}}, []string{"./custom.tmp", "./dist/unrelated"}},
		{"no-ignore", SourceOptions{NoIgnore: true}, nil},
		{"no-ignore-named", SourceOptions{NoIgnore: true, ForceInclude: []string{"bin/named"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patterns, err := loadSourcePatterns(fsys, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			got, err := compileSourceExcludes(entries, patterns, tc.opts)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v: %v", got, tc.want, err)
			}
		})
	}
	for _, force := range []string{"bin/missing", "../bin/named", "/bin/named", "C:/bin/named", "//host/share", "bin/*", ""} {
		if _, err := compileSourceExcludes(entries, nil, SourceOptions{ForceInclude: []string{force}}); err == nil {
			t.Errorf("invalid force accepted: %q", force)
		}
	}
	for _, entries := range [][]sourceEntry{{{name: "../escape"}}, {{name: "same"}, {name: "same"}}} {
		if _, err := compileSourceExcludes(entries, nil, SourceOptions{}); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
}

func TestSourceIgnoreDiagnostics(t *testing.T) {
	fsys := fstest.MapFS{".gitignore": {Data: []byte("# comment\nbin/\n![broken\n")}}
	_, err := loadSourcePatterns(fsys, SourceOptions{})
	if err == nil || !strings.Contains(err.Error(), ".gitignore:3") || !strings.Contains(err.Error(), "![broken") {
		t.Fatalf("missing actionable error: %v", err)
	}
	if _, err := loadSourcePatterns(fsys, SourceOptions{NoIgnore: true}); err != nil {
		t.Fatalf("no-ignore still applied ignored policy: %v", err)
	}
}

func TestSourceOrderedIgnoreNegations(t *testing.T) {
	for _, tc := range []struct {
		name, git, goneat, file string
		want                    bool
	}{
		{"reopened-source", "**/sumpter\n!cmd/sumpter/\n", "", "cmd/sumpter/main.go", true},
		{"reopened-dir-not-child", "**/sumpter\n!cmd/sumpter/\n", "", "cmd/sumpter/sumpter", false},
		{"unrelated-binary", "**/sumpter\n!cmd/sumpter/\n", "", "other/sumpter", false},
		{"dead-child", ".cursor/\n!.cursor/rules/\n!.cursor/rules/*\n", "", ".cursor/rules/x.md", false},
		{"reopened-parents", ".cursor/\n!.cursor/\n.cursor/*\n!.cursor/rules/\n!.cursor/rules/*\n", "", ".cursor/rules/x.md", true},
		{"later-parent-exclusion", ".cursor/\n!.cursor/\n.cursor/*\n!.cursor/rules/\n!.cursor/rules/*\n.cursor/rules/private/\n", "", ".cursor/rules/private/x.md", false},
		{"anchored-gitkeep", "/assets/*\n!/assets/.gitkeep\n", "", "assets/.gitkeep", true},
		{"anchored-sibling", "/assets/*\n!/assets/.gitkeep\n", "", "assets/data.txt", false},
		{"anchored-nested", "/assets/*\n!/assets/.gitkeep\n", "", "nested/assets/data.txt", true},
		{"later-reexclusion", "/assets/*\n!/assets/.gitkeep\n/assets/.gitkeep\n", "", "assets/.gitkeep", false},
		{"closed-example-parent", "private/\n!*.example.yaml\n!*.example.json\n", "", "private/settings.example.yaml", false},
		{"noop-negation", "!config/examples/\n!*.example.yaml\n!docs/.gitkeep\n", "", "config/examples/settings.example.yaml", true},
		{"basename-negation", "**/Dockerfile.dev\n**/Dockerfile.local\n!Dockerfile.*\n", "", "nested/Dockerfile.local", true},
		{"escaped-literal", "\\!literal\n", "", "!literal", false},
		{"later-policy-source", "*.tmp\n", "!keep.tmp\n", "keep.tmp", true},
		{"later-policy-exclusion", "*.tmp\n!keep.tmp\n", "keep.tmp\n", "keep.tmp", false},
		{"default-not-negatable", "!bin/\n!bin/**\n", "", "bin/goneat", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patterns, err := loadSourcePatterns(fstest.MapFS{
				".gitignore": {Data: []byte(tc.git)}, ".goneatignore": {Data: []byte(tc.goneat)},
			}, SourceOptions{})
			if err != nil {
				t.Fatal(err)
			}
			selection, err := planSourceSelection([]sourceEntry{{name: tc.file}}, patterns, SourceOptions{})
			if err != nil || selection.selected[tc.file] != tc.want {
				t.Fatalf("selection %q want=%v: %v", tc.file, tc.want, err)
			}
		})
	}
}

func TestSourceConfiguredExclusionsRemainHard(t *testing.T) {
	fsys := fstest.MapFS{".gitignore": {Data: []byte("!kept.tmp\n")}}
	opts := SourceOptions{ExcludePatterns: []string{"*.tmp"}}
	patterns, err := loadSourcePatterns(fsys, opts)
	if err != nil {
		t.Fatal(err)
	}
	entries := []sourceEntry{{name: "kept.tmp"}, {name: "other.tmp"}}
	for _, tc := range []struct {
		opts SourceOptions
		want []string
	}{
		{opts, []string{"./kept.tmp", "./other.tmp"}},
		{SourceOptions{ForceInclude: []string{"kept.tmp"}}, []string{"./other.tmp"}},
		{SourceOptions{NoIgnore: true}, nil},
	} {
		got, err := compileSourceExcludes(entries, patterns, tc.opts)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("got=%v want=%v err=%v", got, tc.want, err)
		}
	}
	if _, err := loadSourcePatterns(fsys, SourceOptions{ExcludePatterns: []string{"!kept.tmp"}}); err == nil || !strings.Contains(err.Error(), "configured:1") {
		t.Fatalf("configured negation accepted or lacks context: %v", err)
	}
}

func TestSourceLiteralExcludes(t *testing.T) {
	for _, name := range []string{"bin/a", "bin/[literal]", "bin/*.bin", "bin/?.bin", "space name", "#name", "!name", "é.txt"} {
		if _, err := literalSourceExclude(name); err != nil {
			t.Errorf("literal %q: %v", name, err)
		}
	}
	for _, name := range []string{"../escape", "bin\\ambiguous", "C:/escape", "bad\nname"} {
		if _, err := literalSourceExclude(name); err == nil {
			t.Errorf("inexact path accepted: %q", name)
		}
	}
}
