package format

import (
	"errors"
	"testing"
)

func TestResultErrorClassification(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		class    ResultClass
		sentinel error
	}{
		{name: "format drift", err: FormatDrift("sample.py"), class: ResultFormatDrift, sentinel: ErrFormatDrift},
		{name: "tool unavailable", err: ToolUnavailable("", "ruff", "install ruff"), class: ResultToolUnavailable, sentinel: ErrToolUnavailable},
		{name: "tool execution", err: ToolExecution("sample.py", "ruff", errors.New("failed")), class: ResultToolExecution, sentinel: ErrToolExecution},
		{name: "file io", err: FileIO("sample.py", errors.New("read failed")), class: ResultFileIO, sentinel: ErrFileIO},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassOf(test.err); got != test.class {
				t.Fatalf("ClassOf() = %q, want %q", got, test.class)
			}
			if !errors.Is(test.err, test.sentinel) {
				t.Fatalf("error does not unwrap to %v: %v", test.sentinel, test.err)
			}
		})
	}
}
