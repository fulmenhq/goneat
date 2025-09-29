package format

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// PrettifyJSON prettifies JSON content using Go's json.Indent or json.Marshal for compact
// Returns prettified content, whether changes were made, and any error
func PrettifyJSON(input []byte, indent string, sizeWarningMB int) ([]byte, bool, error) {
	// Validate JSON first (quick check)
	if !json.Valid(input) {
		return nil, false, fmt.Errorf("invalid JSON")
	}

	// For very large files, warn but proceed (configurable threshold)
	if sizeWarningMB > 0 && len(input) > sizeWarningMB*1024*1024 {
		logger.Warn(fmt.Sprintf("Processing very large JSON file (>%dMB); may consume significant memory", sizeWarningMB))
	}

	var output []byte
	var err error

	if indent == "" {
		// Compact mode: use json.Marshal for true compactness
		var v interface{}
		if err := json.Unmarshal(input, &v); err != nil {
			return nil, false, err
		}
		output, err = json.Marshal(v)
		if err != nil {
			return nil, false, err
		}
	} else {
		// Pretty mode: use json.Indent
		var buf bytes.Buffer
		if err := json.Indent(&buf, input, "", indent); err != nil {
			return nil, false, err
		}
		output = buf.Bytes()
	}

	changed := !bytes.Equal(input, output)
	return output, changed, nil
}
