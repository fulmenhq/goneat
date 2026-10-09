package main

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNativeWorkflowReleaseMatrixAndRequiredAggregate(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/native-contracts.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			If       string                 `yaml:"if"`
			Needs    []string               `yaml:"needs"`
			Steps    []struct{ Run string } `yaml:"steps"`
			Strategy struct {
				Matrix struct {
					Include []struct{ Runner, Target string } `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"linux/amd64":   "ubuntu-latest-x64-m",
		"linux/arm64":   "ubuntu-latest-arm64-m",
		"darwin/arm64":  "macos-15",
		"windows/amd64": "windows-latest",
		"windows/arm64": "windows-latest-arm64-s",
	}
	for _, cell := range workflow.Jobs["native"].Strategy.Matrix.Include {
		if runner, ok := expected[cell.Target]; !ok || cell.Runner != runner {
			t.Fatalf("unexpected or duplicate native cell: %+v", cell)
		}
		delete(expected, cell.Target)
	}
	if len(expected) != 0 {
		t.Fatalf("missing native cells: %v", expected)
	}
	required := workflow.Jobs["required"]
	if required.If != "always()" || len(required.Needs) != 2 || required.Needs[0] != "candidate" || required.Needs[1] != "native" {
		t.Fatalf("aggregate must run even if producer/cells fail or skip: %+v", required)
	}
	requireTools(t, "bash")
	for _, results := range [][2]string{{"success", "success"}, {"failure", "success"}, {"success", "failure"}, {"cancelled", "success"}, {"success", "skipped"}, {"skipped", "success"}} {
		env := append(os.Environ(), "CANDIDATE_RESULT="+results[0], "NATIVE_RESULT="+results[1])
		output, err := runErr(".", env, "bash", "-c", required.Steps[0].Run)
		wantSuccess := results[0] == "success" && results[1] == "success"
		if (err == nil) != wantSuccess {
			t.Fatalf("aggregate accepted incorrect results %v: %v\n%s", results, err, output)
		}
	}
}
