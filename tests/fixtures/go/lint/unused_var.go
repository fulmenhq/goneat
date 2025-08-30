package lintfixture

import "fmt"

// Intentional unused variable
func UnusedVar() {
	var unused string
	fmt.Println("demo", unused)
}
