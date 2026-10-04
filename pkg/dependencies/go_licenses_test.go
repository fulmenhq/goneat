package dependencies

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/google/go-licenses/v2/licenses"
)

func TestLicensePackageCoverage(t *testing.T) {
	root := filepath.Join(t.TempDir(), "golang.org", "toolchain@go1.26.6")
	module := &goListModule{Path: "example.test/dependency"}
	base := []goListPackage{
		{ImportPath: "fmt", Standard: true, Goroot: true, Dir: filepath.Join(root, "src", "fmt")},
		{ImportPath: "unsafe", Standard: true, Goroot: true, Dir: filepath.Join(root, "src", "unsafe")},
		{ImportPath: "example.test/dependency/subpkg", Module: module, GoFiles: []string{"file.go"}},
	}
	ignored, expected, err := licensePackageCoverage(base, root)
	if err != nil || len(ignored) != 2 || len(expected) != 1 {
		t.Fatalf("toolchain-module stdlib coverage: ignored=%v expected=%v err=%v", ignored, expected, err)
	}
	if ignored, expected, err := licensePackageCoverage(base[2:], root); err != nil || ignored == nil || len(expected) != 1 {
		t.Fatalf("a proven graph with no stdlib imports is still complete: %v %v %v", ignored, expected, err)
	}
	for _, tc := range []struct {
		name string
		pkgs []goListPackage
	}{
		{"empty", nil},
		{"no_third_party", base[:2]},
		{"missing_module", append(append([]goListPackage{}, base...), goListPackage{ImportPath: "example.test/missing"})},
		{"prefix_collision", append(append([]goListPackage{}, base...), goListPackage{ImportPath: "fmtlocal.test/thirdparty", Module: module, GoFiles: []string{"file.go"}})},
		{"wrong_toolchain", []goListPackage{{ImportPath: "fmt", Standard: true, Goroot: true, Dir: filepath.Join(root+"-other", "src", "fmt")}, base[2]}},
		{"empty_standard_path", []goListPackage{{Standard: true, Goroot: true, Dir: filepath.Join(root, "src", "fmt")}, base[2]}},
		{"not_goroot", []goListPackage{{ImportPath: "fmt", Standard: true, Dir: filepath.Join(root, "src", "fmt")}, base[2]}},
		{"src_sibling", []goListPackage{{ImportPath: "fmt", Standard: true, Goroot: true, Dir: filepath.Join(root, "src2", "fmt")}, base[2]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ignored, _, err := licensePackageCoverage(tc.pkgs, root)
			if err == nil || ignored != nil {
				t.Fatalf("invalid inventory reached ignore API: ignored=%v err=%v", ignored, err)
			}
		})
	}
	withEmptyRoot := append(append([]goListPackage{}, base...), goListPackage{ImportPath: "example.test/tests-only", Module: module, TestGoFiles: []string{"file_test.go"}})
	if _, expected, err := licensePackageCoverage(withEmptyRoot, root); err != nil || len(expected) != 1 {
		t.Fatalf("test-only roots must match upstream includeTests=false: %v %v", expected, err)
	}
	withAssembly := append(append([]goListPackage{}, base...), goListPackage{ImportPath: "example.test/assembly", Module: module, SFiles: []string{"source.s"}})
	if _, expected, err := licensePackageCoverage(withAssembly, root); err != nil || len(expected) != 2 {
		t.Fatalf("non-Go source cannot be dropped: %v %v", expected, err)
	}
	withUnclassified := append(append([]goListPackage{}, base...), goListPackage{ImportPath: "example.test/unclassified", Module: module})
	if _, _, err := licensePackageCoverage(withUnclassified, root); err == nil {
		t.Fatal("unclassified source was excused as a test-only root")
	}
}

func TestDecodeLicensePackages(t *testing.T) {
	pkgs, err := decodeLicensePackages([]byte(`{"ImportPath":"fmt","Standard":true} {"ImportPath":"example.test/pkg"}`))
	if err != nil || len(pkgs) != 2 {
		t.Fatalf("decode: %v %v", pkgs, err)
	}
	if _, err := decodeLicensePackages([]byte(`{"ImportPath":"fmt"} garbage`)); err == nil {
		t.Fatal("malformed trailing inventory accepted")
	}
}

type failedLicenseClassifier struct{}

