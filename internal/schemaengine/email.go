package schemaengine

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

// Owned email and idn-email checkers.
//
// email:     RFC 5321 section 4.1.2 Mailbox.
// idn-email: RFC 6531 section 3.3 extended Mailbox: UTF8-non-ascii is added
//            to atext and qtextSMTP (bare and quoted local parts, as written),
//            and a domain label containing non-ASCII must be a U-label.
//
// Checking extent (owned profile, syntax only; no DNS, no deliverability):
//   - U-labels are validated as written with x/net/idna's registration
//     profile (UTS 46 registration validation: NFC check without
//     normalising, disallowed/mapped/unknown/ignored code points refused,
//     STD3, hyphen and joiner rules, Bidi rule, DNS length). It is not strict
//     IDNA2008: NV8 code points are not excluded. Input is never mapped, so
//     U+3002 is not a dot and uppercase is not folded. The "--" in third and
//     fourth position rule is checked here on code points, because x/net/idna
//     checks byte offsets.
//   - ASCII labels follow the LDH rule. "xn--" labels are not decoded.
//   - Address literals: untagged IPv4 and "IPv6:" (case-insensitive) only;
//     other General-address-literal tags are refused. IPv6 "::" follows the
//     published RFC 5321 text (at least two groups); RFC 5321 erratum 4315
//     is not applied.
//   - Lengths in UTF-8 octets as written: local part <= 64 (including
//     quotes), labels <= 63 (A-label form for U-labels), whole mailbox <= 254.
//   - Quoted-pair stays ASCII only; C0 and DEL are never accepted.
//
// Diagnostics never print the instance raw; see SafeText.

var (
	errEmailSyntax  = errors.New("not a valid mailbox")
	errEmailLength  = errors.New("mailbox, local part or label exceeds its length limit")
	errEmailDomain  = errors.New("domain is not valid")
	errEmailLiteral = errors.New("address literal is not a supported IPv4 or IPv6 literal")
)

// IDNAUnicodeVersion is the Unicode version of the IDNA tables compiled into
// this build. It depends on the Go toolchain; tests pin it.
const IDNAUnicodeVersion = idna.UnicodeVersion

var idnaRegistration = idna.New(idna.ValidateForRegistration())

func checkEmail(v any) error    { return checkMailbox(v, false) }
func checkIDNEmail(v any) error { return checkMailbox(v, true) }

func checkMailbox(v any, intl bool) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if len(s) > 254 {
		return errEmailLength
	}
	if !utf8.ValidString(s) {
		return errEmailSyntax
	}
	local, rest, ok := splitLocal(s, intl)
	if !ok {
		return errEmailSyntax
	}
	if len(local) > 64 {
		return errEmailLength
	}
	if rest == "" {
		return errEmailSyntax
	}
	if rest[0] == '[' {
		if rest[len(rest)-1] != ']' || !validAddressLiteral(rest[1:len(rest)-1]) {
			return errEmailLiteral
		}
		return nil
	}
	return checkDomain(rest, intl)
}

// splitLocal parses the local part and returns it with the text after "@".
func splitLocal(s string, intl bool) (local, domain string, ok bool) {
	if s == "" {
		return "", "", false
	}
	var i int
	if s[0] == '"' {
		i = 1
		for {
			if i >= len(s) {
				return "", "", false
			}
			r, size := utf8.DecodeRuneInString(s[i:])
			switch {
			case r == '"':
				i++
				goto done
			case r == '\\':
				// quoted-pairSMTP = "\" %d32-126 (never extended)
				if i+1 >= len(s) || s[i+1] < 32 || s[i+1] > 126 {
					return "", "", false
				}
				i += 2
			case qtextSMTP(r, intl):
				i += size
			default:
				return "", "", false
			}
		}
	}
	// Dot-string = Atom *("." Atom)
	{
		atomStart := 0
		for i < len(s) && s[i] != '@' {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == '.' {
				if i == atomStart {
					return "", "", false
				}
				atomStart = i + 1
			} else if !atext(r, intl) {
				return "", "", false
			}
			i += size
		}
		if i == atomStart {
			return "", "", false
		}
	}
done:
	if i >= len(s) || s[i] != '@' {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

func atext(r rune, intl bool) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case strings.ContainsRune("!#$%&'*+-/=?^_`{|}~", r):
		return true
	case r >= 0x80:
		return intl // includes a genuine U+FFFD; invalid UTF-8 is refused earlier
	}
	return false
}

// qtextSMTP = %d32-33 / %d35-91 / %d93-126, plus UTF8-non-ascii for idn-email.
func qtextSMTP(r rune, intl bool) bool {
	switch {
	case r == 32 || r == 33 || (r >= 35 && r <= 91) || (r >= 93 && r <= 126):
		return true
	case r >= 0x80:
		return intl
	}
	return false
}

