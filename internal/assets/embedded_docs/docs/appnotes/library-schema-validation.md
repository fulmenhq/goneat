---
title: "Using the Schema Validation Library"
description: "Programmatic JSON Schema validation in Go projects using Goneat's pkg/schema with file operations, batch processing, and real-world implementation patterns"
author: "@forge-neat"
date: "2025-09-10"
last_updated: "2025-09-14"
status: "stable"
tags:
  [
    "library",
    "schema",
    "validation",
    "go",
    "security",
    "batch",
    "file-processing",
  ]
category: "appnotes"
---

# Using the Schema Validation Library

## Overview

Goneat's `pkg/schema` module provides a comprehensive public API for validating data against JSON Schemas (Draft-07 and Draft-2020-12 only). This is ideal for runtime config validation, API request validation, or integrating schema checks into your Go applications.

**New in v0.2.3**: Enhanced with file operations, batch processing, security controls, and thread-safe concurrent validation.

**New in v0.2.4**: Added ergonomic helper functions for common validation patterns, addressing DX friction points identified by sibling teams.

**New in v0.2.5**: Added comprehensive real-world implementation patterns based on team feedback, showing practical usage examples that mirror how Goneat's own codebase validates configurations.

### Key Features

- **Multiple Input Formats**: Validate raw bytes, files, or parsed data structures
- **File Operations**: Direct file-to-file validation with security controls
- **Batch Processing**: Concurrent validation of multiple files or directories
- **Security Controls**: Path traversal protection, file size limits, and draft enforcement
- **Thread Safety**: Race-free concurrent operations with proper synchronization
- **Enhanced Context**: Detailed error information with file paths and line numbers
- **Real-World Patterns**: Practical implementation examples based on team feedback and Goneat's own usage

The library is offline-first (no network calls) and leverages `gojsonschema` under the hood for validation.

## Installation

Add to your `go.mod`:

```bash
go get github.com/fulmenhq/goneat/pkg/schema
```

## API Reference

### Core Types

- **ValidationError**:

  ```go
  type ValidationError struct {
      Path    string            `json:"path,omitempty"`    // JSON path to validation error
      Message string            `json:"message"`           // Human-readable error message
      Context ValidationContext `json:"context,omitempty"` // Enhanced context (file, line, severity)
  }
  ```

- **ValidationContext**:

  ```go
  type ValidationContext struct {
      SourceFile string `json:"source_file,omitempty"` // File path for file-based validation
      SourceType string `json:"source_type,omitempty"` // "file", "bytes", "string"
      LineNumber int    `json:"line_number,omitempty"` // Line number (when available)
      Severity   string `json:"severity,omitempty"`    // "error", "warning"
  }
  ```

- **Result**:
  ```go
  type Result struct {
      Valid  bool               `json:"valid"`
      Errors []ValidationError `json:"errors,omitempty"`
  }
  ```

### Security Types

- **SecurityContext**:

  ```go
  type SecurityContext struct {
      AllowedDirs  []string `json:"allowed_dirs,omitempty"`        // Allowed directory paths
      MaxFileSize  int64    `json:"max_file_size_bytes,omitempty"` // Maximum file size (default: 10MB)
      EnforceDraft bool     `json:"enforce_draft_only,omitempty"`  // Enforce Draft-07/2020-12 only
  }
  ```

- **ValidationOptions**:
  ```go
  type ValidationOptions struct {
      Context ValidationContext // Additional context for errors
      Audit   bool             // Enable audit logging (currently disabled)
  }
  ```

### Batch Processing Types

- **BatchOptions**:

  ```go
  type BatchOptions struct {
      MaxConcurrency int           `json:"max_concurrency,omitempty"` // Concurrent workers (default: CPU count)
      Timeout        time.Duration `json:"timeout,omitempty"`         // Operation timeout (default: 30s)
      Security       SecurityContext                              // Security constraints
  }
  ```

