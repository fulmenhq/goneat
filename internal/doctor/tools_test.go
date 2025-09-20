package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetToolByName_Known(t *testing.T) {
	tool, ok := GetToolByName("GoSeC")
	if !ok {
		t.Fatalf("expected to find known tool 'gosec'")
	}
	if tool.Name != "gosec" {
		t.Fatalf("expected tool name 'gosec', got %q", tool.Name)
	}
}

func TestGetToolByName_Gitleaks(t *testing.T) {
	tool, ok := GetToolByName("gitleaks")
	if !ok {
		t.Fatalf("expected to find known tool 'gitleaks'")
	}
	if tool.Name != "gitleaks" {
		t.Fatalf("expected tool name 'gitleaks', got %q", tool.Name)
	}
	if tool.Kind != "go" || tool.InstallPackage == "" {
		t.Fatalf("expected gitleaks to be go-installable with a package path")
	}
	// Ensure correct module path is used
	if tool.InstallPackage != "github.com/zricethezav/gitleaks/v8@latest" {
		t.Fatalf("unexpected gitleaks install path: %q", tool.InstallPackage)
	}
}

func TestGetToolByName_Unknown(t *testing.T) {
	_, ok := GetToolByName("not-a-real-tool")
	if ok {
		t.Fatalf("expected unknown tool to return ok=false")
	}
}

func TestGoInstallCommand(t *testing.T) {
	tool := Tool{
		Name:           "gosec",
		Kind:           "go",
		InstallPackage: "github.com/securego/gosec/v2/cmd/gosec@latest",
	}
	cmd := goInstallCommand(tool)
	if !strings.Contains(cmd, "go install") || !strings.Contains(cmd, tool.InstallPackage) {
		t.Fatalf("unexpected go install command: %q", cmd)
	}
}

func TestInstallInstruction_Go(t *testing.T) {
	tool := Tool{
		Name:           "govulncheck",
		Kind:           "go",
		InstallPackage: "golang.org/x/vuln/cmd/govulncheck@latest",
	}
	inst := installInstruction(tool)
	if !strings.HasPrefix(inst, "go install ") || !strings.Contains(inst, tool.InstallPackage) {
		t.Fatalf("unexpected install instruction for go tool: %q", inst)
	}
}

func TestSanitizeVersion_CommonPatterns(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"gosec 2.19.0", "2.19.0"},
		{"version v1.2.3", "v1.2.3"},
		{"Version 1.0.0", "1.0.0"},
		{"govulncheck: version v1.1.0", "v1.1.0"},
	}
	for _, c := range cases {
		got := sanitizeVersion(c.in)
		if got != c.want {
			t.Fatalf("sanitizeVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractFirstVersionToken(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"usage: something v0.9.0 build xyz", "v0.9.0"},
		{"tool 1.2.3 extra", "1.2.3"},
		{"no version tokens here", ""},
	}
	for _, c := range cases {
		got := extractFirstVersionToken(c.in)
		if got != c.want {
			t.Fatalf("extractFirstVersionToken(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLooksLikeVersion(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"v1.2.3", true},
		{"1.2.3", true},
		{"1.2", true},
		{"v1", false},
		{"version", false},
	}
	for _, c := range cases {
		if got := looksLikeVersion(c.in); got != c.want {
			t.Fatalf("looksLikeVersion(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestKnownSecurityTools(t *testing.T) {
	tools := KnownSecurityTools() //nolint:golint,errcheck,staticcheck // function exists in tools.go
	if len(tools) == 0 {
		t.Fatal("KnownSecurityTools should return at least one tool")
	}

	// Check that all tools have required fields
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("Tool should have a non-empty name")
		}
		if tool.Kind == "" {
			t.Error("Tool should have a non-empty kind")
		}
		if tool.Kind == "go" && tool.InstallPackage == "" {
			t.Errorf("Go tool %s should have an install package", tool.Name)
		}
	}

	// Check for expected tools
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}

	expected := []string{"gosec", "govulncheck", "gitleaks"}
	for _, exp := range expected {
		found := false
		for _, name := range names {
			if name == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %s not found in KnownSecurityTools", exp)
		}
	}
}

func TestKnownFormatTools(t *testing.T) {
	tools := KnownFormatTools()
	if len(tools) == 0 {
		t.Fatal("KnownFormatTools should return at least one tool")
	}

	// Check that all tools have required fields
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("Tool should have a non-empty name")
		}
		if tool.Kind == "" {
			t.Error("Tool should have a non-empty kind")
		}
	}

	// Check for expected tools
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}

	expected := []string{"goimports", "gofmt"}
	for _, exp := range expected {
		found := false
		for _, name := range names {
			if name == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %s not found in KnownFormatTools", exp)
		}
	}
}

