//go:build !windows

package sbom

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSourceCapturePrivacyMutation(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	writeSourceFixture(t, root, "file", "private data")
	capture, err := captureSource(context.Background(), root, parent, SourceOptions{}, sourceLimits{entries: 10, bytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := capture.cleanup(); err != nil {
			t.Error(err)
		}
	})
	if err := os.Chmod(filepath.Join(capture.path, "file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := capture.verify(context.Background()); err == nil {
		t.Fatal("widened private-file permissions accepted")
	}
	if err := os.Chmod(filepath.Join(capture.path, "file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(capture.path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := capture.verify(context.Background()); err == nil {
		t.Fatal("widened private-root permissions accepted")
	}
}

func TestSourceCaptureRejectsSpecialFile(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(filepath.Join(root, "bin/pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := captureSource(context.Background(), root, parent, SourceOptions{}, sourceLimits{entries: 10, bytes: 100}); err == nil {
		t.Fatal("excluded special file accepted")
	}
	left, err := os.ReadDir(parent)
	if err != nil || len(left) != 0 {
		t.Fatalf("private staging data remains: %v %v", left, err)
	}
}
