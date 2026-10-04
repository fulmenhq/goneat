package dependencies

import "fmt"

// enforceRequestedLicenseAssurance bounds the currently implemented collectors.
// It is applied only to an explicit license request, not cooling-only analysis.
func enforceRequestedLicenseAssurance(result *AnalysisResult, cfg AnalysisConfig, language Language) {
	if !cfg.CheckLicenses || hasLicenseCollectionError(result.Issues) {
		return
	}
	message := ""
	switch language {
	case LanguageTypeScript, LanguagePython, LanguageCSharp:
		message = fmt.Sprintf("License analysis unsupported for %s: no license inventory was produced", language)
	case LanguageRust:
		for _, issue := range result.Issues {
			if issue.Type == "configuration" {
				message = "Rust license collection unavailable: " + issue.Message + "; no license inventory was produced"
				break
			}
		}
	}
	if message == "" && len(result.Dependencies) == 0 {
		message = "Requested license inventory is empty"
	}
	if message != "" {
		result.Passed = false
		result.Issues = append(result.Issues, Issue{Type: "license_error", Severity: "critical", Message: message})
	}
}
