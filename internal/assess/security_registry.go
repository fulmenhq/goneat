package assess

import (
	"strings"
)

// SecurityToolFactory constructs a SecurityTool for a given runner/context
type SecurityToolFactory func(r *SecurityAssessmentRunner, moduleRoot string, cfg AssessmentConfig) SecurityTool

type securityToolEntry struct {
	name      string
	dimension string // "code", "vuln", "secrets"
	factory   SecurityToolFactory
}

// SecurityToolRegistry maintains available security tool adapters
type SecurityToolRegistry struct {
	entries []securityToolEntry
}

var securityRegistry = &SecurityToolRegistry{}

// RegisterSecurityTool registers a security tool adapter with its dimension
func RegisterSecurityTool(name, dimension string, factory SecurityToolFactory) {
	securityRegistry.entries = append(securityRegistry.entries, securityToolEntry{
		name:      strings.ToLower(strings.TrimSpace(name)),
		dimension: strings.ToLower(strings.TrimSpace(dimension)),
		factory:   factory,
	})
}

// GetSecurityToolRegistry returns the global security tool registry
func GetSecurityToolRegistry() *SecurityToolRegistry { return securityRegistry }

// SelectAdapters returns adapters based on config flags and availability
func (r *SecurityToolRegistry) SelectAdapters(cfg AssessmentConfig, runner *SecurityAssessmentRunner, moduleRoot string) []SecurityTool {
	// Determine which dimensions are enabled
	enableCode := cfg.EnableCode || (!cfg.EnableVuln && !cfg.EnableSecrets)
	enableVuln := cfg.EnableVuln || (!cfg.EnableCode && !cfg.EnableSecrets)
	enableSecrets := cfg.EnableSecrets

	// Helper to check name filter
	allowedByName := func(name string) bool {
		if len(cfg.SecurityTools) == 0 {
			return true
		}
		for _, t := range cfg.SecurityTools {
			if strings.EqualFold(strings.TrimSpace(t), name) {
				return true
			}
		}
		return false
	}

	// Helper to check dimension filter
	allowedByDim := func(dim string) bool {
		switch strings.ToLower(dim) {
		case "code":
			return enableCode
		case "vuln", "vulnerability", "dependencies":
			return enableVuln
		case "secrets":
			return enableSecrets
		default:
			return true
		}
	}

	var adapters []SecurityTool
	for _, e := range r.entries {
		if !allowedByName(e.name) || !allowedByDim(e.dimension) {
			continue
		}
		a := e.factory(runner, moduleRoot, cfg)
		if a == nil {
			continue
		}
		if a.IsAvailable() {
			adapters = append(adapters, a)
		}
	}
	return adapters
}
