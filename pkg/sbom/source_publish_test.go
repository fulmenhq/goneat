package sbom

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSourcePublicationCleanupBeforeOutput(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "cleanup-failure"}[fail], func(t *testing.T) {
			parent := t.TempDir()
			destination := filepath.Join(parent, "inventory.json")
			if err := os.WriteFile(destination, []byte("previous"), 0o600); err != nil {
				t.Fatal(err)
			}
			cleanupErr := errors.New("private data retained at test capture")
			called := false
			err := publishSourceOutput(context.Background(), []byte("new inventory"), destination, func() error {
				called = true
				entries, err := os.ReadDir(parent)
				if err != nil || len(entries) != 1 {
					t.Fatalf("output staged before source cleanup: %v, %v", entries, err)
				}
				data, err := os.ReadFile(destination)
				if err != nil || string(data) != "previous" {
					t.Fatalf("destination changed before cleanup: %s, %v", data, err)
				}
				if fail {
					return cleanupErr
				}
				return nil
			})
			if !called || (fail && !errors.Is(err, cleanupErr)) || (!fail && err != nil) {
				t.Fatalf("publication: called=%v err=%v", called, err)
			}
			want := "new inventory"
			if fail {
				want = "previous"
			}
			data, err := os.ReadFile(destination)
			if err != nil || string(data) != want {
				t.Fatalf("destination=%q want=%q err=%v", data, want, err)
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 1 {
				t.Fatalf("staged output retained: %v %v", entries, err)
			}
		})
	}
}

func TestSourcePublicationStdoutCleanupFailure(t *testing.T) {
	want := errors.New("cleanup failed")
	if err := publishSourceOutput(context.Background(), []byte("inventory"), "", func() error { return want }); !errors.Is(err, want) {
		t.Fatalf("stdout publication ignored cleanup: %v", err)
	}
}

func TestSourcePublicationCancelledStillCleans(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	parent := t.TempDir()
	err := publishSourceOutput(ctx, []byte("inventory"), filepath.Join(parent, "new.json"), func() error { called = true; return nil })
	if !called || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled publication: cleanup=%v err=%v", called, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cancelled output staged: %v %v", entries, err)
	}
}
