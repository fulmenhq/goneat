package assets

import (
	"embed"
	"io/fs"
	"os"
)

// Curated JSON Schema meta-schemas (embedded)

//go:embed jsonschema/draft-04/schema.json
var JSONSchemaDraft04 []byte

//go:embed jsonschema/draft-06/schema.json
var JSONSchemaDraft06 []byte

//go:embed jsonschema/draft-07/schema.json
var JSONSchemaDraft07 []byte

//go:embed jsonschema/draft-2019-09/schema.json
var JSONSchemaDraft2019_09 []byte

//go:embed jsonschema/draft-2020-12/schema.json
var JSONSchemaDraft2020_12 []byte

//go:embed embedded_templates
var Templates embed.FS

//go:embed embedded_schemas
var Schemas embed.FS

//go:embed embedded_docs
var Docs embed.FS

//go:embed embedded_config
var Config embed.FS

func GetJSONSchemaMeta(draft string) ([]byte, bool) {
	offline := os.Getenv("GONEAT_OFFLINE_SCHEMA_VALIDATION") == "true"
	switch draft {
	case "draft-04", "04", "4":
		return JSONSchemaDraft04, len(JSONSchemaDraft04) > 0
	case "draft-06", "06", "6":
		return JSONSchemaDraft06, len(JSONSchemaDraft06) > 0
	case "draft-07", "07", "7":
		return JSONSchemaDraft07, len(JSONSchemaDraft07) > 0
	case "2019-09", "2019", "201909":
		if offline {
			if data, ok := GetSchema("embedded_schemas/schemas/meta/draft-2019-09/offline.schema.json"); ok {
				return data, true
			}
		}
		return JSONSchemaDraft2019_09, len(JSONSchemaDraft2019_09) > 0
	case "2020-12", "2020", "202012":
		if offline {
			if data, ok := GetSchema("embedded_schemas/schemas/meta/draft-2020-12/offline.schema.json"); ok {
				return data, true
			}
		}
		return JSONSchemaDraft2020_12, len(JSONSchemaDraft2020_12) > 0
	default:
		// Unknown draft requested; do not fallback implicitly
		return nil, false
	}
}

func GetTemplatesFS() fs.FS {
	if sub, err := fs.Sub(Templates, "embedded_templates"); err == nil {
		return sub
	}
	return Templates
}

func GetSchemasFS() fs.FS {
	if sub, err := fs.Sub(Schemas, "embedded_schemas"); err == nil {
		return sub
	}
	return Schemas
}

func GetDocsFS() fs.FS {
	if sub, err := fs.Sub(Docs, "embedded_docs"); err == nil {
		return sub
	}
	return Docs
}

func GetConfigFS() fs.FS {
	if sub, err := fs.Sub(Config, "embedded_config"); err == nil {
		return sub
	}
	return Config
}

// GetEmbeddedAsset retrieves an embedded asset by path
func GetEmbeddedAsset(path string) ([]byte, error) {
	// Try templates first (embedded_templates is the root)
	fullPath := path
	if data, err := fs.ReadFile(Templates, fullPath); err == nil {
		return data, nil
	}

	// Try schemas (embedded_schemas is the root)
	if data, err := fs.ReadFile(Schemas, fullPath); err == nil {
		return data, nil
	}

	// Try docs (embedded_docs is the root)
	if data, err := fs.ReadFile(Docs, fullPath); err == nil {
		return data, nil
	}

	// Try config (embedded_config is the root)
	if data, err := fs.ReadFile(Config, fullPath); err == nil {
		return data, nil
	}

	return nil, fs.ErrNotExist
}

// GetAsset returns a specific embedded asset by name
func GetAsset(name string) ([]byte, bool) {
	switch name {
	case "terminal-overrides.yaml":
		// Read from embedded config
		if data, err := fs.ReadFile(Config, "embedded_config/config/ascii/terminal-overrides.yaml"); err == nil {
			return data, true
		}
		return nil, false
	default:
		return nil, false
	}
}
