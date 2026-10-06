package sbom

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Collector-boundary proof, separate from doublestar's in-process matcher.
// Full source-capture, subject/provenance, and consumer controls are separate.
func TestSourceLiteralSyftExclusions(t *testing.T) {
	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Skipf("Syft unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	version, err := invoker.GetVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Capture canonicalizes its outside-target staging parent before creation.
	// Mirror that boundary here: Syft's ./ exclusion base does not consistently
	// follow host aliases such as macOS /var -> /private/var.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"[named].bin", "sibling-stale"} {
		in, err := os.Open(invoker.syftPath)
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.OpenFile(filepath.Join(root, "bin", name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = in.Close()
			t.Fatal(err)
		}
		_, copyErr := io.Copy(out, in)
		inErr, outErr := in.Close(), out.Close()
		if copyErr != nil || inErr != nil || outErr != nil {
			t.Fatalf("fixture copy failed: %v %v %v", copyErr, inErr, outErr)
		}
	}
	patterns := map[string][]string{
		"all":             nil,
		"escaped-literal": {`./bin/\[named\].bin`},
		"only-named":      {"./bin/sibling-stale"},
		"broad":           {"./bin/**"},
	}
	for name, excludes := range patterns {
		t.Run(name, func(t *testing.T) {
			args := []string{"scan", root, "--override-default-catalogers", "go-module-binary-cataloger", "--output", "syft-json"}
			for _, exclude := range excludes {
				args = append(args, "--exclude", exclude)
			}
			cmd := exec.CommandContext(ctx, invoker.syftPath, args...)
			cmd.Env = append(os.Environ(), "SYFT_CHECK_FOR_APP_UPDATE=false", "SYFT_GOLANG_SEARCH_LOCAL_MOD_CACHE_LICENSES=false", "SYFT_GOLANG_SEARCH_REMOTE_LICENSES=false")
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("Syft %s argv=%q failed: %v", version, args, err)
			}
			var document struct {
				Artifacts []struct {
					Locations []struct {
						Path string `json:"path"`
					} `json:"locations"`
				} `json:"artifacts"`
			}
			if err := json.Unmarshal(out, &document); err != nil {
				t.Fatal(err)
			}
			named, sibling := false, false
			for _, artifact := range document.Artifacts {
				for _, location := range artifact.Locations {
					path := filepath.ToSlash(location.Path)
					named = named || strings.HasSuffix(path, "/[named].bin")
					sibling = sibling || strings.HasSuffix(path, "/sibling-stale")
				}
			}
			wantNamed, wantSibling := name == "all" || name == "only-named", name == "all" || name == "escaped-literal"
			if named != wantNamed || sibling != wantSibling {
				t.Fatalf("Syft %s literal select mismatch: named=%v sibling=%v; argv=%q", version, named, sibling, args)
			}
			t.Logf("Syft %s argv=%q packages=%d named=%v sibling=%v", version, args, len(document.Artifacts), named, sibling)
			for _, file := range []string{"[named].bin", "sibling-stale"} {
				if _, err := os.Stat(filepath.Join(root, "bin", file)); err != nil {
					t.Fatal("original fixture artifact removed")
				}
			}
		})
	}
}
