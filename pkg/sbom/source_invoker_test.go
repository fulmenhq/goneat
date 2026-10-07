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
	writeSourceFixture(t, root, ".gitignore", "/generated/**\n**/sumpter\n!cmd/sumpter/\n*.mod\n*.sum\n")
	writeSourceFixture(t, root, "go.mod", "module example.com/scoped\nrequire github.com/bmatcuk/doublestar/v4 v4.10.0\n")
	writeSourceFixture(t, root, "cmd/sumpter/main.go", "package main\n")
	writeSourceFixture(t, root, ".syft.yaml", "exclude: [ './bin/**', './dist/**', './go.mod' ]\nselect-catalogers: [ '-go-module-file-cataloger', '-go-module-binary-cataloger' ]\n")
	t.Chdir(root)
	// Inventory policy may not leak through caller CWD, profiles, global config
	// selectors, cataloger selectors, or even an invalid ambient config path.
	t.Setenv("SYFT_EXCLUDE", "**")
	t.Setenv("SYFT_CONFIG", filepath.Join(root, "must-not-load.yaml"))
	t.Setenv("SYFT_PROFILE", "must-not-load")
	t.Setenv("SYFT_DEFAULT_CATALOGERS", "javascript-package-cataloger")
	t.Setenv("SYFT_SELECT_CATALOGERS", "-go-module-file-cataloger,-go-module-binary-cataloger")
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
		inventoryIdentities := make(map[string]map[string]bool)
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
				inventoryIdentities[control.name] = sourceTestPackageIdentities(document, format)
				rootDependencyPresent := false
				for identity := range inventoryIdentities[control.name] {
					fields := strings.Split(identity, "\x00")
					rootDependencyPresent = rootDependencyPresent || (fields[0] == "github.com/bmatcuk/doublestar/v4" && fields[1] == "v4.10.0")
				}
				if !rootDependencyPresent {
					t.Fatal("independently declared root require identity missing from inventory")
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
				if strings.Contains(string(result.SBOMContent), "goneat-source-collector-") || !strings.Contains(string(result.SBOMContent), "doublestar") || !strings.Contains(string(result.SBOMContent), "v4.10.0") {
					t.Fatal("isolated default catalogers lost root dependency identity or leaked collector state")
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
			document, err := decodeSourceJSON(result.SBOMContent)
			if err != nil {
				t.Fatal(err)
			}
			artifactIdentities := sourceTestPackageIdentities(document, format)
			if len(artifactIdentities) == 0 {
				t.Fatal("artifact default catalogers did not return package identities")
			}
			for identity := range artifactIdentities {
				for _, control := range []string{"named-file", "named-directory", "no-ignore"} {
					if !inventoryIdentities[control][identity] {
						t.Fatalf("%s did not retain artifact package identity %s", control, identity)
					}
				}
			}
			t.Logf("Syft %s explicit artifact %s packages=%d", version, format, result.PackageCount)
		})
	}
}

// Compare package identities, not counts or opaque per-document relationship IDs.
// Reference validation and provenance mapping separately preserve those IDs.
func sourceTestPackageIdentities(document map[string]any, format string) map[string]bool {
	identities := make(map[string]bool)
	values := sourceArray(document["components"])
	if format == "spdx-json" {
		values = sourceArray(document["packages"])
	}
	for _, value := range values {
		object := sourceObject(value)
		name, _ := object["name"].(string)
		version, _ := object["version"].(string)
		purl, _ := object["purl"].(string)
		if format == "spdx-json" {
			version, _ = object["versionInfo"].(string)
			for _, ref := range sourceArray(object["externalRefs"]) {
				external := sourceObject(ref)
				if external["referenceType"] == "purl" {
					purl, _ = external["referenceLocator"].(string)
				}
			}
		}
		if purl != "" {
			identities[name+"\x00"+version+"\x00"+purl] = true
		}
	}
	return identities
}

func TestSourceInvokerRejectsUnsupportedFormatBeforeCollection(t *testing.T) {
	invoker := &SyftInvoker{syftPath: filepath.Join(t.TempDir(), "must-not-run")}
	result, err := invoker.Generate(context.Background(), Config{TargetPath: t.TempDir(), Format: "syft-json", Stdout: true})
	if err == nil || result != nil || !strings.Contains(err.Error(), "unsupported SBOM format") {
		t.Fatalf("unsupported format not rejected before collector: result=%v err=%v", result, err)
	}
}
