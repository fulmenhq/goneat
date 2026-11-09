package schemaengine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Admission failure reasons.
const (
	ReasonInvalidJSON       = "invalid-json"
	ReasonLimit             = "limit-exceeded"
	ReasonUnsupportedOption = "unsupported-option"
	ReasonDialect           = "unsupported-dialect"
	ReasonUnknownFormat     = "unknown-format"
	ReasonResource          = "resource"
	ReasonUnresolvedRef     = "unresolved-reference"
	ReasonInvalidSchema     = "invalid-schema"
)

// AdmissionError means the schema was not evaluated. It is distinct from an
// instance failing validation.
type AdmissionError struct {
	Reason   string
	Location string
	Detail   string
	Err      error
}

func (e *AdmissionError) Error() string {
	var b strings.Builder
	b.WriteString("schema not admitted (")
	b.WriteString(e.Reason)
	b.WriteString(")")
	if e.Location != "" {
		b.WriteString(" at ")
		b.WriteString(e.Location)
	}
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return SafeText(b.String())
}

func (e *AdmissionError) Unwrap() error { return e.Err }

// Limits bound the work a schema can cause before and during compilation.
type Limits struct {
	MaxSchemaBytes   int // per admitted document
	MaxInstanceBytes int // for ValidateJSON
	MaxDepth         int // JSON nesting depth per document
	MaxResources     int // admitted resources besides the root
	MaxSchemaNodes   int // compiled schema objects, across all references
	MaxPatternBytes  int // pattern and patternProperties sources
}

// DefaultLimits are generous for configuration and SBOM schemas while
// bounding hostile input.
func DefaultLimits() Limits {
	return Limits{
		MaxSchemaBytes:   8 << 20,
		MaxInstanceBytes: 64 << 20,
		MaxDepth:         256,
		MaxResources:     512,
		MaxSchemaNodes:   200000,
		MaxPatternBytes:  4096,
	}
}

func (l Limits) withDefaults() Limits {
	d := DefaultLimits()
	if l.MaxSchemaBytes > 0 {
		d.MaxSchemaBytes = l.MaxSchemaBytes
	}
	if l.MaxInstanceBytes > 0 {
		d.MaxInstanceBytes = l.MaxInstanceBytes
	}
	if l.MaxDepth > 0 {
		d.MaxDepth = l.MaxDepth
	}
	if l.MaxResources > 0 {
		d.MaxResources = l.MaxResources
	}
	if l.MaxSchemaNodes > 0 {
		d.MaxSchemaNodes = l.MaxSchemaNodes
	}
	if l.MaxPatternBytes > 0 {
		d.MaxPatternBytes = l.MaxPatternBytes
	}
	return d
}

// admitDocument applies byte and depth budgets, then decodes with exact
// numbers.
func admitDocument(name string, data []byte, lim Limits) (any, error) {
	if len(data) > lim.MaxSchemaBytes {
		return nil, &AdmissionError{Reason: ReasonLimit, Location: name,
			Detail: fmt.Sprintf("%d bytes exceed the limit of %d", len(data), lim.MaxSchemaBytes)}
	}
	if err := checkDepth(data, lim.MaxDepth); err != nil {
		return nil, &AdmissionError{Reason: ReasonLimit, Location: name, Err: err}
	}
	doc, err := DecodeJSON(data, 0)
	if err != nil {
		return nil, &AdmissionError{Reason: ReasonInvalidJSON, Location: name, Err: err}
	}
	return doc, nil
}

func checkDepth(data []byte, max int) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	depth := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return nil // syntax errors are reported by the decoder
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
				if depth > max {
					return fmt.Errorf("nesting depth exceeds the limit of %d", max)
				}
			default:
				depth--
			}
		}
	}
}

func requireAbsoluteURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || !u.IsAbs() {
		return &AdmissionError{Reason: ReasonResource, Location: s, Detail: "resource URL must be absolute"}
	}
	if strings.EqualFold(u.Scheme, "file") {
		return &AdmissionError{Reason: ReasonResource, Location: s, Detail: "file URLs are not admitted; pass document bytes"}
	}
	if isMetaschemaHost(u) {
		return &AdmissionError{Reason: ReasonResource, Location: s, Detail: "json-schema.org URLs are reserved for the embedded metaschemas"}
	}
	return nil
}

func isMetaschemaHost(u *url.URL) bool {
	return strings.EqualFold(u.Host, "json-schema.org")
}

