package assess

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Parse arrays incrementally, and retain the existing NDJSON compatibility.
// A malformed tail or failed reader leaves prior complete findings available.
func (r *SecurityAssessmentRunner) parseGitleaksStream(input io.Reader) ([]Issue, error) {
	buffer := bufio.NewReader(input)
	for {
		b, err := buffer.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("gitleaks report is absent or unreadable: %w", err)
		}
		if b == ' ' || b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		if err := buffer.UnreadByte(); err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(buffer)
		decoder.UseNumber()
		var issues []Issue
		decodeFinding := func() error {
			var finding map[string]interface{}
			if err := decoder.Decode(&finding); err != nil {
				return fmt.Errorf("gitleaks finding JSON: %w", err)
			}
			issue := r.mapGitleaksFinding(finding)
			if issue == nil || issue.File == "" {
				return errors.New("gitleaks report has an invalid finding")
			}
			issues = append(issues, *issue)
			return nil
		}
		if b == '[' {
			if _, err := decoder.Token(); err != nil {
				return issues, err
			}
			for decoder.More() {
				if err := decodeFinding(); err != nil {
					return issues, err
				}
			}
			token, err := decoder.Token()
			if err != nil {
				return issues, fmt.Errorf("gitleaks report array completion: %w", err)
			}
			if token != json.Delim(']') {
				return issues, errors.New("gitleaks report array is truncated")
			}
			var trailing json.RawMessage
			if err := decoder.Decode(&trailing); err != nil && !errors.Is(err, io.EOF) {
				return issues, fmt.Errorf("gitleaks report trailing read: %w", err)
			} else if err == nil {
				return issues, errors.New("gitleaks report has invalid trailing data")
			}
			return issues, nil
		}
		if b != '{' {
			return nil, errors.New("gitleaks report is not an array or finding stream")
		}
		for {
			if err := decodeFinding(); err != nil {
				if errors.Is(err, io.EOF) && len(issues) > 0 {
					return issues, nil
				}
				return issues, err
			}
		}
	}
}

func (r *SecurityAssessmentRunner) parseGitleaksOutput(data []byte) ([]Issue, error) {
	return r.parseGitleaksStream(strings.NewReader(string(data)))
}
