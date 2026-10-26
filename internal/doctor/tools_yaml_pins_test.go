package doctor

import (
	"strings"
	"testing"
)

// TestRepositoryYQPinIsMikefarah fails if .goneat/tools.yaml stops pinning
// mikefarah/yq (minimum 4.40.0) as a userland go-install, which is what
// distinguishes it from kislyuk/yq on distro PATH.
func TestRepositoryYQPinIsMikefarah(t *testing.T) {
	t.Parallel()
	cfg, err := LoadToolsConfig()
	if err != nil {
		t.Fatalf("LoadToolsConfig: %v", err)
	}
	yq, ok := cfg.Tools["yq"]
	if !ok {
		t.Fatal("yq missing from .goneat/tools.yaml")
	}
	const wantPackage = "github.com/mikefarah/yq/v4@v4.53.3"
	if yq.InstallPackage != wantPackage {
		t.Fatalf("yq install_package = %q, want %q", yq.InstallPackage, wantPackage)
	}
	if yq.MinimumVersion != "4.40.0" {
		t.Fatalf("yq minimum_version = %q, want 4.40.0", yq.MinimumVersion)
	}
	if yq.RecommendedVersion != "4.53.3" {
		t.Fatalf("yq recommended_version = %q, want 4.53.3", yq.RecommendedVersion)
	}
	if !contains(yq.InstallerPriority["linux"], "go-install") {
		t.Fatalf("yq linux installer_priority = %v, want go-install first-class", yq.InstallerPriority["linux"])
	}
}

// TestRepositoryMinisignIsReportOnlyOnLinux fails if Linux minisign grows an
// auto-run sudo installer (apt-get) or drops per-OS install instructions.
func TestRepositoryMinisignIsReportOnlyOnLinux(t *testing.T) {
	t.Parallel()
	cfg, err := LoadToolsConfig()
	if err != nil {
		t.Fatalf("LoadToolsConfig: %v", err)
	}
	ms, ok := cfg.Tools["minisign"]
	if !ok {
		t.Fatal("minisign missing from .goneat/tools.yaml")
	}
	linux := ms.InstallerPriority["linux"]
	for _, kind := range []string{"apt-get", "apt", "pacman", "dnf", "yum"} {
		if contains(linux, kind) {
			t.Fatalf("minisign linux installer_priority includes %q (would auto-run sudo); got %v", kind, linux)
		}
	}
	if !contains(linux, "manual") {
		t.Fatalf("minisign linux installer_priority = %v, want manual (report-only)", linux)
	}
	if ms.InstallCommands["brew"] != "brew install minisign" {
		t.Fatalf("minisign brew install_commands = %q, want brew install minisign", ms.InstallCommands["brew"])
	}
	if ms.InstallCommands["scoop"] != "scoop install minisign" {
		t.Fatalf("minisign scoop install_commands = %q, want scoop install minisign", ms.InstallCommands["scoop"])
	}
	var all strings.Builder
	for _, cmd := range ms.InstallCommands {
		all.WriteString(cmd)
		all.WriteByte('\n')
	}
	blob := all.String()
	for _, want := range []string{
		"sudo apt-get install -y minisign",
		"sudo apk add minisign",
		"brew install minisign",
		"scoop install minisign",
	} {
		if !strings.Contains(blob, want) {
			t.Errorf("minisign install_commands missing %q; got %#v", want, ms.InstallCommands)
		}
	}
}
