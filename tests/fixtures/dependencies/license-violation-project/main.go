package main

import (
	"fmt"
)

func main() {
	// This code intentionally includes Apache-2.0 licensed dependencies
	// in go.mod to test license policy enforcement in hooks
	// The github.com/apache/thrift dependency has Apache-2.0 license
	// which violates the strict BSD-3-Clause-only policy in .goneat/dependencies.yaml

	fmt.Println("This project contains Apache-2.0 licensed dependencies for testing")
	fmt.Println("License policy hooks should block commits containing this code")
}