func TestKnownAllTools(t *testing.T) {
	allTools := KnownAllTools()
	secTools := KnownSecurityTools() //nolint:golint,errcheck,staticcheck // function exists in tools.go
	fmtTools := KnownFormatTools()
	infraTools := KnownInfrastructureTools()

	expectedCount := len(secTools) + len(fmtTools) + len(infraTools)
	if len(allTools) != expectedCount {
		t.Fatalf("KnownAllTools should return %d tools, got %d", expectedCount, len(allTools))
	}

	// Check that all security tools are included
	for _, secTool := range secTools {
		found := false
		for _, allTool := range allTools {
			if allTool.Name == secTool.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Security tool %s not found in KnownAllTools", secTool.Name)
		}
	}

	// Check that all format tools are included
	for _, fmtTool := range fmtTools {
		found := false
		for _, allTool := range allTools {
			if allTool.Name == fmtTool.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Format tool %s not found in KnownAllTools", fmtTool.Name)
		}
	}

	// Check that all foundation tools are included
	for _, infraTool := range infraTools {
		found := false
		for _, allTool := range allTools {
			if allTool.Name == infraTool.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Infrastructure tool %s not found in KnownAllTools", infraTool.Name)
		}
	}
}

func TestFirstLine(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"single line", "single line"},
		{"first line\nsecond line", "first line"},
		{"", ""},
		{"no newline", "no newline"},
	}
	for _, c := range cases {
		got := firstLine(c.in)
		if got != c.want {
			t.Errorf("firstLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInstallInstruction_BundledGo(t *testing.T) {
	tool := Tool{
		Name: "gofmt",
		Kind: "bundled-go",
	}
	inst := installInstruction(tool)
	if !strings.Contains(inst, "Go toolchain") || !strings.Contains(inst, "gofmt is included") {
		t.Errorf("unexpected install instruction for bundled-go tool: %q", inst)
	}
}

func TestInstallInstruction_System(t *testing.T) {
	tool := Tool{
		Name: "some-system-tool",
		Kind: "system",
	}
	inst := installInstruction(tool)
	if inst == "" {
		t.Fatalf("installInstruction should not return empty string")
	}
	if !strings.Contains(inst, packageManagerDocPath) && !strings.Contains(inst, tool.Name) {
		t.Errorf("unexpected install instruction for system tool: %q", inst)
	}
}

func TestGetGoBinPath(t *testing.T) {
	// Test with GOBIN set
	oldGoBin := os.Getenv("GOBIN")
	defer func() {
		if oldGoBin == "" {
			os.Unsetenv("GOBIN") //nolint:errcheck // test cleanup, error is not critical
		} else {
			os.Setenv("GOBIN", oldGoBin) //nolint:errcheck // test cleanup, error is not critical
		}
	}()

	testPath := "/test/go/bin"
	os.Setenv("GOBIN", testPath) //nolint:errcheck // test setup, error is not critical
	if got := getGoBinPath(); got != testPath {
		t.Errorf("getGoBinPath() = %q, want %q", got, testPath)
	}

	// Test with GOPATH set (no GOBIN)
	os.Unsetenv("GOBIN") //nolint:errcheck // test setup, error is not critical
	oldGoPath := os.Getenv("GOPATH")
	defer func() {
		if oldGoPath == "" {
			os.Unsetenv("GOPATH") //nolint:errcheck // test cleanup, error is not critical
		} else {
			os.Setenv("GOPATH", oldGoPath) //nolint:errcheck // test cleanup, error is not critical
		}
	}()

	os.Setenv("GOPATH", "/test/gopath") //nolint:errcheck // test setup, error is not critical
	expected := filepath.Join("/test/gopath", "bin")
	if got := getGoBinPath(); got != expected {
		t.Errorf("getGoBinPath() = %q, want %q", got, expected)
	}

	// Test default case (no GOBIN, no GOPATH)
	os.Unsetenv("GOPATH") //nolint:errcheck // test setup, error is not critical
	if got := getGoBinPath(); got == "" {
		t.Error("getGoBinPath() should not return empty when home directory is accessible")
	} else {
		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, "go", "bin")
		if got != expected {
			t.Errorf("getGoBinPath() = %q, want %q", got, expected)
		}
	}
}

func TestDetectVersion_NoArgs(t *testing.T) {
	tool := Tool{
		Name:        "echo",
		VersionArgs: []string{},
		CheckArgs:   []string{"hello"},
	}
	version := detectVersion(tool)
	// echo with "hello" should return something, but we don't care about the exact value
	// just that it doesn't panic and returns a string
	if version == "" {
		t.Log("detectVersion returned empty string (this may be normal)")
	}
}

func TestTryCommand(t *testing.T) {
	// Test with a command that should work
	output, ok := tryCommand("echo", "test")
	if !ok {
		t.Error("tryCommand with echo should succeed")
	}
	if !strings.Contains(output, "test") {
		t.Errorf("tryCommand output should contain 'test', got %q", output)
	}

	// Test with a command that should fail
	_, ok = tryCommand("nonexistent-command", "arg")
	if ok {
		t.Error("tryCommand with nonexistent command should fail")
	}
}

func TestCheckTool_Present(t *testing.T) {
	// Test with a tool that should be present (echo)
	tool := Tool{
		Name:        "echo",
		Kind:        "system",
		VersionArgs: []string{"--version"},
		CheckArgs:   []string{"hello"},
	}

	status := CheckTool(tool)
	if !status.Present {
		t.Errorf("CheckTool should find echo as present")
	}
	if status.Name != "echo" {
		t.Errorf("CheckTool should return correct tool name")
	}
	if status.Error != nil {
		t.Errorf("CheckTool should not return error for present tool: %v", status.Error)
	}
}

func TestCheckTool_NotPresent(t *testing.T) {
	// Test with a tool that should not be present
	tool := Tool{
		Name:        "nonexistent-tool-12345",
		Kind:        "system",
		VersionArgs: []string{"--version"},
		CheckArgs:   []string{"--help"},
	}

	status := CheckTool(tool)
	if status.Present {
		t.Errorf("CheckTool should not find nonexistent tool as present")
	}
	if status.Name != "nonexistent-tool-12345" {
		t.Errorf("CheckTool should return correct tool name")
	}
	if status.Error != nil {
		t.Errorf("CheckTool should not return error for non-present tool: %v", status.Error)
	}
	if status.Instructions == "" {
		t.Error("CheckTool should provide installation instructions for non-present tool")
	}
}

func TestInstallTool_NonGo(t *testing.T) {
	// Test installing a system tool (should provide installation instructions)
	tool := Tool{
		Name: "some-system-tool",
		Kind: "system",
		InstallCommands: map[string]string{
			"linux":  "false", // force failure without sudo
			"darwin": "false",
		},
		InstallerPriority: map[string][]string{
			"linux":  {string(installerMise), string(installerAptGet)},
			"darwin": {string(installerMise), string(installerBrew)},
		},
	}

	status := InstallTool(tool)
	if status.Installed {
		t.Error("InstallTool should not mark system tools as installed when installation fails")
	}
	if status.Error == nil {
		t.Error("InstallTool should return error for failed system tool installation")
	}
	if !strings.Contains(status.Instructions, packageManagerDocPath) {
		t.Errorf("InstallTool should reference package manager guidance, got: %s", status.Instructions)
	}
}

func TestLoadToolsConfig(t *testing.T) {
	config, err := LoadToolsConfig()
	if err != nil {
		t.Errorf("LoadToolsConfig should not return error: %v", err)
	}

	if config == nil {
		t.Fatal("LoadToolsConfig should return a non-nil config")
	}

	if len(config.Scopes) == 0 {
		t.Error("Config should have at least one scope")
	}

	if len(config.Tools) == 0 {
		t.Error("Config should have at least one tool")
	}
}

func TestParseConfig(t *testing.T) {
	yamlConfig := `
scopes:
  foundation:
    description: "Foundation tools"
    tools: ["ripgrep", "jq"]
tools:
  ripgrep:
    name: "ripgrep"
    description: "Fast text search"
    kind: "system"
    detect_command: "rg --version"
`

	config, err := ParseConfig([]byte(yamlConfig))
	if err != nil {
		t.Errorf("ParseConfig should not return error: %v", err)
	}

	if config == nil {
		t.Fatal("ParseConfig should return a non-nil config")
	}

	if len(config.Scopes) != 1 {
		t.Errorf("Expected 1 scope, got %d", len(config.Scopes))
	}

	if len(config.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(config.Tools))
	}

	// Check that the tool was parsed correctly
	tool, exists := config.Tools["ripgrep"]
	if !exists {
		t.Error("ripgrep tool should exist in config")
	}

	if tool.Name != "ripgrep" {
		t.Errorf("Expected tool name 'ripgrep', got '%s'", tool.Name)
	}

	if tool.Kind != "system" {
		t.Errorf("Expected tool kind 'system', got '%s'", tool.Kind)
	}
}

