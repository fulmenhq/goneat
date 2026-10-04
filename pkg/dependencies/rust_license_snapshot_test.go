package dependencies

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRustLicenseMissingTools(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mock command fixture")
	}
	for _, mode := range []string{"cargo", "cargo-deny"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname='missing-tool'\nversion='0.1.0'\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if mode == "cargo-deny" {
				if err := os.WriteFile(filepath.Join(root, "cargo"), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", root)
			result, err := NewRustAnalyzer().Analyze(context.Background(), root, AnalysisConfig{CheckLicenses: true})
			if err != nil || result.Passed || len(result.Issues) != 1 || result.Issues[0].Type != "license_error" || !strings.Contains(result.Issues[0].Message, mode) {
				t.Fatalf("missing-tool license error: %#v %v", result, err)
			}
		})
	}
}

func TestRustLicenseSnapshotInputStability(t *testing.T) {
	root := t.TempDir()
	policy := filepath.Join(root, "deny.toml")
	metadata := filepath.Join(root, "metadata.json")
	write := func(path, data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(policy, "[licenses]\nallow=['MIT']\n")
	write(metadata, "{}")
	exceptionPath, exceptionHash, err := rustLicenseExceptions(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &rustLicenseSnapshot{ConfigPath: policy, Path: metadata, ConfigHash: sha256.Sum256([]byte("[licenses]\nallow=['MIT']\n")), MetadataHash: sha256.Sum256([]byte("{}")), ExceptionPath: exceptionPath, ExceptionHash: exceptionHash}
	if err := snapshot.verifyConfig(); err != nil {
		t.Fatal(err)
	}
	write(metadata, "{} ")
	if err := snapshot.verifyConfig(); err == nil {
		t.Fatal("changed snapshot bytes accepted")
	}
	write(metadata, "{}")
	write(policy, "[licenses]\nallow=['Apache-2.0']\n")
	if err := snapshot.verifyConfig(); err == nil {
		t.Fatal("changed configured policy accepted")
	}
	write(policy, "[licenses]\nallow=['MIT']\n")
	write(filepath.Join(root, "deny.exceptions.toml"), "exceptions=[]\n")
	if err := snapshot.verifyConfig(); err == nil {
		t.Fatal("changed local exception selection accepted")
	}
}

func TestRustLicenseGraphConfig(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		wantError    bool
	}{
		{"default", "[licenses]\nallow = ['MIT']\n", false},
		{"graph_features", "[graph]\nfeatures = ['optional']\nall-features = false\nno-default-features = true\n", false},
		{"all_features", "[graph]\nall-features = true\n", false},
		{"empty_filters", "[graph]\ntargets = []\nexclude = []\n", false},
		{"target_filter", "[graph]\ntargets = [{ triple = 'x86_64-unknown-linux-gnu' }]\n", true},
		{"legacy_target", "targets = [{ triple = 'aarch64-apple-darwin' }]\n", true},
		{"graph_exclusion", "[graph]\nexclude = ['excluded-crate']\n", true},
		{"mixed_graph", "features = ['legacy']\n[graph]\nall-features = true\n", true},
		{"malformed", "[licenses", true},
		{"wrong_feature_type", "[graph]\nfeatures = true\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := licenseGraphConfig([]byte(tc.config), "fixture/deny.toml")
			if (err != nil) != tc.wantError {
				t.Fatalf("scope contract: %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), "fixture/deny.toml") {
				t.Fatalf("error omitted the configured policy path: %v", err)
			}
		})
	}
	scope, err := licenseGraphConfig([]byte("[graph]\nno-default-features = true\nfeatures = ['one', 'two']\n"), "deny.toml")
	if err != nil || strings.Join(scope.featureArgs(), " ") != "--no-default-features --features one,two" {
		t.Fatalf("metadata/deny feature arguments differ: %v %v", scope.featureArgs(), err)
	}
}

func TestRustLicenseGitSourceIdentity(t *testing.T) {
	source := "git+https://example.test/dep?rev=v1#0123456789abcdef"
	pkg := rustLicensePackage{ID: "git+https://example.test/dep?rev=v1#dep@1.0.0", Name: "dep", Version: "1.0.0", Source: &source}
	key, err := rustLicenseCrateKey(pkg)
	if err != nil || key != "dep 1.0.0 git+https://example.test/dep?rev=v1" {
		t.Fatalf("git identity reconciliation: %q %v", key, err)
	}
	expected := map[string]rustLicensePackage{key: pkg}
	data, err := json.Marshal(map[string]interface{}{key: map[string]interface{}{"licenses": []string{}}})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := reconcileRustLicenseList(data, expected)
	if err != nil || rows[0].Source != source || rows[0].PackageID != pkg.ID {
		t.Fatalf("git commit/source lost: %#v %v", rows, err)
	}
}

func TestRustLicenseSnapshotReal(t *testing.T) {
	if !IsCargoAvailable() || !CheckCargoDenyPresence().Present {
		t.Skip("cargo and cargo-deny required")
	}
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("Cargo.toml", "[package]\nname = 'snapshot-root'\nversion = '0.1.0'\nedition = '2021'\nlicense = 'MIT'\n[dependencies]\nsnapshot-dep = { path = 'dep' }\n")
	write("src/lib.rs", "pub fn root() {}\n")
	write("dep/Cargo.toml", "[package]\nname = 'snapshot-dep'\nversion = '0.1.0'\nedition = '2021'\nlicense = 'MIT AND Apache-2.0'\n")
	write("dep/src/lib.rs", "pub fn dep() {}\n")
	write("deny.toml", "[licenses]\nallow = ['MIT', 'Apache-2.0']\n")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	project := DetectRustProject(root)
	snapshot, err := captureRustLicenseSnapshot(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.cleanup()
	list, err := runCargoDenyListWithSnapshot(ctx, root, time.Minute, snapshot)
	if err != nil || len(list.Dependencies) != 2 {
		t.Fatalf("real crate inventory: %#v %v", list, err)
	}
	check, err := runCargoDenyWithSnapshot(ctx, project, root, []CargoDenyCheckType{CargoDenyCheckLicenses, CargoDenyCheckBans}, time.Minute, snapshot)
	if err != nil || check.Failed {
		t.Fatalf("real passing path: %#v %v", check, err)
	}
	result, err := NewRustAnalyzer().Analyze(ctx, root, AnalysisConfig{CheckLicenses: true})
	if err != nil || !result.Passed || len(result.Dependencies) != 2 {
		t.Fatalf("real analyzer pass: %#v %v", result, err)
	}
	if result.Dependencies[0].License.Type != "MIT AND Apache-2.0" {
		t.Fatalf("SPDX operators lost: %#v", result.Dependencies)
	}
	write("deny.toml", "[licenses]\nallow = ['MIT', 'Apache-2.0']\n[bans]\ndeny = [{ name = 'snapshot-dep' }]\n")
	result, err = NewRustAnalyzer().Analyze(ctx, root, AnalysisConfig{CheckLicenses: true})
	if err != nil || result.Passed {
		t.Fatalf("real nonzero ban soft-passed: %#v %v", result, err)
	}
	write("deny.toml", "[licenses]\nallow = ['MIT']\n")
	result, err = NewRustAnalyzer().Analyze(ctx, root, AnalysisConfig{CheckLicenses: true})
	if err != nil || result.Passed {
		t.Fatalf("rejected AND expression soft-passed: %#v %v", result, err)
	}
	write("deny.toml", "[licenses]\nallow = ['MIT']\n[graph]\ntargets = ['x86_64-unknown-linux-gnu']\n")
	result, err = NewRustAnalyzer().Analyze(ctx, root, AnalysisConfig{CheckLicenses: true})
	if err != nil || result.Passed || len(result.Issues) == 0 || result.Issues[0].Type != "license_error" {
		t.Fatalf("unsupported scope not explicit: %#v %v", result, err)
	}
}

func TestRustLicenseSnapshotCommandControls(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mock command fixture")
	}
	root := t.TempDir()
	write := func(name string, data []byte, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), data, mode); err != nil {
			t.Fatal(err)
		}
	}
	write("Cargo.toml", []byte("[package]\nname='root'\nversion='0.1.0'\n"), 0o600)
	write("deny.toml", []byte("[licenses]\nallow=['MIT','Apache-2.0']\n"), 0o600)
	write("metadata.json", rustMetadataFixture(), 0o600)
	expected, err := expectedRustLicensePackages(rustMetadataFixture())
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]interface{}{}
	for key, pkg := range expected {
		rows[key] = map[string]interface{}{"licenses": parseLicenseExpression(*pkg.License)}
	}
	list, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	write("list.json", list, 0o600)
	const summary = `{"type":"summary","fields":{"licenses":{"errors":0,"warnings":0,"notes":0,"helps":0},"bans":{"errors":0,"warnings":0,"notes":0,"helps":0}}}`
	write("check.json", []byte(summary+"\n"), 0o600)
	// The mock records command arguments to prove that list/check share the
	// same privately captured snapshot and metadata is collected just once.
	write("cargo", []byte(`#!/bin/sh
case "$*" in
  "deny --version") echo 'cargo-deny 0.20.2'; exit 0 ;;
esac
printf '%s\n' "$*" >> "$RUST_CALLS"
case "$1" in
  metadata) /bin/cat "$RUST_METADATA"; exit "${RUST_METADATA_EXIT:-0}" ;;
esac
case "$*" in
  *" list "*) /bin/cat "$RUST_LIST"; printf '%s' "$RUST_LIST_STDERR" >&2; exit "${RUST_LIST_EXIT:-0}" ;;
  *" check "*) /bin/cat "$RUST_CHECK" >&2; exit "${RUST_CHECK_EXIT:-0}" ;;
esac
exit 9
`), 0o700)
	t.Setenv("PATH", root)
	t.Setenv("RUST_METADATA", filepath.Join(root, "metadata.json"))
	t.Setenv("RUST_LIST", filepath.Join(root, "list.json"))
	t.Setenv("RUST_CHECK", filepath.Join(root, "check.json"))
	t.Setenv("RUST_CALLS", filepath.Join(root, "calls.log"))
	for _, mode := range []string{"pass", "ban", "unknown_exit", "list_nonzero", "metadata_nonzero", "degraded_list", "empty_list", "omitted_crate", "missing_summary", "malformed_check", "truncated_check", "summary_error_without_diagnostic", "scope_exclusion", "scope_exclude_dev", "bad_config", "combined"} {
		t.Run(mode, func(t *testing.T) {
			write("list.json", list, 0o600)
			write("check.json", []byte(summary+"\n"), 0o600)
			write("calls.log", nil, 0o600)
			write("deny.toml", []byte("[licenses]\nallow=['MIT','Apache-2.0']\n"), 0o600)
			for _, env := range []string{"RUST_CHECK_EXIT", "RUST_LIST_EXIT", "RUST_METADATA_EXIT"} {
				t.Setenv(env, "0")
			}
			t.Setenv("RUST_LIST_STDERR", "")
			switch mode {
			case "ban":
				t.Setenv("RUST_CHECK_EXIT", "2")
				write("check.json", []byte(`{"type":"diagnostic","fields":{"code":"banned","severity":"error","message":"banned crate"}}`+"\n"+strings.Replace(summary, `"bans":{"errors":0`, `"bans":{"errors":1`, 1)+"\n"), 0o600)
			case "unknown_exit":
				t.Setenv("RUST_CHECK_EXIT", "3")
			case "list_nonzero":
				t.Setenv("RUST_LIST_EXIT", "2")
			case "metadata_nonzero":
				t.Setenv("RUST_METADATA_EXIT", "1")
			case "degraded_list":
				t.Setenv("RUST_LIST_STDERR", "graph collection degraded")
			case "empty_list":
				write("list.json", []byte(`{}`), 0o600)
			case "omitted_crate":
				write("list.json", []byte(`{"root 0.1.0 path+file:///workspace":{"licenses":["MIT"]}}`), 0o600)
			case "missing_summary":
				write("check.json", nil, 0o600)
			case "malformed_check":
				write("check.json", []byte("not json\n"+summary), 0o600)
			case "truncated_check":
				write("check.json", []byte(summary[:len(summary)-1]), 0o600)
			case "summary_error_without_diagnostic":
				write("check.json", []byte(strings.Replace(summary, `"errors":0`, `"errors":1`, 1)), 0o600)
			case "scope_exclusion":
				write("deny.toml", []byte("[graph]\nexclude=['dep']\n"), 0o600)
			case "scope_exclude_dev":
				write("deny.toml", []byte("[graph]\nexclude-dev=true\n"), 0o600)
			case "bad_config":
				write("deny.toml", []byte("[licenses"), 0o600)
			case "combined":
				t.Setenv("RUST_CHECK_EXIT", "2")
			}
			result, err := NewRustAnalyzer().Analyze(context.Background(), root, AnalysisConfig{CheckLicenses: true, CheckCooling: mode == "combined"})
			if err != nil || result == nil {
				t.Fatalf("requested evidence errors must be structured: %v %v", result, err)
			}
			if result.Passed != (mode == "pass") {
				t.Fatalf("%s contract: %#v", mode, result)
			}
			if mode == "pass" || mode == "ban" {
				if len(result.Dependencies) != 3 {
					t.Fatalf("crate count inflated: %d", len(result.Dependencies))
				}
				calls, err := os.ReadFile(filepath.Join(root, "calls.log"))
				if err != nil {
					t.Fatal(err)
				}
				lines := strings.Split(strings.TrimSpace(string(calls)), "\n")
				if len(lines) != 3 || !strings.HasPrefix(lines[0], "metadata ") {
					t.Fatalf("metadata must run once: %s", calls)
				}
				var metadataPaths []string
				for _, line := range lines[1:] {
					args := strings.Fields(line)
					for i, arg := range args {
						if arg == "--metadata-path" {
							metadataPaths = append(metadataPaths, args[i+1])
						}
					}
				}
				if len(metadataPaths) != 2 || metadataPaths[0] != metadataPaths[1] {
					t.Fatalf("snapshot drift between consumers: %s", calls)
				}
			} else {
				foundError := false
				for _, issue := range result.Issues {
					if issue.Type == "license_error" {
						foundError = true
					}
				}
				if !foundError {
					t.Fatalf("collection failure mislabeled as policy: %#v", result.Issues)
				}
			}
		})
	}
}

