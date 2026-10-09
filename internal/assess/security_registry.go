package assess

import (
	"context"
	"fmt"
	"sort"
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
	adapters, _, _ := r.selectAdmissions(cfg, runner, moduleRoot)
	return adapters
}

type securityToolAdmission struct {
	Tool     string `json:"tool"`
	State    string `json:"state"`
	Reason   string `json:"reason,omitempty"`
	Explicit bool   `json:"explicit"`
}

// Applicability is separate from executable presence. Adapters that already
// have project checks can expose them without changing their selection rules.
type applicableSecurityTool interface {
	IsApplicable() bool
}

// Structured completion evidence is additive security metadata. It does not
// change the assessment-engine API, findings thresholds or suppression policy.
type securityToolWithMetadata interface {
	RunWithMetadata(context.Context) ([]Issue, map[string]interface{}, error)
}

func (r *SecurityToolRegistry) selectAdmissions(cfg AssessmentConfig, runner *SecurityAssessmentRunner, moduleRoot string) ([]SecurityTool, []securityToolAdmission, []error) {
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
	var admissions []securityToolAdmission
	var admissionErrors []error
	for _, e := range r.entries {
		admission := securityToolAdmission{Tool: e.name, Explicit: len(cfg.SecurityTools) > 0 && allowedByName(e.name)}
		if !allowedByName(e.name) {
			admission.State, admission.Reason = "not_selected", "excluded by tool selection"
			admissions = append(admissions, admission)
			continue
		}
		if !allowedByDim(e.dimension) {
			admission.State, admission.Reason = "not_selected", "dimension disabled"
			admissions = append(admissions, admission)
			continue
		}
		a := e.factory(runner, moduleRoot, cfg)
		if a == nil {
			admission.State, admission.Reason = "inapplicable", "adapter not applicable to target"
			admissions = append(admissions, admission)
			continue
		}
		if applicable, ok := a.(applicableSecurityTool); ok && !applicable.IsApplicable() {
			admission.State, admission.Reason = "inapplicable", "project not applicable to tool"
			admissions = append(admissions, admission)
			continue
		}
		if a.IsAvailable() {
			adapters = append(adapters, a)
			admission.State = "selected"
		} else {
			admission.State, admission.Reason = "unavailable", "applicable executable unavailable"
			if admission.Explicit {
				admissionErrors = append(admissionErrors, fmt.Errorf("%s: explicitly requested applicable security tool is unavailable", e.name))
			}
		}
		admissions = append(admissions, admission)
	}
	sort.Slice(adapters, func(i, j int) bool { return adapters[i].Name() < adapters[j].Name() })
	sort.Slice(admissions, func(i, j int) bool { return admissions[i].Tool < admissions[j].Tool })
	sort.Slice(admissionErrors, func(i, j int) bool { return admissionErrors[i].Error() < admissionErrors[j].Error() })
	return adapters, admissions, admissionErrors
}
