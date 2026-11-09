package schemaengine

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// knownFormats are the format names goneat asserts. Any other name at an
// executable schema location is an admission error.
//
// date, time and date-time use the owned checkers in datetime.go; email and
// idn-email use email.go. The rest use the engine's built-in checkers;
// "regex" is checked with Go RE2. idn-hostname is not listed until an owned
// checker exists, and the engine's non-standard "period" and "semver" are not
// admitted.
var knownFormats = map[string]bool{
	"date":                  true,
	"time":                  true,
	"date-time":             true,
	"duration":              true,
	"email":                 true,
	"idn-email":             true,
	"hostname":              true,
	"ipv4":                  true,
	"ipv6":                  true,
	"uri":                   true,
	"uri-reference":         true,
	"iri":                   true,
	"iri-reference":         true,
	"uuid":                  true,
	"uri-template":          true,
	"json-pointer":          true,
	"relative-json-pointer": true,
	"regex":                 true,
}

func isKnownFormat(name string) bool { return knownFormats[name] }

// ownedFormats override the engine's built-in checkers.
func ownedFormats() []*jsonschema.Format {
	return []*jsonschema.Format{
		{Name: "date", Validate: checkDate},
		{Name: "time", Validate: checkTime},
		{Name: "date-time", Validate: checkDateTime},
		{Name: "email", Validate: checkEmail},
		{Name: "idn-email", Validate: checkIDNEmail},
	}
}

// contentBaseURL names a root that has no $id. It is hierarchical so that
// relative references resolve to distinct URLs, which the closed loader then
// refuses. (An opaque URN base would resolve every relative reference back to
// the root itself.) The .invalid TLD is reserved and never resolves.
func contentBaseURL(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "https://root.goneat.invalid/sha256/" + hex.EncodeToString(sum[:]) + "/schema.json"
}