- **BatchResult**:
  ```go
  type BatchResult struct {
      Valid           bool               `json:"valid"`                       // Overall validity
      TotalFiles      int                `json:"total_files"`                 // Total files processed
      ValidFiles      int                `json:"valid_files"`                 // Files that passed validation
      InvalidFiles    int                `json:"invalid_files"`               // Files that failed validation
      OverallSeverity string             `json:"overall_severity,omitempty"` // "pass", "fail"
      Summary         []string           `json:"summary,omitempty"`           // Summary messages
      FileResults     map[string]*Result `json:"file_results"`                // Per-file results
  }
  ```

### Core Functions

- **Validate(data interface{}, schemaName string) (\*Result, error)**:
  - Validates parsed data against an embedded schema by name
  - Use for quick validation against Goneat's built-in schemas
  - Errors if schema not found in registry

- **ValidateFromBytes(schemaBytes []byte, data interface{}) (\*Result, error)**:
  - Validates parsed data against arbitrary schema bytes (JSON or YAML)
  - Auto-detects format (YAML first, then JSON fallback)
  - Enforces Draft-07/2020-12 support

### New Raw Data Functions

- **ValidateDataFromBytes(schemaBytes, dataBytes []byte, opts ...ValidationOptions) (\*Result, error)**:
  - Validates raw data bytes against schema bytes without manual parsing
  - Auto-detects data format (YAML/JSON) and handles conversion
  - Supports validation options for enhanced context

- **NewSecurityContext() SecurityContext**:
  - Returns a SecurityContext with secure defaults
  - 10MB file size limit, draft enforcement, current directory access

### File Operation Functions

- **ValidateFile(schemaBytes []byte, dataFilePath string) (\*Result, error)**:
  - Validates a file against schema bytes
  - Automatic path sanitization and format detection

- **ValidateFileFromSchemaFile(schemaFilePath, dataFilePath string) (\*Result, error)**:
  - Validates a data file against a separate schema file
  - Both paths are sanitized for security

- **ValidateFileWithSecurity(schemaBytes []byte, dataFilePath string, sec SecurityContext) (\*Result, error)**:
  - Validates a file with comprehensive security controls
  - Path traversal protection, file size limits, allowed directory enforcement

### Batch Processing Functions

- **ValidateFiles(schemaBytes []byte, dataFilePaths []string) (\*BatchResult, error)**:
  - Validates multiple files concurrently against schema bytes
  - Uses default batch options

- **ValidateFilesWithOptions(schemaBytes []byte, dataFilePaths []string, opts BatchOptions) (\*BatchResult, error)**:
  - Validates multiple files with custom batch options
  - Configurable concurrency, timeout, and security constraints
  - Thread-safe concurrent processing

- **ValidateDirectory(schemaBytes []byte, dirPath, pattern string) (\*BatchResult, error)**:
  - Validates all matching files in a directory
  - Supports glob patterns for file selection

- **ValidateDirectoryWithOptions(schemaBytes []byte, dirPath, pattern string, opts BatchOptions) (\*BatchResult, error)**:
  - Directory validation with custom options
  - Full batch processing capabilities for directories

### Ergonomic Helper Functions (New in v0.2.4)

- **ValidateFileWithSchemaPath(schemaPath string, dataPath string) (\*Result, error)**:
  - Simple file-to-file validation with automatic format detection
  - Reads schema file and data file, auto-detects JSON/YAML formats
  - Includes path sanitization and security checks

- **ValidateFromFileWithBytes(schemaPath string, dataBytes []byte) (\*Result, error)**:
  - Validate raw data bytes against a schema file
  - Reads schema file, auto-detects data format (JSON/YAML)
  - Ideal for validating in-memory data against file-based schemas

- **ValidateWithOptions(schemaBytes []byte, data interface{}, opts ValidationOptions) (\*Result, error)**:
  - Validate with enhanced context and options
  - Supports custom validation context for better error reporting
  - Extensible for future advanced features

Data input for traditional functions is `interface{}` (typically `map[string]interface{}` from parsing). New functions handle raw bytes and files automatically.

## Examples

### 1. Validate Against Embedded Schema

```go
package main

import (
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
	"gopkg.in/yaml.v3"
)

func main() {
	// Parse data (YAML example)
	dataYAML := []byte(`
format:
  go:
    simplify: true
security:
  timeout: 5m
