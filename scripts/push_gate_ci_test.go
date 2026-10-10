package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPushGateCIFetchesHistoryAndInstallsTools(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string         `yaml:"name"`
				Uses string         `yaml:"uses"`
				Run  string         `yaml:"run"`
				With map[string]any `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	job, ok := workflow.Jobs["build-test-lint"]
	if !ok {
		t.Fatal("missing build-test-lint job")
	}
	checkout, install, preflight, prepush := -1, -1, -1, -1
	for i, step := range job.Steps {
		switch {
		case step.Uses == "actions/checkout@v4" && checkout < 0:
			checkout = i
			depth, present := step.With["fetch-depth"]
			if !present {
				t.Fatal("build-test-lint checkout has no fetch-depth")
			}
			if number, ok := asInt(depth); !ok || number != 0 {
				t.Fatalf("build-test-lint fetch-depth = %#v, want 0", depth)
			}
		case strings.Contains(step.Run, "scripts/install-push-gate-tools.sh"):
			install = i
		case strings.Contains(step.Run, "scripts/push-gate-preflight.py"):
			preflight = i
		case strings.TrimSpace(step.Run) == "make prepush":
			prepush = i
		}
	}
	if checkout < 0 || install < 0 || preflight < 0 || prepush < 0 {
		t.Fatalf("missing push-gate steps: checkout=%d install=%d preflight=%d prepush=%d", checkout, install, preflight, prepush)
	}
	if !(checkout < install && install < preflight && preflight < prepush) {
		t.Fatalf("push-gate step order checkout=%d install=%d preflight=%d prepush=%d", checkout, install, preflight, prepush)
	}

	script, err := os.ReadFile("install-push-gate-tools.sh")
	if err != nil {
		t.Fatal(err)
	}
	tools, err := os.ReadFile("../.goneat/tools.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"yamllint", "shellcheck"} {
		minimum := toolMinimum(t, string(tools), tool)
		recommended := toolRecommended(t, string(tools), tool)
		if !strings.Contains(string(script), recommended) {
			t.Fatalf("install-push-gate-tools.sh does not pin %s %s", tool, recommended)
		}
		if versionLess(recommended, minimum) {
			t.Fatalf("%s recommended %s is below minimum %s", tool, recommended, minimum)
		}
	}
}

func toolMinimum(t *testing.T, text, name string) string {
	t.Helper()
	return toolVersionField(t, text, name, "minimum_version")
}

func toolRecommended(t *testing.T, text, name string) string {
	t.Helper()
	return toolVersionField(t, text, name, "recommended_version")
}

func toolVersionField(t *testing.T, text, name, field string) string {
	t.Helper()
	section := regexp.MustCompile(`(?s)\n  ` + regexp.QuoteMeta(name) + `:\n(.*?)(?:\n  [A-Za-z0-9_-]+:\n|\z)`)
	found := section.FindStringSubmatch(text)
	if found == nil {
		t.Fatalf("missing %s in tools.yaml", name)
	}
	version := regexp.MustCompile(field + `:\s*"([^"]+)"`).FindStringSubmatch(found[1])
	if version == nil {
		t.Fatalf("missing %s for %s", field, name)
	}
	return version[1]
}

func versionLess(left, right string) bool {
	lp, rp := splitVersion(left), splitVersion(right)
	n := len(lp)
	if len(rp) > n {
		n = len(rp)
	}
	for i := 0; i < n; i++ {
		var a, b int
		if i < len(lp) {
			a = lp[i]
		}
		if i < len(rp) {
			b = rp[i]
		}
		if a != b {
			return a < b
		}
	}
	return false
}

func splitVersion(value string) []int {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

func asInt(value any) (int, bool) {
	switch n := value.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case uint64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
