package main

import (
	"fmt"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

func main() {
	id := uuid.New()
	fmt.Printf("UUID: %s\n", id)

	data := map[string]string{"test": "value"}
	b, _ := yaml.Marshal(data)
	fmt.Printf("YAML: %s\n", b)
}
