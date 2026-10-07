package sbom

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSourceRootGoEvidenceProtection(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod":        {Data: []byte("module example.com/root\n")},
		"go.sum":        {Data: []byte("retained sums\n")},
		"go.work":       {Data: []byte("go 1.26.6\nuse .\n")},
		"go.work.sum":   {Data: []byte("retained workspace sums\n")},
		".goneatignore": {Data: []byte("*.mod\n*.sum\ngo.work\ngo.work.sum\n")},
	}
	for _, tc := range []struct {
		name     string
		opts     SourceOptions
		want     []string
		conflict bool
	}{
		{"ignore-exceptions", SourceOptions{}, rootGoEvidenceNames, false},
		{"broad-configured-conflict", SourceOptions{ExcludePatterns: []string{"*.mod"}}, nil, true},
		{"literal-configured-conflict", SourceOptions{ExcludePatterns: []string{"go.sum"}}, nil, true},
		{"force-resolves-conflict", SourceOptions{ExcludePatterns: []string{"*.mod"}, ForceInclude: []string{"go.mod"}}, []string{"go.sum", "go.work", "go.work.sum"}, false},
		{"no-ignore-resolves-conflict", SourceOptions{NoIgnore: true, ExcludePatterns: []string{"*.mod"}}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection, err := planGoFixture(fsys, tc.opts)
			if tc.conflict {
				if err == nil || !strings.Contains(err.Error(), "configured pattern") || !strings.Contains(err.Error(), "--force-include") {
					t.Fatalf("missing actionable configured-evidence refusal: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(selection.protectedEvidence, tc.want) {
				t.Fatalf("protected=%v want=%v", selection.protectedEvidence, tc.want)
			}
			for _, name := range rootGoEvidenceNames {
				if !selection.selected[name] {
					t.Fatalf("root evidence suppressed: %s", name)
				}
			}
		})
	}
}

func TestSourceGoWorkspaceScope(t *testing.T) {
	for _, tc := range []struct {
		name      string
		files     map[string]string
		opts      SourceOptions
		errorPart string
	}{
		{"optional-absence", map[string]string{"go.mod": "module example.com/root\n"}, SourceOptions{}, ""},
		{"in-root-workspace", map[string]string{"go.work": "go 1.26.6\nuse ./member\n", "member/go.mod": "module example.com/member\n"}, SourceOptions{}, ""},
		{"excluded-workspace-member", map[string]string{"go.work": "go 1.26.6\nuse ./member\n", "member/go.mod": "module example.com/member\n", ".gitignore": "member/\n"}, SourceOptions{}, "required member evidence"},
		{"force-member", map[string]string{"go.work": "go 1.26.6\nuse ./member\n", "member/go.mod": "module example.com/member\n", ".gitignore": "member/\n"}, SourceOptions{ForceInclude: []string{"member/go.mod"}}, ""},
		{"excluded-member-sum", map[string]string{"go.work": "go 1.26.6\nuse ./member\n", "member/go.mod": "module example.com/member\n", "member/go.sum": "sum", ".gitignore": "*.sum\n"}, SourceOptions{}, "member evidence"},
		{"external-workspace", map[string]string{"go.work": "go 1.26.6\nuse ../outside\n"}, SourceOptions{}, "outside the captured subject"},
		{"missing-member", map[string]string{"go.work": "go 1.26.6\nuse ./missing\n"}, SourceOptions{}, "required member evidence"},
		{"local-replacement", map[string]string{"go.mod": "module example.com/root\nrequire example.com/local v1.0.0\nreplace example.com/local => ./local\n", "local/go.mod": "module example.com/local\n"}, SourceOptions{}, ""},
		{"external-replacement", map[string]string{"go.mod": "module example.com/root\nrequire example.com/local v1.0.0\nreplace example.com/local => ../outside\n"}, SourceOptions{NoIgnore: true}, "outside the captured subject"},
		{"excluded-replacement", map[string]string{"go.mod": "module example.com/root\nrequire example.com/local v1.0.0\nreplace example.com/local => ./local\n", "local/go.mod": "module example.com/local\n", ".goneatignore": "local/\n"}, SourceOptions{}, "required member evidence"},
		{"unused-replacement", map[string]string{"go.mod": "module example.com/root\nreplace example.com/unused => ../outside\n"}, SourceOptions{}, ""},
		{"workspace-replacement", map[string]string{"go.work": "go 1.26.6\nuse ./member\nreplace example.com/local => ./local\n", "member/go.mod": "module example.com/member\nrequire example.com/local v1.0.0\nreplace example.com/local => ../../outside\n", "local/go.mod": "module example.com/local\n"}, SourceOptions{}, ""},
		{"versioned-replacement-precedence", map[string]string{"go.mod": "module example.com/root\nrequire example.com/local v1.0.0\nreplace example.com/local v1.0.0 => ./local\nreplace example.com/local => ../outside\n", "local/go.mod": "module example.com/local\n"}, SourceOptions{}, ""},
		{"cyclic-local-references", map[string]string{"go.mod": "module example.com/root\nrequire example.com/local v1.0.0\nreplace example.com/local => ./local\n", "local/go.mod": "module example.com/local\nrequire example.com/root v1.0.0\nreplace example.com/root => ..\n"}, SourceOptions{}, ""},
		{"malformed-go-mod", map[string]string{"go.mod": "not a module\n"}, SourceOptions{}, "invalid Go evidence"},
		{"no-module-declaration", map[string]string{"go.mod": "go 1.26.6\n"}, SourceOptions{}, "no module declaration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys := make(fstest.MapFS)
			for name, data := range tc.files {
				fsys[name] = &fstest.MapFile{Data: []byte(data)}
			}
			_, err := planGoFixture(fsys, tc.opts)
			if tc.errorPart == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.errorPart) {
				t.Fatalf("expected %q, got %v", tc.errorPart, err)
			}
		})
	}
}

func planGoFixture(fsys fstest.MapFS, opts SourceOptions) (*sourceSelection, error) {
	patterns, err := loadSourcePatterns(fsys, opts)
	if err != nil {
		return nil, err
	}
	var entries []sourceEntry
	for name, file := range fsys {
		entries = append(entries, sourceEntry{name: name, directory: file.Mode.IsDir()})
	}
	selection, err := planSourceSelection(entries, patterns, opts)
	if err != nil {
		return nil, err
	}
	if err := protectSourceGoEvidence(fsys, selection, patterns, opts); err != nil {
		return nil, err
	}
	return selection, nil
}
