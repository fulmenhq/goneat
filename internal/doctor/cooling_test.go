package doctor

import "testing"

func TestInferRepoFromGoInstallPackage(t *testing.T) {
	if got := inferRepoFromGoInstallPackage("github.com/rhysd/actionlint/cmd/actionlint@latest"); got != "rhysd/actionlint" {
		t.Fatalf("expected rhysd/actionlint, got %q", got)
	}
	if got := inferRepoFromGoInstallPackage("github.com/google/go-licenses@latest"); got != "google/go-licenses" {
		t.Fatalf("expected google/go-licenses, got %q", got)
	}
	if got := inferRepoFromGoInstallPackage("golang.org/x/vuln/cmd/govulncheck@latest"); got != "" {
		t.Fatalf("expected empty for non-github module, got %q", got)
	}
}

func TestInferRepoFromPythonInstall(t *testing.T) {
	cmds := map[string]string{
		"darwin": "uv tool install --system yamllint",
	}
	if got := inferRepoFromPythonInstall(cmds); got != "pypi:yamllint" {
		t.Fatalf("expected pypi:yamllint, got %q", got)
	}

	cmds = map[string]string{
		"linux": "pip install yamllint",
	}
	if got := inferRepoFromPythonInstall(cmds); got != "pypi:yamllint" {
		t.Fatalf("expected pypi:yamllint, got %q", got)
	}
}
