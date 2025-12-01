package guardian

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// ErrApprovalRequired indicates the operation requires approval before proceeding.
var ErrApprovalRequired = errors.New("guardian approval required")

// ApprovalRequiredError carries additional context for approval requirements.
type ApprovalRequiredError struct {
	Scope     string
	Operation string
	Policy    *ResolvedPolicy
}

// Error implements the error interface.
func (e *ApprovalRequiredError) Error() string {
	return fmt.Sprintf("guardian approval required for %s.%s", e.Scope, e.Operation)
}

// Unwrap allows errors.Is comparisons with ErrApprovalRequired sentinel.
func (e *ApprovalRequiredError) Unwrap() error {
	return ErrApprovalRequired
}

// Engine evaluates guardian policies.
type Engine struct {
	config *ConfigRoot
}

// NewEngine loads configuration and returns a ready guardian engine.
func NewEngine() (*Engine, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return &Engine{config: cfg}, nil
}

// Check evaluates the policy for the given scope/operation.
// It returns (nil, nil) when no policy is enforced.
func (e *Engine) Check(scope, operation string, ctx OperationContext) (*ResolvedPolicy, error) {
	policy, enforced, err := e.config.ResolvePolicy(scope, operation)
	if err != nil {
		return nil, err
	}
	if !enforced || policy == nil {
		return nil, nil
	}

	if !passesConditions(policy, ctx) {
		return nil, nil
	}

	// Allow single-use grants to satisfy guardian checks before re-evaluating policy.
	if used, grantErr := consumeGrant(scope, operation, ctx); used {
		return nil, nil
	} else if grantErr != nil && !errors.Is(grantErr, errGrantNotFound) {
		logger.Debug("Guardian grant lookup failed", logger.String("scope", scope), logger.String("operation", operation), logger.Err(grantErr))
	}

	// Future commits will integrate grants/approvals. For now return the policy and signal approval requirement.
	return policy, &ApprovalRequiredError{Scope: scope, Operation: operation, Policy: policy}
}

func passesConditions(policy *ResolvedPolicy, ctx OperationContext) bool {
	if len(policy.Conditions) == 0 {
		return true
	}

	for key, values := range policy.Conditions {
		switch strings.ToLower(key) {
		case "branches":
			if ctx.Branch == "" {
				return false
			}
			if !matchesAny(values, ctx.Branch) {
				return false
			}
		case "remote_patterns", "remotes":
			if ctx.Remote == "" {
				return false
			}
			// Security fix: If remote looks like a URL (contains :// or starts with git@),
			// we cannot reliably match it against name-based patterns like "origin".
			// Fail-closed: treat URL remotes as matching the condition to require approval.
			// This prevents bypass via URL-based remotes that should require approval.
			if looksLikeURL(ctx.Remote) {
				// URL detected - fail closed by treating as a match (requiring approval)
				// rather than auto-approving because URL doesn't match "origin"
				logger.Debug("Guardian: URL-based remote detected, requiring approval for safety",
					logger.String("remote", ctx.Remote))
				continue // Condition passes - proceed to next condition
			}
			if !matchesAny(values, ctx.Remote) {
				return false
			}
		default:
			// Unknown conditions default to pass for forward compatibility.
		}
	}

	return true
}

// looksLikeURL returns true if the value appears to be a URL rather than a remote name.
// This detects patterns like:
//   - https://github.com/user/repo.git
//   - http://gitlab.example.com/repo
//   - git@github.com:user/repo.git
//   - ssh://git@github.com/user/repo.git
func looksLikeURL(value string) bool {
	// Check for protocol schemes
	if strings.Contains(value, "://") {
		return true
	}
	// Check for SSH-style URLs (git@host:path)
	if strings.HasPrefix(value, "git@") && strings.Contains(value, ":") {
		return true
	}
	// Check for common hosting domains (additional safety)
	lowerVal := strings.ToLower(value)
	urlIndicators := []string{
		"github.com", "gitlab.com", "bitbucket.org",
		"azure.com", "visualstudio.com", "codecommit",
	}
	for _, indicator := range urlIndicators {
		if strings.Contains(lowerVal, indicator) {
			return true
		}
	}
	return false
}

func matchesAny(patterns []string, value string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if ok, err := filepath.Match(pattern, value); err == nil && ok {
			return true
		}
		if pattern == value {
			return true
		}
	}
	return false
}

// CheckAndExplain runs Check and annotates common errors for CLI consumption.
func CheckAndExplain(scope, operation string, ctx OperationContext) (*ResolvedPolicy, error) {
	engine, err := NewEngine()
	if err != nil {
		return nil, err
	}
	policy, err := engine.Check(scope, operation, ctx)
	if err == nil {
		return policy, nil
	}
	if errors.Is(err, ErrApprovalRequired) {
		return policy, err
	}
	return nil, fmt.Errorf("guardian check failed: %w", err)
}

// IsApprovalRequired reports whether the error represents a guardian approval requirement.
func IsApprovalRequired(err error) bool {
	return errors.Is(err, ErrApprovalRequired)
}
