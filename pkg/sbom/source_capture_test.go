package sbom

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func writeSourceFixture(t *testing.T, root, name, content string) {
	t.Helper()
	name = filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSourceCapturePrivateComplete(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	writeSourceFixture(t, root, ".gitignore", "bin/\n")
	writeSourceFixture(t, root, "bin/named", "current")
	writeSourceFixture(t, root, "bin/stale", "stale")
	writeSourceFixture(t, root, "dist/unrelated", "unrelated")
	writeSourceFixture(t, root, "go.mod", "module example.com/fixture\n")
	capture, err := captureSource(context.Background(), root, parent, SourceOptions{ForceInclude: []string{"bin/named"}}, sourceLimits{entries: 100, bytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := capture.cleanup(); err != nil {
			t.Error(err)
		}
	})
	if capture.original != root || capture.digest == "" || !reflect.DeepEqual(capture.excludes, []string{"./bin/stale", "./dist/unrelated"}) {
		t.Fatalf("bad capture: %+v", capture)
	}
	if err := capture.verify(context.Background()); err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(capture.path)
	if err != nil || canonical != capture.path {
		t.Fatalf("noncanonical collector staging root: %s %s %v", capture.path, canonical, err)
	}
	for _, name := range []string{"bin/named", "bin/stale", "dist/unrelated"} {
		original, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		staged, err := os.ReadFile(filepath.Join(capture.path, name))
		if err != nil || !reflect.DeepEqual(original, staged) {
			t.Fatalf("excluded content not copied: %s: %v", name, err)
		}
	}
	// Subsequent live changes do not redefine the reproducible captured subject.
	writeSourceFixture(t, root, "bin/named", "live tree changed after capture")
	if err := capture.verify(context.Background()); err != nil {
		t.Fatal(err)
	}
	writeSourceFixture(t, capture.path, "bin/named", "mutated snapshot")
	if err := capture.verify(context.Background()); err == nil {
		t.Fatal("snapshot mutation accepted")
	}
	if err := capture.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(capture.path); !os.IsNotExist(err) {
		t.Fatalf("snapshot remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin/stale")); err != nil {
		t.Fatal("original artifact removed")
	}
}

func TestSourceCaptureErrorsCleanup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		setup  func(*testing.T, string)
		limits sourceLimits
		opts   SourceOptions
	}{
		{"entry-limit", func(t *testing.T, root string) { writeSourceFixture(t, root, "bin/excluded", "x") }, sourceLimits{entries: 2, bytes: 100}, SourceOptions{}},
		{"byte-limit", func(t *testing.T, root string) { writeSourceFixture(t, root, "bin/excluded", "12345") }, sourceLimits{entries: 10, bytes: 4}, SourceOptions{}},
		{"bad-policy", func(t *testing.T, root string) { writeSourceFixture(t, root, ".goneatignore", "!restore\n") }, sourceLimits{entries: 10, bytes: 100}, SourceOptions{}},
		{"missing-force", func(t *testing.T, root string) {}, sourceLimits{entries: 10, bytes: 100}, SourceOptions{ForceInclude: []string{"missing"}}},
		{"unreadable-file", func(t *testing.T, root string) {
			writeSourceFixture(t, root, "bin/excluded", "x")
			if err := os.Chmod(filepath.Join(root, "bin/excluded"), 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "bin/excluded"), 0o600) })
		}, sourceLimits{entries: 10, bytes: 100}, SourceOptions{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && tc.name == "unreadable-file" {
				t.Skip("Windows chmod does not represent POSIX read permissions")
			}
			root, parent := t.TempDir(), t.TempDir()
			tc.setup(t, root)
			capture, err := captureSource(context.Background(), root, parent, tc.opts, tc.limits)
			if err == nil || capture != nil {
				t.Fatalf("bad subject accepted: %v", err)
			}
			left, err := os.ReadDir(parent)
			if err != nil || len(left) != 0 {
				t.Fatalf("failed capture retained private data: %v %v", left, err)
			}
		})
	}
}

