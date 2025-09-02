package assess

import (
	"context"
)

// SecurityTool is an adapter interface for security scanners
type SecurityTool interface {
	Name() string
	IsAvailable() bool
	Run(ctx context.Context) ([]Issue, error)
}

// SecurityToolWithSuppressions extends SecurityTool to support suppression tracking
type SecurityToolWithSuppressions interface {
	SecurityTool
	RunWithSuppressions(ctx context.Context) ([]Issue, []Suppression, error)
}

// gosecAdapter wraps the built-in gosec execution
type gosecAdapter struct {
	runner     *SecurityAssessmentRunner
	moduleRoot string
	cfg        AssessmentConfig
}

func (g *gosecAdapter) Name() string      { return "gosec" }
func (g *gosecAdapter) IsAvailable() bool { return g.runner.toolAvailable("gosec") }
func (g *gosecAdapter) Run(ctx context.Context) ([]Issue, error) {
	issues, _, err := g.runner.runGosec(ctx, g.moduleRoot, g.cfg)
	return issues, err
}

func (g *gosecAdapter) RunWithSuppressions(ctx context.Context) ([]Issue, []Suppression, error) {
	return g.runner.runGosec(ctx, g.moduleRoot, g.cfg)
}

// govulncheckAdapter wraps the built-in govulncheck execution
type govulncheckAdapter struct {
	runner     *SecurityAssessmentRunner
	moduleRoot string
	cfg        AssessmentConfig
}

func (g *govulncheckAdapter) Name() string      { return "govulncheck" }
func (g *govulncheckAdapter) IsAvailable() bool { return g.runner.toolAvailable("govulncheck") }
func (g *govulncheckAdapter) Run(ctx context.Context) ([]Issue, error) {
	return g.runner.runGovulncheck(ctx, g.moduleRoot, g.cfg)
}

// gitleaksAdapter scans for secrets using gitleaks
type gitleaksAdapter struct {
	runner     *SecurityAssessmentRunner
	moduleRoot string
	cfg        AssessmentConfig
}

func (g *gitleaksAdapter) Name() string      { return "gitleaks" }
func (g *gitleaksAdapter) IsAvailable() bool { return g.runner.toolAvailable("gitleaks") }
func (g *gitleaksAdapter) Run(ctx context.Context) ([]Issue, error) {
	return g.runner.runGitleaks(ctx, g.moduleRoot, g.cfg)
}

// Register built-in adapters
func init() {
	RegisterSecurityTool("gosec", "code", func(r *SecurityAssessmentRunner, moduleRoot string, cfg AssessmentConfig) SecurityTool {
		return &gosecAdapter{runner: r, moduleRoot: moduleRoot, cfg: cfg}
	})
	RegisterSecurityTool("govulncheck", "vuln", func(r *SecurityAssessmentRunner, moduleRoot string, cfg AssessmentConfig) SecurityTool {
		return &govulncheckAdapter{runner: r, moduleRoot: moduleRoot, cfg: cfg}
	})
	RegisterSecurityTool("gitleaks", "secrets", func(r *SecurityAssessmentRunner, moduleRoot string, cfg AssessmentConfig) SecurityTool {
		return &gitleaksAdapter{runner: r, moduleRoot: moduleRoot, cfg: cfg}
	})
}
