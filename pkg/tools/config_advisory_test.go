package tools

import (
	"fmt"
	"testing"
)

func TestAdvisorySchema(t *testing.T) {
	for _, tt := range []struct {
		value string
		valid bool
	}{
		{`{linux: true}`, true}, {`{all: true, linux: false}`, true},
		{`{linux: "true"}`, false}, {`{linuz: true}`, false}, {`true`, false},
	} {
		t.Run(tt.value, func(t *testing.T) {
			data := []byte(fmt.Sprintf("scopes:\n  foundation:\n    description: Test tools\n    tools: [fixture]\ntools:\n  fixture:\n    name: fixture\n    description: Test tool\n    kind: system\n    detect_command: fixture --version\n    advisory: %s\n", tt.value))
			if err := ValidateBytes(data); (err == nil) != tt.valid {
				t.Fatalf("validation = %v, valid = %v", err, tt.valid)
			}
		})
	}
}
