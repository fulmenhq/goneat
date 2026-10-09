package assess

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Callers retain each completely decoded member or array entry before this
// reader reaches a malformed tail. No permissive extraction marks it complete.
func readSecurityReportObject(decoder *json.Decoder, member func(string, *json.Decoder) error) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('{') {
		return errors.New("report object is absent or invalid")
	}
	seen := make(map[string]bool)
	var errs []error
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return errors.Join(append(errs, err)...)
		}
		key, ok := token.(string)
		if !ok {
			return errors.Join(append(errs, errors.New("report member name is invalid"))...)
		}
		if seen[key] {
			var duplicate json.RawMessage
			if err := decoder.Decode(&duplicate); err != nil {
				return errors.Join(append(errs, err)...)
			}
			errs = append(errs, errors.New("report contains a duplicate member"))
			continue
		}
		seen[key] = true
		if err := member(key, decoder); err != nil {
			return errors.Join(append(errs, err)...)
		}
	}
	if token, err := decoder.Token(); err != nil {
		return errors.Join(append(errs, err)...)
	} else if token != json.Delim('}') {
		return errors.Join(append(errs, errors.New("report object is incomplete"))...)
	}
	return errors.Join(errs...)
}

func readSecurityReportArray(decoder *json.Decoder, entry func(json.RawMessage)) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('[') {
		return errors.New("report list is absent or invalid")
	}
	for decoder.More() {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return err
		}
		entry(raw)
	}
	if token, err := decoder.Token(); err != nil {
		return err
	} else if token != json.Delim(']') {
		return errors.New("report list is incomplete")
	}
	return nil
}

func requireSecurityReportCount(raw json.RawMessage, label string) error {
	var count *uint64
	if err := json.Unmarshal(raw, &count); err != nil || count == nil {
		return fmt.Errorf("report %s count is missing or invalid", label)
	}
	return nil
}
