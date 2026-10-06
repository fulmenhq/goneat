package sbom

import (
	"fmt"
	"strings"
)

// Reference validation complements JSON Schema: schemas validate the shape of
// references but cannot require their targets to exist in this document.
func validateSourceReferences(document map[string]any, format string) error {
	ids := make(map[string]bool)
	add := func(value any) error {
		id, ok := value.(string)
		if !ok || id == "" {
			return nil // Required fields and types are checked by JSON Schema.
		}
		if ids[id] {
			return fmt.Errorf("source SBOM has duplicate identifier %q", id)
		}
		ids[id] = true
		return nil
	}
	if format == "cyclonedx-json" {
		return validateSourceCycloneDXReferences(document)
	}
	if format != "spdx-json" {
		return fmt.Errorf("unsupported source SBOM reference format %q", format)
	}
	if err := add(document["SPDXID"]); err != nil {
		return err
	}
	for _, key := range []string{"packages", "files", "snippets"} {
		for _, value := range sourceArray(document[key]) {
			if err := add(sourceObject(value)["SPDXID"]); err != nil {
				return err
			}
		}
	}
	external := make(map[string]bool)
	for _, value := range sourceArray(document["externalDocumentRefs"]) {
		id, _ := sourceObject(value)["externalDocumentId"].(string)
		if external[id] {
			return fmt.Errorf("source SBOM has duplicate external document identifier %q", id)
		}
		external[id] = true
	}
	check := func(value any, special bool) error {
		id, ok := value.(string)
		if ok {
			if ids[id] || (special && (id == "NONE" || id == "NOASSERTION")) {
				return nil
			}
			parts := strings.SplitN(id, ":", 2)
			if len(parts) == 2 && external[parts[0]] && strings.HasPrefix(parts[1], "SPDXRef-") && len(parts[1]) > len("SPDXRef-") {
				return nil
			}
		}
		return fmt.Errorf("source SBOM has unresolved SPDX reference %v", value)
	}
	for _, value := range sourceArray(document["relationships"]) {
		edge := sourceObject(value)
		if err := check(edge["spdxElementId"], false); err != nil {
			return err
		}
		if err := check(edge["relatedSpdxElement"], true); err != nil {
			return err
		}
	}
	for _, ref := range sourceArray(document["documentDescribes"]) {
		if err := check(ref, false); err != nil {
			return err
		}
	}
	for _, value := range sourceArray(document["files"]) {
		for _, ref := range sourceArray(sourceObject(value)["fileDependencies"]) {
			if err := check(ref, false); err != nil {
				return err
			}
		}
	}
	for _, value := range sourceArray(document["packages"]) {
		for _, ref := range sourceArray(sourceObject(value)["hasFiles"]) {
			if err := check(ref, false); err != nil {
				return err
			}
		}
	}
	for _, value := range sourceArray(document["snippets"]) {
		snippet := sourceObject(value)
		if err := check(snippet["snippetFromFile"], false); err != nil {
			return err
		}
		for _, value := range sourceArray(snippet["ranges"]) {
			for _, key := range []string{"startPointer", "endPointer"} {
				if err := check(sourceObject(sourceObject(value)[key])["reference"], false); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
