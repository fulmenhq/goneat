package signature

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Detector evaluates file snippets against compiled signatures.
type Detector struct {
	signatures []compiledSignature
}

// DetectOptions constrains detection to specific IDs or categories.
type DetectOptions struct {
	AllowedIDs        map[string]struct{}
	AllowedCategories map[string]struct{}
}

// Match represents a successful signature detection.
type Match struct {
	Signature Signature
	Score     float64
	Matchers  []Matcher
}

type compiledSignature struct {
	signature  Signature
	matchers   []compiledMatcher
	threshold  float64
	extensions map[string]struct{}
}

type compiledMatcher struct {
	matcher  Matcher
	contains []byte
	regex    *regexp.Regexp
}

// NewDetector compiles the manifest for fast evaluation.
func NewDetector(manifest *Manifest) (*Detector, error) {
	if manifest == nil {
		return &Detector{}, nil
	}

	compiled := make([]compiledSignature, 0, len(manifest.Signatures))
	for _, sig := range manifest.Signatures {
		cs := compiledSignature{
			signature: sig,
			threshold: sig.ConfidenceThreshold,
		}
		if cs.threshold <= 0 {
			cs.threshold = defaultConfidenceThreshold
		}
		if len(sig.FileExtensions) > 0 {
			cs.extensions = make(map[string]struct{}, len(sig.FileExtensions))
			for _, ext := range sig.FileExtensions {
				cs.extensions[strings.ToLower(ext)] = struct{}{}
			}
		}
		for _, m := range sig.Matchers {
			matcher := compiledMatcher{matcher: m}
			switch m.Type {
			case "contains", "prefix", "suffix":
				value := normalizeLiteral(m.Value)
				if m.IgnoreCase {
					matcher.contains = []byte(strings.ToLower(value))
				} else {
					matcher.contains = []byte(value)
				}
			case "regex":
				pattern := m.Pattern
				if pattern == "" {
					continue
				}
				var expr string
				if strings.HasPrefix(pattern, "(?") {
					expr = pattern
				} else {
					expr = pattern
				}
				if m.IgnoreCase && !strings.Contains(expr, "(?i") {
					expr = "(?i)" + expr
				}
				if !strings.Contains(expr, "(?m") {
					expr = "(?m)" + expr
				}
				re, err := regexp.Compile(expr)
				if err != nil {
					return nil, err
				}
				matcher.regex = re
			default:
				continue
			}
			cs.matchers = append(cs.matchers, matcher)
		}
		if len(cs.matchers) == 0 {
			continue
		}
		compiled = append(compiled, cs)
	}

	return &Detector{signatures: compiled}, nil
}

// Detect finds the best matching signature for the given file path/snippet.
func (d *Detector) Detect(path string, snippet []byte, opts DetectOptions) (Match, bool) {
	if len(d.signatures) == 0 {
		return Match{}, false
	}

	bestScore := 0.0
	var bestSig *compiledSignature
	var bestMatchers []Matcher

	for i := range d.signatures {
		sig := &d.signatures[i]
		if !allowsSignature(sig.signature, opts) {
			continue
		}
		if len(sig.extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			if _, ok := sig.extensions[ext]; !ok {
				continue
			}
		}

		score, matched := evaluateSignature(sig, snippet)
		if score >= sig.threshold && score > bestScore {
			bestScore = score
			bestSig = sig
			bestMatchers = matched
		}
	}

	if bestSig == nil {
		return Match{}, false
	}

	return Match{
		Signature: bestSig.signature,
		Score:     clampScore(bestScore),
		Matchers:  bestMatchers,
	}, true
}

// DetectAll returns every signature that meets its threshold, sorted by score descending.
func (d *Detector) DetectAll(path string, snippet []byte, opts DetectOptions) []Match {
	var matches []Match
	for i := range d.signatures {
		sig := &d.signatures[i]
		if !allowsSignature(sig.signature, opts) {
			continue
		}
		if len(sig.extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			if _, ok := sig.extensions[ext]; !ok {
				continue
			}
		}
		score, matched := evaluateSignature(sig, snippet)
		if score >= sig.threshold {
			matches = append(matches, Match{
				Signature: sig.signature,
				Score:     clampScore(score),
				Matchers:  matched,
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].Signature.ID < matches[j].Signature.ID
		}
		return matches[i].Score > matches[j].Score
	})
	return matches
}

func allowsSignature(sig Signature, opts DetectOptions) bool {
	if len(opts.AllowedIDs) == 0 && len(opts.AllowedCategories) == 0 {
		return true
	}
	if len(opts.AllowedIDs) > 0 {
		if _, ok := opts.AllowedIDs[strings.ToLower(sig.ID)]; ok {
			return true
		}
		for _, alias := range sig.Aliases {
			if _, ok := opts.AllowedIDs[strings.ToLower(alias)]; ok {
				return true
			}
		}
		return false
	}
	if len(opts.AllowedCategories) > 0 {
		if _, ok := opts.AllowedCategories[strings.ToLower(sig.Category)]; ok {
			return true
		}
		return false
	}
	return true
}

func evaluateSignature(sig *compiledSignature, snippet []byte) (float64, []Matcher) {
	var score float64
	var matched []Matcher
	sample := snippet
	lowerSample := bytes.ToLower(sample)

	for _, matcher := range sig.matchers {
		var ok bool
		switch matcher.matcher.Type {
		case "contains":
			if matcher.matcher.IgnoreCase {
				ok = bytes.Contains(lowerSample, matcher.contains)
			} else {
				ok = bytes.Contains(sample, matcher.contains)
			}
		case "prefix":
			if matcher.matcher.IgnoreCase {
				ok = bytes.HasPrefix(lowerSample, matcher.contains)
			} else {
				ok = bytes.HasPrefix(sample, matcher.contains)
			}
		case "suffix":
			if matcher.matcher.IgnoreCase {
				ok = bytes.HasSuffix(lowerSample, matcher.contains)
			} else {
				ok = bytes.HasSuffix(sample, matcher.contains)
			}
		case "regex":
			ok = matcher.regex != nil && matcher.regex.Find(sample) != nil
		default:
			continue
		}
		if ok {
			score += matcher.matcher.Weight
			matched = append(matched, matcher.matcher)
		}
	}

	return score, matched
}

func clampScore(score float64) float64 {
	if score > 1 {
		return 1
	}
	if score < 0 {
		return 0
	}
	return score
}

func normalizeLiteral(value string) string {
	if value == "" {
		return value
	}
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer(`\"`, `"`, `\\`, `\\`)
	return replacer.Replace(value)
}
