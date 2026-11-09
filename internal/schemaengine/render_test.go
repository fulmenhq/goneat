package schemaengine

import (
	"errors"
	"strings"
	"testing"
)

func TestSafeTextEscapesUnsafeScalars(t *testing.T) {
	unsafe := []rune{0x00, 0x09, 0x0A, 0x1B, 0x7F, 0x80, 0x85, 0x9F, 0x2028, 0x2029, 0xFEFF, 0xFFFD,
		0x202A, 0x202B, 0x202C, 0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069, 0x200E, 0x200F, 0x061C,
		0xFDD0, 0xFDEF, 0xFFFE, 0xFFFF, 0x1FFFE, 0x10FFFF}
	for _, r := range unsafe {
		out := SafeText("a" + string(r) + "b")
		if strings.ContainsRune(out, r) || !strings.HasPrefix(out, "a\\") || !strings.HasSuffix(out, "b") {
			t.Errorf("U+%04X not escaped: %q", r, out)
		}
	}
	for _, s := range []string{"plain", "café", "用户@例子.广告", "Ωmega", "a b", "😀", "פריט"} {
		if got := SafeText(s); got != s {
			t.Errorf("safe text changed: %q -> %q", s, got)
		}
	}
	// Escaping is unambiguous: a literal backslash sequence is not confused
	// with an escaped control.
	if SafeText(`\u0000`) == SafeText("\x00") {
		t.Error("escape is ambiguous")
	}
}

func TestDiagnosticsAreRenderedSafely(t *testing.T) {
	c := mustCompile(t, "{\"properties\":{\"a\u202eb\":{\"format\":\"date-time\"}}}", Options{})
	r, err := c.ValidateJSON([]byte("{\"a\u202eb\":\"x\u0085\u202e y\"}"))
	if err != nil || r.Valid {
		t.Fatalf("%+v %v", r, err)
	}
	for _, is := range r.Issues {
		for _, field := range []string{is.Message, is.InstanceLocation, is.KeywordLocation} {
			for _, bad := range []rune{0x85, 0x202E, 0x2028} {
				if strings.ContainsRune(field, bad) {
					t.Errorf("raw U+%04X in diagnostic %q", bad, field)
				}
			}
		}
	}
	_, err = Compile([]byte("{\"format\":\"x\u202ey\"}"), Options{})
	var ae *AdmissionError
	if !errors.As(err, &ae) || strings.ContainsRune(err.Error(), 0x202E) || !strings.Contains(err.Error(), `x\u202Ey`) {
		t.Errorf("admission error not escaped: %v", err)
	}
}