`)
	var dataDoc interface{}
	if err := yaml.Unmarshal(dataYAML, &dataDoc); err != nil {
		log.Fatal(err)
	}

	// Validate against embedded schema
	result, err := schema.Validate(dataDoc, "goneat-config-v1.0.0")
	if err != nil {
		log.Fatal(err)
	}
	if result.Valid {
		fmt.Println("✅ Config is valid!")
	} else {
		fmt.Println("❌ Validation failed:")
		for _, e := range result.Errors {
			fmt.Printf("- %s: %s\n", e.Path, e.Message)
		}
	}
}
```

### 2. Validate Against Arbitrary Schema (JSON Bytes)

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
	// Arbitrary JSON schema
	schemaJSON := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "name": {"type": "string"},
    "age": {"type": "number"}
  },
  "required": ["name"]
}`)

	// Parse data
	dataJSON := []byte(`{"name": "Alice", "age": 30}`)
	var dataMap map[string]interface{}
	if err := json.Unmarshal(dataJSON, &dataMap); err != nil {
		log.Fatal(err)
	}

	// Validate
	result, err := schema.ValidateFromBytes(schemaJSON, dataMap)
	if err != nil {
		log.Fatal(err)
	}
	if result.Valid {
		fmt.Println("✅ Data matches schema!")
	} else {
		fmt.Println("❌ Invalid data:")
		for _, e := range result.Errors {
			fmt.Printf("- %s: %s\n", e.Path, e.Message)
		}
	}
}
```

### 3. Validate Against Arbitrary YAML Schema

```go
package main

import (
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
	"gopkg.in/yaml.v3"
)

func main() {
	// Arbitrary YAML schema bytes
	schemaYAML := []byte(`
$schema: https://json-schema.org/draft/2020-12/schema
type: object
properties:
  name:
    type: string
required:
  - name
`)

	// Parse data (JSON example)
	dataJSON := []byte(`{"name": "Bob"}`)
	var dataDoc interface{}
	if err := json.Unmarshal(dataJSON, &dataDoc); err != nil {
		log.Fatal(err)
	}

	// Validate
	result, err := schema.ValidateFromBytes(schemaYAML, dataDoc)
	if err != nil {
		log.Fatal(err)
	}
	if result.Valid {
		fmt.Println("✅ YAML schema validation passed!")
	} else {
		fmt.Println("❌ Validation failed:")
		for _, e := range result.Errors {
			fmt.Printf("- %s: %s\n", e.Path, e.Message)
		}
	}
}
```

### 4. Validate Raw Bytes (New in v0.2.3)

```go
package main

import (
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
	// Schema as raw bytes
	schemaBytes := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "name": {"type": "string"},
    "age": {"type": "number"}
  },
  "required": ["name"]
}`)

	// Data as raw bytes (no manual parsing needed!)
	dataBytes := []byte(`{"name": "Alice", "age": 30}`)

	// Validate raw bytes directly
	result, err := schema.ValidateDataFromBytes(schemaBytes, dataBytes)
	if err != nil {
		log.Fatal(err)
	}

	if result.Valid {
		fmt.Println("✅ Raw bytes validation passed!")
	} else {
		fmt.Println("❌ Validation failed:")
		for _, e := range result.Errors {
			fmt.Printf("- %s: %s\n", e.Path, e.Message)
		}
	}
}
```

### 5. Validate Files with Security Controls (New in v0.2.3)

```go
package main

import (
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
	// Schema as bytes
	schemaBytes := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "name": {"type": "string"}
  },
  "required": ["name"]
}`)

	// Create security context with defaults
	secCtx := schema.NewSecurityContext()
	secCtx.AllowedDirs = []string{"./", "/tmp/configs"} // Allow current dir and /tmp/configs

	// Validate file with security controls
	result, err := schema.ValidateFileWithSecurity(
		schemaBytes,
		"./config.json",
		secCtx,
	)
	if err != nil {
		log.Fatal(err) // Could be path traversal rejection or file size limit
	}

	if result.Valid {
		fmt.Println("✅ File validation passed!")
	} else {
		fmt.Println("❌ File validation failed:")
		for _, e := range result.Errors {
			fmt.Printf("- %s: %s (file: %s)\n",
				e.Path, e.Message, e.Context.SourceFile)
		}
	}
}
```

