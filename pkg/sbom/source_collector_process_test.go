package sbom

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The package's own native test executable is a portable collector injector.
// Its command gate never enters when running the ordinary test suite.
func TestMain(m *testing.M) {
	if action := os.Getenv("GONEAT_SBOM_TEST_COLLECTOR_ACTION"); action != "" && len(os.Args) > 1 && (os.Args[1] == "scan" || os.Args[1] == "version") {
		os.Exit(runSourceCollectorTestProcess(action))
	}
	os.Exit(m.Run())
}

type sourceCollectorTestReceipt struct {
	Args              []string `json:"args"`
	CWD               string   `json:"cwd"`
	Config            string   `json:"config"`
	PolicyEnvironment []string `json:"policy_environment_keys"`
}

func runSourceCollectorTestProcess(action string) int {
	if os.Args[1] == "version" {
		fmt.Println(`{"version":"test-collector"}`)
		return 0
	}
	var receipt sourceCollectorTestReceipt
	receipt.Args = os.Args[1:]
	for i, arg := range receipt.Args {
		if arg == "--config" && i+1 < len(receipt.Args) {
			receipt.Config = receipt.Args[i+1]
		}
	}
	var err error
	receipt.CWD, err = os.Getwd()
	if err != nil {
		return 10
	}
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if strings.HasPrefix(strings.ToUpper(key), "SYFT_") {
			receipt.PolicyEnvironment = append(receipt.PolicyEnvironment, key)
		}
	}
	data, err := json.Marshal(receipt)
	if err != nil || os.WriteFile(os.Getenv("GONEAT_SBOM_TEST_COLLECTOR_RECEIPT"), data, 0o600) != nil {
		return 11
	}
	switch action {
	case "collector-error":
		return 9
	case "config-mutation":
		if err := os.WriteFile(receipt.Config, []byte("exclude: [ '**' ]\n"), 0o600); err != nil {
			return 12
		}
	case "working-state-mutation":
		if err := os.WriteFile(filepath.Join(receipt.CWD, "unrequested-state"), []byte("changed"), 0o600); err != nil {
			return 13
		}
	case "collector-residue":
		data, _ := json.Marshal(map[string]any{
			"bomFormat": "CycloneDX", "specVersion": "1.6", "version": 1,
			"properties": []map[string]string{{"name": "opaque-retained-evidence", "value": receipt.Config}},
		})
		fmt.Println(string(data))
		return 0
	}
	fmt.Println(`{"bomFormat":"CycloneDX","specVersion":"1.6","version":1}`)
	return 0
}

func TestSourceCollectorInvocationPublication(t *testing.T) {
	for _, route := range []string{"source", "artifact"} {
		for _, action := range []string{"success", "collector-error", "config-mutation", "working-state-mutation", "collector-residue"} {
			t.Run(route+"/"+action, func(t *testing.T) {
				root, work := t.TempDir(), t.TempDir()
				writeSourceFixture(t, root, ".gitignore", "!keep\n")
				writeSourceFixture(t, root, ".syft.yaml", "exclude: [ '**' ]\n")
				writeSourceFixture(t, root, "artifact", "retained artifact bytes")
				t.Chdir(root)
				t.Setenv("SYFT_CONFIG", filepath.Join(root, ".syft.yaml"))
				t.Setenv("SYFT_EXCLUDE", "**")
				t.Setenv("SYFT_SELECT_CATALOGERS", "none")
				t.Setenv("GONEAT_SBOM_TEST_COLLECTOR_ACTION", action)
				receiptPath := filepath.Join(work, "receipt.json")
				t.Setenv("GONEAT_SBOM_TEST_COLLECTOR_RECEIPT", receiptPath)
				binary, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				target := root
				if route == "artifact" {
					target = filepath.Join(root, "artifact")
				}
				output := filepath.Join(work, "inventory.json")
				if err := os.WriteFile(output, []byte("sentinel"), 0o600); err != nil {
					t.Fatal(err)
				}
				invoker := &SyftInvoker{syftPath: binary}
				result, generateErr := invoker.Generate(context.Background(), Config{TargetPath: target, OutputPath: output})
				if (generateErr == nil) != (action == "success") || (result != nil) != (action == "success") {
					t.Fatalf("unexpected publication result: result=%v err=%v", result, generateErr)
				}
				content, err := os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				if action != "success" && string(content) != "sentinel" {
					t.Fatal("failed collector replaced output sentinel")
				}
				data, err := os.ReadFile(receiptPath)
				if err != nil {
					t.Fatal(err)
				}
				var receipt sourceCollectorTestReceipt
				if err := json.Unmarshal(data, &receipt); err != nil {
					t.Fatal(err)
				}
				if receipt.Config == "" || len(receipt.PolicyEnvironment) != 0 || filepath.Dir(receipt.Config) != receipt.CWD || containedSourcePath(root, receipt.CWD) {
					t.Fatal("collector did not receive isolated effective config/CWD/environment")
				}
				if _, err := os.Stat(receipt.CWD); !os.IsNotExist(err) {
					t.Fatalf("collector working state not cleaned: %v", err)
				}
				if route == "source" {
					if _, err := os.Stat(receipt.Args[1]); !os.IsNotExist(err) {
						t.Fatalf("source capture not cleaned: %v", err)
					}
				}
				original, err := os.ReadFile(filepath.Join(root, "artifact"))
				if err != nil || string(original) != "retained artifact bytes" {
					t.Fatalf("original changed: %v", err)
				}
				if action == "collector-error" && !strings.Contains(generateErr.Error(), "exit status 9") {
					t.Fatalf("primary collection error lost: %v", generateErr)
				}
			})
		}
	}
}