func rustMetadataFixture() []byte {
	return []byte(`{
		"packages": [
			{"id":"path+file:///workspace#root@0.1.0","name":"root","version":"0.1.0","source":null,"license":"MIT","manifest_path":"/workspace/Cargo.toml"},
			{"id":"registry+https://example.test/index#dep@1.0.0","name":"dep","version":"1.0.0","source":"registry+https://example.test/index","license":"MIT AND Apache-2.0"},
			{"id":"path+file:///workspace/dep#dep@1.0.0","name":"dep","version":"1.0.0","source":null,"license":"MIT OR Apache-2.0"},
			{"id":"registry+https://example.test/index#unused@1.0.0","name":"unused","version":"1.0.0","source":"registry+https://example.test/index","license":"MIT"}
		],
		"workspace_members": ["path+file:///workspace#root@0.1.0"],
		"resolve": {"nodes": [
			{"id":"path+file:///workspace#root@0.1.0","dependencies":["registry+https://example.test/index#dep@1.0.0","path+file:///workspace/dep#dep@1.0.0"],"deps":[{"pkg":"registry+https://example.test/index#dep@1.0.0","dep_kinds":[{"kind":null}]},{"pkg":"path+file:///workspace/dep#dep@1.0.0","dep_kinds":[{"kind":null}]}]},
			{"id":"registry+https://example.test/index#dep@1.0.0","dependencies":[]},
			{"id":"path+file:///workspace/dep#dep@1.0.0","dependencies":[]},
			{"id":"registry+https://example.test/index#unused@1.0.0","dependencies":[]}
		]}
	}`)
}

