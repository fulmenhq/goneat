package finalizer

import (
	"bytes"
	"testing"
)

func TestNormalizeEOF_PreserveMarkdownHardBreaks(t *testing.T) {
	in := []byte("Hello  \nWorld   \nLine\t\n")
	out, changed, err := NormalizeEOF(in, true, true, true, "\n", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes, got unchanged")
	}
	// Expect exactly two trailing spaces preserved where 2+ existed
	want := []byte("Hello  \nWorld  \nLine\n")
	if !bytes.Equal(out, want) {
		t.Fatalf("unexpected output:\nwant=%q\n got=%q", string(want), string(out))
	}
}

func TestComprehensiveFileNormalization_EncodingPolicy_Utf8Only_SkipsNonUtf8(t *testing.T) {
	// UTF-16LE BOM + 'A' (0x41) + NUL (0x00) sequence -> non-UTF8 content
	in := []byte{0xFF, 0xFE, 0x41, 0x00}
	opts := NormalizationOptions{
		EnsureEOF:              true,
		TrimTrailingWhitespace: true,
		EncodingPolicy:         "utf8-only",
	}
	out, changed, err := ComprehensiveFileNormalization(in, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatalf("expected no changes for non-UTF8 with utf8-only policy")
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("content should be unchanged for non-UTF8 inputs")
	}
}

func TestComprehensiveFileNormalization_EncodingPolicy_Utf8OrBOM_AllowsUtf8BOM(t *testing.T) {
	in := []byte{0xEF, 0xBB, 0xBF, 'a', '\n'} // UTF-8 BOM + content
	opts := NormalizationOptions{
		EnsureEOF:              true,
		TrimTrailingWhitespace: true,
		RemoveUTF8BOM:          true,
		EncodingPolicy:         "utf8-or-bom",
	}
	out, changed, err := ComprehensiveFileNormalization(in, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes due to BOM removal or normalization")
	}
	// BOM should be removed
	if bytes.HasPrefix(out, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("expected UTF-8 BOM to be removed")
	}
}
