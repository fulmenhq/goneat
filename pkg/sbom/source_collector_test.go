package sbom

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSourceCollectorEnvironment(t *testing.T) {
	environment := []string{"PATH=/tools", "HOME=/home", "SYFT_EXCLUDE=**", "SYFT_CONFIG=/caller", "syft_profile=hidden", "SyFt_CATALOGERS=none", "AUTH_VALUE=preserved", "SYFT_CHECK_FOR_APP_UPDATE=true"}
	want := []string{"PATH=/tools", "HOME=/home", "AUTH_VALUE=preserved"}
	if got := sourceCollectorEnvironment(environment); !reflect.DeepEqual(got, want) {
		t.Fatalf("collector environment did not isolate policy/preserve unrelated settings")
	}
	if !reflect.DeepEqual(environment, []string{"PATH=/tools", "HOME=/home", "SYFT_EXCLUDE=**", "SYFT_CONFIG=/caller", "syft_profile=hidden", "SyFt_CATALOGERS=none", "AUTH_VALUE=preserved", "SYFT_CHECK_FOR_APP_UPDATE=true"}) {
		t.Fatal("caller environment mutated")
	}
}

func TestSourceCollectorOwnershipAndCleanup(t *testing.T) {
	ctx := context.Background()
	collector, err := newSourceCollector(ctx, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	owned := collector.owned
	t.Cleanup(func() {
		if err := owned.cleanup(); err != nil {
			t.Error(err)
		}
	})
	data, err := os.ReadFile(collector.config)
	if err != nil || string(data) != "{}\n" || filepath.Dir(collector.config) != owned.path {
		t.Fatalf("wrong owned config: %v", err)
	}
	if err := owned.verify(ctx); err != nil {
		t.Fatal(err)
	}
	if err := owned.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(owned.path); !os.IsNotExist(err) {
		t.Fatalf("collector directory retained: %v", err)
	}
}

func TestSourceCollectorRefusesMutation(t *testing.T) {
	ctx := context.Background()
	collector, err := newSourceCollector(ctx, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := collector.owned.cleanup(); err != nil {
			t.Error(err)
		}
	})
	if err := os.WriteFile(collector.config, []byte("exclude: [ '**' ]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := collector.owned.verify(ctx); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("mutated collector configuration accepted: %v", err)
	}
}

func TestSourceCollectorRefusesInputWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	collector, err := newSourceCollector(context.Background(), root, root)
	if err == nil || collector != nil || !strings.Contains(err.Error(), "outside target") {
		t.Fatalf("collector working directory within input accepted: %v", err)
	}
}

func TestSourceCollectorCreationFailure(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-directory")
	if err := os.WriteFile(parent, []byte("retained parent"), 0o600); err != nil {
		t.Fatal(err)
	}
	collector, err := newSourceCollector(context.Background(), t.TempDir(), parent)
	if err == nil || collector != nil || !strings.Contains(err.Error(), "create private collector") {
		t.Fatalf("invalid collector parent accepted: %v", err)
	}
	data, err := os.ReadFile(parent)
	if err != nil || string(data) != "retained parent" {
		t.Fatalf("failed collector setup modified parent: %v", err)
	}
}
