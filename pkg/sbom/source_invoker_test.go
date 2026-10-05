package sbom

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSourceInvokerFullSyftSelection(t *testing.T) {
	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Skipf("Syft unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	version, err := invoker.GetVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir() // Exercise the real /var alias -> canonical capture path.
	paths := []string{"bin/named", "bin/[sibling].bin", "dist/unrelated", "generated/root-ignored"}
	writeSourceFixture(t, root, ".gitignore", "/generated/**\n")
	for _, name := range paths {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		in, err := os.Open(invoker.syftPath)
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(path)
		if err != nil {
			_ = in.Close()
			t.Fatal(err)
		}
		_, copyErr := io.Copy(out, in)
		inErr, outErr := in.Close(), out.Close()
		if copyErr != nil || inErr != nil || outErr != nil {
			t.Fatalf("fixture copy: %v %v %v", copyErr, inErr, outErr)
		}
	}
	for _, format := range []string{"cyclonedx-json", "spdx-json"} {
		for _, control := range []struct {
			name    string
			options SourceOptions
			want    []bool
		}{
			{"defaults", SourceOptions{}, []bool{false, false, false, false}},
			{"no-ignore", SourceOptions{NoIgnore: true}, []bool{true, true, true, true}},
			{"named-file", SourceOptions{ForceInclude: []string{"bin/named"}}, []bool{true, false, false, false}},
			{"named-directory", SourceOptions{ForceInclude: []string{"bin"}}, []bool{true, true, false, false}},
		} {
			t.Run(format+"/"+control.name, func(t *testing.T) {
				result, err := invoker.Generate(ctx, Config{TargetPath: root, Format: format, Stdout: true, SourceOptions: control.options})
				if err != nil {
					t.Fatalf("Syft %s integrated generation: %v", version, err)
				}
				document, err := decodeSourceJSON(result.SBOMContent)
				if err != nil {
					t.Fatal(err)
				}
				files := make(map[string]bool)
				if format == "cyclonedx-json" {
					for _, value := range sourceArray(document["components"]) {
						component := sourceObject(value)
						if component["type"] == "file" {
							name, _ := component["name"].(string)
							files[filepath.ToSlash(name)] = true
						}
					}
				} else {
					for _, value := range sourceArray(document["files"]) {
						name, _ := sourceObject(value)["fileName"].(string)
						files[filepath.ToSlash(name)] = true
					}
				}
				for i, name := range paths {
					found := false
					for path := range files {
						found = found || path == name || strings.HasSuffix(path, "/"+name)
					}
					if found != control.want[i] {
						t.Fatalf("Syft %s %s: %s present=%v want=%v", version, format, name, found, control.want[i])
					}
					if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
						t.Fatal("original artifact removed: ", err)
					}
				}
				if strings.Contains(string(result.SBOMContent), "goneat-source-sbom-") || !strings.Contains(string(result.SBOMContent), "scoped-source-inventory") {
					t.Fatal("missing scope or residual staging provenance")
				}
				t.Logf("Syft %s full-cataloger %s/%s packages=%d files=%v", version, format, control.name, result.PackageCount, files)
			})
		}
		t.Run(format+"/explicit-artifact", func(t *testing.T) {
			result, err := invoker.Generate(ctx, Config{TargetPath: filepath.Join(root, "bin", "named"), Format: format, Stdout: true})
			if err != nil || result == nil {
				t.Fatalf("explicit ignored artifact: %v", err)
			}
			if result.PackageCount == 0 || strings.Contains(string(result.SBOMContent), "scoped-source-inventory") {
				t.Fatal("explicit artifact was excluded or mislabeled as source inventory")
			}
			t.Logf("Syft %s explicit artifact %s packages=%d", version, format, result.PackageCount)
		})
	}
}

func TestSourceInvokerRejectsUnsupportedFormatBeforeCollection(t *testing.T) {
	invoker := &SyftInvoker{syftPath: filepath.Join(t.TempDir(), "must-not-run")}
	result, err := invoker.Generate(context.Background(), Config{TargetPath: t.TempDir(), Format: "syft-json", Stdout: true})
	if err == nil || result != nil || !strings.Contains(err.Error(), "unsupported SBOM format") {
		t.Fatalf("unsupported format not rejected before collector: result=%v err=%v", result, err)
	}
}