func stripEmptyFragment(s string) string {
	return strings.TrimSuffix(s, "#")
}

// declaredID returns the top-level $id (or draft-04 id) of a document.
func declaredID(doc any) (string, bool) {
	obj, ok := doc.(map[string]any)
	if !ok {
		return "", false
	}
	for _, k := range []string{"$id", "id"} {
		if s, ok := obj[k].(string); ok && s != "" {
			if k == "id" && !isDraft4(obj) {
				continue
			}
			return s, true
		}
	}
	return "", false
}

func isDraft4(obj map[string]any) bool {
	s, _ := obj["$schema"].(string)
	return knownDrafts[normalizeDialectURL(s)] == "draft-04"
}

// rootBase picks the URL the root is compiled under. An absolute top-level
// $id must agree with an explicit BaseURL; with neither, a content-addressed
// base URL is used.
func rootBase(doc any, raw []byte, explicit string) (string, error) {
	id, hasID := declaredID(doc)
	if hasID {
		u, err := url.Parse(id)
		if err == nil && u.IsAbs() {
			id = stripEmptyFragment(id)
			if explicit != "" && stripEmptyFragment(explicit) != id {
				return "", &AdmissionError{Reason: ReasonResource, Location: explicit,
					Detail: fmt.Sprintf("root $id \"%s\" conflicts with the base URL", id)}
			}
			if err := requireAbsoluteURL(id); err != nil {
				return "", err
			}
			return id, nil
		}
	}
	if explicit != "" {
		if err := requireAbsoluteURL(explicit); err != nil {
			return "", err
		}
		return stripEmptyFragment(explicit), nil
	}
	return contentBaseURL(raw), nil
}

// knownDrafts maps normalised dialect URLs to draft names. The unversioned
// json-schema.org/schema alias is deliberately absent: it means "latest" and
// would change meaning with the engine version.
var knownDrafts = map[string]string{
	"json-schema.org/draft-04/schema":      "draft-04",
	"json-schema.org/draft-06/schema":      "draft-06",
	"json-schema.org/draft-07/schema":      "draft-07",
	"json-schema.org/draft/2019-09/schema": "draft/2019-09",
	"json-schema.org/draft/2020-12/schema": "draft/2020-12",
}

func normalizeDialectURL(s string) string {
	s = stripEmptyFragment(s)
	if r, ok := strings.CutPrefix(s, "http://"); ok {
		return r
	}
	r, _ := strings.CutPrefix(s, "https://")
	return r
}

// rootDialect reports the root's effective dialect and where it came from.
func rootDialect(doc any, defaultName string) (dialect, source string, err error) {
	obj, ok := doc.(map[string]any)
	if !ok {
		return defaultName, "defaulted", nil
	}
	raw, present := obj["$schema"]
	if !present {
		return defaultName, "defaulted", nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", "", &AdmissionError{Reason: ReasonDialect, Location: "/$schema", Detail: "$schema must be a string"}
	}
	if name, ok := knownDrafts[normalizeDialectURL(s)]; ok {
		return name, "declared", nil
	}
	if normalizeDialectURL(s) == "json-schema.org/schema" {
		return "", "", &AdmissionError{Reason: ReasonDialect, Location: "/$schema",
			Detail: "the unversioned json-schema.org/schema dialect is not pinned; declare a specific draft"}
	}
	u, perr := url.Parse(s)
	if perr != nil || !u.IsAbs() {
		return "", "", &AdmissionError{Reason: ReasonDialect, Location: "/$schema",
			Detail: fmt.Sprintf("\"%s\" is not an absolute dialect URI", s)}
	}
	if isMetaschemaHost(u) {
		return "", "", &AdmissionError{Reason: ReasonDialect, Location: "/$schema",
			Detail: fmt.Sprintf("\"%s\" is not a recognised JSON Schema dialect", s)}
	}
	// A custom metaschema: it must be an admitted resource, and the engine
	// enforces its declared vocabularies.
	return stripEmptyFragment(s), "metaschema", nil
}

// closedLoader serves only admitted documents. The engine serves its own
// embedded draft metaschemas before consulting the loader.
type closedLoader struct {
	docs map[string]any
}

func newClosedLoader() *closedLoader { return &closedLoader{docs: map[string]any{}} }