func TestExpectedRustLicensePackages(t *testing.T) {
	expected, err := expectedRustLicensePackages(rustMetadataFixture())
	if err != nil || len(expected) != 3 {
		t.Fatalf("reachable package denominator: %v %v", expected, err)
	}
	if _, ok := expected["unused 1.0.0 registry+https://example.test/index"]; ok {
		t.Fatal("raw metadata packages are not the reachable graph")
	}
	for _, mode := range []string{"empty", "unresolved", "missing_package", "missing_node", "duplicate_package", "duplicate_node", "wrong_source"} {
		t.Run(mode, func(t *testing.T) {
			var data map[string]interface{}
			if err := json.Unmarshal(rustMetadataFixture(), &data); err != nil {
				t.Fatal(err)
			}
			packages := data["packages"].([]interface{})
			resolve := data["resolve"].(map[string]interface{})
			nodes := resolve["nodes"].([]interface{})
			switch mode {
			case "empty":
				data["workspace_members"] = []string{}
			case "unresolved":
				data["resolve"] = nil
			case "missing_package":
				data["packages"] = packages[1:]
			case "missing_node":
				resolve["nodes"] = nodes[1:]
			case "duplicate_package":
				data["packages"] = append(packages, packages[0])
			case "duplicate_node":
				resolve["nodes"] = append(nodes, nodes[0])
			case "wrong_source":
				packages[1].(map[string]interface{})["source"] = "registry+https://other.test/index"
			}
			encoded, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := expectedRustLicensePackages(encoded); err == nil {
				t.Fatal("invalid/unreconciled graph accepted")
			}
		})
	}
}

