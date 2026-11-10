package schemaengine

import (
	"fmt"
	"strings"
)

// SafeText escapes characters that must never be printed raw in goneat
// diagnostics: C0 and C1 controls, DEL, line and paragraph separators, the
// byte-order mark, U+FFFD, Unicode noncharacters and bidirectional format
// controls. Other text, including non-ASCII letters, is unchanged. Format
// checkers validate syntax only; rendering is where unsafe scalars are made
// visible instead of interpreted.
func SafeText(s string) string {
	if !needsEscape(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	for _, r := range s {
		if unsafeRune(r) {
			if r <= 0xFFFF {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				fmt.Fprintf(&b, `\U%08X`, r)
			}
			continue
		}
		if r == '\\' {
			b.WriteString(`\\`)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func needsEscape(s string) bool {
	for _, r := range s {
		if unsafeRune(r) || r == '\\' {
			return true
		}
	}
	return false
}

func unsafeRune(r rune) bool {
	switch {
	case r < 0x20, r == 0x7F, r >= 0x80 && r <= 0x9F: // C0, DEL, C1
		return true
	case r == 0x2028, r == 0x2029, r == 0xFEFF, r == 0xFFFD:
		return true
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069: // bidi embeddings, overrides, isolates
		return true
	case r == 0x200E, r == 0x200F, r == 0x061C: // bidi marks
		return true
	case r >= 0xFDD0 && r <= 0xFDEF, r&0xFFFE == 0xFFFE: // noncharacters
		return true
	}
	return false
}
