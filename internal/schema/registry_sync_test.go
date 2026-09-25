package schema

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/internal/assets"
)

// Every registry schema must come from the tree that make embed-assets syncs
// from schemas/, and the embedded copy must match its source. A hand-copied
// or stale embedded path would otherwise validate against an old schema.
func TestRegistryUsesSyncedSchemas(t *testing.T) {
	const synced = "embedded_schemas/schemas/"
	for name, path := range registrySchemaPaths {
		if !strings.HasPrefix(path, synced) {
			t.Errorf("%s: %s is outside %s", name, path, synced)
			continue
		}
		embedded, ok := assets.GetSchema(path)
		if !ok {
			t.Errorf("%s: %s is not embedded", name, path)
			continue
		}
		source, err := os.ReadFile(filepath.Join("..", "..", "schemas", strings.TrimPrefix(path, synced)))
		if err != nil {
			t.Errorf("%s: no source schema for %s: %v", name, path, err)
			continue
		}
		if !bytes.Equal(embedded, source) {
			t.Errorf("%s: embedded %s differs from source; run make embed-assets", name, path)
		}
		if _, ok := registry[name]; !ok {
			t.Errorf("%s: schema did not compile into the registry", name)
		}
	}
}