func TestExpectedRustLicenseDevEdges(t *testing.T) {
	for _, tc := range []struct {
		name          string
		workspaceEdge bool
		kind          string
		wantCount     int
		wantError     bool
	}{
		{"workspace_dev", true, "dev", 4, false},
		{"dependency_dev", false, "dev", 3, false},
		{"dependency_build", false, "build", 4, false},
		{"unknown_kind", false, "future-kind", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var metadata map[string]interface{}
			if err := json.Unmarshal(rustMetadataFixture(), &metadata); err != nil {
				t.Fatal(err)
			}
			nodes := metadata["resolve"].(map[string]interface{})["nodes"].([]interface{})
			index := 1
			if tc.workspaceEdge {
				index = 0
			}
			node := nodes[index].(map[string]interface{})
			id := "registry+https://example.test/index#unused@1.0.0"
			node["dependencies"] = append(node["dependencies"].([]interface{}), id)
			deps, _ := node["deps"].([]interface{})
			node["deps"] = append(deps, map[string]interface{}{"pkg": id, "dep_kinds": []interface{}{map[string]interface{}{"kind": tc.kind}}})
			data, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			set, err := expectedRustLicensePackages(data)
			if (err != nil) != tc.wantError || (!tc.wantError && len(set) != tc.wantCount) {
				t.Fatalf("dev-edge graph contract: %v %v", set, err)
			}
		})
	}
}

