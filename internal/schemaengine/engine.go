// Package schemaengine is goneat's single JSON Schema compilation and
// validation adapter.
//
// Every compile uses a closed resource set (no filesystem or network
// loading), an explicit draft-07 default for schemas that declare no
// $schema, owned date/time/date-time checkers, format assertion for every
// dialect, and admission checks that fail closed on unknown format names,
// unpinned dialects and resource budgets. Compiled schemas are immutable and
// safe to share between goroutines.
package schemaengine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Engine identifies the underlying validator in capability reports.
const Engine = "github.com/santhosh-tekuri/jsonschema/v6@v6.0.3"

// FormatMode selects how the format keyword is treated. Only FormatAssert
// exists: the engine cannot disable every format assertion for pre-2019
// drafts (the regex format cannot be overridden), so an annotation-only mode
// is not offered. Any other value is refused at compile time.
type FormatMode int

// FormatAssert validates format for every dialect.
const FormatAssert FormatMode = 0

func (m FormatMode) String() string {
	if m == FormatAssert {
		return "assert"
	}
	return fmt.Sprintf("FormatMode(%d)", int(m))
}

// Resource is a schema document admitted for $ref resolution, identified by
// its absolute URL.
type Resource struct {
	URL   string
	Bytes []byte
}

// Options configures one compilation. Every field is part of the cache key.
type Options struct {
	// BaseURL is the absolute URL of the root document. When empty and the
	// root has no $id, a content-addressed https://root.goneat.invalid/ URL
	// is used; relative references from it can only resolve to admitted
	// resources.
	BaseURL string
	// Resources are the only documents references may resolve to, apart
	// from the engine's embedded draft metaschemas.
	Resources []Resource
	// DefaultDraft is the dialect for a root that declares no $schema:
	// "draft-04", "draft-06", "draft-07" (the default when empty),
	// "draft/2019-09" or "draft/2020-12". It is reported as "defaulted".
	DefaultDraft string
	// FormatMode defaults to FormatAssert.
	FormatMode FormatMode
	// Limits default to DefaultLimits when zero.
	Limits Limits
}

// Capability describes what a compiled schema was actually checked with.
type Capability struct {
	Engine        string `json:"engine"`
	Dialect       string `json:"dialect"`
	DialectSource string `json:"dialect_source"` // "declared", "defaulted" or "metaschema"
	FormatMode    string `json:"format_mode"`
	LeapPolicy    string `json:"leap_policy"`
}

// Issue is one validation failure, flattened from the engine's output.
type Issue struct {
	InstanceLocation string `json:"instance_location"`
	KeywordLocation  string `json:"keyword_location"`
	// Keyword is the keyword that failed, taken from the engine's error kind
	// (for example "not", "required", "additionalProperties").
	Keyword string `json:"keyword,omitempty"`
	Message string `json:"message"`
}

// Result is the outcome of validating one instance. An invalid instance is
// a Result with Valid false; it is never reported as an error.
type Result struct {
	Valid  bool    `json:"valid"`
	Issues []Issue `json:"issues,omitempty"`
}

// Compiled is an admitted, compiled schema. It is immutable.
type Compiled struct {
	schema     *jsonschema.Schema
	capability Capability
	key        string
	lim        Limits
}

// Capability reports the effective dialect and checking policy.
func (c *Compiled) Capability() Capability { return c.capability }

// Key is the cache identity of this compilation.
func (c *Compiled) Key() string { return c.key }

// Validate checks a decoded instance. Numbers must be json.Number (see
// DecodeJSON) for exact numeric comparison.
func (c *Compiled) Validate(instance any) Result {
	err := c.schema.Validate(instance)
	if err == nil {
		return Result{Valid: true}
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return Result{Issues: []Issue{{Message: SafeText(err.Error())}}}
	}
	// Leaf failures come from the detailed (tree) output. v6.0.3's flattened
	// basic output drops the causes beneath a single-cause $ref and labels
	// the remaining unit "validation failed".
	out := ve.DetailedOutput()
	issues := leafIssues(out, nil)
	if len(issues) == 0 {
		issues = []Issue{{Message: SafeText(ve.Error())}}
	}
	return Result{Issues: issues}
}

