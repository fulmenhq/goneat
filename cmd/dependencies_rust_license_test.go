package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/pkg/dependencies"
	"github.com/spf13/cobra"
)

func TestDependenciesRustLicenseOutput(t *testing.T) {
	if !dependencies.IsCargoAvailable() || !dependencies.CheckCargoDenyPresence().Present {
		t.Skip("cargo and cargo-deny required")
	}
	t.Setenv("CARGO_NET_OFFLINE", "true")
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"Cargo.toml": "[package]\nname='cli-license-root'\nversion='0.1.0'\nedition='2021'\nlicense='MIT'\n",
		"src/lib.rs": "pub fn f() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"pass", "ban", "config_error"} {
		for _, format := range []string{"json", "text"} {
			t.Run(mode+"/"+format, func(t *testing.T) {
				policy := "[licenses]\nallow=['MIT']\n"
				if mode == "ban" {
					policy += "[bans]\ndeny=[{name='cli-license-root'}]\n"
				}
				if mode == "config_error" {
					policy = "[licenses"
				}
				if err := os.WriteFile(filepath.Join(root, "deny.toml"), []byte(policy), 0o600); err != nil {
					t.Fatal(err)
				}
				output := filepath.Join(root, "result."+format)
				command := &cobra.Command{Use: "dependencies"}
				command.Flags().Bool("licenses", true, "")
				command.Flags().Bool("cooling", false, "")
				command.Flags().String("policy", "", "")
				command.Flags().String("format", format, "")
				command.Flags().String("output", output, "")
				command.Flags().String("fail-on", "none", "")
				err := runDependencies(command, []string{root})
				wantFailure := mode != "pass"
				if (err != nil) != wantFailure {
					t.Fatalf("--fail-on none cleared Rust gate: %v", err)
				}
				data, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if format == "json" {
					var result dependencies.AnalysisResult
					if err := json.Unmarshal(data, &result); err != nil {
						t.Fatalf("invalid machine output: %v\n%s", err, data)
					}
					if result.Passed == wantFailure {
						t.Fatalf("false JSON gate: %s", data)
					}
					if mode == "pass" && (len(result.Dependencies) != 1 || result.Dependencies[0].Metadata["cargo_package_id"] == "") {
						t.Fatalf("incomplete identity: %s", data)
					}
				} else {
					if !strings.Contains(string(data), "Passed: "+map[bool]string{true: "false", false: "true"}[wantFailure]) {
						t.Fatalf("false human gate: %s", data)
					}
					if mode == "ban" && !strings.Contains(string(data), "banned") {
						t.Fatalf("policy diagnostic omitted: %s", data)
					}
					if mode == "config_error" && !strings.Contains(string(data), "license_error") {
						t.Fatalf("collection diagnostic omitted: %s", data)
					}
				}
			})
		}
	}
}
