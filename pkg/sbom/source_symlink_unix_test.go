//go:build !windows

package sbom

import "testing"

func prepareSourceSymlinkFixture(t *testing.T) {
	t.Helper()
}

func sourceReplacementDenied(err error) bool {
	return false
}
