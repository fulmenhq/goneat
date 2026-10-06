package sbom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

// decodeSourceJSON retains unknown members and exact numeric tokens. Duplicate
// members are rejected rather than allowing validation and mapping to disagree
// about which value is authoritative.
func decodeSourceJSON(data []byte) (map[string]any, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("source SBOM contains invalid UTF-8")
	}
	if err := validateSourceUnicode(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := readSourceJSON(decoder, 0)
	if err != nil {
		return nil, fmt.Errorf("source SBOM JSON: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("source SBOM JSON has trailing content")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("source SBOM JSON must be an object")
	}
	return object, nil
}

// encoding/json replaces unpaired UTF-16 escapes with U+FFFD. Reject those
// inputs so decoding cannot silently change collected evidence.
func validateSourceUnicode(data []byte) error {
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		i++
		if i >= len(data) || data[i] != 'u' || i+4 >= len(data) {
			continue // The JSON decoder diagnoses malformed escapes.
		}
		code, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil {
			continue
		}
		i += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return fmt.Errorf("source SBOM contains an unpaired UTF-16 escape")
		}
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
			return fmt.Errorf("source SBOM contains an unpaired UTF-16 escape")
		}
		low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return fmt.Errorf("source SBOM contains an unpaired UTF-16 escape")
		}
		i += 6
	}
	return nil
}

func readSourceJSON(decoder *json.Decoder, depth int) (any, error) {
	if depth > 256 {
		return nil, fmt.Errorf("nesting exceeds 256 levels")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("object member is not a string")
			}
			if _, exists := object[name]; exists {
				return nil, fmt.Errorf("duplicate object member %q", name)
			}
			value, err := readSourceJSON(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return nil, fmt.Errorf("unterminated object: %v", err)
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := readSourceJSON(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return nil, fmt.Errorf("unterminated array: %v", err)
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delimiter)
	}
}