func (failedLicenseClassifier) Identify(string) ([]licenses.License, error) {
	return nil, errors.New("candidate read failed")
}

func TestStrictLicenseClassifier(t *testing.T) {
	c := &strictLicenseClassifier{classifier: failedLicenseClassifier{}}
	if c.collectionError() != nil {
		t.Fatal("clean classifier reported a failure")
	}
	if _, err := c.Identify("LICENSE"); err == nil || c.collectionError() == nil {
		t.Fatal("upstream candidate failure was suppressed")
	}
}

func TestMapLicenseLibraries(t *testing.T) {
	expected := map[string]goListModule{
		"example.test/module/subpkg": {Path: "example.test/module"},
		"example.test/module/other":  {Path: "example.test/module"},
	}
	libraries := []*licenses.Library{
		{Packages: []string{"example.test/module/subpkg"}, LicenseFile: "LICENSE", Licenses: []licenses.License{{Name: "MIT"}}},
		{Packages: []string{"example.test/module/other"}, LicenseFile: "LICENSE", Licenses: []licenses.License{{Name: "BSD-3-Clause"}}},
	}
	inventory, err := mapLicenseLibraries(libraries, expected)
	if err != nil || inventory["example.test/module@"] == nil || inventory["example.test/module@"].Type != "BSD-3-Clause AND MIT" {
		t.Fatalf("library common-name must map to actual module: %v %v", inventory, err)
	}
	for _, tc := range []struct {
		name string
		libs []*licenses.Library
		pkgs map[string]goListModule
	}{
		{"empty", nil, nil},
		{"partial", libraries[:1], expected},
		{"unexpected", libraries, map[string]goListModule{}},
		{"nil_library", []*licenses.Library{nil}, expected},
		{"version_mismatch", libraries, map[string]goListModule{"example.test/module/subpkg": {Path: "example.test/module", Version: "v1.0.0"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := mapLicenseLibraries(tc.libs, tc.pkgs); err == nil {
				t.Fatal("incomplete/mismatched collection accepted")
			}
		})
	}
	unknown, err := mapLicenseLibraries([]*licenses.Library{{Packages: []string{"example.test/pkg"}}}, map[string]goListModule{"example.test/pkg": {Path: "example.test/pkg"}})
	if err != nil || unknown["example.test/pkg@"].Type != "Unknown" {
		t.Fatalf("complete unresolved is a policy input, not a collection error: %v %v", unknown, err)
	}
}

func TestApplyGoLicenseInventoryFailsClosed(t *testing.T) {
	moduleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleDir, "LICENSE"), []byte("MIT License"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		inventory map[string]*License
		degraded  bool
		err       error
		wantPass  bool
	}{
		{"complete", map[string]*License{"example.test/dep@v1.0.0": {Type: "MIT"}}, false, nil, true},
		{"error_with_rows", map[string]*License{"example.test/dep@v1.0.0": {Type: "MIT"}}, false, errors.New("collector failed"), false},
		{"degraded_with_rows", map[string]*License{"example.test/dep@v1.0.0": {Type: "MIT"}}, true, nil, false},
		{"empty", nil, false, nil, false},
		{"partial", map[string]*License{"example.test/other@v1.0.0": {Type: "MIT"}}, false, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps := []Dependency{{Module: Module{Name: "example.test/dep", Version: "v1.0.0"}, Metadata: map[string]interface{}{"module_dir": moduleDir}}}
			issues, passed := applyGoLicenseInventory(deps, tc.inventory, tc.degraded, tc.err)
			if passed != tc.wantPass || (!passed && !hasLicenseCollectionError(issues)) {
				t.Fatalf("passed=%v issues=%v", passed, issues)
			}
			if !tc.wantPass && deps[0].License == nil {
				t.Fatal("diagnostic fallback was not retained")
			}
		})
	}
	if _, passed := applyGoLicenseInventory(nil, map[string]*License{"extra@": {Type: "MIT"}}, false, nil); passed {
		t.Fatal("empty dependency denominator passed")
	}
}

