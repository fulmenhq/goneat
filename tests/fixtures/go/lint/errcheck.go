package lintfixture

import (
	"fmt"
	"os"
)

// Intentional unchecked errors for lint testing
func Errcheck() {
	fmt.Fprintf(os.Stdout, "lint errcheck")                   //nolint:errcheck // intentional for lint fixture testing
	os.WriteFile("/tmp/goneat-fixture", []byte("demo"), 0600) //nolint:errcheck // #nosec G104 - intentional for lint fixture
}