func TestRustLicenseRealFeatureWorkspace(t *testing.T) {
	if !IsCargoAvailable() || !CheckCargoDenyPresence().Present {
		t.Skip("cargo and cargo-deny required")
	}
	t.Setenv("CARGO_NET_OFFLINE", "true")
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("Cargo.toml", "[workspace]\nmembers=['a','b']\ndefault-members=['a']\nexclude=['optional','dev']\nresolver='2'\n")
	write("a/Cargo.toml", "[package]\nname='member-a'\nversion='0.1.0'\nedition='2021'\nlicense='MIT'\n[dependencies]\noptional = { path = '../optional', optional = true }\n[features]\nextra=['dep:optional']\n[dev-dependencies]\ndev={path='../dev'}\n")
	write("b/Cargo.toml", "[package]\nname='member-b'\nversion='0.1.0'\nedition='2021'\nlicense='MIT OR Apache-2.0'\n")
	for _, name := range []string{"optional", "dev"} {
		write(name+"/Cargo.toml", "[package]\nname='"+name+"'\nversion='0.1.0'\nedition='2021'\nlicense='MIT'\n")
	}
	for _, name := range []string{"a", "b", "optional", "dev"} {
		write(name+"/src/lib.rs", "pub fn f() {}\n")
	}
	for _, tc := range []struct {
		name, graph string
		count       int
	}{
		{"defaults", "", 3},
		{"explicit_feature", "[graph]\nfeatures=['member-a/extra']\n", 4},
		{"all_features", "[graph]\nall-features=true\n", 4},
		{"no_default_features", "[graph]\nno-default-features=true\n", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write("deny.toml", "[licenses]\nallow=['MIT','Apache-2.0']\ninclude-dev=true\n"+tc.graph)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			result, err := NewRustAnalyzer().Analyze(ctx, root, AnalysisConfig{CheckLicenses: true})
			if err != nil || result == nil || !result.Passed || len(result.Dependencies) != tc.count {
				t.Fatalf("configured feature/workspace scope: %#v %v", result, err)
			}
			foundB := false
			for _, dep := range result.Dependencies {
				if dep.Name == "member-b" && dep.License.Type == "MIT OR Apache-2.0" {
					foundB = true
				}
			}
			if !foundB {
				t.Fatal("non-default workspace member or OR expression lost")
			}
		})
	}
	write("deny.toml", "[licenses]\nallow=['MIT','Apache-2.0']\n")
	result, err := NewRustAnalyzer().Analyze(context.Background(), root, AnalysisConfig{CheckLicenses: true})
	if err != nil || result.Passed || !strings.Contains(result.Issues[0].Message, "licenses.include-dev=false") || !strings.Contains(result.Issues[0].Message, "dev#") {
		t.Fatalf("default dev-stage filter not actionable: %#v %v", result, err)
	}
	// A flag alone does not fail. Here include-build=false removes nothing.
	write("deny.toml", "[licenses]\nallow=['MIT','Apache-2.0']\ninclude-dev=true\ninclude-build=false\n")
	result, err = NewRustAnalyzer().Analyze(context.Background(), root, AnalysisConfig{CheckLicenses: true})
	if err != nil || !result.Passed {
		t.Fatalf("ineffective build filter falsely rejected: %#v %v", result, err)
	}
	write("Cargo.toml", "[workspace]\nmembers=['a','b']\nexclude=['optional','dev','build-only']\nresolver='2'\n")
	write("build-only/Cargo.toml", "[package]\nname='build-only'\nversion='0.1.0'\nedition='2021'\nlicense='MIT'\n")
	write("build-only/src/lib.rs", "pub fn f() {}\n")
	write("a/Cargo.toml", "[package]\nname='member-a'\nversion='0.1.0'\nedition='2021'\nlicense='MIT'\n[build-dependencies]\nbuild-only={path='../build-only'}\n")
	result, err = NewRustAnalyzer().Analyze(context.Background(), root, AnalysisConfig{CheckLicenses: true})
	if err != nil || result.Passed || !strings.Contains(result.Issues[0].Message, "licenses.include-build=false") || !strings.Contains(result.Issues[0].Message, "build-only#") {
		t.Fatalf("build-stage filter not actionable: %#v %v", result, err)
	}
	write("a/Cargo.toml", "[package]\nname='member-a'\nversion='0.1.0'\nedition='2021'\nlicense='MIT'\n[build-dependencies]\nbuild-only={path='../build-only'}\n[dev-dependencies]\nbuild-only={path='../build-only'}\n")
	write("deny.toml", "[licenses]\nallow=['MIT','Apache-2.0']\ninclude-dev=false\ninclude-build=false\n")
	result, err = NewRustAnalyzer().Analyze(context.Background(), root, AnalysisConfig{CheckLicenses: true})
	if err != nil || !result.Passed {
		t.Fatalf("separate dev/build paths must use cargo-deny's set intersection: %#v %v", result, err)
	}
}

