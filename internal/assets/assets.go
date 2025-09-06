package assets

import _ "embed"

// Curated JSON Schema meta-schemas (embedded)

//go:embed jsonschema/draft-07/schema.json
var JSONSchemaDraft07 []byte

//go:embed jsonschema/draft-2020-12/schema.json
var JSONSchemaDraft2020_12 []byte

func GetJSONSchemaMeta(draft string) ([]byte, bool) {
	switch draft {
	case "draft-07", "07", "7":
		return JSONSchemaDraft07, len(JSONSchemaDraft07) > 0
	case "2020-12", "2020", "202012":
		return JSONSchemaDraft2020_12, len(JSONSchemaDraft2020_12) > 0
	default:
		// default to 2020-12 if available
		if len(JSONSchemaDraft2020_12) > 0 {
			return JSONSchemaDraft2020_12, true
		}
		return nil, false
	}
}