// leafIssues collects the units that carry an error (the leaves) in tree
// order.
func leafIssues(u *jsonschema.OutputUnit, acc []Issue) []Issue {
	if u == nil {
		return acc
	}
	if u.Error != nil {
		kw := ""
		if u.Error.Kind != nil {
			kw = kindKeyword(u.Error.Kind)
		}
		acc = append(acc, Issue{
			InstanceLocation: SafeText(u.InstanceLocation),
			KeywordLocation:  SafeText(u.KeywordLocation),
			Keyword:          SafeText(kw),
			Message:          SafeText(u.Error.String()),
		})
	}
	for i := range u.Errors {
		acc = leafIssues(&u.Errors[i], acc)
	}
	return acc
}

// kindKeyword names the keyword an error kind reports. v6.0.3 returns no
// keyword path for some kinds (such as Not); their type name, in lower
// camel case, is the keyword.
func kindKeyword(k jsonschema.ErrorKind) string {
	if path := k.KeywordPath(); len(path) > 0 {
		return path[len(path)-1]
	}
	name := reflect.TypeOf(k).String()
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return ""
	}
	return strings.ToLower(name[:1]) + name[1:]
}

// ValidateJSON decodes data with DecodeJSON and validates it.
func (c *Compiled) ValidateJSON(data []byte) (Result, error) {
	v, err := DecodeJSON(data, c.limits().MaxInstanceBytes)
	if err != nil {
		return Result{}, err
	}
	return c.Validate(v), nil
}

func (c *Compiled) limits() Limits { return c.lim }

// DecodeJSON decodes exactly one JSON value, keeping numbers as json.Number.
// Trailing data and documents larger than maxBytes (when > 0) are errors.
func DecodeJSON(data []byte, maxBytes int) (any, error) {
	if maxBytes > 0 && len(data) > maxBytes {
		return nil, fmt.Errorf("document is %d bytes, limit is %d", len(data), maxBytes)
	}
	// encoding/json silently replaces invalid UTF-8 and unpaired surrogate
	// escapes with U+FFFD. Refuse them so a format checker never sees a
	// replacement character that was not in the input.
	if err := checkJSONText(data); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after JSON value")
	}
	return v, nil
}

// checkJSONText rejects input that is not valid UTF-8 and string escapes
// that encode an unpaired UTF-16 surrogate.
func checkJSONText(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("JSON text is not valid UTF-8")
	}
	inString := false
	pendingHigh := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if !inString {
			if c == '"' {
				inString = true
			}
			continue
		}
		if c == '\\' && i+1 < len(data) {
			if data[i+1] == 'u' && i+5 < len(data) {
				r, ok := hex4(data[i+2 : i+6])
				if ok {
					switch {
					case r >= 0xD800 && r <= 0xDBFF:
						if pendingHigh {
							return errors.New("JSON string has an unpaired surrogate escape")
						}
						pendingHigh = true
					case r >= 0xDC00 && r <= 0xDFFF:
						if !pendingHigh {
							return errors.New("JSON string has an unpaired surrogate escape")
						}
						pendingHigh = false
					default:
						if pendingHigh {
							return errors.New("JSON string has an unpaired surrogate escape")
						}
					}
				}
				i += 5
				continue
			}
			if pendingHigh {
				return errors.New("JSON string has an unpaired surrogate escape")
			}
			i++
			continue
		}
		if pendingHigh {
			return errors.New("JSON string has an unpaired surrogate escape")
		}
		if c == '"' {
			inString = false
		}
	}
	return nil
}

func hex4(b []byte) (rune, bool) {
	var r rune
	for _, c := range b {
		r <<= 4
		switch {
		case c >= '0' && c <= '9':
			r |= rune(c - '0')
		case c >= 'a' && c <= 'f':
			r |= rune(c-'a') + 10
		case c >= 'A' && c <= 'F':
			r |= rune(c-'A') + 10
		default:
			return 0, false
		}
	}
	return r, true
}

