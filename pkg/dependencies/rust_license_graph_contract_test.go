package dependencies

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// rustGraphFixture writes a path-only workspace. Every crate is MIT so the
// policy never fails; these fixtures exercise graph membership only.
type rustGraphFixture struct {
	members, excluded []string
	crates            []rustGraphCrate
}

type rustGraphCrate struct {
	dir, name, version, lib, extra string
}

func (f rustGraphFixture) write(t *testing.T, root string) {
	t.Helper()
	quote := func(items []string) string {
		quoted := make([]string, len(items))
		for i, item := range items {
			quoted[i] = "'" + item + "'"
		}
		return "[" + strings.Join(quoted, ",") + "]"
	}
	put := func(name, content string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	put("Cargo.toml", "[workspace]\nmembers="+quote(f.members)+"\nexclude="+quote(f.excluded)+"\nresolver='2'\n")
	for _, c := range f.crates {
		version := c.version
		if version == "" {
			version = "0.1.0"
		}
		lib := ""
		if c.lib != "" {
			lib = "[lib]\nname='" + c.lib + "'\n"
		}
		put(c.dir+"/Cargo.toml", "[package]\nname='"+c.name+"'\nversion='"+version+"'\nedition='2021'\nlicense='MIT'\n"+lib+c.extra)
		put(c.dir+"/src/lib.rs", "")
	}
}

func simpleCrates(names ...string) []rustGraphCrate {
	crates := make([]rustGraphCrate, len(names))
	for i, name := range names {
		crates[i] = rustGraphCrate{dir: name, name: name}
	}
	return crates
}

// rustGraphContractFixtures mirror the cargo-deny 0.20.2 / krates 0.21.3
// graph contract: inactive optional edges, never-true cfg constants, root
// feature seeding, stage filtering without feature re-resolution, and
// declaration identity.
func rustGraphContractFixtures() map[string]rustGraphFixture {
	return map[string]rustGraphFixture{
		// An optional, target-gated dependency named only by a weak feature.
		"weak_optional_target": {
			members: []string{"app"}, excluded: []string{"host", "wasm"},
			crates: []rustGraphCrate{
				{dir: "app", name: "app", extra: "[dependencies]\nhost={path='../host'}\n"},
				{dir: "host", name: "host", extra: "[features]\ndefault=['std']\nstd=['wasm?/std']\n[target.'cfg(target_arch = \"wasm32\")'.dependencies]\nwasm={path='../wasm',optional=true}\n"},
				{dir: "wasm", name: "wasm", extra: "[features]\nstd=[]\n"},
			},
		},
		// An optional dependency that a registry-style crate also lists as a
		// dev-dependency; dev edges never apply below the workspace.
		"optional_plus_dev_decl": {
			members: []string{"app"}, excluded: []string{"t", "macros"},
			crates: []rustGraphCrate{
				{dir: "app", name: "app", extra: "[dependencies]\nt={path='../t',features=['parsing']}\n"},
				{dir: "t", name: "t", extra: "[features]\nparsing=['macros?/parsing']\n[dependencies]\nmacros={path='../macros',optional=true}\n[dev-dependencies]\nmacros={path='../macros'}\n"},
				{dir: "macros", name: "macros", extra: "[features]\nparsing=[]\n"},
			},
		},
		// Only predicate-free false expressions are dropped; any expression
		// with a target predicate is retained even when logically false.
		"cfg_constants": {
			members: []string{"app"}, excluded: []string{"never", "notany", "allempty", "mixed", "notmixed"},
			crates: append([]rustGraphCrate{{dir: "app", name: "app", extra: "" +
				"[target.'cfg(any())'.dependencies]\nnever={path='../never'}\n" +
				"[target.'cfg(not(any()))'.dependencies]\nnotany={path='../notany'}\n" +
				"[target.'cfg(all())'.dependencies]\nallempty={path='../allempty'}\n" +
				"[target.'cfg(all(any(), unix))'.dependencies]\nmixed={path='../mixed'}\n" +
				"[target.'cfg(not(all(any(), windows)))'.dependencies]\nnotmixed={path='../notmixed'}\n"}},
				simpleCrates("never", "notany", "allempty", "mixed", "notmixed")...),
		},
		// Activations that must be retained: dep:, strong name/feature, implicit.
		"activations": {
			members: []string{"app"}, excluded: []string{"host", "a1", "a2", "a3"},
			crates: append([]rustGraphCrate{
				{dir: "app", name: "app", extra: "[dependencies]\nhost={path='../host',features=['one','two','a3']}\n"},
				{dir: "host", name: "host", extra: "[features]\none=['dep:a1']\ntwo=['a2/x']\n[dependencies]\na1={path='../a1',optional=true}\na2={path='../a2',optional=true}\na3={path='../a3',optional=true}\n"},
				{dir: "a2", name: "a2", extra: "[features]\nx=[]\n"},
			}, simpleCrates("a1", "a3")...),
		},
		// Metadata reports a feature enabled through an inactive edge; it must
		// not activate anything below the roots.
		"leaked_feature": {
			members: []string{"app"}, excluded: []string{"gate", "gated", "shared", "webtime"},
			crates: append([]rustGraphCrate{
				{dir: "app", name: "app", extra: "[dependencies]\ngate={path='../gate'}\nshared={path='../shared'}\n"},
				{dir: "gate", name: "gate", extra: "[features]\ndefault=['web']\nweb=['gated?/web']\n[target.'cfg(target_arch = \"wasm32\")'.dependencies]\ngated={path='../gated',optional=true}\n"},
				{dir: "gated", name: "gated", extra: "[features]\nweb=[]\n[dependencies]\nshared={path='../shared',features=['web']}\n"},
				{dir: "shared", name: "shared", extra: "[features]\nweb=['dep:webtime']\n[dependencies]\nwebtime={path='../webtime',optional=true}\n"},
			}, simpleCrates("webtime")...),
		},
		// Workspace roots keep their metadata feature set, even when the
		// enabling edge is itself never-true.
		"root_features": {
			members: []string{"a", "b"}, excluded: []string{"leaf"},
			crates: append([]rustGraphCrate{
				{dir: "a", name: "a", extra: "[dependencies]\nleaf={path='../leaf',optional=true}\n[features]\nextra=['dep:leaf']\n"},
				{dir: "b", name: "b", extra: "[target.'cfg(any())'.dependencies]\na={path='../a',features=['extra']}\n"},
			}, simpleCrates("leaf")...),
		},
		// A dev helper enables a feature on a normal dependency; removing dev
		// edges must not undo that activation.
		"dev_stage_activation": {
			members: []string{"root"}, excluded: []string{"shared", "helper", "leaf"},
			crates: append([]rustGraphCrate{
				{dir: "root", name: "root", extra: "[dependencies]\nshared={path='../shared'}\n[dev-dependencies]\nhelper={path='../helper'}\n"},
				{dir: "shared", name: "shared", extra: "[dependencies]\nleaf={path='../leaf',optional=true}\n[features]\nextra=['dep:leaf']\n"},
				{dir: "helper", name: "helper", extra: "[dependencies]\nshared={path='../shared',features=['extra']}\n"},
			}, simpleCrates("leaf")...),
		},
		// Build-stage activation plus a crate reached by both build and dev
		// edges, which only the both-false intersection removes.
		"build_stage_intersection": {
			members: []string{"root"}, excluded: []string{"shared", "bhelper", "leaf", "both"},
			crates: append([]rustGraphCrate{
				{dir: "root", name: "root", extra: "[dependencies]\nshared={path='../shared'}\n[build-dependencies]\nbhelper={path='../bhelper'}\nboth={path='../both'}\n[dev-dependencies]\nboth={path='../both'}\n"},
				{dir: "shared", name: "shared", extra: "[dependencies]\nleaf={path='../leaf',optional=true}\n[features]\nextra=['dep:leaf']\n"},
				{dir: "bhelper", name: "bhelper", extra: "[dependencies]\nshared={path='../shared',features=['extra']}\n"},
			}, simpleCrates("leaf", "both")...),
		},
		// Two versions of one package under different targets with different
		// features: features must reach the right version only.
		"multi_version_target_features": {
			members: []string{"app"}, excluded: []string{"v1", "v2", "o1", "o2"},
			crates: append([]rustGraphCrate{
				{dir: "app", name: "app", extra: "[dependencies]\nlib1={package='dupe',path='../v1',features=['f']}\n[target.'cfg(unix)'.dependencies]\nlib2={package='dupe',path='../v2'}\n"},
				{dir: "v1", name: "dupe", version: "1.0.0", extra: "[features]\nf=['dep:o1']\n[dependencies]\no1={path='../o1',optional=true}\n"},
				{dir: "v2", name: "dupe", version: "2.0.0", extra: "[features]\nf=['dep:o2']\n[dependencies]\no2={path='../o2',optional=true}\n"},
			}, simpleCrates("o1", "o2")...),
		},
		// Renamed dependency and a custom [lib] name (resolve uses lib names).
		"rename_and_custom_lib": {
			members: []string{"app"}, excluded: []string{"real", "customlib", "opt"},
			crates: append([]rustGraphCrate{
				{dir: "app", name: "app", extra: "[dependencies]\nalias-name={package='real',path='../real',features=['x']}\ncustom-lib-pkg={path='../customlib'}\n"},
				{dir: "real", name: "real", extra: "[features]\nx=['dep:opt']\n[dependencies]\nopt={path='../opt',optional=true}\n"},
				{dir: "customlib", name: "custom-lib-pkg", lib: "weird_lib"},
			}, simpleCrates("opt")...),
		},
		// uses_default_features on a dependency that defines no default.
		"no_default_feature_dep": {
			members: []string{"app"}, excluded: []string{"plain"},
			crates: append([]rustGraphCrate{
				{dir: "app", name: "app", extra: "[dependencies]\nplain={path='../plain'}\n"},
			}, simpleCrates("plain")...),
		},
	}
}

// TestRustLicenseGraphContract compares goneat's expected license set with a
// direct cargo-deny list over the same metadata snapshot, for every fixture
// and every include-dev/include-build stage.
func TestRustLicenseGraphContract(t *testing.T) {
	presence := CheckCargoDenyPresence()
	if !IsCargoAvailable() || !presence.Present {
		t.Skip("cargo and cargo-deny required")
	}
	if presence.Version == "0.19.0" {
		t.Skip("cargo-deny 0.19.0 does not apply include-dev to list; contract evidenced on 0.20.x")
	}
	t.Setenv("CARGO_NET_OFFLINE", "true")
	fixtures := rustGraphContractFixtures()
	names := make([]string, 0, len(fixtures))
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			fixtures[name].write(t, root)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			metadataCmd := exec.CommandContext(ctx, "cargo", "metadata", "--format-version", "1", "--manifest-path", filepath.Join(root, "Cargo.toml"))
			metadataCmd.Dir = root
			data, err := metadataCmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			metadataPath := filepath.Join(t.TempDir(), "metadata.json")
			if err := os.WriteFile(metadataPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			for _, stage := range []struct{ dev, build bool }{{true, true}, {true, false}, {false, true}, {false, false}} {
				t.Run(fmt.Sprintf("dev=%t,build=%t", stage.dev, stage.build), func(t *testing.T) {
					configPath := filepath.Join(root, "deny.toml")
					config := fmt.Sprintf("[licenses]\nallow=['MIT']\ninclude-dev=%t\ninclude-build=%t\n", stage.dev, stage.build)
					if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
						t.Fatal(err)
					}
					// Independent oracle: direct list on the saved snapshot.
					listCmd := exec.CommandContext(ctx, "cargo", "deny", "--metadata-path", metadataPath, "--config", configPath, "list", "--format", "json", "--layout", "crate")
					listCmd.Dir = root
					listed, err := listCmd.Output()
					if err != nil {
						t.Fatal(err)
					}
					var rows map[string]json.RawMessage
					if err := json.Unmarshal(listed, &rows); err != nil {
						t.Fatal(err)
					}
					var want []string
					for key := range rows {
						want = append(want, key)
					}
					sort.Strings(want)

					scope := rustLicenseScope{IncludeDev: stage.dev, IncludeBuild: stage.build}
					stageSet, err := expectedRustLicenseStage(data, scope)
					if err != nil {
						t.Fatalf("stage graph: %v", err)
					}
					var got []string
					for key := range stageSet {
						got = append(got, key)
					}
					sort.Strings(got)
					if !slices.Equal(got, want) {
						t.Fatalf("license stage set differs from cargo-deny %s:\n got  %v\n want %v", presence.Version, got, want)
					}
				})
			}
		})
	}
}

