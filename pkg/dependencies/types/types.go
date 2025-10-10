package types

// Language represents a programming language
type Language string

const (
	LanguageGo         Language = "go"
	LanguageTypeScript Language = "typescript"
	LanguagePython     Language = "python"
	LanguageRust       Language = "rust"
	LanguageCSharp     Language = "csharp"
)

// Module represents a dependency module
type Module struct {
	Name     string
	Version  string
	Language Language
}

// License represents a software license
type License struct {
	Name string
	URL  string
	Type string // e.g., MIT, Apache-2.0
}

// Dependency represents an analyzed dependency
type Dependency struct {
	Module
	License  *License
	Metadata map[string]interface{}
}