// Compile admits and compiles root under opts. All admission failures are
// returned as *AdmissionError.
func Compile(root []byte, opts Options) (*Compiled, error) {
	if opts.FormatMode != FormatAssert {
		return nil, &AdmissionError{Reason: ReasonUnsupportedOption,
			Detail: fmt.Sprintf("format mode %s is not supported; only assert is available", opts.FormatMode)}
	}
	lim := opts.Limits.withDefaults()

	rootDoc, err := admitDocument("root", root, lim)
	if err != nil {
		return nil, err
	}
	if len(opts.Resources) > lim.MaxResources {
		return nil, &AdmissionError{Reason: ReasonLimit,
			Detail: fmt.Sprintf("%d resources exceed the limit of %d", len(opts.Resources), lim.MaxResources)}
	}

	base, err := rootBase(rootDoc, root, opts.BaseURL)
	if err != nil {
		return nil, err
	}

	loader := newClosedLoader()
	if err := loader.admit(base, rootDoc); err != nil {
		return nil, err
	}
	for _, r := range opts.Resources {
		if err := requireAbsoluteURL(r.URL); err != nil {
			return nil, err
		}
		doc, err := admitDocument(r.URL, r.Bytes, lim)
		if err != nil {
			return nil, err
		}
		if err := loader.admit(r.URL, doc); err != nil {
			return nil, err
		}
	}

	defName, defDraft, err := defaultDraft(opts.DefaultDraft)
	if err != nil {
		return nil, err
	}
	dialect, source, err := rootDialect(rootDoc, defName)
	if err != nil {
		return nil, err
	}

	adm := &admission{limits: lim}
	re := &boundedRegexp{max: lim.MaxPatternBytes}
	re.compiling.Store(true)
	defer re.compiling.Store(false)
	c := jsonschema.NewCompiler()
	c.UseRegexpEngine(re.engine) // Go RE2 with the pattern budget; never a backtracking engine
	c.UseLoader(loader)
	c.DefaultDraft(defDraft)
	c.AssertFormat()
	for _, f := range ownedFormats() {
		c.RegisterFormat(f)
	}
	c.RegisterVocabulary(adm.vocabulary())
	c.AssertVocabs()

	sch, err := c.Compile(base)
	if err != nil {
		// The engine may reach the regexp engine first through metaschema
		// validation (format "regex" on pattern), which wraps the error.
		if re.tripped.Load() {
			return nil, &AdmissionError{Reason: ReasonLimit,
				Detail: fmt.Sprintf("a pattern exceeds the limit of %d bytes", lim.MaxPatternBytes)}
		}
		return nil, toAdmissionError(err)
	}
	if adm.err != nil {
		return nil, adm.err
	}

	return &Compiled{
		schema: sch,
		capability: Capability{
			Engine:        Engine,
			Dialect:       dialect,
			DialectSource: source,
			FormatMode:    opts.FormatMode.String(),
			LeapPolicy:    LeapPolicy,
		},
		key: cacheKey(opts.BaseURL, root, opts, lim),
		lim: lim,
	}, nil
}

func defaultDraft(name string) (string, *jsonschema.Draft, error) {
	switch name {
	case "", "draft-07":
		return "draft-07", jsonschema.Draft7, nil
	case "draft-04":
		return name, jsonschema.Draft4, nil
	case "draft-06":
		return name, jsonschema.Draft6, nil
	case "draft/2019-09":
		return name, jsonschema.Draft2019, nil
	case "draft/2020-12":
		return name, jsonschema.Draft2020, nil
	}
	return "", nil, &AdmissionError{Reason: ReasonUnsupportedOption, Detail: fmt.Sprintf("unknown default draft %q", name)}
}

// cacheKey binds the root identity and bytes, every admitted URL to its
// content, and the effective options.
func cacheKey(baseOpt string, root []byte, opts Options, lim Limits) string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			_, _ = fmt.Fprintf(h, "%d:%s", len(p), p) // hash.Hash writes never fail
		}
	}
	sum := sha256.Sum256(root)
	write("v1", Engine, LeapPolicy, opts.FormatMode.String(), opts.DefaultDraft, fmt.Sprintf("%+v", lim),
		baseOpt, hex.EncodeToString(sum[:]))
	rs := make([]string, 0, len(opts.Resources))
	for _, r := range opts.Resources {
		s := sha256.Sum256(r.Bytes)
		rs = append(rs, r.URL+"\x00"+hex.EncodeToString(s[:]))
	}
	sort.Strings(rs)
	write(rs...)
	return hex.EncodeToString(h.Sum(nil))
}

// Cache memoises compilations by cache key. Compile errors are cached too,
// so a failing schema reports the same admission error on every lookup.
type Cache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	once     sync.Once
	compiled *Compiled
	err      error
}

// Get compiles root under opts once per cache key.
func (c *Cache) Get(root []byte, opts Options) (*Compiled, error) {
	key := cacheKey(opts.BaseURL, root, opts, opts.Limits.withDefaults())
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[string]*cacheEntry{}
	}
	e, ok := c.entries[key]
	if !ok {
		e = &cacheEntry{}
		c.entries[key] = e
	}
	c.mu.Unlock()
	e.once.Do(func() { e.compiled, e.err = Compile(root, opts) })
	return e.compiled, e.err
}
