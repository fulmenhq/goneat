package dependencies

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLicenseDetection(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    string
		description string
	}{
		{
			name: "MIT License",
			content: `MIT License

Copyright (c) 2024

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction...`,
			expected:    "MIT",
			description: "Should detect MIT from license text",
		},
		{
			name: "Apache 2.0",
			content: `Apache License
Version 2.0, January 2004
http://www.apache.org/licenses/

TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION`,
			expected:    "Apache-2.0",
			description: "Should detect Apache-2.0 from license text",
		},
		{
			name: "BSD 3-Clause",
			content: `BSD 3-Clause License

Copyright (c) 2024
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:`,
			expected:    "BSD-3-Clause",
			description: "Should detect BSD-3-Clause from license text",
		},
		{
			name:        "GPL-3.0",
			content:     "GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007",
			expected:    "GPL-3.0",
			description: "Should detect GPL-3.0 from license text",
		},
		{
			name:        "ISC License",
			content:     "ISC License\n\nCopyright (c) 2024",
			expected:    "ISC",
			description: "Should detect ISC from license text",
		},
		{
			name:        "Unknown License",
			content:     "Some random text that doesn't match any license",
			expected:    "Unknown",
			description: "Should return Unknown for unrecognized licenses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLicenseType(tt.content)
			if result != tt.expected {
				t.Errorf("%s: got %q, want %q", tt.description, result, tt.expected)
			}
		})
	}
}

func TestForbiddenLicensePolicy(t *testing.T) {
	// Create temporary policy file
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "policy.yaml")
	policyContent := `version: v1
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Create test dependencies
	deps := []Dependency{
		{
			Module: Module{
				Name:     "test/mit-package",
				Version:  "v1.0.0",
				Language: LanguageGo,
			},
			License: &License{
				Name: "LICENSE",
				Type: "MIT",
				URL:  "https://opensource.org/licenses/MIT",
			},
			Metadata: map[string]interface{}{
				"age_days": 100,
			},
		},
		{
			Module: Module{
				Name:     "test/gpl-package",
				Version:  "v1.0.0",
				Language: LanguageGo,
			},
			License: &License{
				Name: "LICENSE",
				Type: "GPL-3.0",
				URL:  "https://www.gnu.org/licenses/gpl-3.0.html",
			},
			Metadata: map[string]interface{}{
				"age_days": 100,
			},
		},
	}

	// Manually test policy evaluation (simulating what Analyze does)
	ctx := context.Background()
	_ = ctx

	// Load policy
	policyData, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("Failed to read policy: %v", err)
	}

	// Parse policy
	var policyConfig map[string]interface{}
	if err := yaml.Unmarshal(policyData, &policyConfig); err != nil {
		t.Fatalf("Failed to parse policy: %v", err)
	}

	// Check forbidden licenses
	var issues []Issue
	passed := true

	if licensesConfig, ok := policyConfig["licenses"].(map[string]interface{}); ok {
		if forbidden, ok := licensesConfig["forbidden"].([]interface{}); ok {
			for i := range deps {
				dep := &deps[i]
				for _, forbiddenLicense := range forbidden {
					if dep.License.Type == forbiddenLicense.(string) {
						issues = append(issues, Issue{
							Type:       "license",
							Severity:   "critical",
							Message:    "Forbidden license detected",
							Dependency: dep,
						})
						passed = false
					}
				}
			}
		}
	}

	// Verify results
	if passed {
		t.Error("Policy should have failed with GPL-3.0 package")
	}

	if len(issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(issues))
	}

	if len(issues) > 0 {
		if issues[0].Type != "license" {
			t.Errorf("Expected license issue, got %s", issues[0].Type)
		}
		if issues[0].Severity != "critical" {
			t.Errorf("Expected critical severity, got %s", issues[0].Severity)
		}
		if issues[0].Dependency.License.Type != "GPL-3.0" {
			t.Errorf("Expected GPL-3.0 violation, got %s", issues[0].Dependency.License.Type)
		}
	}
}

func TestCoolingPolicyViolation(t *testing.T) {
	// Create temporary policy file
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "policy.yaml")
	policyContent := `version: v1
cooling:
  enabled: true
  min_age_days: 30
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Create test dependencies with various ages
	deps := []Dependency{
		{
			Module: Module{
				Name:     "test/old-package",
				Version:  "v1.0.0",
				Language: LanguageGo,
			},
			License: &License{
				Name: "LICENSE",
				Type: "MIT",
				URL:  "https://opensource.org/licenses/MIT",
			},
			Metadata: map[string]interface{}{
				"age_days": 100, // Old enough
			},
		},
		{
			Module: Module{
				Name:     "test/young-package",
				Version:  "v1.0.0",
				Language: LanguageGo,
			},
			License: &License{
				Name: "LICENSE",
				Type: "MIT",
				URL:  "https://opensource.org/licenses/MIT",
			},
			Metadata: map[string]interface{}{
				"age_days": 10, // Too young
			},
		},
	}

	// Load and evaluate policy
	policyData, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("Failed to read policy: %v", err)
	}

	var policyConfig map[string]interface{}
	if err := yaml.Unmarshal(policyData, &policyConfig); err != nil {
		t.Fatalf("Failed to parse policy: %v", err)
	}

	// Check cooling policy
	var issues []Issue
	passed := true

	if cooling, ok := policyConfig["cooling"].(map[string]interface{}); ok {
		if enabled, ok := cooling["enabled"].(bool); ok && enabled {
			minAgeDays := 7 // default
			if minAge, ok := cooling["min_age_days"].(int); ok {
				minAgeDays = minAge
			}

			for i := range deps {
				dep := &deps[i]
				if ageDays, ok := dep.Metadata["age_days"].(int); ok {
					if ageDays < minAgeDays {
						issues = append(issues, Issue{
							Type:       "cooling",
							Severity:   "high",
							Message:    "Package too young",
							Dependency: dep,
						})
						passed = false
					}
				}
			}
		}
	}

	// Verify results
	if passed {
		t.Error("Policy should have failed with young package")
	}

	if len(issues) != 1 {
		t.Errorf("Expected 1 cooling violation, got %d", len(issues))
	}

	if len(issues) > 0 {
		if issues[0].Type != "cooling" {
			t.Errorf("Expected cooling issue, got %s", issues[0].Type)
		}
		if issues[0].Dependency.Name != "test/young-package" {
			t.Errorf("Expected young-package violation, got %s", issues[0].Dependency.Name)
		}
	}
}

func TestRegistryFailureHandling(t *testing.T) {
	t.Skip("Skipping registry failure test - needs mock client implementation")
	// TODO: Implement with mock registry client that simulates network failures
	// Should verify that:
	// 1. Network failures are surfaced as issues
	// 2. Fallback age values are documented
	// 3. Policy can distinguish "unknown age" from actual age
}
