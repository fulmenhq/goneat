package format

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/beevik/etree"
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

// PrettifyXML prettifies XML content using etree
// Returns prettified content, whether changes were made, and any error
func PrettifyXML(input []byte, indent string, sizeWarningMB int) ([]byte, bool, error) {
	// For very large files, warn but proceed (configurable threshold)
	if sizeWarningMB > 0 && len(input) > sizeWarningMB*1024*1024 {
		logger.Warn(fmt.Sprintf("Processing very large XML file (>%dMB); may consume significant memory", sizeWarningMB))
	}

	// Parse XML to check well-formedness and for formatting
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(input); err != nil {
		return nil, false, fmt.Errorf("XML is not well-formed: %v", err)
	}

	// If indent is empty, skip prettification
	if indent == "" {
		return input, false, nil
	}

	// Format the document with specified indent
	doc.Indent(2) // etree uses 2 spaces by default; we can adjust if needed

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		return nil, false, fmt.Errorf("failed to format XML: %v", err)
	}

	output := buf.Bytes()
	changed := !bytes.Equal(input, output)
	return output, changed, nil
}
