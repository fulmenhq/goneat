package mapping

import (
	"fmt"
)

// Version constants for schema mapping manifests.
const (
	ManifestVersionV1 = "1.0.0"
)

// InferenceMethod describes how a mapping should resolve schemas when a pattern matches.
type InferenceMethod string

const (
	InferenceNone       InferenceMethod = "none"
	InferenceContent    InferenceMethod = "content"
	InferenceMetaSchema InferenceMethod = "meta-schema"
)

// SchemaSource identifies where a schema should be loaded from.
type SchemaSource string

const (
	SourceEmbedded SchemaSource = "embedded"
	SourceExternal SchemaSource = "external"
	SourceLocal    SchemaSource = "local"
)

// MappingPriority describes ordering hints for competing mapping rules.
type MappingPriority string

const (
	PriorityLow    MappingPriority = "low"
	PriorityNormal MappingPriority = "normal"
	PriorityHigh   MappingPriority = "high"
)

// ExclusionAction describes how exclusions should be applied when matched.
type ExclusionAction string

const (
	ExclusionSkip           ExclusionAction = "skip"
	ExclusionRetryInference ExclusionAction = "retry_inference"
)

// Manifest describes the full mapping declaration for a repository.
type Manifest struct {
	Version    string          `json:"version" yaml:"version"`
	Config     ConfigSettings  `json:"config,omitempty" yaml:"config,omitempty"`
	Mappings   []MappingRule   `json:"mappings,omitempty" yaml:"mappings,omitempty"`
	Exclusions []ExclusionRule `json:"exclusions,omitempty" yaml:"exclusions,omitempty"`
	Overrides  []OverrideRule  `json:"overrides,omitempty" yaml:"overrides,omitempty"`
}

// ConfigSettings tunes mapper behaviour.
type ConfigSettings struct {
	InferenceEnabled      *bool    `json:"inference_enabled,omitempty" yaml:"inference_enabled,omitempty"`
	FallbackToContent     *bool    `json:"fallback_to_content,omitempty" yaml:"fallback_to_content,omitempty"`
	StrictMode            *bool    `json:"strict_mode,omitempty" yaml:"strict_mode,omitempty"`
	CacheInferences       *bool    `json:"cache_inferences,omitempty" yaml:"cache_inferences,omitempty"`
	RespectExclusions     *bool    `json:"respect_exclusions,omitempty" yaml:"respect_exclusions,omitempty"`
	AutoSuggestExclusions *bool    `json:"auto_suggest_exclusions,omitempty" yaml:"auto_suggest_exclusions,omitempty"`
	MinConfidence         *float64 `json:"min_confidence,omitempty" yaml:"min_confidence,omitempty"`
	MaxSuggestions        *int     `json:"max_suggestions,omitempty" yaml:"max_suggestions,omitempty"`
}

// MappingRule defines how to associate files with schemas.
type MappingRule struct {
	Pattern         string          `json:"pattern" yaml:"pattern"`
	SchemaID        string          `json:"schema_id,omitempty" yaml:"schema_id,omitempty"`
	InferenceMethod InferenceMethod `json:"inference_method,omitempty" yaml:"inference_method,omitempty"`
	FallbackSchema  string          `json:"fallback_schema,omitempty" yaml:"fallback_schema,omitempty"`
	Source          SchemaSource    `json:"source,omitempty" yaml:"source,omitempty"`
	Priority        MappingPriority `json:"priority,omitempty" yaml:"priority,omitempty"`
	Conditions      []Condition     `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// Condition adds field-based detection logic to mapping rules.
type Condition struct {
	Field    string `json:"field" yaml:"field"`
	Exists   *bool  `json:"exists,omitempty" yaml:"exists,omitempty"`
	Equals   any    `json:"equals,omitempty" yaml:"equals,omitempty"`
	SchemaID string `json:"schema_id,omitempty" yaml:"schema_id,omitempty"`
}

// ExclusionRule captures explicit opt-outs for mapping matches.
type ExclusionRule struct {
	Pattern        string            `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	ContentPattern *ContentCondition `json:"content_pattern,omitempty" yaml:"content_pattern,omitempty"`
	ExcludeSchema  string            `json:"exclude_schema,omitempty" yaml:"exclude_schema,omitempty"`
	Reason         string            `json:"reason,omitempty" yaml:"reason,omitempty"`
	Action         ExclusionAction   `json:"action,omitempty" yaml:"action,omitempty"`
}

