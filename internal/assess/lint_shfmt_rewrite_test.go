package assess

import "testing"

func TestShfmtRewrites(t *testing.T) {
	cases := []struct {
		name    string
		config  AssessmentConfig
		yamlFix bool
		want    bool
	}{
		{name: "check", config: AssessmentConfig{Mode: AssessmentModeCheck}, want: false},
		{name: "yaml fix outside hook", config: AssessmentConfig{Mode: AssessmentModeCheck}, yamlFix: true, want: true},
		{name: "flag outside hook", config: AssessmentConfig{Mode: AssessmentModeCheck, LintShellFix: true}, want: true},
		{name: "fix mode", config: AssessmentConfig{Mode: AssessmentModeFix}, want: true},
		{name: "hook ignores yaml fix", config: AssessmentConfig{Mode: AssessmentModeCheck, HookReadOnly: true, LintShellFix: true}, yamlFix: true, want: false},
		{name: "hook ignores fix mode", config: AssessmentConfig{Mode: AssessmentModeFix, HookReadOnly: true}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shfmtRewrites(tc.config, tc.yamlFix); got != tc.want {
				t.Fatalf("shfmtRewrites() = %v, want %v", got, tc.want)
			}
		})
	}
}
