package schemaengine

import (
	"testing"
	"unicode"
)

// ownedIDNEmailCorpusSHA256 pins owned-idn-email-corpus/v2. Expected verdicts
// for U-labels come from a strict IDNA2008 oracle (Python idna 3.20).
const ownedIDNEmailCorpusSHA256 = "29879517705afb3e49703cb02a911d1196ddffce4112b225f332c6cf0a9f138d"

// idnaNV8Exceptions are cases where the strict IDNA2008 oracle refuses a
// U-label that UTS 46 registration validation (x/net/idna) accepts: the
// characters are NV8 (valid under UTS 46, DISALLOWED under IDNA2008).
// Goneat names its domain check as UTS 46 registration validation, so these
// are accepted and listed rather than hidden.
var idnaNV8Exceptions = map[string]string{
	"idn-email-00699": "U+2603 SNOWMAN is NV8",
	"idn-email-00701": "U+1F600 emoji is NV8",
	"idn-email-00703": "U+2764 HEAVY BLACK HEART is NV8",
	"idn-email-00705": "U+20AC EURO SIGN is NV8",
	"idn-email-00707": "U+2665 BLACK HEART SUIT is NV8",
	"idn-email-00709": "U+2713 CHECK MARK is NV8",
	"idn-email-00711": "U+00A9 COPYRIGHT SIGN is NV8",
	"idn-email-00713": "U+2010 HYPHEN is NV8",
	"idn-email-02725": "U+1F600 emoji is NV8",
	"idn-email-02727": "U+1F600 emoji is NV8",
}

func TestOwnedEmailFormatsCorpus(t *testing.T) {
	cases := loadFormatCorpus(t, "testdata/owned-idn-email-corpus-v2.json", ownedIDNEmailCorpusSHA256)
	seen := map[string]bool{}
	for _, c := range cases {
		check := checkEmail
		switch c.Format {
		case "email":
		case "idn-email":
			check = checkIDNEmail
		default:
			t.Fatalf("%s: unexpected format %q", c.ID, c.Format)
		}
		got := check(c.Instance) == nil
		if reason, ok := idnaNV8Exceptions[c.ID]; ok {
			seen[c.ID] = true
			if c.Valid || !got {
				t.Errorf("%s (%s): listed exception no longer differs: want=%v got=%v", c.ID, reason, c.Valid, got)
			}
			continue
		}
		if got != c.Valid {
			t.Errorf("%s %s %+q (%s): got valid=%v, want %v", c.ID, c.Format, c.Instance, c.Rule, got, c.Valid)
		}
	}
	for id := range idnaNV8Exceptions {
		if !seen[id] {
			t.Errorf("exception %s is not in the corpus", id)
		}
	}
	if len(cases) != 2884 {
		t.Errorf("corpus case count = %d, want 2884", len(cases))
	}
}

// The IDNA tables come from x/net/idna and depend on the Go toolchain (Unicode
// 15.0.0 before go1.27, 17.0.0 from go1.27). A change alters idn-email
// verdicts, so it must be reviewed rather than picked up silently.
func TestIDNAUnicodeVersionPinned(t *testing.T) {
	if IDNAUnicodeVersion != "15.0.0" {
		t.Fatalf("IDNA tables are Unicode %s; re-qualify idn-email against the corpus and update this pin", IDNAUnicodeVersion)
	}
}

func TestIDNEmailAcceptsEveryEmail(t *testing.T) {
	cases := loadFormatCorpus(t, "testdata/owned-idn-email-corpus-v2.json", ownedIDNEmailCorpusSHA256)
	for _, c := range cases {
		if c.Format == "email" && checkEmail(c.Instance) == nil && checkIDNEmail(c.Instance) != nil {
			t.Errorf("%s: valid email %+q rejected as idn-email", c.ID, c.Instance)
		}
	}
}

// Go's standard-library Unicode tables back the owned CONTEXTO script checks.
func TestGoUnicodeVersionPinned(t *testing.T) {
	if unicode.Version != "15.0.0" {
		t.Fatalf("Go unicode tables are %s; re-qualify the CONTEXTO rules and update this pin", unicode.Version)
	}
}

// Positive and negative boundaries for each owned RFC 5892 CONTEXTO rule and
// the code-point "--" rule, checked through the full idn-email path.
func TestOwnedContextORuleBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name, domain string
		ok           bool
	}{
		{"A.3 middle dot between l", "l·l", true},
		{"A.3 middle dot in word", "col·lecció", true},
		{"A.3 middle dot leading", "·l", false},
		{"A.3 middle dot trailing", "l·", false},
		{"A.3 middle dot after other letter", "a·l", false},
		{"A.3 middle dot before other letter", "l·a", false},
		{"A.3 middle dot uppercase L", "L·l", false},
		{"A.4 keraia before Greek", "͵αβ", true},
		{"A.4 keraia before Latin", "͵ab", false},
		{"A.4 keraia trailing", "α͵", false},
		{"A.5 geresh after Hebrew", "א׳", true},
		{"A.5 geresh leading", "׳א", false},
		{"A.5 geresh after Latin", "a׳", false},
		{"A.6 gershayim after Hebrew", "א״ב", true},
		{"A.6 gershayim leading", "״א", false},
		{"A.7 katakana dot with katakana", "ア・イ", true},
		{"A.7 katakana dot with hiragana", "あ・", true},
		{"A.7 katakana dot with han", "漢・", true},
		{"A.7 katakana dot alone", "・", false},
		{"A.7 katakana dot with latin", "a・b", false},
		{"A.8 arabic-indic digits", "ب٠١", true},
		{"A.9 extended arabic-indic digits", "ب۰۱", true},
		{"A.8/A.9 mixed digit sets", "ب٠۱", false},
		{"hyphen 3-4 code points after multi-byte", "mü--nchen", false},
		{"hyphen 3-4 ascii prefix U-label", "ab--ü", false},
		{"hyphen 2-3 allowed", "mü-n-chen", true},
		{"hyphen 4-5 allowed", "mün--chen", true},
		{"hyphen at code points 2-3 after multi-byte (byte offsets 2-3)", "ü--ab", true},
		{"hyphen 3-4 after two multi-byte", "üb--x", false},
		{"leading hyphen U-label", "-ü", false},
		{"trailing hyphen U-label", "ü-", false},
		{"single hyphen U-label interior", "ü-b", true},
	} {
		got := checkIDNEmail("user@"+tt.domain+".example") == nil
		if got != tt.ok {
			t.Errorf("%s: %+q valid=%v, want %v", tt.name, tt.domain, got, tt.ok)
		}
	}
}
