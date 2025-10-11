package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// This test ensures testify is included in dependencies
func TestSynthetic(t *testing.T) {
	assert.True(t, true, "synthetic test fixture")
}