### 6. Batch File Validation (New in v0.2.3)

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
	// Schema for user configuration
	schemaBytes := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "username": {"type": "string"},
    "enabled": {"type": "boolean"}
  },
  "required": ["username"]
}`)

	// Files to validate
	files := []string{
		"./users/alice.json",
		"./users/bob.json",
		"./users/charlie.json",
	}

	// Configure batch options
	opts := schema.BatchOptions{
		MaxConcurrency: 4,                    // 4 concurrent workers
		Timeout:        60 * time.Second,     // 1 minute timeout
		Security:       schema.NewSecurityContext(), // Secure defaults
	}

	// Validate all files concurrently
	batchResult, err := schema.ValidateFilesWithOptions(schemaBytes, files, opts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Batch validation complete:\n")
	fmt.Printf("- Total files: %d\n", batchResult.TotalFiles)
	fmt.Printf("- Valid files: %d\n", batchResult.ValidFiles)
	fmt.Printf("- Invalid files: %d\n", batchResult.InvalidFiles)
	fmt.Printf("- Overall result: %s\n", batchResult.OverallSeverity)

	// Show individual file results
	for filePath, result := range batchResult.FileResults {
		status := "✅"
		if !result.Valid {
			status = "❌"
		}
		fmt.Printf("%s %s: %d errors\n", status, filePath, len(result.Errors))
	}
}
```

### 7. Directory Validation (New in v0.2.3)

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
	// YAML schema for configuration files
	schemaYAML := []byte(`
$schema: https://json-schema.org/draft/2020-12/schema
type: object
properties:
  service:
    type: string
  port:
    type: number
required:
  - service
`)

	// Validate all YAML files in configs directory
	opts := schema.BatchOptions{
		MaxConcurrency: 8,
		Timeout:        30 * time.Second,
		Security:       schema.NewSecurityContext(),
	}

	result, err := schema.ValidateDirectoryWithOptions(
		schemaYAML,
		"./configs",
		"*.yaml", // Only YAML files
		opts,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Directory validation:\n")
	fmt.Printf("- Processed: %d files\n", result.TotalFiles)
	fmt.Printf("- Valid: %d files\n", result.ValidFiles)
	fmt.Printf("- Invalid: %d files\n", result.InvalidFiles)

	if result.InvalidFiles > 0 {
		fmt.Println("\nInvalid files:")
		for filePath, fileResult := range result.FileResults {
			if !fileResult.Valid {
				fmt.Printf("- %s\n", filePath)
				for _, e := range fileResult.Errors {
					fmt.Printf("  • %s: %s\n", e.Path, e.Message)
				}
			}
		}
	}
}
```

### 8. File-to-File Validation (New in v0.2.3)

```go
package main

import (
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
	// Validate a data file against a separate schema file
	result, err := schema.ValidateFileFromSchemaFile(
		"./schemas/user-config.json",  // Schema file
		"./data/user123.json",         // Data file to validate
	)
	if err != nil {
		log.Fatal(err) // Could be file not found or path security issue
	}

	if result.Valid {
		fmt.Println("✅ File-to-file validation passed!")
	} else {
		fmt.Println("❌ File validation failed:")
		for _, e := range result.Errors {
			fmt.Printf("- %s: %s\n", e.Path, e.Message)
		}
	}
}
```

### 10. Real-World Implementation Patterns

Based on team feedback, here are practical implementation patterns that mirror how Goneat's own codebase uses schema validation:

#### 10.1 Configuration File Validation Pattern

```go
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fulmenhq/goneat/pkg/schema"
)

