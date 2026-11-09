package schemaengine

import (
	"errors"

	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func mustCompile(t *testing.T, schema string, opts Options) *Compiled {
	t.Helper()
	c, err := Compile([]byte(schema), opts)
	if err != nil {
		t.Fatalf("compile %s: %v", schema, err)
	}
	return c
}

func admissionReason(t *testing.T, schema string, opts Options) string {
	t.Helper()
	_, err := Compile([]byte(schema), opts)
	if err == nil {
		t.Fatalf("schema admitted, want admission error: %s", schema)
	}
	var ae *AdmissionError
	if !errors.As(err, &ae) {
		t.Fatalf("error is not an AdmissionError: %T %v", err, err)
	}
	return ae.Reason
}

func valid(t *testing.T, c *Compiled, instance string) bool {
	t.Helper()
	r, err := c.ValidateJSON([]byte(instance))
	if err != nil {
		t.Fatalf("decode %s: %v", instance, err)
	}
	if !r.Valid && len(r.Issues) == 0 {
		t.Fatalf("invalid result without issues for %s", instance)
	}
	return r.Valid
}

func TestDialectReporting(t *testing.T) {
	for _, tt := range []struct {
		schema, dialect, source string
	}{
		{`{"type":"string"}`, "draft-07", "defaulted"},
		{`true`, "draft-07", "defaulted"},
		{`{"$schema":"http://json-schema.org/draft-07/schema#"}`, "draft-07", "declared"},
		{`{"$schema":"https://json-schema.org/draft/2020-12/schema"}`, "draft/2020-12", "declared"},
		{`{"$schema":"http://json-schema.org/draft-04/schema#"}`, "draft-04", "declared"},
	} {
		c := mustCompile(t, tt.schema, Options{})
		got := c.Capability()
		if got.Dialect != tt.dialect || got.DialectSource != tt.source || got.Engine != Engine || got.FormatMode != "assert" || got.LeapPolicy != LeapPolicy {
			t.Errorf("%s: capability %+v", tt.schema, got)
		}
	}
	c := mustCompile(t, `{"type":"string"}`, Options{DefaultDraft: "draft/2020-12"})
	if got := c.Capability(); got.Dialect != "draft/2020-12" || got.DialectSource != "defaulted" {
		t.Errorf("explicit default: %+v", got)
	}
}

func TestDialectAdmission(t *testing.T) {
	for name, schema := range map[string]string{
		"unversioned latest alias": `{"$schema":"https://json-schema.org/schema"}`,
		"spoofed draft path":       `{"$schema":"https://json-schema.org/draft/2020-12/schema-evil"}`,
		"draft-03":                 `{"$schema":"http://json-schema.org/draft-03/schema#"}`,
		"non-string":               `{"$schema":7}`,
		"relative":                 `{"$schema":"draft-07"}`,
		"unadmitted metaschema":    `{"$schema":"https://example.com/meta"}`,
	} {
		if r := admissionReason(t, schema, Options{}); r != ReasonDialect && r != ReasonUnresolvedRef {
			t.Errorf("%s: reason %s", name, r)
		}
	}
	if _, err := Compile([]byte(`{}`), Options{DefaultDraft: "draft-05"}); err == nil {
		t.Error("unknown default draft accepted")
	}
}

// 2020-12 applies $ref siblings; draft-07 ignores them. The engine must
// execute the declared dialect, never silently substitute the older one.
func TestRefSiblingsFollowDeclaredDialect(t *testing.T) {
	modern := mustCompile(t, `{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{"s":{"type":"string"}},"$ref":"#/$defs/s","maxLength":2}`, Options{})
	if valid(t, modern, `"abc"`) {
		t.Error("2020-12 ignored maxLength sibling of $ref")
	}
	old := mustCompile(t, `{"$schema":"http://json-schema.org/draft-07/schema#","definitions":{"s":{"type":"string"}},"$ref":"#/definitions/s","maxLength":2}`, Options{})
	if !valid(t, old, `"abc"`) {
		t.Error("draft-07 applied a $ref sibling")
	}
}

func TestFormatAssertedInEveryDialect(t *testing.T) {
	for _, d := range []string{"http://json-schema.org/draft-04/schema#", "http://json-schema.org/draft-06/schema#", "http://json-schema.org/draft-07/schema#", "https://json-schema.org/draft/2019-09/schema", "https://json-schema.org/draft/2020-12/schema"} {
		c := mustCompile(t, fmt.Sprintf(`{"$schema":%q,"format":"date-time"}`, d), Options{})
		if valid(t, c, `"2020-01-01"`) || !valid(t, c, `"2020-01-01T00:00:00Z"`) || !valid(t, c, `5`) {
			t.Errorf("%s: format not asserted with owned checker", d)
		}
		if !valid(t, c, `"1998-12-31T23:59:60Z"`) || valid(t, c, `"2019-12-30T23:59:60Z"`) {
			t.Errorf("%s: structural leap policy not applied", d)
		}
	}
}

func TestUnknownFormatAdmission(t *testing.T) {
	for name, schema := range map[string]string{
		"root":           `{"format":"foo"}`,
		"draft-04":       `{"$schema":"http://json-schema.org/draft-04/schema#","format":"foo"}`,
		"2020-12 nested": `{"$schema":"https://json-schema.org/draft/2020-12/schema","properties":{"a":{"items":{"format":"foo"}}}}`,
		"ref target":     `{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{"d":{"format":"foo"}},"$ref":"#/$defs/d"}`,
		"idn-hostname":   `{"format":"idn-hostname"}`,
		"semver":         `{"format":"semver"}`,
	} {
		if r := admissionReason(t, schema, Options{}); r != ReasonUnknownFormat {
			t.Errorf("%s: reason %s", name, r)
		}
	}
	// format names inside instance data are not schema locations.
	for name, schema := range map[string]string{
		"const":         `{"const":{"format":"foo"}}`,
		"enum":          `{"enum":[{"format":"foo"}]}`,
		"default":       `{"default":{"format":"foo"}}`,
		"examples":      `{"examples":[{"format":"foo"}]}`,
		"property name": `{"properties":{"format":{"type":"string"}}}`,
		"unknown kw":    `{"x-note":{"format":"foo"}}`,
	} {
		if _, err := Compile([]byte(schema), Options{}); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestFormatAnnotateModeRefused(t *testing.T) {
	if r := admissionReason(t, `{"format":"date"}`, Options{FormatMode: FormatMode(1)}); r != ReasonUnsupportedOption {
		t.Errorf("annotate: %s", r)
	}
}

// A custom metaschema that requires the format-assertion vocabulary is
// honoured, and one requiring an unknown vocabulary is rejected.
func TestVocabularyAdmission(t *testing.T) {
	meta := `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://example.com/meta","$vocabulary":{"https://json-schema.org/draft/2020-12/vocab/core":true,"https://json-schema.org/draft/2020-12/vocab/applicator":true,"https://json-schema.org/draft/2020-12/vocab/format-assertion":true},"$dynamicAnchor":"meta"}`
	opts := Options{Resources: []Resource{{URL: "https://example.com/meta", Bytes: []byte(meta)}}}
	c := mustCompile(t, `{"$schema":"https://example.com/meta","format":"date"}`, opts)
	if valid(t, c, `"2020-02-30"`) || c.Capability().DialectSource != "metaschema" {
		t.Errorf("format-assertion metaschema: %+v", c.Capability())
	}
	unknown := strings.Replace(meta, `"https://json-schema.org/draft/2020-12/vocab/format-assertion":true`, `"https://example.com/vocab/unknown":true`, 1)
	opts.Resources[0].Bytes = []byte(unknown)
	if r := admissionReason(t, `{"$schema":"https://example.com/meta"}`, opts); r != ReasonDialect && r != ReasonInvalidSchema {
		t.Errorf("unknown required vocabulary: %s", r)
	}
}

func TestClosedResourceSet(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.json")
	if err := os.WriteFile(secret, []byte(`{"type":"string"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, schema := range map[string]string{
		"file url ref":        fmt.Sprintf(`{"$ref":"file://%s"}`, filepath.ToSlash(secret)),
		"absolute path ref":   fmt.Sprintf(`{"$ref":%q}`, secret),
		"relative ref":        `{"$ref":"secret.json"}`,
		"parent relative ref": `{"$id":"https://example.com/a/root.json","$ref":"../secret.json"}`,
		"https ref":           `{"$ref":"https://example.com/other.json"}`,
	} {
		if r := admissionReason(t, schema, Options{}); r != ReasonUnresolvedRef && r != ReasonResource && r != ReasonInvalidSchema {
			t.Errorf("%s: reason %s", name, r)
		}
	}
	// Admitted resources resolve, including relative refs from an $id base.
	opts := Options{Resources: []Resource{{URL: "https://example.com/a/item.json", Bytes: []byte(`{"$id":"https://example.com/a/item.json","type":"integer"}`)}}}
	c := mustCompile(t, `{"$id":"https://example.com/a/root.json","$ref":"item.json"}`, opts)
	if valid(t, c, `"x"`) || !valid(t, c, `1`) {
		t.Error("admitted relative reference not applied")
	}
	// Without $id the root gets a content-addressed base; relative and
	// path references fail admission instead of resolving to the root.
	for _, ref := range []string{"item.json", "/item.json", "../item.json", "./", ""} {
		if r := admissionReason(t, `{"$ref":"`+ref+`#/x"}`, opts); r != ReasonUnresolvedRef && r != ReasonInvalidSchema {
			t.Errorf("relative ref %q from content base: %s", ref, r)
		}
	}
	for _, ref := range []string{"item.json", "/item.json", "../item.json"} {
		if r := admissionReason(t, `{"$ref":"`+ref+`"}`, opts); r != ReasonUnresolvedRef {
			t.Errorf("relative ref %q from content base: %s", ref, r)
		}
	}
}

func TestOpaqueRootRelativeReference(t *testing.T) {
	for _, ref := range []string{"other.json", "/other.json", "../x#/a"} {
		if r := admissionReason(t, `{"$id":"urn:example:root","$ref":"`+ref+`"}`, Options{}); r != ReasonUnresolvedRef {
			t.Errorf("%q: %s", ref, r)
		}
	}
	c := mustCompile(t, `{"$id":"urn:example:root","$defs":{"i":{"type":"integer"}},"$ref":"#/$defs/i"}`, Options{})
	if valid(t, c, `"x"`) || !valid(t, c, `1`) {
		t.Error("fragment ref against URN root")
	}
	opts := Options{Resources: []Resource{{URL: "urn:example:item", Bytes: []byte(`{"type":"integer"}`)}}}
	c = mustCompile(t, `{"$id":"urn:example:root","$ref":"urn:example:item"}`, opts)
	if valid(t, c, `"x"`) {
		t.Error("absolute URN ref")
	}
}

// Dialect policy applies at every resource boundary, not only the root.
func TestDialectPolicyAtEveryResource(t *testing.T) {
	latest := `{"$schema":"https://json-schema.org/schema","type":"string"}`
	spoof := `{"$schema":"https://json-schema.org/draft/2020-12/schema-evil","type":"string"}`
	for name, tt := range map[string]struct {
		root string
		res  []Resource
	}{
		"referenced latest alias": {`{"$ref":"https://example.com/child"}`, []Resource{{URL: "https://example.com/child", Bytes: []byte(latest)}}},
		"referenced spoofed":      {`{"$ref":"https://example.com/child"}`, []Resource{{URL: "https://example.com/child", Bytes: []byte(spoof)}}},
		"unreferenced resource":   {`{}`, []Resource{{URL: "https://example.com/child", Bytes: []byte(latest)}}},
		"custom metaschema chain": {`{"$schema":"https://example.com/meta"}`, []Resource{{URL: "https://example.com/meta", Bytes: []byte(`{"$schema":"https://json-schema.org/schema"}`)}}},
		"embedded resource":       {`{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{"c":{"$id":"https://example.com/c","$schema":"https://json-schema.org/schema"}},"$ref":"https://example.com/c"}`, nil},
	} {
		if r := admissionReason(t, tt.root, Options{Resources: tt.res}); r != ReasonDialect {
			t.Errorf("%s: reason %s", name, r)
		}
	}
	// Pinned declarations on referenced resources are fine, as is a
	// const/default value that merely looks like a $schema declaration.
	ok := []Resource{{URL: "https://example.com/child", Bytes: []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"string"}`)}}
	c := mustCompile(t, `{"$ref":"https://example.com/child"}`, Options{Resources: ok})
	if valid(t, c, `5`) {
		t.Error("referenced draft-07 resource not applied")
	}
	mustCompile(t, `{"const":{"$schema":"https://json-schema.org/schema"},"default":{"$schema":7}}`, Options{})
}

// Relative references resolve against each resource's own base.
func TestOpaqueBaseIsPerResource(t *testing.T) {
	leaf := Resource{URL: "https://example.com/leaf", Bytes: []byte(`{"type":"integer"}`)}
	// URN root -> hierarchical child with a relative ref: valid.
	child := Resource{URL: "https://example.com/child", Bytes: []byte(`{"$ref":"leaf"}`)}
	c := mustCompile(t, `{"$id":"urn:example:root","$ref":"https://example.com/child"}`, Options{Resources: []Resource{child, leaf}})
	if valid(t, c, `"x"`) || !valid(t, c, `1`) {
		t.Error("hierarchical child under URN root not resolved")
	}
	// HTTPS root -> URN child with a relative ref: refused at admission.
	urnChild := Resource{URL: "urn:example:child", Bytes: []byte(`{"$ref":"missing.json"}`)}
	if r := admissionReason(t, `{"$ref":"urn:example:child"}`, Options{Resources: []Resource{urnChild}}); r != ReasonUnresolvedRef {
		t.Errorf("relative ref in URN child: %s", r)
	}
	// Nested base change to a URN inside a hierarchical document.
	if r := admissionReason(t, `{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{"n":{"$id":"urn:example:nested","$ref":"x.json"}},"$ref":"urn:example:nested"}`, Options{}); r != ReasonUnresolvedRef {
		t.Errorf("relative ref under nested URN base: %s", r)
	}
	// Nested hierarchical base under a URN root resolves.
	nested := `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"urn:example:root","$defs":{"h":{"$id":"https://example.com/h/","$ref":"../leaf"}},"$ref":"https://example.com/h/"}`
	c = mustCompile(t, nested, Options{Resources: []Resource{leaf}})
	if valid(t, c, `"x"`) {
		t.Error("nested hierarchical base not resolved")
	}
	// A hierarchical base whose file name matches any fixed probe name is
	// still hierarchical.
	probeName := Resource{URL: "https://example.com/goneat-opaque-base-probe.invalid.json", Bytes: []byte(`{"$ref":"leaf"}`)}
	c = mustCompile(t, `{"$ref":"https://example.com/goneat-opaque-base-probe.invalid.json"}`, Options{Resources: []Resource{probeName, leaf}})
	if valid(t, c, `"x"`) {
		t.Error("hierarchical base with probe-like name misclassified")
	}
	// Determining the base adds no graph edges: an admitted but
	// unreferenced resource is never compiled.
	unused := Resource{URL: "https://example.com/unused.json", Bytes: []byte(`{"format":"unused-format"}`)}
	mustCompile(t, `{"$id":"https://example.com/root","$ref":"leaf"}`, Options{Resources: []Resource{leaf, unused}})
	// Fragment-only refs under opaque bases still work.
	mustCompile(t, `{"$id":"urn:example:r","$defs":{"a":{"type":"string"}},"$ref":"#/$defs/a"}`, Options{})
}

// resourceBase reads the engine's private resource base; a dependency change
// that breaks it must fail here, not silently weaken admission.
func TestResourceBaseIntrospection(t *testing.T) {
	var seen []string
	c := jsonschema.NewCompiler()
	c.UseLoader(newClosedLoader())
	c.RegisterVocabulary(&jsonschema.Vocabulary{URL: "urn:goneat:test:base", Compile: func(ctx *jsonschema.CompilerContext, _ map[string]any) (jsonschema.SchemaExt, error) {
		b, ok := resourceBase(ctx)
		if !ok {
			t.Fatal("resource base introspection failed")
		}
		seen = append(seen, b)
		return nil, nil
	}})
	c.AssertVocabs()
	if err := c.AddResource("https://example.com/a/root.json", map[string]any{"$id": "https://example.com/a/root.json", "properties": map[string]any{"x": map[string]any{"$id": "urn:example:x"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Compile("https://example.com/a/root.json"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "https://example.com/a/root.json,urn:example:x" {
		t.Fatalf("bases: %v", seen)
	}
}

// The pattern budget applies before regexp compilation and only to schema
// patterns; validating a long instance against format regex is unaffected.
func TestPatternBudgetBeforeCompile(t *testing.T) {
	long := strings.Repeat("a", 100)
	for name, schema := range map[string]string{
		"pattern":           `{"pattern":"` + long + `"}`,
		"patternProperties": `{"patternProperties":{"` + long + `":{}}}`,
		"invalid and long":  `{"pattern":"(` + long + `"}`,
	} {
		if r := admissionReason(t, schema, Options{Limits: Limits{MaxPatternBytes: 50}}); r != ReasonLimit {
			t.Errorf("%s: reason %s", name, r)
		}
	}
	// Over-budget patterns in admitted and referenced resources.
	for name, res := range map[string]string{
		"resource pattern":           `{"pattern":"(` + long + `"}`,
		"resource patternProperties": `{"patternProperties":{"(` + long + `":{}}}`,
		"referenced subschema":       `{"$defs":{"p":{"pattern":"(` + long + `"}}}`,
	} {
		root := `{"$ref":"https://example.com/r"}`
		if name == "referenced subschema" {
			root = `{"$ref":"https://example.com/r#/$defs/p"}`
			res = `{"$schema":"https://json-schema.org/draft/2020-12/schema",` + res[1:]
		}
		opts := Options{Limits: Limits{MaxPatternBytes: 50}, Resources: []Resource{{URL: "https://example.com/r", Bytes: []byte(res)}}}
		if r := admissionReason(t, root, opts); r != ReasonLimit {
			t.Errorf("%s: reason %s", name, r)
		}
	}
	c := mustCompile(t, `{"format":"regex"}`, Options{Limits: Limits{MaxPatternBytes: 50}})
	if !valid(t, c, `"`+long+`"`) || valid(t, c, `"(`+long+`"`) {
		t.Error("format regex validation affected by the schema pattern budget")
	}
}

func TestResourceIdentityConflicts(t *testing.T) {
	doc := []byte(`{"$id":"https://example.com/b.json","type":"integer"}`)
	for name, opts := range map[string]Options{
		"url differs from $id":    {Resources: []Resource{{URL: "https://example.com/a.json", Bytes: doc}}},
		"admitted twice":          {Resources: []Resource{{URL: "https://example.com/b.json", Bytes: doc}, {URL: "https://example.com/b.json", Bytes: doc}}},
		"reserved metaschema url": {Resources: []Resource{{URL: "https://json-schema.org/draft/2020-12/schema", Bytes: []byte(`{}`)}}},
		"relative url":            {Resources: []Resource{{URL: "b.json", Bytes: doc}}},
		"file url":                {Resources: []Resource{{URL: "file:///tmp/b.json", Bytes: doc}}},
		"base conflicts with $id": {BaseURL: "https://example.com/other.json"},
	} {
		if r := admissionReason(t, `{"$id":"https://example.com/root.json"}`, opts); r != ReasonResource {
			t.Errorf("%s: reason %s", name, r)
		}
	}
	if r := admissionReason(t, `{"$id":"file:///folder/file.json"}`, Options{}); r != ReasonResource {
		t.Errorf("file $id: %s", r)
	}
}

func TestLimits(t *testing.T) {
	deep := strings.Repeat(`{"items":`, 40) + `{}` + strings.Repeat(`}`, 40)
	for name, tt := range map[string]struct {
		schema string
		lim    Limits
	}{
		"schema bytes":   {`{"type":"string"}`, Limits{MaxSchemaBytes: 8}},
		"depth":          {deep, Limits{MaxDepth: 10}},
		"schema nodes":   {deep, Limits{MaxSchemaNodes: 10}},
		"pattern":        {`{"pattern":"` + strings.Repeat("a", 100) + `"}`, Limits{MaxPatternBytes: 50}},
		"patternProps":   {`{"patternProperties":{"` + strings.Repeat("a", 100) + `":{}}}`, Limits{MaxPatternBytes: 50}},
		"resource count": {`{}`, Limits{MaxResources: 1}},
	} {
		opts := Options{Limits: tt.lim}
		if name == "resource count" {
			opts.Resources = []Resource{{URL: "https://example.com/1", Bytes: []byte(`{}`)}, {URL: "https://example.com/2", Bytes: []byte(`{}`)}}
		}
		if r := admissionReason(t, tt.schema, opts); r != ReasonLimit {
			t.Errorf("%s: reason %s", name, r)
		}
	}
	c := mustCompile(t, `{}`, Options{Limits: Limits{MaxInstanceBytes: 4}})
	if _, err := c.ValidateJSON([]byte(`"abcdef"`)); err == nil {
		t.Error("instance byte limit not enforced")
	}
}

// Exact-number scanning: huge exponents must not stop the depth scan.
func TestDepthBudgetWithLargeNumbers(t *testing.T) {
	for _, n := range []string{"1e1000", "-1e1000", "1e-1000", "123456789012345678901234567890"} {
		deep := `{"const":` + n + `,"properties":{"x":{"items":{}}}}`
		if r := admissionReason(t, deep, Options{Limits: Limits{MaxDepth: 2}}); r != ReasonLimit {
			t.Errorf("root %s: reason %s", n, r)
		}
		res := []Resource{{URL: "https://example.com/r", Bytes: []byte(deep)}}
		if r := admissionReason(t, `{"$ref":"https://example.com/r"}`, Options{Resources: res, Limits: Limits{MaxDepth: 2}}); r != ReasonLimit {
			t.Errorf("resource %s: reason %s", n, r)
		}
	}
}

func TestInvalidSchemasAreAdmissionErrors(t *testing.T) {
	for name, schema := range map[string]string{
		"bad type":         `{"type":5}`,
		"RE2 lookahead":    `{"pattern":"(?=a)"}`,
		"bad json":         `{"type":`,
		"trailing data":    `{} {}`,
		"broken pointer":   `{"$ref":"#/definitions/missing"}`,
		"invalid minItems": `{"minItems":-1}`,
	} {
		_, err := Compile([]byte(schema), Options{})
		var ae *AdmissionError
		if !errors.As(err, &ae) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestExactNumbers(t *testing.T) {
	c := mustCompile(t, `{"const":9007199254740993}`, Options{})
	if valid(t, c, `9007199254740992`) || !valid(t, c, `9007199254740993`) {
		t.Error("numbers lost precision")
	}
	m := mustCompile(t, `{"multipleOf":0.01}`, Options{})
	if !valid(t, m, `19.99`) {
		t.Error("decimal multipleOf")
	}
	if _, err := DecodeJSON([]byte(`1 2`), 0); err == nil {
		t.Error("trailing instance data accepted")
	}
}

func TestDecodeRejectsReplacementSources(t *testing.T) {
	for name, raw := range map[string][]byte{
		"invalid utf-8 byte":   []byte("\"a\xffb\""),
		"truncated sequence":   []byte("\"a\xe2\x82\""),
		"overlong encoding":    []byte("\"\xc0\xaf\""),
		"encoded surrogate":    []byte("\"\xed\xa0\x80\""),
		"lone high escape":     []byte(`"a\ud800b"`),
		"lone high at end":     []byte(`"\ud800"`),
		"lone low escape":      []byte(`"\udc00"`),
		"high then escape":     []byte(`"\ud800\n"`),
		"high then high":       []byte(`"\ud800\ud800"`),
		"reversed pair":        []byte(`"\udc00\ud800"`),
		"surrogate in key":     []byte(`{"\ud800":1}`),
		"invalid in schema id": []byte("{\"$id\":\"https://example.com/\xff\"}"),
	} {
		if _, err := DecodeJSON(raw, 0); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for name, raw := range map[string][]byte{
		"paired escape":     []byte(`"\ud83d\ude00"`),
		"literal U+FFFD":    []byte("\"\xef\xbf\xbd\""),
		"escaped U+FFFD":    []byte(`"\ufffd"`),
		"escaped backslash": []byte(`"\\ud800"`),
		"astral literal":    []byte("\"\xf0\x9f\x98\x80\""),
	} {
		v, err := DecodeJSON(raw, 0)
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
		_ = v
	}
	// Schema bytes go through the same check.
	if r := admissionReason(t, "{\"description\":\"\xff\"}", Options{}); r != ReasonInvalidJSON {
		t.Errorf("schema with invalid UTF-8: %s", r)
	}
}

func TestInvalidResultCarriesLocations(t *testing.T) {
	c := mustCompile(t, `{"properties":{"when":{"format":"date-time"}}}`, Options{})
	r, err := c.ValidateJSON([]byte(`{"when":"2020-01-01"}`))
	if err != nil || r.Valid || len(r.Issues) == 0 {
		t.Fatalf("result: %+v %v", r, err)
	}
	found := false
	for _, is := range r.Issues {
		if is.InstanceLocation == "/when" && strings.Contains(is.KeywordLocation, "format") {
			found = true
		}
	}
	if !found {
		t.Errorf("issues lack locations: %+v", r.Issues)
	}
}

func TestCacheIdentity(t *testing.T) {
	root := []byte(`{"$id":"https://example.com/root.json","$ref":"item.json"}`)
	item := func(typ string) []Resource {
		return []Resource{{URL: "https://example.com/item.json", Bytes: []byte(`{"type":"` + typ + `"}`)}}
	}
	var cache Cache
	a, err := cache.Get(root, Options{Resources: item("integer")})
	if err != nil {
		t.Fatal(err)
	}
	b, err := cache.Get(root, Options{Resources: item("string")})
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a.Key() == b.Key() {
		t.Fatal("cache ignored resource content")
	}
	again, _ := cache.Get(root, Options{Resources: item("integer")})
	if again != a {
		t.Error("cache miss for identical inputs")
	}
	moved := []Resource{{URL: "https://example.com/other.json", Bytes: item("integer")[0].Bytes}}
	if _, err := cache.Get([]byte(`{"$id":"https://example.com/root.json","$ref":"other.json"}`), Options{Resources: moved}); err != nil {
		t.Fatal(err)
	}
	if k1, k2 := cacheKey("", root, Options{Resources: item("integer")}, DefaultLimits()), cacheKey("", root, Options{Resources: moved}, DefaultLimits()); k1 == k2 {
		t.Error("cache key ignores resource URL")
	}
	if k1, k2 := cacheKey("", root, Options{}, DefaultLimits()), cacheKey("", root, Options{DefaultDraft: "draft/2020-12"}, DefaultLimits()); k1 == k2 {
		t.Error("cache key ignores default draft")
	}
	// Compile errors are cached and returned consistently.
	_, e1 := cache.Get([]byte(`{"format":"foo"}`), Options{})
	_, e2 := cache.Get([]byte(`{"format":"foo"}`), Options{})
	if e1 == nil || e1 != e2 {
		t.Errorf("cached error: %v %v", e1, e2)
	}
}

func TestConcurrentUse(t *testing.T) {
	schema := `{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{"s":{"type":"string"}},"type":"object","properties":{"at":{"format":"date-time"},"name":{"$ref":"#/$defs/s","maxLength":3}},"required":["at"]}`
	var cache Cache
	instances := []struct {
		doc string
		ok  bool
	}{
		{`{"at":"2020-01-01T00:00:00Z","name":"abc"}`, true},
		{`{"at":"2020-01-01","name":"abc"}`, false},
		{`{"at":"1998-12-31T23:59:60Z"}`, true},
		{`{"at":"2020-01-01T00:00:00Z","name":"abcd"}`, false},
		{`{"name":"a"}`, false},
	}
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := cache.Get([]byte(schema), Options{})
			if err != nil {
				errs <- err.Error()
				return
			}
			for i := 0; i < 200; i++ {
				in := instances[i%len(instances)]
				r, err := c.ValidateJSON([]byte(in.doc))
				if err != nil || r.Valid != in.ok {
					errs <- fmt.Sprintf("%s: %+v %v", in.doc, r, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
