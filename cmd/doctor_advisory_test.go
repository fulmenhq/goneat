package cmd

import (
	"runtime"
	"testing"

	"github.com/fulmenhq/goneat/internal/doctor"
)

func TestDoctorConversionPreservesAdvisory(t *testing.T) {
	for _, advisory := range []bool{false, true} {
		tool, err := convertToolConfigToTool(doctor.ToolConfig{
			Name: "absent-fixture", Kind: "system",
			Advisory:          map[string]bool{runtime.GOOS: advisory},
			InstallerPriority: map[string][]string{"all": {"manual"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if doctor.CountsTowardMissing(tool, doctor.Status{}) == advisory {
			t.Fatalf("doctor conversion lost advisory=%v", advisory)
		}
	}
}