func TestSourceCaptureRejectsSymlinksEvenExcluded(t *testing.T) {
	prepareSourceSymlinkFixture(t)
	root, parent := t.TempDir(), t.TempDir()
	writeSourceFixture(t, root, "bin/excluded", "x")
	for _, target := range []string{filepath.Join(root, "bin/excluded"), parent} {
		link := filepath.Join(root, "bin/link")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		capture, err := captureSource(context.Background(), root, parent, SourceOptions{NoIgnore: true}, sourceLimits{entries: 10, bytes: 100})
		if err == nil || capture != nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("link accepted: %v", err)
		}
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSourceCaptureBoundariesAndCancellation(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	writeSourceFixture(t, root, "file", "1234")
	capture, err := captureSource(context.Background(), root, parent, SourceOptions{}, sourceLimits{entries: 2, bytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := capture.cleanup(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := captureSource(ctx, root, parent, SourceOptions{}, sourceLimits{entries: 2, bytes: 4}); err == nil {
		t.Fatal("cancellation accepted")
	}
	if _, err := captureSource(context.Background(), root, root, SourceOptions{}, sourceLimits{entries: 2, bytes: 4}); err == nil {
		t.Fatal("snapshot within source accepted")
	}
}

func TestSourceCapturePolicyPreflight(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	writeSourceFixture(t, root, ".gitignore", "!unsupported\n")
	// The invalid grammar must be diagnosed before entry/byte traversal caps.
	writeSourceFixture(t, root, "large-file", strings.Repeat("x", 1024))
	if _, err := captureSource(context.Background(), root, parent, SourceOptions{}, sourceLimits{entries: 2, bytes: 100}); err == nil || !strings.Contains(err.Error(), ".gitignore:1") {
		t.Fatalf("policy not preflighted: %v", err)
	}
	if _, err := captureSource(context.Background(), root, parent, SourceOptions{NoIgnore: true, ForceInclude: []string{"../escape"}}, sourceLimits{entries: 2, bytes: 100}); err == nil || !strings.Contains(err.Error(), "force-include") {
		t.Fatalf("force syntax not preflighted: %v", err)
	}
	left, err := os.ReadDir(parent)
	if err != nil || len(left) != 0 {
		t.Fatalf("preflight created capture: %v %v", left, err)
	}
}

func TestSourceManifestIdentityReconciliation(t *testing.T) {
	root := t.TempDir()
	writeSourceFixture(t, root, "file", "unchanged bytes")
	handle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	limits := sourceLimits{entries: 10, bytes: 100}
	want, err := scanSourceTree(context.Background(), handle, nil, limits)
	if err != nil {
		t.Fatal(err)
	}
	// Same content with a replaced inode must fail source capture reconciliation.
	writeSourceFixture(t, root, "replacement", "unchanged bytes")
	if err := os.Rename(filepath.Join(root, "replacement"), filepath.Join(root, "file")); err != nil {
		t.Fatal(err)
	}
	got, err := scanSourceTree(context.Background(), handle, nil, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := compareSourceManifests(want, got, true); err == nil {
		t.Fatal("replaced pathname accepted")
	}
}

type sourceMutationContext struct {
	context.Context
	mutate func()
}

func (ctx *sourceMutationContext) Err() error {
	if ctx.mutate != nil {
		mutate := ctx.mutate
		ctx.mutate = nil
		mutate()
	}
	return ctx.Context.Err()
}

func TestSourceCaptureDetectsMutationDuringHandleRead(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(fmt.Sprint("replace=", replacement), func(t *testing.T) {
			root := t.TempDir()
			writeSourceFixture(t, root, "file", "before")
			handle, err := os.OpenRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := handle.Close(); err != nil {
					t.Error(err)
				}
			})
			before, err := handle.Lstat("file")
			if err != nil {
				t.Fatal(err)
			}
			var prevented bool
			ctx := &sourceMutationContext{Context: context.Background(), mutate: func() {
				if replacement {
					writeSourceFixture(t, root, "replacement", "before")
					if err := os.Rename(filepath.Join(root, "replacement"), filepath.Join(root, "file")); err != nil {
						if !sourceReplacementDenied(err) {
							t.Fatal(err)
						}
						prevented = true
						t.Logf("replacement prevented by Windows open-handle protection (not injected mutation): %v", err)
					}
				} else {
					writeSourceFixture(t, root, "file", "changed bytes")
				}
			}}
			digest, copyErr := copySourceFile(ctx, handle, nil, "file", before, 100)
			if prevented {
				current, err := handle.Lstat("file")
				if err != nil || !sameSourceFile(before, current) {
					t.Fatalf("prevented replacement changed original identity: %v", err)
				}
				for _, name := range []string{"file", "replacement"} {
					data, err := os.ReadFile(filepath.Join(root, name))
					if err != nil || string(data) != "before" {
						t.Fatalf("prevented replacement changed %s bytes: %q %v", name, data, err)
					}
				}
				want := fmt.Sprintf("%x", sha256.Sum256([]byte("before")))
				if copyErr != nil || digest != want {
					t.Fatalf("unchanged source after denied replacement must copy successfully: %s %v", digest, copyErr)
				}
			} else if copyErr == nil {
				t.Fatal("mutation during copy/hash accepted")
			}
		})
	}
}

func TestSourceArgumentBudgets(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		if err := checkSourceArguments("syft", []string{"scan", "root", "--exclude", "./bin/stale"}, []string{"PATH=/bin"}, goos); err != nil {
			t.Fatal(err)
		}
		if err := checkSourceArguments("syft", []string{strings.Repeat("é", 150_000)}, nil, goos); err == nil {
			t.Fatalf("argument budget ignored: %s", goos)
		}
		if err := checkSourceArguments("syft", nil, []string{"LARGE=" + strings.Repeat("x", 150_000)}, goos); err == nil {
			t.Fatalf("environment ignored: %s", goos)
		}
	}
	if err := checkSourceArguments("s", []string{strings.Repeat("😀", 7500)}, nil, "windows"); err == nil {
		t.Fatal("UTF16 argument units not counted")
	}
}

func TestSourceCleanupRejectsReplacedRoot(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	writeSourceFixture(t, root, "file", "private source bytes")
	capture, err := captureSource(context.Background(), root, parent, SourceOptions{}, sourceLimits{entries: 10, bytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	// Close the directory handle for Windows rename semantics. The cleanup
	// identity check is independent of whether a retained handle remains open.
	if err := capture.root.Close(); err != nil {
		t.Fatal(err)
	}
	capture.root = nil
	if err := os.Rename(capture.path, capture.path+"-moved"); err != nil {
		t.Fatal(err)
	}
	writeSourceFixture(t, capture.path, "user-owned", "do not delete")
	if err := capture.cleanup(); err == nil || !strings.Contains(err.Error(), capture.path) {
		t.Fatalf("changed snapshot pathname not rejected: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(capture.path, "user-owned"))
	if err != nil || string(data) != "do not delete" {
		t.Fatalf("replacement directory deleted: %q %v", data, err)
	}
}
