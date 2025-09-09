package assets

import (
	"embed"
	"io/fs"
)

// Curated JSON Schema meta-schemas (embedded)

//go:embed jsonschema/draft-07/schema.json
var JSONSchemaDraft07 []byte

//go:embed jsonschema/draft-2020-12/schema.json
var JSONSchemaDraft2020_12 []byte

//go:embed embedded_templates
var Templates embed.FS

//go:embed embedded_schemas
var Schemas embed.FS

//go:embed embedded_docs
var Docs embed.FS

func GetJSONSchemaMeta(draft string) ([]byte, bool) {
	switch draft {
	case "draft-07", "07", "7":
		return JSONSchemaDraft07, len(JSONSchemaDraft07) > 0
	case "2020-12", "2020", "202012":
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
