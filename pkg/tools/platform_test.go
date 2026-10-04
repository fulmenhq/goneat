package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestArtifactPlatformSelection(t *testing.T) {
	artifacts := VersionArtifacts{
		DarwinAMD64:  &Artifact{URL: "darwin-amd64"},
		DarwinARM64:  &Artifact{URL: "darwin-arm64"},
		LinuxAMD64:   &Artifact{URL: "linux-amd64"},
		LinuxARM64:   &Artifact{URL: "linux-arm64"},
		WindowsAMD64: &Artifact{URL: "windows-amd64"},
		WindowsARM64: &Artifact{URL: "windows-arm64"},
	}
	for _, platform := range []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"} {
		t.Run(platform, func(t *testing.T) {
			parts := strings.Split(platform, "/")
			artifact, err := selectArtifact(artifacts, parts[0], parts[1])
			if err != nil || artifact.URL != strings.ReplaceAll(platform, "/", "-") {
				t.Fatalf("selected %+v, error %v", artifact, err)
			}
		})
	}
	artifacts.WindowsARM64 = nil
	if artifact, err := selectArtifact(artifacts, "windows", "arm64"); err == nil || artifact != nil {
		t.Fatalf("must refuse missing ARM64 artifact, not fall back to amd64: %+v, %v", artifact, err)
	}
	if artifact, err := selectArtifact(artifacts, "linux", "386"); err == nil || artifact != nil {
		t.Fatalf("must refuse unsupported platform: %+v, %v", artifact, err)
	}
}

func TestWindowsARM64ArtifactConfigRoundTrip(t *testing.T) {
	data := []byte(`scopes:
  native:
    description: Native artifact test scope
    tools: [native-tool]
tools:
  native-tool:
    name: native-tool
    description: Native artifact test
    kind: system
    detect_command: native-tool --version
    artifacts:
      default_version: "1.0.0"
      versions:
        "1.0.0":
          windows_arm64:
            url: https://example.test/native-arm64.zip
            sha256: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
            extract_path: native-tool.exe
`)
	if err := ValidateBytes(data); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"yaml", "json"} {
		var encoded []byte
		if format == "yaml" {
			encoded, err = yaml.Marshal(cfg)
		} else {
			encoded, err = json.Marshal(cfg)
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateBytes(encoded); err != nil {
			t.Fatal(err)
		}
		roundTrip, err := ParseConfig(encoded)
		if err != nil {
			t.Fatal(err)
		}
		artifact := roundTrip.Tools["native-tool"].Artifacts.Versions["1.0.0"].WindowsARM64
		if artifact == nil || artifact.ExtractPath != "native-tool.exe" || len(artifact.SHA256) != 64 {
			t.Fatalf("%s round trip lost ARM64 artifact: %+v", format, artifact)
		}
	}
	bad := strings.Replace(string(data), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "invalid", 1)
	if err := ValidateBytes([]byte(bad)); err == nil {
		t.Fatal("invalid ARM64 checksum must be rejected")
	}
}