func TestReconcileRustLicenseList(t *testing.T) {
	expected, err := expectedRustLicensePackages(rustMetadataFixture())
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]interface{}{}
	for key, pkg := range expected {
		rows[key] = map[string]interface{}{"licenses": parseLicenseExpression(*pkg.License)}
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	crates, err := reconcileRustLicenseList(data, expected)
	if err != nil || len(crates) != 3 {
		t.Fatalf("exact crate set: %v %v", crates, err)
	}
	expressions := map[string]string{}
	for _, crate := range crates {
		expressions[crate.PackageID] = crate.Expression
	}
	if expressions["registry+https://example.test/index#dep@1.0.0"] != "MIT AND Apache-2.0" || expressions["path+file:///workspace/dep#dep@1.0.0"] != "MIT OR Apache-2.0" {
		t.Fatalf("source identities merged or AND/OR reconstructed from flat IDs: %v", expressions)
	}
	for _, mode := range []string{"empty", "malformed", "truncated", "omitted", "unexpected", "duplicate", "missing_licenses", "license_mismatch", "trailing"} {
		t.Run(mode, func(t *testing.T) {
			var encoded []byte
			switch mode {
			case "empty":
				encoded = []byte(`{}`)
			case "malformed":
				encoded = []byte(`not JSON`)
			case "truncated":
				encoded = data[:len(data)-1]
			case "omitted":
				encoded = []byte(`{"root 0.1.0 path+file:///workspace":{"licenses":["MIT"]}}`)
			case "unexpected":
				encoded = []byte(`{"unknown 1.0.0 path+file:///other":{"licenses":["MIT"]}}`)
			case "duplicate":
				encoded = []byte(`{"root 0.1.0 path+file:///workspace":{"licenses":["MIT"]},"root 0.1.0 path+file:///workspace":{"licenses":["MIT"]}}`)
			case "missing_licenses":
				encoded = []byte(`{"root 0.1.0 path+file:///workspace":{}}`)
			case "license_mismatch":
				encoded = []byte(strings.Replace(string(data), `"MIT"`, `"GPL-3.0-only"`, 1))
			case "trailing":
				encoded = append(append([]byte(nil), data...), []byte(` {}`)...)
			}
			if _, err := reconcileRustLicenseList(encoded, expected); err == nil {
				t.Fatal("incomplete/invalid crate list passed")
			}
		})
	}
}