func TestParseConfig_InvalidYAML(t *testing.T) {
	invalidYAML := `
scopes:
  foundation:
    description: "Foundation tools"
    tools: ["ripgrep", "jq"]
  invalid: not a map
`

	_, err := ParseConfig([]byte(invalidYAML))
	if err == nil {
		t.Error("ParseConfig should return error for invalid YAML")
	}
}

func TestValidateConfig(t *testing.T) {
	validConfigPath := ".goneat/tools.yaml"
	err := ValidateConfig(validConfigPath)
	// This might fail if the file doesn't exist, which is OK for this test
	// The important thing is that the function doesn't panic
	if err != nil && !strings.Contains(err.Error(), "no such file") {
		t.Errorf("ValidateConfig should handle file operations gracefully: %v", err)
	}
}

func TestToolsConfig_GetToolsForScope(t *testing.T) {
	config := &ToolsConfig{
		Scopes: map[string]ScopeConfig{
			"foundation": {
				Description: "Foundation tools",
				Tools:       []string{"ripgrep", "jq"},
			},
		},
		Tools: map[string]ToolConfig{
			"ripgrep": {
				Name:          "ripgrep",
				Description:   "Fast text search",
				Kind:          "system",
				DetectCommand: "rg --version",
			},
			"jq": {
				Name:          "jq",
				Description:   "JSON processor",
				Kind:          "system",
				DetectCommand: "jq --version",
			},
		},
	}

	tools, err := config.GetToolsForScope("foundation")
	if err != nil {
		t.Errorf("GetToolsForScope should not return error: %v", err)
	}

	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	// Check that tools are returned in the correct order
	if tools[0].Name != "ripgrep" {
		t.Errorf("First tool should be ripgrep, got %s", tools[0].Name)
	}

	if tools[1].Name != "jq" {
		t.Errorf("Second tool should be jq, got %s", tools[1].Name)
	}
}

