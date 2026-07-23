package format

import (
	"errors"
	"fmt"
)

// ResultClass identifies the reason a formatting operation did not succeed.
// These values are also emitted as structured log fields by the CLI.
type ResultClass string

const (
	ResultFormatDrift     ResultClass = "format-drift"
	ResultToolUnavailable ResultClass = "tool-unavailable"
	ResultToolExecution   ResultClass = "tool-execution"
	ResultFileIO          ResultClass = "file-io"
)

var (
	ErrFormatDrift      = errors.New("format drift")
	ErrToolUnavailable  = errors.New("format tool unavailable")
	ErrToolExecution    = errors.New("format tool execution failed")
	ErrFileIO           = errors.New("format file I/O failed")
	ErrAlreadyFormatted = errors.New("already formatted")
	ErrFinalized        = errors.New("finalized")
)

// ResultError carries a stable classification without changing the
// human-readable error returned by an existing formatter.
type ResultError struct {
	Class ResultClass
	Path  string
	Tool  string
	Err   error
}

func (e *ResultError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Class)
	}
	return e.Err.Error()
}

func (e *ResultError) Unwrap() []error {
	if e == nil {
		return nil
	}
	errs := []error{sentinelForClass(e.Class)}
	if e.Err != nil {
		errs = append(errs, e.Err)
	}
	return errs
}

func sentinelForClass(class ResultClass) error {
	switch class {
	case ResultFormatDrift:
		return ErrFormatDrift
	case ResultToolUnavailable:
		return ErrToolUnavailable
	case ResultToolExecution:
		return ErrToolExecution
	case ResultFileIO:
		return ErrFileIO
	default:
		return errors.New(string(class))
	}
}

// NewResultError wraps err with a stable result class.
func NewResultError(class ResultClass, path, tool string, err error) error {
	if err == nil {
		err = fmt.Errorf("%s", class)
	}
	return &ResultError{Class: class, Path: path, Tool: tool, Err: err}
}

// ClassOf returns the stable class carried by err, if any.
func ClassOf(err error) ResultClass {
	var resultErr *ResultError
	if errors.As(err, &resultErr) {
		return resultErr.Class
	}
	return ""
}

// DetailsOf returns classification metadata carried by err.
func DetailsOf(err error) (class ResultClass, path, tool string) {
	var resultErr *ResultError
	if errors.As(err, &resultErr) {
		return resultErr.Class, resultErr.Path, resultErr.Tool
	}
	return "", "", ""
}

// FormatDrift reports a file whose formatted representation differs.
func FormatDrift(path string) error {
	message := "needs formatting"
	if path != "" {
		message = fmt.Sprintf("file %s needs formatting", path)
	}
	return NewResultError(ResultFormatDrift, path, "", errors.New(message))
}

// ToolUnavailable reports a required formatter that could not be resolved.
func ToolUnavailable(path, tool, guidance string) error {
	message := fmt.Sprintf("%s not found", tool)
	if guidance != "" {
		message += ". " + guidance
	}
	return NewResultError(ResultToolUnavailable, path, tool, errors.New(message))
}

// ToolExecution reports a formatter invocation that failed unexpectedly.
func ToolExecution(path, tool string, err error) error {
	return NewResultError(ResultToolExecution, path, tool, err)
}

// FileIO reports a read or write failure while formatting.
func FileIO(path string, err error) error {
	return NewResultError(ResultFileIO, path, "", err)
}