func checkDomain(d string, intl bool) error {
	if d == "" || len(d) > 255 {
		return errEmailDomain
	}
	for _, label := range strings.Split(d, ".") {
		if label == "" {
			return errEmailDomain
		}
		if isASCII(label) {
			if !ldhLabel(label) {
				return errEmailDomain
			}
			continue
		}
		if !intl {
			return errEmailDomain
		}
		// x/net/idna checks "--" at byte offsets 2-3; RFC 5891 section
		// 4.2.3.1 means the third and fourth code points.
		if r := []rune(label); len(r) >= 4 && r[2] == '-' && r[3] == '-' {
			return errEmailDomain
		}
		if !contextORules(label) {
			return errEmailDomain
		}
		a, err := idnaRegistration.ToASCII(label)
		if err != nil || len(a) > 63 || strings.Contains(a, ".") {
			return errEmailDomain
		}
	}
	return nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// sub-domain = Let-dig [Ldh-str]; at most 63 octets.
func ldhLabel(l string) bool {
	if len(l) > 63 || !letDig(l[0]) || !letDig(l[len(l)-1]) {
		return false
	}
	for i := 0; i < len(l); i++ {
		if !letDig(l[i]) && l[i] != '-' {
			return false
		}
	}
	return true
}

func letDig(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func validAddressLiteral(lit string) bool {
	if len(lit) >= 5 && strings.EqualFold(lit[:5], "IPv6:") {
		return validIPv6Literal(lit[5:])
	}
	return validIPv4Literal(lit)
}

// IPv4-address-literal = Snum 3("." Snum); Snum = 1*3DIGIT, value 0-255.
func validIPv4Literal(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if len(p) < 1 || len(p) > 3 {
			return false
		}
		n := 0
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				return false
			}
			n = n*10 + int(p[i]-'0')
		}
		if n > 255 {
			return false
		}
	}
	return true
}

// IPv6-addr per RFC 5321 section 4.1.3 as published: full forms have 8
// groups (6 + IPv4); in compressed forms "::" stands for at least two groups,
// so at most 6 explicit groups (4 with an IPv4 tail).
func validIPv6Literal(s string) bool {
	head, tail, compressed := strings.Cut(s, "::")
	if compressed && strings.Contains(tail, "::") {
		return false
	}
	ipv4 := false
	groups := func(part string, allowV4 bool) (int, bool) {
		if part == "" {
			return 0, true
		}
		fields := strings.Split(part, ":")
		n := 0
		for i, f := range fields {
			if allowV4 && i == len(fields)-1 && strings.Contains(f, ".") {
				if !validIPv4Literal(f) {
					return 0, false
				}
				ipv4 = true
				n += 2
				continue
			}
			if len(f) < 1 || len(f) > 4 {
				return 0, false
			}
			for j := 0; j < len(f); j++ {
				c := f[j]
				if _, ok := hex4([]byte{'0', '0', '0', c}); !ok {
					return 0, false
				}
			}
			n++
		}
		return n, true
	}
	if !compressed {
		n, ok := groups(s, true)
		return ok && n == 8
	}
	h, ok1 := groups(head, false)
	t, ok2 := groups(tail, true)
	if !ok1 || !ok2 {
		return false
	}
	explicit := h + t
	if ipv4 {
		return explicit-2 <= 4
	}
	return explicit <= 6
}

// contextORules applies the RFC 5892 Appendix A.3-A.9 CONTEXTO rules, which
// x/net/idna does not implement. CONTEXTJ (A.1, A.2) is checked by the idna
// profile. Script lookups use the Go standard library Unicode tables.
func contextORules(label string) bool {
	r := []rune(label)
	hasArabicIndic, hasExtArabicIndic, hasKanaHan := false, false, false
	for _, c := range r {
		switch {
		case c >= 0x0660 && c <= 0x0669:
			hasArabicIndic = true
		case c >= 0x06F0 && c <= 0x06F9:
			hasExtArabicIndic = true
		}
		if c != 0x30FB && (unicode.Is(unicode.Hiragana, c) || unicode.Is(unicode.Katakana, c) || unicode.Is(unicode.Han, c)) {
			hasKanaHan = true
		}
	}
	if hasArabicIndic && hasExtArabicIndic { // A.8, A.9
		return false
	}
	for i, c := range r {
		switch c {
		case 0x00B7: // A.3 MIDDLE DOT: between two "l"
			if i == 0 || i == len(r)-1 || r[i-1] != 'l' || r[i+1] != 'l' {
				return false
			}
		case 0x0375: // A.4 GREEK KERAIA: followed by Greek
			if i == len(r)-1 || !unicode.Is(unicode.Greek, r[i+1]) {
				return false
			}
		case 0x05F3, 0x05F4: // A.5, A.6 HEBREW GERESH/GERSHAYIM: preceded by Hebrew
			if i == 0 || !unicode.Is(unicode.Hebrew, r[i-1]) {
				return false
			}
		case 0x30FB: // A.7 KATAKANA MIDDLE DOT: label has Hiragana, Katakana or Han
			if !hasKanaHan {
				return false
			}
		}
	}
	return true
}