// TestRustLicenseGraphEvidence covers the metadata evidence the graph builder
// requires (fail closed) and the gaps cargo-deny tolerates (must not fail).
func TestRustLicenseGraphEvidence(t *testing.T) {
	mutate := func(t *testing.T, edit func(packages []interface{}, nodes []interface{})) []byte {
		t.Helper()
		var data map[string]interface{}
		if err := json.Unmarshal(rustMetadataFixture(), &data); err != nil {
			t.Fatal(err)
		}
		edit(data["packages"].([]interface{}), data["resolve"].(map[string]interface{})["nodes"].([]interface{}))
		encoded, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	root := func(packages []interface{}) map[string]interface{} { return packages[0].(map[string]interface{}) }
	rootDecls := func(packages []interface{}) []interface{} { return root(packages)["dependencies"].([]interface{}) }
	for name, edit := range map[string]func([]interface{}, []interface{}){
		"missing_dependencies": func(p, _ []interface{}) { delete(root(p), "dependencies") },
		"missing_features":     func(p, _ []interface{}) { delete(root(p), "features") },
		"missing_node_features": func(_, n []interface{}) {
			delete(n[0].(map[string]interface{}), "features")
		},
		"unmatched_declaration": func(p, _ []interface{}) { root(p)["dependencies"] = rootDecls(p)[1:] },
		"ambiguous_declaration": func(p, _ []interface{}) {
			// Same name, kind, target and req, and no source/path to tell them apart.
			for _, d := range rootDecls(p) {
				decl := d.(map[string]interface{})
				delete(decl, "source")
				delete(decl, "path")
				decl["rename"] = nil
				decl["req"] = "*"
			}
		},
		"unparseable_req": func(p, _ []interface{}) {
			rootDecls(p)[0].(map[string]interface{})["req"] = "^one"
		},
		"malformed_cfg": func(p, n []interface{}) {
			rootDecls(p)[0].(map[string]interface{})["target"] = "cfg(all(unix"
			dep := n[0].(map[string]interface{})["deps"].([]interface{})[0].(map[string]interface{})
			dep["dep_kinds"] = []interface{}{map[string]interface{}{"kind": nil, "target": "cfg(all(unix"}}
		},
	} {
		t.Run("rejects_"+name, func(t *testing.T) {
			if _, err := expectedRustLicensePackages(mutate(t, edit)); err == nil {
				t.Fatal("incomplete or ambiguous graph evidence accepted")
			}
		})
	}
	for name, tc := range map[string]struct {
		edit func([]interface{}, []interface{})
		want int
	}{
		// Resolved feature absent from the manifest: skipped, as krates does.
		"unknown_root_feature": {func(_, n []interface{}) {
			n[0].(map[string]interface{})["features"] = []interface{}{"index-only"}
		}, 3},
		// A feature naming a dependency with no resolve edge (inactive everywhere).
		"feature_without_edge": {func(p, n []interface{}) {
			root(p)["features"] = map[string]interface{}{"wasm": []interface{}{"dep:wasm-only", "wasm-only?/std"}}
			n[0].(map[string]interface{})["features"] = []interface{}{"wasm"}
		}, 3},
		// uses_default_features with no default feature on the dependency.
		"no_default_feature": {func(p, _ []interface{}) {
			rootDecls(p)[0].(map[string]interface{})["uses_default_features"] = true
		}, 3},
		// Predicate-free false target is dropped; mixed predicates are kept.
		"cfg_any_dropped": {func(p, n []interface{}) {
			rootDecls(p)[0].(map[string]interface{})["target"] = "cfg(any())"
			dep := n[0].(map[string]interface{})["deps"].([]interface{})[0].(map[string]interface{})
			dep["dep_kinds"] = []interface{}{map[string]interface{}{"kind": nil, "target": "cfg(any())"}}
		}, 2},
		"cfg_mixed_kept": {func(p, n []interface{}) {
			rootDecls(p)[0].(map[string]interface{})["target"] = "cfg(all(any(), unix))"
			dep := n[0].(map[string]interface{})["deps"].([]interface{})[0].(map[string]interface{})
			dep["dep_kinds"] = []interface{}{map[string]interface{}{"kind": nil, "target": "cfg(all(any(), unix))"}}
		}, 3},
		// An optional declaration with nothing enabling it contributes no edge.
		"optional_inactive": {func(p, _ []interface{}) {
			rootDecls(p)[0].(map[string]interface{})["optional"] = true
		}, 2},
		"optional_activated": {func(p, n []interface{}) {
			rootDecls(p)[0].(map[string]interface{})["optional"] = true
			root(p)["features"] = map[string]interface{}{"remote": []interface{}{"dep:dep"}}
			n[0].(map[string]interface{})["features"] = []interface{}{"remote"}
		}, 3},
	} {
		t.Run("tolerates_"+name, func(t *testing.T) {
			set, err := expectedRustLicensePackages(mutate(t, tc.edit))
			if err != nil || len(set) != tc.want {
				t.Fatalf("got %d packages (%v), want %d", len(set), err, tc.want)
			}
		})
	}
}

func TestRustCfgNeverTrue(t *testing.T) {
	for target, want := range map[string]bool{
		"cfg(any())":                       true,
		"cfg(not(all()))":                  true,
		"cfg(all(any(), any()))":           true,
		"cfg(all())":                       false,
		"cfg(not(any()))":                  false,
		"cfg(all(any(), unix))":            false, // has a predicate: retained
		"cfg(not(all(any(), windows)))":    false,
		`cfg(all(any(), target_os = "x"))`: false,
		"cfg(unix)":                        false,
		"x86_64-unknown-linux-gnu":         false,
	} {
		got, err := rustCfgNeverTrue(target)
		if err != nil || got != want {
			t.Errorf("rustCfgNeverTrue(%q) = %t, %v; want %t", target, got, err, want)
		}
	}
	for _, target := range []string{"cfg(all(unix)", "cfg(not(any(), any()))", "cfg(any(,)", `cfg(target_os = "x)`} {
		if _, err := rustCfgNeverTrue(target); err == nil {
			t.Errorf("rustCfgNeverTrue(%q) accepted malformed cfg", target)
		}
	}
}

func TestRustOmissionDiagnostic(t *testing.T) {
	expected := map[string]rustLicensePackage{
		"dev 0.1.0 path+file:///w/dev":   {ID: "path+file:///w/dev#dev@0.1.0"},
		"norm 0.1.0 path+file:///w/norm": {ID: "path+file:///w/norm#norm@0.1.0"},
	}
	devOnly := map[string]bool{"dev 0.1.0 path+file:///w/dev": true}
	historical := "cargo-deny before 0.20 does not apply licenses.include-dev=true"
	for _, tc := range []struct {
		name, version string
		omitted       []string
		wantHistoric  bool
	}{
		{"old_version_dev_only", "0.19.0", []string{"dev 0.1.0 path+file:///w/dev"}, true},
		{"old_version_mixed", "0.19.0", []string{"dev 0.1.0 path+file:///w/dev", "norm 0.1.0 path+file:///w/norm"}, false},
		{"old_version_non_dev", "0.19.0", []string{"norm 0.1.0 path+file:///w/norm"}, false},
		{"current_version_dev_only", "0.20.2", []string{"dev 0.1.0 path+file:///w/dev"}, false},
		{"unknown_version_dev_only", "", []string{"dev 0.1.0 path+file:///w/dev"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := rustOmissionError(tc.omitted, expected, rustOmissionContext{DenyVersion: tc.version, DevOnly: devOnly}).Error()
			if strings.Contains(msg, historical) != tc.wantHistoric {
				t.Fatalf("historical cargo-deny advice gating wrong: %s", msg)
			}
			if !tc.wantHistoric {
				for _, banned := range []string{"before 0.20", "include-dev", "upgrade", "use cargo-deny"} {
					if strings.Contains(msg, banned) {
						t.Fatalf("neutral omission message must not mention %q: %s", banned, msg)
					}
				}
				want := tc.version
				if want == "" {
					want = "(version unknown)"
				}
				if !strings.Contains(msg, "cargo-deny "+want+" list did not include") || !strings.Contains(msg, "goneat license-graph defect") {
					t.Fatalf("neutral omission message incomplete: %s", msg)
				}
			}
			for _, key := range tc.omitted {
				if !strings.Contains(msg, expected[key].ID) {
					t.Fatalf("omitted ID %s not reported: %s", expected[key].ID, msg)
				}
			}
		})
	}
}