// ContentCondition describes field-based exclusion detection.
type ContentCondition struct {
	Field  string `json:"field" yaml:"field"`
	Exists *bool  `json:"exists,omitempty" yaml:"exists,omitempty"`
	Equals any    `json:"equals,omitempty" yaml:"equals,omitempty"`
}

// OverrideRule allows schema source overrides (future-facing).
type OverrideRule struct {
	SchemaID string `json:"schema_id" yaml:"schema_id"`
	Source   string `json:"source,omitempty" yaml:"source,omitempty"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
}

// Clone returns a deep copy of the manifest for safe mutation.
func (m Manifest) Clone() Manifest {
	clone := Manifest{
		Version:    m.Version,
		Config:     m.Config.Clone(),
		Mappings:   cloneMappings(m.Mappings),
		Exclusions: cloneExclusions(m.Exclusions),
		Overrides:  cloneOverrides(m.Overrides),
	}
	return clone
}

// Clone returns a copy of the config settings.
func (c ConfigSettings) Clone() ConfigSettings {
	return ConfigSettings{
		InferenceEnabled:      cloneBoolPtr(c.InferenceEnabled),
		FallbackToContent:     cloneBoolPtr(c.FallbackToContent),
		StrictMode:            cloneBoolPtr(c.StrictMode),
		CacheInferences:       cloneBoolPtr(c.CacheInferences),
		RespectExclusions:     cloneBoolPtr(c.RespectExclusions),
		AutoSuggestExclusions: cloneBoolPtr(c.AutoSuggestExclusions),
		MinConfidence:         cloneFloatPtr(c.MinConfidence),
		MaxSuggestions:        cloneIntPtr(c.MaxSuggestions),
	}
}

// Merge overlays non-nil values from overlay onto receiver, returning the merged config.
func (c ConfigSettings) Merge(overlay ConfigSettings) ConfigSettings {
	result := c.Clone()

	if overlay.InferenceEnabled != nil {
		result.InferenceEnabled = cloneBoolPtr(overlay.InferenceEnabled)
	}
	if overlay.FallbackToContent != nil {
		result.FallbackToContent = cloneBoolPtr(overlay.FallbackToContent)
	}
	if overlay.StrictMode != nil {
		result.StrictMode = cloneBoolPtr(overlay.StrictMode)
	}
	if overlay.CacheInferences != nil {
		result.CacheInferences = cloneBoolPtr(overlay.CacheInferences)
	}
	if overlay.RespectExclusions != nil {
		result.RespectExclusions = cloneBoolPtr(overlay.RespectExclusions)
	}
	if overlay.AutoSuggestExclusions != nil {
		result.AutoSuggestExclusions = cloneBoolPtr(overlay.AutoSuggestExclusions)
	}
	if overlay.MinConfidence != nil {
		result.MinConfidence = cloneFloatPtr(overlay.MinConfidence)
	}
	if overlay.MaxSuggestions != nil {
		result.MaxSuggestions = cloneIntPtr(overlay.MaxSuggestions)
	}

	return result
}

// WithDefaults applies builtin defaults for unset fields.
func (c ConfigSettings) WithDefaults(defaults ConfigSettings) ConfigSettings {
	merged := defaults.Merge(c)
	return merged
}

func cloneMappings(in []MappingRule) []MappingRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]MappingRule, len(in))
	for i, rule := range in {
		clone := rule
		if len(rule.Conditions) > 0 {
			conds := make([]Condition, len(rule.Conditions))
			copy(conds, rule.Conditions)
			clone.Conditions = conds
		}
		out[i] = clone
	}
	return out
}

func cloneExclusions(in []ExclusionRule) []ExclusionRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]ExclusionRule, len(in))
	for i, rule := range in {
		ruleCopy := rule
		if rule.ContentPattern != nil {
			cp := *rule.ContentPattern
			cp.Exists = cloneBoolPtr(rule.ContentPattern.Exists)
			cp.Equals = cloneAny(rule.ContentPattern.Equals)
			ruleCopy.ContentPattern = &cp
		}
		out[i] = ruleCopy
	}
	return out
}

func cloneOverrides(in []OverrideRule) []OverrideRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]OverrideRule, len(in))
	copy(out, in)
	return out
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func cloneFloatPtr(in *float64) *float64 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func cloneIntPtr(in *int) *int {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}

func cloneAny(in any) any {
	switch v := in.(type) {
	case nil:
		return nil
	case string, bool, int, int64, float64, float32:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
