package sbom

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceArtifactIdentity(t *testing.T) {
	for _, mutation := range []string{"unchanged", "content", "replacement", "parent-replacement", "deleted"} {
		t.Run(mutation, func(t *testing.T) {
			parent := filepath.Join(t.TempDir(), "artifact")
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(parent, "binary")
			if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			identity, err := inspectArtifact(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "content":
				if err := os.WriteFile(path, []byte("modified"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, identity.info.ModTime(), identity.info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "replacement", "parent-replacement":
				old := path
				if mutation == "parent-replacement" {
					old = parent
				}
				if err := os.Rename(old, old+".old"); err != nil {
					t.Fatal(err)
				}
				if mutation == "parent-replacement" {
					if err := os.Mkdir(parent, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, identity.info.ModTime(), identity.info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			err = identity.verify(context.Background())
			if (err == nil) != (mutation == "unchanged") {
				t.Fatalf("verify after %s: %v", mutation, err)
			}
		})
	}
}

func TestSourceArtifactRejectsDirectory(t *testing.T) {
	if _, err := inspectArtifact(context.Background(), t.TempDir()); err == nil {
		t.Fatal("accepted directory as explicit artifact")
	}
}