// validateConfigFile validates a configuration file against a schema
func validateConfigFile(configPath, schemaPath string) error {
	result, err := schema.ValidateFileWithSchemaPath(schemaPath, configPath)
	if err != nil {
		return fmt.Errorf("failed to validate config: %w", err)
	}

	if !result.Valid {
		fmt.Printf("❌ Configuration validation failed:\n")
		for _, err := range result.Errors {
			fmt.Printf("  • %s: %s\n", err.Path, err.Message)
			if err.Context.SourceFile != "" {
				fmt.Printf("    File: %s\n", err.Context.SourceFile)
			}
		}
		return fmt.Errorf("configuration validation failed")
	}

	fmt.Printf("✅ Configuration file %s is valid\n", filepath.Base(configPath))
	return nil
}

// validateAllConfigsInDirectory batch validates all config files
func validateAllConfigsInDirectory(configDir, schemaPath string) error {
	// Read schema once
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	// Find all config files
	configFiles, err := filepath.Glob(filepath.Join(configDir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to find config files: %w", err)
	}

	yamlFiles, err := filepath.Glob(filepath.Join(configDir, "*.yml"))
	if err != nil {
		return fmt.Errorf("failed to find yaml files: %w", err)
	}

	// Need time import for timeout
	opts := schema.BatchOptions{
		MaxConcurrency: 4,
		Timeout:        30 * time.Second,
		Security:       schema.NewSecurityContext(),
	}

	allFiles := append(configFiles, yamlFiles...)

	if len(allFiles) == 0 {
		return fmt.Errorf("no configuration files found in %s", configDir)
	}

	// Validate all files concurrently
	opts := schema.BatchOptions{
		MaxConcurrency: 4,
		Timeout:        30 * time.Second,
		Security:       schema.NewSecurityContext(),
	}

	batchResult, err := schema.ValidateFilesWithOptions(schemaBytes, allFiles, opts)
	if err != nil {
		return fmt.Errorf("batch validation failed: %w", err)
	}

	fmt.Printf("Batch validation results:\n")
	fmt.Printf("- Total files: %d\n", batchResult.TotalFiles)
	fmt.Printf("- Valid files: %d\n", batchResult.ValidFiles)
	fmt.Printf("- Invalid files: %d\n", batchResult.InvalidFiles)

	if batchResult.InvalidFiles > 0 {
		fmt.Printf("\n❌ Invalid files:\n")
		for filePath, result := range batchResult.FileResults {
			if !result.Valid {
				fmt.Printf("- %s:\n", filepath.Base(filePath))
				for _, err := range result.Errors {
					fmt.Printf("  • %s: %s\n", err.Path, err.Message)
				}
			}
		}
		return fmt.Errorf("some configuration files failed validation")
	}

	fmt.Printf("✅ All configuration files passed validation\n")
	return nil
}
```

#### 10.2 Runtime Data Validation Pattern

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
)

// UserConfig represents user configuration
type UserConfig struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// validateUserConfig validates user configuration from JSON bytes
func validateUserConfig(jsonData []byte) (*UserConfig, error) {
	// Define schema inline (or load from file)
	userSchema := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"name": {"type": "string", "minLength": 1},
			"email": {"type": "string", "format": "email"},
			"role": {"type": "string", "enum": ["admin", "user", "guest"]}
		},
		"required": ["name", "email"]
	}`)

	// Validate raw bytes (no manual parsing needed!)
	result, err := schema.ValidateDataFromBytes(userSchema, jsonData)
	if err != nil {
		return nil, fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid {
		fmt.Printf("❌ User configuration validation failed:\n")
		for _, err := range result.Errors {
			fmt.Printf("  • %s: %s\n", err.Path, err.Message)
		}
		return nil, fmt.Errorf("invalid user configuration")
	}

	// Safe to parse now that validation passed
	var config UserConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse validated config: %w", err)
	}

	return &config, nil
}

