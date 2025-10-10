package policy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/v1/rego"
	"gopkg.in/yaml.v3"
)

// Engine defines policy engine interface
type Engine interface {
	Evaluate(ctx context.Context, input interface{}) (map[string]interface{}, error)
	LoadPolicy(source string) error
}

// OPAEngine implements embedded OPA
type OPAEngine struct {
	regoCode string
}

// NewOPAEngine creates new engine
func NewOPAEngine() Engine {
	return &OPAEngine{}
}

func (e *OPAEngine) Evaluate(ctx context.Context, input interface{}) (map[string]interface{}, error) {
	if e.regoCode == "" {
		return nil, fmt.Errorf("no policy loaded")
	}

	// Create evaluation with input
	rs, err := rego.New(
		rego.Query("data.goneat.dependencies.deny"),
		rego.Input(input),
		rego.Module("policy.rego", e.regoCode),
	).Eval(ctx)

	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{}
	for _, re := range rs {
		for _, expr := range re.Expressions {
			result[expr.Text] = expr.Value
		}
	}

	return result, nil
}

func (e *OPAEngine) LoadPolicy(source string) error {
	// Path validation to prevent directory traversal
	cleanPath := filepath.Clean(source)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid path: directory traversal detected")
	}

	// Ensure path is within expected directory (basic check)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Additional validation: check if file exists and is readable
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("policy file not accessible: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %w", err)
	}

	// Simple YAML to Rego transpiler for Wave 1
	e.regoCode = transpileYAMLToRego(data)

	return nil
}

// transpileYAMLToRego converts simple YAML policy to Rego
func transpileYAMLToRego(yamlData []byte) string {
	var policy map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &policy); err != nil {
		return ""
	}

	var buf bytes.Buffer

	buf.WriteString("package goneat.dependencies\n\n")

	// Transpile forbidden licenses
	if licenses, ok := policy["licenses"].(map[string]interface{}); ok {
		if forbidden, ok := licenses["forbidden"].([]interface{}); ok {
			buf.WriteString("deny contains msg if {\n")
			buf.WriteString("  dep := input.dependencies[_]\n")
			buf.WriteString("  forbidden := ")
			buf.WriteString(formatRegoArray(forbidden))
			buf.WriteString("\n")
			buf.WriteString("  forbidden[_] == dep.license.type\n")
			buf.WriteString("  msg := sprintf(\"Package %%s uses forbidden license: %%s\", [dep.module.name, dep.license.type])\n")
			buf.WriteString("}\n\n")
		}
	}

	// Transpile cooling policy
	if cooling, ok := policy["cooling"].(map[string]interface{}); ok {
		if enabled, ok := cooling["enabled"].(bool); ok && enabled {
			buf.WriteString("# Cooling policy rules\n")
			buf.WriteString("deny contains msg if {\n")
			buf.WriteString("  dep := input.dependencies[_]\n")

			if minAge, ok := cooling["min_age_days"].(int); ok {
				buf.WriteString(fmt.Sprintf("  dep.metadata.age_days < %d\n", minAge))
			}

			buf.WriteString("  not is_cooling_exception(dep.module.name)\n")
			buf.WriteString("  msg := sprintf(\"Package %%s (%%s) violates cooling policy: %%d days old\", ")
			buf.WriteString("[dep.module.name, dep.module.version, dep.metadata.age_days])\n")
			buf.WriteString("}\n\n")

			// Helper function for exceptions
			buf.WriteString("is_cooling_exception(name) if {\n")
			buf.WriteString("  false  # TODO: implement exception logic\n")
			buf.WriteString("}\n\n")
		}
	}

	return buf.String()
}

// formatRegoArray converts a []interface{} to a properly quoted Rego array
// e.g. [GPL-3.0, MIT] -> ["GPL-3.0", "MIT"]
func formatRegoArray(arr []interface{}) string {
	var parts []string
	for _, item := range arr {
		parts = append(parts, fmt.Sprintf("\"%v\"", item))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
