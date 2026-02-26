package assess

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

type TypecheckAssessmentRunner struct {
	commandName string
}

type typecheckOverrides struct {
	Enabled    *bool                      `yaml:"enabled"`
	Typescript *typescriptTypecheckConfig `yaml:"typescript"`
}

type typescriptTypecheckConfig struct {
	Enabled      *bool  `yaml:"enabled"`
	Config       string `yaml:"config"`
	Strict       *bool  `yaml:"strict"`
	SkipLibCheck *bool  `yaml:"skip_lib_check"`
	FileMode     *bool  `yaml:"file_mode"`
}

func NewTypecheckAssessmentRunner() *TypecheckAssessmentRunner {
	return &TypecheckAssessmentRunner{commandName: "typecheck"}
}

func (r *TypecheckAssessmentRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	startTime := time.Now()
	logger.Info(fmt.Sprintf("Running typecheck assessment on %s", target))

	langFiles, err := collectLanguageFiles(target, config)
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryTypecheck,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("failed to discover language files: %v", err),
		}, nil
	}

	overrides := loadAssessOverrides(target)
	if overrides != nil && overrides.Typecheck != nil {
		if !boolWithDefault(overrides.Typecheck.Enabled, true) {
			return r.skippedResult(startTime, "typecheck disabled via assess.yaml"), nil
		}
	}

	if len(langFiles.TS) == 0 {
		return r.skippedResult(startTime, "no TypeScript files detected"), nil
	}

	resolver := NewConfigResolver(target)
	workingDir := resolver.GetWorkingDir()
	var tsOverrides *typescriptTypecheckConfig
	if overrides != nil && overrides.Typecheck != nil {
		tsOverrides = overrides.Typecheck.Typescript
	}
	if tsOverrides != nil && !boolWithDefault(tsOverrides.Enabled, true) {
		return r.skippedResult(startTime, "typescript typecheck disabled via assess.yaml"), nil
	}

	configPath, err := resolveTsconfigPath(workingDir, tsOverrides)
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryTypecheck,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         err.Error(),
		}, nil
	}
	if configPath == "" {
		return r.skippedResult(startTime, "no tsconfig found"), nil
	}

	tscBin, err := resolveTscBinary(workingDir)
	if err != nil {
		logger.Info("tsc not found; skipping TypeScript typecheck")
		return r.skippedResult(startTime, "tsc not found"), nil
	}

	useFileMode := tsOverrides != nil && boolWithDefault(tsOverrides.FileMode, false)
	selectedFile := ""
	if useFileMode {
		if len(langFiles.TS) == 1 {
			selectedFile = langFiles.TS[0]
		} else {
			logger.Warn("Typecheck file_mode requires a single TypeScript file; falling back to project-wide check")
			useFileMode = false
		}
	}

	cleanupTemp := func() {}
	if useFileMode {
		tempConfig, cleanup, tempErr := writeTempTsconfig(workingDir, configPath, selectedFile)
		if tempErr != nil {
			return &AssessmentResult{
				CommandName:   r.commandName,
				Category:      CategoryTypecheck,
				Success:       false,
				ExecutionTime: HumanReadableDuration(time.Since(startTime)),
				Error:         tempErr.Error(),
			}, nil
		}
		cleanupTemp = cleanup
		configPath = tempConfig
	}
	defer cleanupTemp()

	args := []string{"--noEmit", "--pretty", "false", "--project", configPath}
	if useFileMode && (strings.HasSuffix(strings.ToLower(selectedFile), ".tsx") || strings.HasSuffix(strings.ToLower(selectedFile), ".jsx")) {
		args = append(args, "--jsx", "react-jsx")
	}
	if tsOverrides != nil {
		if boolWithDefault(tsOverrides.Strict, false) {
			args = append(args, "--strict")
		}
		if boolWithDefault(tsOverrides.SkipLibCheck, true) {
			args = append(args, "--skipLibCheck")
		}
	}

	out, exitCode, runErr := runToolCapture(workingDir, tscBin, args, config.Timeout)
	if runErr != nil && exitCode == 0 {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryTypecheck,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("tsc execution failed: %v", runErr),
		}, nil
	}

	issues := parseTscOutput(string(out))
	if exitCode != 0 && len(issues) == 0 {
		errMsg := strings.TrimSpace(string(out))
		if errMsg == "" {
			errMsg = fmt.Sprintf("tsc failed with exit code %d", exitCode)
		}
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryTypecheck,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         errMsg,
		}, nil
	}

	logger.Info(fmt.Sprintf("typecheck completed: %d issues found", len(issues)))
	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategoryTypecheck,
		Success:       true,
		ExecutionTime: HumanReadableDuration(time.Since(startTime)),
		Issues:        issues,
	}, nil
}