func TestToolsConfig_GetToolsForScope_InvalidScope(t *testing.T) {
	config := &ToolsConfig{
		Scopes: map[string]ScopeConfig{},
	}

	_, err := config.GetToolsForScope("nonexistent")
	if err == nil {
		t.Error("GetToolsForScope should return error for nonexistent scope")
	}
}

func TestToolsConfig_GetTool(t *testing.T) {
	config := &ToolsConfig{
		Tools: map[string]ToolConfig{
			"ripgrep": {
				Name:          "ripgrep",
				Description:   "Fast text search",
				Kind:          "system",
				DetectCommand: "rg --version",
			},
		},
	}

	tool, exists := config.GetTool("ripgrep")
	if !exists {
		t.Error("GetTool should return true for existing tool")
	}

	if tool.Name != "ripgrep" {
		t.Errorf("Expected tool name 'ripgrep', got '%s'", tool.Name)
	}

	_, exists = config.GetTool("nonexistent")
	if exists {
		t.Error("GetTool should return false for nonexistent tool")
	}
}

func TestToolsConfig_GetAllScopes(t *testing.T) {
	config := &ToolsConfig{
		Scopes: map[string]ScopeConfig{
			"foundation": {
				Description: "Foundation tools",
				Tools:       []string{"ripgrep", "jq"},
			},
			"security": {
				Description: "Security tools",
				Tools:       []string{"gosec", "gitleaks"},
			},
		},
	}

	scopes := config.GetAllScopes()

	if len(scopes) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(scopes))
	}

	// Check that both scopes are in the result
	found := make(map[string]bool)
	for _, scope := range scopes {
		found[scope] = true
	}

	if !found["foundation"] {
		t.Error("foundation scope should be in result")
	}

	if !found["security"] {
		t.Error("security scope should be in result")
	}
}

