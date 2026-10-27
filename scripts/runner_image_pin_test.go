package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// expectedGoneatToolsRunnerTag is the single fulmen-toolbox tag every
// goneat-tools-runner image reference in .github/workflows must use.
const expectedGoneatToolsRunnerTag = "v0.5.14"

var goneatToolsRunnerImage = regexp.MustCompile(
	`ghcr\.io/fulmenhq/goneat-tools-runner-(glibc|musl):([^\s"'#]+)`,
)

func TestGoneatToolsRunnerPinsShareExpectedTag(t *testing.T) {
	dir := filepath.Join("..", ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var found int
	var sawGlibc, sawMusl bool
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		matches := goneatToolsRunnerImage.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			found++
			flavor, tag := match[1], match[2]
			switch flavor {
			case "glibc":
				sawGlibc = true
			case "musl":
				sawMusl = true
			}
			if tag != expectedGoneatToolsRunnerTag {
				t.Errorf("%s: %s uses tag %q, want %q", path, match[0], tag, expectedGoneatToolsRunnerTag)
			}
		}
	}

	if found == 0 {
		t.Fatal("no goneat-tools-runner image references found in .github/workflows")
	}
	if found < 5 {
		t.Errorf("found %d goneat-tools-runner image references, want at least 5", found)
	}
	if !sawGlibc {
		t.Error("missing goneat-tools-runner-glibc image reference")
	}
	if !sawMusl {
		t.Error("missing goneat-tools-runner-musl image reference")
	}
}
