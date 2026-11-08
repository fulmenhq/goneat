package doctor

import "testing"

func TestManualToolRequiresExplicitAdvisory(t *testing.T) {
	for _, tt := range []struct {
		name   string
		policy map[string]bool
		want   bool
	}{
		{"absent", nil, false},
		{"platform", map[string]bool{getCurrentPlatform(): true}, true},
		{"all", map[string]bool{"all": true}, true},
		{"override", map[string]bool{"all": true, getCurrentPlatform(): false}, false},
		{"other platform", map[string]bool{"other": true}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tool := Tool{Name: "goneat-absent-manual-fixture", Kind: "system", Advisory: tt.policy,
				InstallerPriority: map[string][]string{"all": {"manual"}},
				InstallCommands:   map[string]string{"manual": "install this tool manually"}}
			if got := IsReportOnly(tool); got != tt.want {
				t.Fatalf("report-only = %v, want %v", got, tt.want)
			}
			status := installSystemToolWith(tool, func(string) (string, int, error) {
				t.Fatal("manual installation must not execute commands")
				return "", 0, nil
			}, func(installerKind) bool { return false })
			if CountsTowardMissing(tool, status) == tt.want {
				t.Fatalf("missing gate did not honor explicit advisory policy: %+v", status)
			}
			if (status.Error == nil) != tt.want {
				t.Fatalf("install error = %v, advisory = %v", status.Error, tt.want)
			}
		})
	}
}

func TestDefaultsPreserveAdvisory(t *testing.T) {
	cfg := ConvertToToolsConfig([]ToolDefinition{{Name: "fixture", Advisory: map[string]bool{"linux": true}}}, "foundation", "fixture")
	if !cfg.Tools["fixture"].Advisory["linux"] {
		t.Fatal("default conversion lost advisory policy")
	}
}
