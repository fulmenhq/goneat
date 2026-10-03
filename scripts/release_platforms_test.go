package main

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleasePackageMatrix(t *testing.T) {
	requireTools(t, "bash", "tar", "zip")
	dir := t.TempDir()
	for _, name := range []string{"package-artifacts.sh", "release-platforms.sh"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("VERSION", "v1.2.3\n")
	write("LICENSE", "license")
	write("NOTICE", "notice")
	targets := []string{"linux_amd64", "linux_arm64", "darwin_arm64", "windows_amd64", "windows_arm64"}
	for _, target := range targets[:4] {
		name := "bin/goneat-" + strings.ReplaceAll(target, "_", "-")
		if strings.HasPrefix(target, "windows_") {
			name += ".exe"
		}
		write(name, target)
	}
	write("dist/release/SHA256SUMS", "original manifest")
	output, err := runErr(dir, os.Environ(), "bash", "package-artifacts.sh")
	if err == nil || !strings.Contains(output, "Missing required binary") {
		t.Fatalf("incomplete matrix must fail: %v\n%s", err, output)
	}
	original, err := os.ReadFile(filepath.Join(dir, "dist/release/SHA256SUMS"))
	if err != nil || string(original) != "original manifest" {
		t.Fatalf("missing-binary preflight modified existing manifest: %s, %v", original, err)
	}
	write("bin/goneat-windows-arm64.exe", "windows_arm64")
	write("dist/release/goneat_v1.2.3_darwin_amd64.tar.gz", "retired")
	output, err = runErr(dir, os.Environ(), "bash", "package-artifacts.sh")
	if err == nil || !strings.Contains(output, "Unexpected release archive") {
		t.Fatalf("retired archive must fail: %v\n%s", err, output)
	}
	if err := os.Remove(filepath.Join(dir, "dist/release/goneat_v1.2.3_darwin_amd64.tar.gz")); err != nil {
		t.Fatal(err)
	}
	env := append(cleanEnv(), "SIGN=0")
	run(t, dir, env, "bash", "package-artifacts.sh")
	for _, algorithm := range []string{"256", "512"} {
		manifest, err := os.ReadFile(filepath.Join(dir, "dist/release/SHA"+algorithm+"SUMS"))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(manifest)), "\n")
		if len(lines) != len(targets) {
			t.Fatalf("SHA%s manifest has %d entries", algorithm, len(lines))
		}
		for i, target := range targets {
			ext := ".tar.gz"
			if strings.HasPrefix(target, "windows_") {
				ext = ".zip"
			}
			name := "goneat_v1.2.3_" + target + ext
			archive, err := os.ReadFile(filepath.Join(dir, "dist/release", name))
			if err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(archive))
			if algorithm == "512" {
				digest = fmt.Sprintf("%x", sha512.Sum512(archive))
			}
			if lines[i] != digest+"  "+name {
				t.Fatalf("invalid SHA%s manifest entry: %s", algorithm, lines[i])
			}
		}
	}
}

func TestVerifyEmbeddedMirrorsFailsClosed(t *testing.T) {
	requireTools(t, "bash", "rsync")
	dir := t.TempDir()
	data, err := os.ReadFile("verify-embeds.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts/verify-embeds.sh"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"templates", "schemas", "config"} {
		for _, path := range []string{kind, "internal/assets/embedded_" + kind + "/" + kind} {
			if err := os.MkdirAll(filepath.Join(dir, path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, path, "fixture"), []byte("same"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	run(t, dir, os.Environ(), "bash", "scripts/verify-embeds.sh")
	mirror := filepath.Join(dir, "internal/assets/embedded_config/config/fixture")
	if err := os.WriteFile(mirror, []byte("drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := runErr(dir, os.Environ(), "bash", "scripts/verify-embeds.sh")
	if err == nil || !strings.Contains(output, "config: drift detected") {
		t.Fatalf("config drift must fail: %v\n%s", err, output)
	}
	if err := os.WriteFile(mirror, []byte("same"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(mirror), "stale"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err = runErr(dir, os.Environ(), "bash", "scripts/verify-embeds.sh")
	if err == nil || !strings.Contains(output, "*deleting") {
		t.Fatalf("stale mirror files must fail: %v\n%s", err, output)
	}
}
