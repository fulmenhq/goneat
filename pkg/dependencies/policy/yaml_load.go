package policy

import (
	"gopkg.in/yaml.v3"
)

// UnmarshalPolicyYAML decodes a dependencies policy file into out.
//
// It behaves like yaml.Unmarshal except that every cooling exception `until`
// scalar is decoded as its literal text. YAML would otherwise turn unquoted
// values such as 2030-1-5 or 2030-01-01T00:00:00Z into time.Time and erase the
// spelling, so a value that is invalid when quoted could still exempt when
// unquoted. With the literal text, an unquoted 2030-01-05 is a valid date and
// everything else follows the same strict rules as a quoted value, including
// null, ~ and an empty value (all invalid; omit the key for no expiry).
func UnmarshalPolicyYAML(data []byte, out *map[string]interface{}) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Kind == 0 {
		return nil // empty document
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		markCoolingUntilLiteral(doc.Content[0])
	}
	return doc.Decode(out)
}

// mappingValue returns the value for key in mapping m, following YAML merge
// keys (<<: *anchor, or a list of anchors) with YAML precedence: keys written
// in the mapping win over merged keys, and earlier merge entries win over later
// ones. The seen set guards against cyclic merges.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	return lookupMapping(m, key, map[*yaml.Node]bool{})
}

func lookupMapping(m *yaml.Node, key string, seen map[*yaml.Node]bool) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode || seen[m] {
		return nil
	}
	seen[m] = true
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Tag != "!!merge" && m.Content[i].Value == key {
			return resolveAlias(m.Content[i+1])
		}
	}
	for _, src := range mergeSources(m) {
		if v := lookupMapping(src, key, seen); v != nil {
			return v
		}
	}
	return nil
}

// mergeSources lists the mappings merged into m through << keys, in order.
func mergeSources(m *yaml.Node) []*yaml.Node {
	var out []*yaml.Node
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Tag != "!!merge" {
			continue
		}
		val := resolveAlias(m.Content[i+1])
		if val == nil {
			continue
		}
		if val.Kind == yaml.SequenceNode {
			for _, item := range val.Content {
				out = append(out, resolveAlias(item))
			}
		} else {
			out = append(out, val)
		}
	}
	return out
}

func resolveAlias(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

func markCoolingUntilLiteral(root *yaml.Node) {
	cooling := mappingValue(resolveAlias(root), "cooling")
	exceptions := mappingValue(cooling, "exceptions")
	if exceptions == nil || exceptions.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range exceptions.Content {
		markUntilInMapping(resolveAlias(item), map[*yaml.Node]bool{})
	}
}

// markUntilInMapping tags every until scalar that one exception mapping can
// resolve to, including ones inherited through merge keys, so they all keep
// their literal text.
func markUntilInMapping(m *yaml.Node, seen map[*yaml.Node]bool) {
	if m == nil || m.Kind != yaml.MappingNode || seen[m] {
		return
	}
	seen[m] = true
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Tag == "!!merge" || m.Content[i].Value != "until" {
			continue
		}
		if val := resolveAlias(m.Content[i+1]); val != nil && val.Kind == yaml.ScalarNode {
			val.Tag = "!!str"
		}
	}
	for _, src := range mergeSources(m) {
		markUntilInMapping(src, seen)
	}
}
