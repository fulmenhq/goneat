package sbom

import (
	"encoding/json"
	"runtime"
	"testing"
)

func TestSourceRuntimeIdentity(t *testing.T) {
	identity, err := json.Marshal(map[string]string{"os": runtime.GOOS, "arch": runtime.GOARCH, "compiler": runtime.Version()})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("SOURCE_TEST_PROCESS=%s", identity)
}