// validateAPIRequest validates incoming API request data
func validateAPIRequest(requestBody []byte) error {
	apiSchema := []byte(`{
		"type": "object",
		"properties": {
			"user_id": {"type": "string"},
			"action": {"type": "string", "enum": ["create", "update", "delete"]},
			"data": {"type": "object"}
		},
		"required": ["user_id", "action"]
	}`)

	result, err := schema.ValidateDataFromBytes(apiSchema, requestBody)
	if err != nil {
		log.Printf("API request validation error: %v", err)
		return fmt.Errorf("invalid request format")
	}

	if !result.Valid {
		log.Printf("API request validation failed:")
		for _, err := range result.Errors {
			log.Printf("  • %s: %s", err.Path, err.Message)
		}
		return fmt.Errorf("request validation failed")
	}

	return nil
}
```

#### 10.3 File-Based Schema Validation Pattern

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/fulmenhq/goneat/pkg/schema"
	"gopkg.in/yaml.v3"
)

// loadAndValidateConfig loads and validates configuration from file
func loadAndValidateConfig(configPath string) (map[string]interface{}, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", configPath)
	}

	// Read and validate config file
	result, err := schema.ValidateFileWithSchemaPath("./schemas/app-config.json", configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to validate configuration: %w", err)
	}

	if !result.Valid {
		fmt.Printf("❌ Configuration validation failed:\n")
		for _, err := range result.Errors {
			fmt.Printf("  • %s: %s\n", err.Path, err.Message)
		}
		return nil, fmt.Errorf("invalid configuration")
	}

	// Read the file content now that validation passed
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read validated config: %w", err)
	}

	// Parse based on file extension
	var config map[string]interface{}
	ext := filepath.Ext(configPath)
	switch ext {
	case ".json":
		if err := json.Unmarshal(configData, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(configData, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config format: %s", ext)
	}

	fmt.Printf("✅ Configuration loaded and validated from %s\n", filepath.Base(configPath))
	return config, nil
}

// validateSchemaFile validates that a schema file itself is well-formed
func validateSchemaFile(schemaPath string) error {
	// Read schema file
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Try to parse as JSON or YAML
	var schema interface{}
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		if err := yaml.Unmarshal(schemaData, &schema); err != nil {
			return fmt.Errorf("schema file is not valid JSON or YAML: %w", err)
		}
	}

	// Basic schema structure validation
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return fmt.Errorf("schema must be a JSON object")
	}

	if _, hasSchema := schemaMap["$schema"]; !hasSchema {
		log.Printf("⚠️  Warning: Schema file missing $schema field")
	}

	fmt.Printf("✅ Schema file %s is well-formed\n", filepath.Base(schemaPath))
	return nil
}
```

### 11. Ergonomic Helper Functions (New in v0.2.4)

Goneat v0.2.4 introduces ergonomic helper functions to address common DX friction points. These helpers provide simpler APIs for the most common validation patterns.

#### 9.1 ValidateFileWithSchemaPath - File + File Validation

```go
package main

import (
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
	// Simple file-to-file validation with automatic format detection
	result, err := schema.ValidateFileWithSchemaPath(
		"./schemas/config.json",  // Schema file path
		"./config.yaml",          // Data file (JSON or YAML auto-detected)
	)
	if err != nil {
		log.Fatal(err)
	}

	if result.Valid {
		fmt.Println("✅ File validation passed!")
	} else {
		fmt.Println("❌ File validation failed:")
		for _, e := range result.Errors {
			fmt.Printf("- %s: %s\n", e.Path, e.Message)
		}
	}
}
```

#### 9.2 ValidateFromFileWithBytes - Schema File + Data Bytes

```go
package main

import (
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
	// Validate raw data bytes against a schema file
	dataBytes := []byte(`{"name": "Alice", "age": 30}`)

	result, err := schema.ValidateFromFileWithBytes(
		"./schemas/user.json",  // Schema file path
		dataBytes,              // Raw data bytes (JSON/YAML auto-detected)
	)
	if err != nil {
		log.Fatal(err)
	}

	if result.Valid {
		fmt.Println("✅ Data bytes validation passed!")
	} else {
		fmt.Println("❌ Data bytes validation failed:")
		for _, e := range result.Errors {
			fmt.Printf("- %s: %s\n", e.Path, e.Message)
		}
	}
}
```

#### 9.3 ValidateWithOptions - Advanced Options