func TestUnresolvedLicensePolicy(t *testing.T) {
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		license  *License
		cfg      *config.LicensePolicyConfig
		wantPass bool
	}{
		{"nil", nil, nil, false},
		{"empty", &License{}, nil, false},
		{"unknown", &License{Type: "Unknown"}, nil, false},
		{"no_assertion", &License{Type: "NOASSERTION"}, nil, false},
		{"aggregate_unknown", &License{Type: "MIT AND Unknown"}, nil, false},
		{"resolved", &License{Type: "MIT"}, nil, true},
		{"exact_exception", &License{Type: "Unknown"}, &config.LicensePolicyConfig{Exceptions: []config.LicenseException{{Package: "example.test/dep", License: "Unknown"}}}, true},
		{"other_exception", &License{Type: "Unknown"}, &config.LicensePolicyConfig{Exceptions: []config.LicenseException{{Package: "example.test/other", License: "Unknown"}}}, false},
		{"expired_exception", &License{Type: "Unknown"}, &config.LicensePolicyConfig{Exceptions: []config.LicenseException{{Package: "example.test/dep", License: "Unknown", Until: "2026-10-03"}}}, false},
		{"aggregate_forbidden", &License{Type: "MIT AND MPL-2.0"}, &config.LicensePolicyConfig{Forbidden: []string{"MPL-2.0"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issues, passed := evaluateForbiddenLicenses([]Dependency{{Module: Module{Name: "example.test/dep"}, License: tc.license}}, tc.cfg, now)
			if passed != tc.wantPass || (!passed && (len(issues) == 0 || issues[0].Type != "license")) {
				t.Fatalf("passed=%v issues=%v", passed, issues)
			}
		})
	}
}

func TestGoAnalyzerCollectorStates(t *testing.T) {
	dir := t.TempDir()
	for path, content := range map[string]string{
		"go.mod":     "module example.test/collector-states\ngo 1.26.0\n",
		"fixture.go": "package fixture\nimport \"fmt\"\nfunc Name() string { return fmt.Sprint(1) }\n",
		"LICENSE":    "MIT License",
	} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name          string
		license       *License
		degraded      bool
		collectionErr error
		wantType      string
	}{
		{"complete", &License{Type: "MIT"}, false, nil, ""},
		{"failure", &License{Type: "MIT"}, false, errors.New("collector failed"), "license_error"},
		{"empty", nil, false, nil, "license_error"},
		{"fallback", nil, true, errors.New("collector degraded"), "license_error"},
		{"unresolved", &License{Type: "Unknown"}, false, nil, "license"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			analyzer := &GoAnalyzer{collect: func(context.Context) (map[string]*License, bool, error) {
				inventory := map[string]*License{}
				if tc.license != nil {
					inventory["example.test/collector-states@"] = tc.license
				}
				return inventory, tc.degraded, tc.collectionErr
			}}
			result, err := analyzer.Analyze(context.Background(), dir, AnalysisConfig{CheckLicenses: true})
			if err != nil {
				t.Fatal(err)
			}
			if result.Passed != (tc.wantType == "") || (tc.wantType != "" && (len(result.Issues) == 0 || result.Issues[0].Type != tc.wantType)) {
				t.Fatalf("analyzer false-pass or wrong error/policy boundary: %+v", result)
			}
		})
	}
}

func TestCollectLicensesSelectedToolchain(t *testing.T) {
	// A network-free module with stdlib imports proves the real upstream API
	// handles auto selection from an older launcher (e.g. Go 1.26.4) to the
	// module's Go 1.26.6 toolchain, including module-cache GOROOT layouts.
	t.Setenv("GOTOOLCHAIN", "auto")
	dir := t.TempDir()
	for path, content := range map[string]string{
		"go.mod":     "module example.test/license-fixture\n\ngo 1.26.6\n",
		"fixture.go": "package fixture\nimport \"fmt\"\nfunc Name() string { return fmt.Sprint(1) }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	license, err := os.ReadFile(filepath.Join("..", "..", "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), license, 0600); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	inventory, degraded, err := collectLicenses(ctx)
	if err != nil || degraded || len(inventory) != 1 || inventory["example.test/license-fixture@"].Type != "Apache-2.0" {
		t.Fatalf("selected-toolchain collector: inventory=%v degraded=%v err=%v", inventory, degraded, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package fixture\nimport _ \"example.test/nonexistent\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, degraded, err = collectLicenses(ctx)
	if err == nil || !degraded || !strings.Contains(err.Error(), "discovery failed") {
		t.Fatalf("real failure injection soft-passed: degraded=%v err=%v", degraded, err)
	}
}
