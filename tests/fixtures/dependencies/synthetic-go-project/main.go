package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"
)

// This is a synthetic test fixture for cooling policy testing.
// It intentionally uses specific dependencies to validate:
// - Mature packages (google/uuid, yaml.v3)
// - Standard library extensions (golang.org/x/*)
// - Transitive dependencies (via testify)

func main() {
	// Use packages to ensure they're in go.mod
	id := uuid.New()
	limiter := rate.NewLimiter(1, 1)

	data := map[string]interface{}{
		"id":        id.String(),
		"timestamp": time.Now(),
		"rate":      limiter.Limit(),
	}

	bytes, _ := yaml.Marshal(data)
	fmt.Printf("Synthetic test fixture: %s\n", bytes)
}
