package lintfixture

import (
	"fmt"
	"os"
)

// Intentional unchecked errors
func Errcheck() {
	fmt.Fprintf(os.Stdout, "lint errcheck")
	os.WriteFile("/tmp/goneat-fixture", []byte("demo"), 0644)
}