func (l *closedLoader) admit(u string, doc any) error {
	u = stripEmptyFragment(u)
	if _, dup := l.docs[u]; dup {
		return &AdmissionError{Reason: ReasonResource, Location: u, Detail: "resource admitted twice"}
	}
	if id, ok := declaredID(doc); ok {
		if pu, err := url.Parse(id); err == nil && pu.IsAbs() && stripEmptyFragment(id) != u {
			return &AdmissionError{Reason: ReasonResource, Location: u,
				Detail: fmt.Sprintf("resource declares $id \"%s\"", id)}
		}
	}
	l.docs[u] = doc
	return nil
}

var errNotAdmitted = errors.New("reference is not in the admitted resource set")

func (l *closedLoader) Load(u string) (any, error) {
	if doc, ok := l.docs[stripEmptyFragment(u)]; ok {
		return doc, nil
	}
	return nil, errNotAdmitted
}

// admission is a compile-time vocabulary hook. The engine calls it for every
// schema object it compiles, including reference targets, so the checks see
// exactly the executable schema locations of each dialect.
type admission struct {
	limits Limits
	// opaqueRoot is set when the root base is an opaque URI such as a URN.
	// URL resolution maps every relative reference against such a base back
	// onto the base itself, so relative references are refused.
	opaqueRoot bool
	nodes      int
	err        *AdmissionError
}

const admissionVocabURL = "urn:goneat:vocab:admission"

func (a *admission) vocabulary() *jsonschema.Vocabulary {
	return &jsonschema.Vocabulary{
		URL:     admissionVocabURL,
		Compile: a.compile,
	}
}

func (a *admission) fail(e *AdmissionError) (jsonschema.SchemaExt, error) {
	if a.err == nil {
		a.err = e
	}
	return nil, e
}

func (a *admission) compile(_ *jsonschema.CompilerContext, obj map[string]any) (jsonschema.SchemaExt, error) {
	a.nodes++
	if a.nodes > a.limits.MaxSchemaNodes {
		return a.fail(&AdmissionError{Reason: ReasonLimit,
			Detail: fmt.Sprintf("more than %d schema objects", a.limits.MaxSchemaNodes)})
	}
	if a.opaqueRoot {
		for _, kw := range []string{"$ref", "$dynamicRef", "$recursiveRef"} {
			ref, ok := obj[kw].(string)
			if !ok || strings.HasPrefix(ref, "#") {
				continue
			}
			if u, err := url.Parse(ref); err != nil || !u.IsAbs() {
				return a.fail(&AdmissionError{Reason: ReasonUnresolvedRef,
					Detail: fmt.Sprintf("relative %s \"%s\" cannot be resolved against an opaque base URI", kw, ref)})
			}
		}
	}
	if f, ok := obj["format"].(string); ok && !isKnownFormat(f) {
		return a.fail(&AdmissionError{Reason: ReasonUnknownFormat,
			Detail: fmt.Sprintf("format \"%s\" is not supported; refusing to skip an asserted format", f)})
	}
	if p, ok := obj["pattern"].(string); ok && len(p) > a.limits.MaxPatternBytes {
		return a.fail(&AdmissionError{Reason: ReasonLimit,
			Detail: fmt.Sprintf("pattern of %d bytes exceeds the limit of %d", len(p), a.limits.MaxPatternBytes)})
	}
	if pp, ok := obj["patternProperties"].(map[string]any); ok {
		for k := range pp {
			if len(k) > a.limits.MaxPatternBytes {
				return a.fail(&AdmissionError{Reason: ReasonLimit,
					Detail: fmt.Sprintf("patternProperties key of %d bytes exceeds the limit of %d", len(k), a.limits.MaxPatternBytes)})
			}
		}
	}
	return nil, nil
}

// toAdmissionError classifies engine compile errors.
func toAdmissionError(err error) error {
	var ae *AdmissionError
	if errors.As(err, &ae) {
		return ae
	}
	reason := ReasonInvalidSchema
	var le *jsonschema.LoadURLError
	switch {
	case errors.As(err, &le):
		reason = ReasonUnresolvedRef
	case errors.Is(err, errNotAdmitted):
		reason = ReasonUnresolvedRef
	}
	var ud *jsonschema.UnsupportedDraftError
	var uv *jsonschema.UnsupportedVocabularyError
	if errors.As(err, &ud) || errors.As(err, &uv) {
		reason = ReasonDialect
	}
	return &AdmissionError{Reason: reason, Err: err}
}
