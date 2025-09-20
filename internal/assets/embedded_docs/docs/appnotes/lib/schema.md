---
title: Schema Validation Library
description: Generic JSON/YAML schema validation using gojsonschema.
---

# Schema Validation

Goneat's `pkg/schema` provides a robust, generic schema validation system for JSON and YAML configurations. It uses the gojsonschema library under the hood but provides a clean, opinionated API that integrates well with hierarchical configuration systems.

## Purpose

This library solves the common problem of validating complex configuration structures in Go applications. Instead of writing custom validation logic or using verbose third-party APIs, `pkg/schema` provides:

- Simple validation API with clear error reporting
- Support for both JSON and YAML schemas
- Integration with goneat's config hierarchy system
- Type-safe result handling

## Key Features

- **Generic validation**: Works with any JSON/YAML schema
- **Detailed error reporting**: Clear paths to validation failures
- **Performance optimized**: Compiles schemas once for repeated use
- **Config integration**: Works seamlessly with `pkg/config`

## Installation

```bash
go get github.com/fulmenhq/goneat/pkg/schema
```

## Basic Usage

### Simple Validation

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
    validator, err := schema.NewValidator("config-schema.json")
    if err != nil {
        panic(err)
    }

    config := map[string]interface{}{
        "database": map[string]interface{}{
            "host": "localhost",
            "port": 5432,
        },
        "logging": map[string]interface{}{
            "level": "info",
        },
    }

    if err := validator.Validate(config); err != nil {
        fmt.Printf("Validation failed: %v\n", err)
        return
    }

    fmt.Println("Configuration is valid!")
}
```

### Advanced Usage with Custom Schemas

```go
package main

import (
    "encoding/json"
    "fmt"
    "github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
    // Define schema inline
    schemaContent := []byte(`
    {
        "type": "object",
        "properties": {
            "name": {"type": "string", "minLength": 1},
            "age": {"type": "integer", "minimum": 0, "maximum": 150},
            "email": {"type": "string", "format": "email"}
        },
        "required": ["name", "email"],
        "additionalProperties": false
    }
    `)

    validator, err := schema.NewValidatorFromBytes(schemaContent, schema.JSON)
    if err != nil {
        panic(err)
    }

    // Test valid data
    validData := map[string]interface{}{
        "name":  "John Doe",
        "age":   30,
        "email": "john@example.com",
    }

    if err := validator.Validate(validData); err != nil {
        fmt.Printf("Valid data failed: %v\n", err)
    } else {
        fmt.Println("Valid data passed!")
    }

    // Test invalid data
    invalidData := map[string]interface{}{
        "name":  "",
        "email": "invalid-email",
    }

    if err := validator.Validate(invalidData); err != nil {
        fmt.Printf("Invalid data correctly rejected: %v\n", err)
    }
}
```

## API Reference

### schema.Validator

```go
type Validator struct {
    // Contains compiled schema and validation logic
}

func NewValidator(schemaPath string) (*Validator, error)
func NewValidatorFromBytes(schemaBytes []byte, format SchemaFormat) (*Validator, error)
func NewValidatorFromString(schemaStr string, format SchemaFormat) (*Validator, error)

func (v *Validator) Validate(data interface{}) error
func (v *Validator) ValidateWithContext(ctx context.Context, data interface{}) error
func (v *Validator) Schema() *gojsonschema.Schema
```

### SchemaFormat

```go
type SchemaFormat string

const (
    JSON SchemaFormat = "json"
    YAML SchemaFormat = "yaml"
)
```

### Validation Errors

The library returns detailed validation errors with context:

```go
type ValidationError struct {
    FieldPath  string
    Value      interface{}
    Constraint string
    Message    string
}

func (e *ValidationError) Error() string
```

## Integration with pkg/config

The schema library integrates seamlessly with goneat's configuration system:

```go
package main

import (
    "context"
    "fmt"
    "github.com/fulmenhq/goneat/pkg/config"
    "github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
    cfg, err := config.New(context.Background(), config.WithSchemaPath("config-schema.json"))
    if err != nil {
        panic(err)
    }

    // Configuration is automatically validated against the schema
    if err := cfg.Validate(); err != nil {
        fmt.Printf("Config validation failed: %v\n", err)
        return
    }

    // Access validated configuration
    dbHost := cfg.GetString("database.host")
    fmt.Printf("Database host: %s\n", dbHost)
}
```

## Error Handling

### Common Validation Errors

```go
// Example validation error output
if err := validator.Validate(invalidData); err != nil {
    if validationErr, ok := err.(*schema.ValidationError); ok {
        fmt.Printf("Validation failed at %s: %s (expected %s, got %v)\n",
            validationErr.FieldPath,
            validationErr.Message,
            validationErr.Constraint,
            validationErr.Value,
        )
    }
}
```

### Custom Validation Rules

You can extend validation with custom rules:

```go
func validatePort(port int) error {
    if port < 1 || port > 65535 {
        return fmt.Errorf("port must be between 1 and 65535")
    }
    return nil
}

// Use in your schema validation pipeline
```

## Performance Considerations

- **Schema compilation**: Compile schemas once and reuse the `Validator` instance
- **Batch validation**: The library supports validating multiple documents efficiently
- **Memory usage**: Schemas are compiled to efficient internal representations

## Security Considerations

- **Schema source validation**: Always validate schema sources to prevent injection attacks
- **Input sanitization**: The library sanitizes validation inputs to prevent path traversal
- **Resource limits**: Set appropriate limits on deeply nested structures to prevent DoS

## Best Practices

1. **Compile schemas at startup**: Don't recompile schemas for each validation
2. **Use descriptive error messages**: Include context in your schema descriptions
3. **Validate early**: Validate configurations during application startup
4. **Provide fallback defaults**: Combine with `pkg/config` for graceful degradation
5. **Log validation failures**: Use structured logging for monitoring configuration issues

## Limitations

- Currently supports JSON Schema Draft 7 (with plans for Draft 2020-12)
- YAML support is experimental and may have edge cases
- Custom format validators require additional setup

## Future Plans

- Support for JSON Schema Draft 2020-12 and later
- Additional YAML schema features
- Integration with OpenAPI schemas
- Performance optimizations for large document validation

## Related Libraries

- [`pkg/config`](config.md) - Hierarchical configuration management
- [`pkg/pathfinder`](pathfinder.md) - Safe file system traversal
- [gojsonschema](https://github.com/xeipuuv/gojsonschema) - Underlying validation engine

For more information, see the [GoDoc documentation](https://pkg.go.dev/github.com/fulmenhq/goneat/pkg/schema).