func TestInstallTool_NoGoToolchain(t *testing.T) {
	// This test would require mocking exec.LookPath to return error for "go"
	// For now, we'll skip this test as it requires more complex mocking
	t.Skip("TestInstallTool_NoGoToolchain requires exec mocking")
}

func TestSanitizeVersion_EdgeCases(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"no version here", "no version here"},
		{"version", "version"},
		{"Version", "Version"},
		{"v", "v"},
		{"govulncheck: version ", "version"}, // After govulncheck: prefix removal
		{"version v1.2.3", "v1.2.3"},         // Normal case
		{"Version 1.0.0", "1.0.0"},           // Normal case
	}
	for _, c := range cases {
		got := sanitizeVersion(c.in)
		if got != c.want {
			t.Errorf("sanitizeVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractFirstVersionToken_EdgeCases(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"no versions", ""},
		{"text without version numbers", ""},
		{"text with v1.2.3 and more text", "v1.2.3"},
		{"text with 1.2.3 and more text", "1.2.3"},
		{"text with 1.2 and more text", "1.2"},
		{"text with v1 and more text", ""}, // v1 should not be considered a version
	}
	for _, c := range cases {
		got := extractFirstVersionToken(c.in)
		if got != c.want {
			t.Errorf("extractFirstVersionToken(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLooksLikeVersion_EdgeCases(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"v", false},
		{"1", false},
		{"1.2.3.4", true},     // 3 dots is within limit (1.2.3.4 has dots at positions 1-2, 2-3, 3-4)
		{"v1.2.3.4", true},    // After 'v' removal: 3 dots
		{"1.2.3.4.5", false},  // 4 dots exceeds limit
		{"v1.2.3.4.5", false}, // After 'v' removal: 4 dots
		{"0.0.0", true},
		{"999.999.999", true},
		{"1.2.3-snapshot", true}, // function only checks dots, not content
		{"version", false},
	}
	for _, c := range cases {
		got := looksLikeVersion(c.in)
		if got != c.want {
			t.Errorf("looksLikeVersion(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
