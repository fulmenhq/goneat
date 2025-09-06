package assets

// Registry lists embedded assets available at runtime.
// Update this when adding/removing curated assets.

type AssetInfo struct {
	Family   string // e.g., jsonschema
	Version  string // e.g., draft-07, 2020-12
	Path     string // embed path
	Checksum string // optional; populated by sync tooling
	Source   string // provenance URL
}

var Registry = []AssetInfo{
	{
		Family:  "jsonschema",
		Version: "draft-07",
		Path:    "jsonschema/draft-07/schema.json",
		Source:  "https://json-schema.org/draft-07/schema",
	},
	{
		Family:  "jsonschema",
		Version: "2020-12",
		Path:    "jsonschema/draft-2020-12/schema.json",
		Source:  "https://json-schema.org/draft/2020-12/schema",
	},
}
