package tools

import (
	"os"
	"testing"
)

func TestInstallUserLocalBrew(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		wantErr bool
	}{
		{"default_prefix", "", false},
		{"custom_prefix", "/tmp/test-brew", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InstallUserLocalBrew(tt.prefix, false, true) // dryRun for test
			if (err != nil) != tt.wantErr {
				t.Errorf("InstallUserLocalBrew() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && tt.prefix != "" {
				os.RemoveAll(tt.prefix) //nolint:errcheck // Cleanup errors are ignored in tests
			}
		})
	}
}
