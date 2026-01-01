package assess

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

const cargoDenyMinVersion = "0.14.0"

type cargoDenyAdapter struct {
	runner     *SecurityAssessmentRunner
	moduleRoot string
	cfg        AssessmentConfig
}

func (c *cargoDenyAdapter) Name() string { return "cargo-deny" }

func (c *cargoDenyAdapter) IsAvailable() bool {
	if !IsCargoAvailable() {
		return false
	}
	project := DetectRustProject(c.moduleRoot)
	if project == nil || project.CargoTomlPath == "" {
		return false
	}
	presence := CheckRustToolPresence("cargo-deny", cargoDenyMinVersion)
	if presence.Present && !presence.MeetsMin && presence.Version != "" {
		logger.Warn(fmt.Sprintf("cargo-deny %s below minimum %s; results may be unreliable", presence.Version, cargoDenyMinVersion))
	}
	return presence.Present
}

func (c *cargoDenyAdapter) Run(_ context.Context) ([]Issue, error) {
	project := DetectRustProject(c.moduleRoot)
	if project == nil || project.CargoTomlPath == "" {
		return nil, nil
	}
	root := project.EffectiveRoot()
	if root == "" {
		root = c.moduleRoot
	}

	args := []string{"deny", "check", "advisories", "sources", "--format", "json"}
	out, err := runToolStdoutOnly(root, "cargo", args, c.cfg.Timeout)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}

	entries, perr := parseCargoDenyEntries(out)
	if perr != nil {
		return nil, perr
	}

	issues := make([]Issue, 0, len(entries))
	reportFile := rustIssueFile(project)
	for _, entry := range entries {
		msg := strings.TrimSpace(entry.Message)
		if msg == "" && entry.Advisory != nil {
			msg = strings.TrimSpace(entry.Advisory.Title)
		}
		if msg == "" {
			msg = "cargo-deny finding"
		}
		if entry.Type != "" {
			msg = fmt.Sprintf("%s: %s", entry.Type, msg)
		}

		id := strings.TrimSpace(entry.ID)
		if id == "" && entry.Advisory != nil {
			id = strings.TrimSpace(entry.Advisory.ID)
		}
		if id != "" {
			msg = fmt.Sprintf("cargo-deny(%s): %s", id, msg)
		} else {
			msg = fmt.Sprintf("cargo-deny: %s", msg)
		}

		issues = append(issues, Issue{
			File:        filepath.ToSlash(reportFile),
			Severity:    mapCargoDenySeverity(entry),
			Message:     msg,
			Category:    CategorySecurity,
			SubCategory: "rust:cargo-deny",
			AutoFixable: false,
		})
	}

	return issues, nil
}

type cargoDenyEntry struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	ID       string `json:"id"`
	Code     string `json:"code"`
	URL      string `json:"url"`
	Advisory *struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Severity string `json:"severity"`
		URL      string `json:"url"`
	} `json:"advisory,omitempty"`
}

func parseCargoDenyEntries(out []byte) ([]cargoDenyEntry, error) {
	var entries []cargoDenyEntry
	if err := json.Unmarshal(out, &entries); err == nil {
		return entries, nil
	}

	var single cargoDenyEntry
	if err := json.Unmarshal(out, &single); err == nil && (single.Type != "" || single.Message != "") {
		return []cargoDenyEntry{single}, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry cargoDenyEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	if len(entries) > 0 {
		return entries, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse cargo-deny output: %w", err)
	}
	return nil, fmt.Errorf("failed to parse cargo-deny output")
}

func mapCargoDenySeverity(entry cargoDenyEntry) IssueSeverity {
	sev := strings.TrimSpace(entry.Severity)
	if sev == "" && entry.Advisory != nil {
		sev = entry.Advisory.Severity
	}
	switch strings.ToLower(sev) {
	case "critical":
		return SeverityCritical
	case "high", "error":
		return SeverityHigh
	case "medium", "moderate", "warning":
		return SeverityMedium
	case "low":
		return SeverityLow
	default:
		return SeverityMedium
	}
}

func init() {
	RegisterSecurityTool("cargo-deny", "vuln", func(r *SecurityAssessmentRunner, moduleRoot string, cfg AssessmentConfig) SecurityTool {
		return &cargoDenyAdapter{runner: r, moduleRoot: moduleRoot, cfg: cfg}
	})
}
