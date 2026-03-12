package safeio

import (
	"runtime"
	"testing"
)

func TestIsNullDevice(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// Unix canonical
		{name: "unix /dev/null", path: "/dev/null", want: true},

		// Windows canonical (exact case)
		{name: "windows NUL uppercase", path: "NUL", want: true},

		// Case variants — only match on Windows
		{name: "nul lowercase", path: "nul", want: runtime.GOOS == "windows"},
		{name: "Nul mixed case", path: "Nul", want: runtime.GOOS == "windows"},

		// Must NOT match
		{name: "empty string", path: "", want: false},
		{name: "regular file", path: "output.json", want: false},
		{name: "absolute path", path: "/tmp/file", want: false},
		{name: "NUL with extension", path: "NUL.txt", want: false},
		{name: "NUL in path", path: "/some/NUL/path", want: false},
		{name: "dev null without slash", path: "dev/null", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNullDevice(tt.path)
			if got != tt.want {
				t.Errorf("IsNullDevice(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestNullWriter(t *testing.T) {
	w := NullWriter()

	t.Run("write succeeds", func(t *testing.T) {
		n, err := w.Write([]byte("hello world"))
		if err != nil {
			t.Fatalf("NullWriter.Write() error = %v", err)
		}
		if n != 11 {
			t.Errorf("NullWriter.Write() = %d bytes, want 11", n)
		}
	})

	t.Run("close succeeds", func(t *testing.T) {
		if err := w.Close(); err != nil {
			t.Fatalf("NullWriter.Close() error = %v", err)
		}
	})

	t.Run("write after close succeeds", func(t *testing.T) {
		// NullWriter is a pure discard — close is a no-op,
		// so subsequent writes must still work.
		n, err := w.Write([]byte("more data"))
		if err != nil {
			t.Fatalf("NullWriter.Write() after Close error = %v", err)
		}
		if n != 9 {
			t.Errorf("NullWriter.Write() = %d bytes, want 9", n)
		}
	})
}
