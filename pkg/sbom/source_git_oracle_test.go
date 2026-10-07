package sbom

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Git is an independent oracle only for this supported root-ignore subset.
// Go evidence exceptions and hard exclusions deliberately differ from Git.
func TestSourceIgnoreGitOracle(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git unavailable for independent optional subset oracle")
	}
	for _, tc := range []struct {
		name, rules string
		paths       []string
	}{
		{"reopened-directory", "**/sumpter\n!cmd/sumpter/\n", []string{"cmd/sumpter/main.go", "cmd/sumpter/sumpter", "other/sumpter"}},
		{"closed-ancestor", ".cursor/\n!.cursor/rules/\n!.cursor/rules/*\n", []string{".cursor/rules/x.md"}},
		{"reopened-ancestors", ".cursor/\n!.cursor/\n.cursor/*\n!.cursor/rules/\n!.cursor/rules/*\n.cursor/rules/private/\n", []string{".cursor/rules/x.md", ".cursor/rules/private/x.md"}},
		{"anchored-files", "/assets/*\n!/assets/.gitkeep\n", []string{"assets/.gitkeep", "assets/data.txt", "nested/assets/data.txt"}},
		{"later-reexclusion", "/assets/*\n!/assets/.gitkeep\n/assets/.gitkeep\n", []string{"assets/.gitkeep"}},
		{"basename-negation", "**/Dockerfile.dev\n**/Dockerfile.local\n!Dockerfile.*\n", []string{"nested/Dockerfile.local", "Dockerfile.dev"}},
		{"escaped-leading-bang", "\\!literal\n", []string{"!literal", "nested/!literal"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			command := exec.Command(git, "-C", root, "init", "--quiet")
			command.Env = sourceGitOracleEnvironment()
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("initialize isolated Git oracle: %v %s", err, output)
			}
			writeSourceFixture(t, root, ".gitignore", tc.rules)
			for _, name := range tc.paths {
				writeSourceFixture(t, root, name, "oracle input")
			}
			fsys := os.DirFS(root)
			patterns, err := loadSourcePatterns(fsys, SourceOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var entries []sourceEntry
			for _, name := range tc.paths {
				entries = append(entries, sourceEntry{name: name})
			}
			selection, err := planSourceSelection(entries, patterns, SourceOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range tc.paths {
				command := exec.Command(git, "-C", root, "-c", "core.excludesFile=", "check-ignore", "--no-index", "-q", "--", name)
				command.Env = sourceGitOracleEnvironment()
				err := command.Run()
				var exit *exec.ExitError
				if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
					t.Fatalf("Git oracle failed for %q: %v", name, err)
				}
				gitSelected := err != nil // exit 1 means not ignored
				if selection.selected[name] != gitSelected {
					t.Fatalf("subset selection disagrees with Git for %q: selected=%v git=%v", name, selection.selected[name], gitSelected)
				}
			}
		})
	}
}

func sourceGitOracleEnvironment() []string {
	// These settings isolate the synthetic oracle from a caller repository and
	// user/global/system Git settings; no remote or consumer repository is used.
	var environment []string
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if !strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			environment = append(environment, value)
		}
	}
	return append(environment, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
}
