package dependencies

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/fulmenhq/goneat/pkg/config"
)

// Detector implements LanguageDetector
type Detector struct {
	Config *config.DependenciesConfig
}

func NewDetector(cfg *config.DependenciesConfig) LanguageDetector {
	return &Detector{Config: cfg}
}

func (d *Detector) Detect(target string) (Language, bool, error) {
	// Honor explicit languages from config
	if d.Config != nil && len(d.Config.Languages) > 0 {
		for _, langCfg := range d.Config.Languages {
			for _, path := range langCfg.Paths {
				if fullPath, err := filepath.Abs(filepath.Join(target, path)); err == nil {
					if _, statErr := os.Stat(fullPath); statErr == nil {
						return Language(langCfg.Language), true, nil
					}
				}
			}
		}
	}

	// Fallback to auto-detection if no explicit match
	if _, err := os.Stat(filepath.Join(target, "go.mod")); err == nil {
		return LanguageGo, true, nil
	}

	// TypeScript/JavaScript: package.json
	if _, err := os.Stat(filepath.Join(target, "package.json")); err == nil {
		return LanguageTypeScript, true, nil
	}

	// Python: pyproject.toml or requirements.txt
	if _, err := os.Stat(filepath.Join(target, "pyproject.toml")); err == nil {
		return LanguagePython, true, nil
	}
	if _, err := os.Stat(filepath.Join(target, "requirements.txt")); err == nil {
		return LanguagePython, true, nil
	}

	// Rust: Cargo.toml
	if _, err := os.Stat(filepath.Join(target, "Cargo.toml")); err == nil {
		return LanguageRust, true, nil
	}

	// C#: *.csproj
	files, _ := filepath.Glob(filepath.Join(target, "*.csproj"))
	if len(files) > 0 {
		return LanguageCSharp, true, nil
	}

	return "", false, nil
}

func (d *Detector) GetManifestFiles(target string) ([]string, error) {
	lang, found, err := d.Detect(target)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("no supported language manifest found")
	}

	switch lang {
	case LanguageGo:
		return []string{"go.mod", "go.sum"}, nil
	case LanguageTypeScript:
		return []string{"package.json", "package-lock.json"}, nil
	case LanguagePython:
		// Check which manifest files exist
		manifests := []string{}
		if _, err := os.Stat(filepath.Join(target, "pyproject.toml")); err == nil {
			manifests = append(manifests, "pyproject.toml")
		}
		if _, err := os.Stat(filepath.Join(target, "requirements.txt")); err == nil {
			manifests = append(manifests, "requirements.txt")
		}
		if _, err := os.Stat(filepath.Join(target, "poetry.lock")); err == nil {
			manifests = append(manifests, "poetry.lock")
		}
		if len(manifests) == 0 {
			return nil, errors.New("no Python manifest files found")
		}
		return manifests, nil
	case LanguageRust:
		return []string{"Cargo.toml", "Cargo.lock"}, nil
	case LanguageCSharp:
		// Find all .csproj files
		files, err := filepath.Glob(filepath.Join(target, "*.csproj"))
		if err != nil || len(files) == 0 {
			return nil, errors.New("no C# project files found")
		}
		// Return just the filenames, not full paths
		var manifests []string
		for _, f := range files {
			manifests = append(manifests, filepath.Base(f))
		}
		return manifests, nil
	default:
		return nil, errors.New("manifest files not defined for language: " + string(lang))
	}
}