func (r *TypecheckAssessmentRunner) CanRunInParallel() bool {
	return true
}

func (r *TypecheckAssessmentRunner) GetCategory() AssessmentCategory {
	return CategoryTypecheck
}

func (r *TypecheckAssessmentRunner) GetEstimatedTime(target string) time.Duration {
	estimated := 5 * time.Minute
	return estimated
}

func (r *TypecheckAssessmentRunner) IsAvailable() bool {
	return true
}

func (r *TypecheckAssessmentRunner) skippedResult(start time.Time, reason string) *AssessmentResult {
	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategoryTypecheck,
		Success:       true,
		ExecutionTime: HumanReadableDuration(time.Since(start)),
		Issues:        []Issue{},
		Metrics: map[string]interface{}{
			"status": "skipped",
			"reason": reason,
		},
	}
}

func resolveTsconfigPath(workingDir string, overrides *typescriptTypecheckConfig) (string, error) {
	if overrides != nil && strings.TrimSpace(overrides.Config) != "" {
		candidate := filepath.Clean(overrides.Config)
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(workingDir, candidate)
		}
		if _, err := os.Stat(candidate); err != nil {
			return "", fmt.Errorf("tsconfig not found at %s", candidate)
		}
		return candidate, nil
	}

	candidates := []string{"tsconfig.json", "tsconfig.build.json"}
	for _, name := range candidates {
		candidate := filepath.Join(workingDir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", nil
}

func resolveTscBinary(workingDir string) (string, error) {
	local := filepath.Join(workingDir, "node_modules", ".bin", "tsc")
	if info, err := os.Stat(local); err == nil && !info.IsDir() {
		return local, nil
	}
	return exec.LookPath("tsc")
}

type tsconfigTemp struct {
	Extends string   `json:"extends"`
	Include []string `json:"include"`
}

func writeTempTsconfig(workingDir, baseConfigPath, targetFile string) (string, func(), error) {
	absoluteConfig := baseConfigPath
	if !filepath.IsAbs(baseConfigPath) {
		if abs, err := filepath.Abs(baseConfigPath); err == nil {
			absoluteConfig = abs
		}
	}
	relTarget := targetFile
	if filepath.IsAbs(targetFile) {
		if rel, err := filepath.Rel(workingDir, targetFile); err == nil {
			relTarget = rel
		}
	}
	payload := tsconfigTemp{
		Extends: filepath.ToSlash(absoluteConfig),
		Include: []string{filepath.ToSlash(relTarget)},
	}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to marshal temporary tsconfig: %w", err)
	}

	file, err := os.CreateTemp(workingDir, "tsconfig.goneat-file-*.json")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create temporary tsconfig: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", func() {}, fmt.Errorf("failed to finalize temporary tsconfig: %w", err)
	}
	if err := os.WriteFile(file.Name(), content, 0600); err != nil { // #nosec G703 - file.Name() from os.CreateTemp(), system-generated temp path
		return "", func() {}, fmt.Errorf("failed to write temporary tsconfig: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(file.Name()) // #nosec G703 - file.Name() from os.CreateTemp(), removing system-generated temp file
	}
	return file.Name(), cleanup, nil // #nosec G703 - returning system-generated temp file path from os.CreateTemp()
}

var tscLinePattern = regexp.MustCompile(`^(.+)\((\d+),(\d+)\): (error|warning) (TS\d+): (.+)$`)

func parseTscOutput(output string) []Issue {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	var issues []Issue
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		matches := tscLinePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		lineNum, _ := strconv.Atoi(matches[2])
		colNum, _ := strconv.Atoi(matches[3])
		severity := SeverityMedium
		if strings.ToLower(matches[4]) == "error" {
			severity = SeverityHigh
		}
		code := strings.TrimSpace(matches[5])
		message := strings.TrimSpace(matches[6])
		if code != "" {
			message = fmt.Sprintf("%s: %s", code, message)
		}
		issues = append(issues, Issue{
			File:        filepath.ToSlash(matches[1]),
			Line:        lineNum,
			Column:      colNum,
			Severity:    severity,
			Message:     message,
			Category:    CategoryTypecheck,
			SubCategory: "typescript:tsc",
		})
	}
	return issues
}

func init() {
	RegisterAssessmentRunner(CategoryTypecheck, NewTypecheckAssessmentRunner())
}