```go
package main

import (
	"fmt"
	"log"

	"github.com/fulmenhq/goneat/pkg/schema"
)

func main() {
	// Validate with enhanced context and options
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		},
		"required": ["name"]
	}`)

	data := map[string]interface{}{"name": "test"}

	// Use options for enhanced error context
	opts := schema.ValidationOptions{
		Context: schema.ValidationContext{
			SourceFile: "config.json",
			SourceType: "json",
			Severity:   "error",
		},
	}

	result, err := schema.ValidateWithOptions(schemaBytes, data, opts)
	if err != nil {
		log.Fatal(err)
	}

	if result.Valid {
		fmt.Println("✅ Validation with options passed!")
	} else {
		fmt.Println("❌ Validation with options failed:")
		for _, e := range result.Errors {
			fmt.Printf("- %s: %s (file: %s)\n", e.Path, e.Message, e.Context.SourceFile)
		}
	}
}
```

**Migration from CLI Patterns:**

```bash
# Old CLI approach - requires shelling out
goneat validate data --schema-file schema.json data.json

# New library approach - direct integration
result, err := schema.ValidateFileWithSchemaPath("schema.json", "data.json")
```

## Security Features

### Path Traversal Protection

All file operations use secure path containment checks:

```go
// This will be BLOCKED (path traversal attempt)
result, err := schema.ValidateFile(schemaBytes, "../../../etc/passwd")
// err: path not in allowed directories

// This will work (within allowed directory)
secCtx := schema.NewSecurityContext()
secCtx.AllowedDirs = []string{"./configs", "/app/data"}
result, err := schema.ValidateFileWithSecurity(schemaBytes, "./configs/app.yaml", secCtx)
```

### File Size Limits

```go
secCtx := schema.NewSecurityContext()
secCtx.MaxFileSize = 5 * 1024 * 1024 // 5MB limit

// Files larger than 5MB will be rejected
result, err := schema.ValidateFileWithSecurity(schemaBytes, "large-file.json", secCtx)
// err: file exceeds max size limit
```

### Thread Safety

All batch operations are thread-safe:

```go
// Safe for concurrent use across multiple goroutines
opts := schema.BatchOptions{MaxConcurrency: runtime.NumCPU()}
result, err := schema.ValidateFilesWithOptions(schemaBytes, files, opts)
// No race conditions, proper mutex protection
```

## Restrictions

- **Drafts**: Only Draft-07 and Draft-2020-12 supported (checked via `$schema` key). Unsupported drafts (e.g., Draft-04) return error: "unsupported $schema: only Draft-07 and Draft-2020-12 supported".
- **Formats**: Schema/data must be JSON or YAML (auto-detected in all functions).
- **Embedded Schemas**: Limited to pre-registered names (use CLI `goneat validate --list-schemas` to see available).
- **Offline**: All validation is offline (embedded or provided bytes; no remote fetches).
- **Data Input**: Parse your data to `interface{}` (e.g., map[string]interface{}) before passing.

### Security & Performance (New in v0.2.3)

- **Path Traversal Protection**: File operations prevent `../../../etc/passwd` style attacks using secure path containment.
- **File Size Limits**: Default 10MB limit prevents memory exhaustion attacks.
- **Thread Safety**: All batch operations are race-free with proper mutex synchronization.
- **Concurrency Control**: Configurable worker pools prevent resource exhaustion.
- **Timeout Protection**: Operations timeout to prevent hanging on large file sets.

## Migration Guide

### From CLI to Library

```bash
# Old CLI approach
goneat validate data --schema-file schema.json data.json

# New library approach
result, err := schema.ValidateFileFromSchemaFile("schema.json", "data.json")
```

### From Manual Parsing to Raw Bytes

```go
// Old approach - manual parsing
dataBytes := []byte(`{"name": "test"}`)
var data interface{}
json.Unmarshal(dataBytes, &data)
result, err := schema.ValidateFromBytes(schemaBytes, data)

// New approach - automatic parsing
result, err := schema.ValidateDataFromBytes(schemaBytes, dataBytes)
```

### From Single File to Batch Processing

```go
// Old approach - individual validation
for _, file := range files {
    result, err := schema.ValidateFile(schemaBytes, file)
    // handle result
}

// New approach - concurrent batch validation
batchResult, err := schema.ValidateFiles(schemaBytes, files)
```

For CLI usage, see [validate.md](../commands/validate.md). This library enables programmatic integration beyond shelling out to the command.
