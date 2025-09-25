package guardian

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

var (
	errGrantNotFound = errors.New("guardian grant not found")
)

// IssueGrant creates a single-use grant for the given operation so subsequent hook checks can succeed.
func IssueGrant(scope, operation string, policy *ResolvedPolicy, ctx OperationContext) (*Grant, error) {
	if policy == nil {
		return nil, errors.New("policy required to issue grant")
	}

	dir, err := GrantsDir()
	if err != nil {
		return nil, err
	}

	// Opportunistically clean expired grants to keep the directory tidy.
	cleanupExpiredGrants()

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expires := now.Add(policy.Expires)
	if max := strings.TrimSpace(cfg.Guardian.Security.Grants.MaxDuration); max != "" {
		if d, derr := time.ParseDuration(max); derr == nil && d > 0 && d < policy.Expires {
			expires = now.Add(d)
		}
	}

	id, err := randomID()
	if err != nil {
		return nil, err
	}

	nonce, err := randomID()
	if err != nil {
		return nil, err
	}

	grant := &Grant{
		ID:        id,
		Scope:     scope,
		Operation: operation,
		Branch:    ctx.Branch,
		Remote:    ctx.Remote,
		User:      ctx.User,
		IssuedAt:  now,
		ExpiresAt: expires,
		Method:    policy.Method,
		Nonce:     nonce,
	}

	data, err := json.MarshalIndent(grant, "", "  ")
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, fmt.Sprintf("%s.json", id))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, fmt.Errorf("write guardian grant: %w", err)
	}

	logger.Debug("Guardian grant issued", logger.String("scope", scope), logger.String("operation", operation), logger.String("grant_id", id), logger.String("expires_at", expires.Format(time.RFC3339)))
	return grant, nil
}

// RevokeGrant removes a grant by ID (used when command execution fails after approval).
func RevokeGrant(id string) {
	if id == "" {
		return
	}
	dir, err := GrantsDir()
	if err != nil {
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("%s.json", id))
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Debug("Guardian grant revoke failed", logger.String("grant_id", id), logger.Err(err))
	}
}

func randomID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func consumeGrant(scope, operation string, ctx OperationContext) (bool, error) {
	dir, err := GrantsDir()
	if err != nil {
		return false, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("read grants dir: %w", err)
	}

	now := time.Now().UTC()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		grant, path, err := loadGrant(filepath.Join(dir, entry.Name()))
		if err != nil {
			logger.Debug("Guardian grant load failed", logger.String("file", entry.Name()), logger.Err(err))
			continue
		}

		if grant.ExpiresAt.Before(now) {
			_ = os.Remove(path)
			continue
		}

		if !grantMatches(grant, scope, operation, ctx) {
			continue
		}

		if err := os.Remove(path); err != nil {
			return false, fmt.Errorf("consume guardian grant: %w", err)
		}

		logger.Debug("Guardian grant consumed", logger.String("grant_id", grant.ID), logger.String("scope", scope), logger.String("operation", operation))
		return true, nil
	}

	return false, errGrantNotFound
}

func loadGrant(path string) (*Grant, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var grant Grant
	if err := json.Unmarshal(data, &grant); err != nil {
		return nil, "", err
	}
	return &grant, path, nil
}

func grantMatches(grant *Grant, scope, operation string, ctx OperationContext) bool {
	if grant.Scope != scope || grant.Operation != operation {
		return false
	}
	if grant.Branch != "" && !strings.EqualFold(grant.Branch, ctx.Branch) {
		return false
	}
	if grant.Remote != "" && !strings.EqualFold(grant.Remote, ctx.Remote) {
		return false
	}
	if grant.User != "" && !strings.EqualFold(grant.User, ctx.User) {
		return false
	}
	return true
}

func cleanupExpiredGrants() {
	dir, err := GrantsDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		grant, _, err := loadGrant(path)
		if err != nil {
			_ = os.Remove(path)
			continue
		}
		if grant.ExpiresAt.Before(now) {
			_ = os.Remove(path)
		}
	}
}
