package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/3leaps/canonprims/canon"
	"github.com/spf13/cobra"
)

func runCanonTest(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	root := newRootCommand()
	root.PersistentPreRun = func(*cobra.Command, []string) { t.Fatal("canon ran package-manager/logger startup hook") }
	root.AddCommand(newCanonCommand())
	var out, diagnostic bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&diagnostic)
	root.SetIn(strings.NewReader(input))
	root.SetArgs(append([]string{"canon"}, args...))
	err := root.Execute()
	if diagnostic.Len() != 0 {
		t.Fatalf("unexpected Cobra diagnostics: %s", diagnostic.String())
	}
	return out.String(), err
}

func TestCanonCanonicalBytes(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{`{"b": 2, "a": [1.0, 1e21]}`, `{"a":[1,1e+21],"b":2}`},
		{`[-0,1e-7,9007199254740992,18446744073709551616]`, `[0,1e-7,9007199254740992,18446744073709552000]`},
		{`{"\ue000":1,"\ud800\udc00":2}`, "{\"\U00010000\":2,\"\ue000\":1}"},
		{`"<é>"`, `"<é>"`},
	} {
		got, err := runCanonTest(t, tt.input, "jcs")
		if err != nil || got != tt.want {
			t.Fatalf("input %s: got %q, want %q; err=%v", tt.input, got, tt.want, err)
		}
	}
}

func TestCanonRefusals(t *testing.T) {
	for _, tt := range []struct {
		name, input, code string
		flags             []string
	}{
		{"invalid", `{`, "invalid_json", nil},
		{"utf8", string([]byte{'"', 0xff, '"'}), "invalid_utf8", nil},
		{"duplicate", `{"a":1,"a":2}`, "duplicate_member", nil},
		{"surrogate", `"\ud800"`, "lone_surrogate", nil},
		{"noncharacter", `"\uffff"`, "noncharacter", nil},
		{"infinite", `1e400`, "non_finite_number", nil},
		{"integer", `9007199254740993`, "integer_outside_binary64", nil},
		{"bytes", `null`, "too_large", []string{"--max-bytes=3"}},
		{"depth", `[[0]]`, "too_deep", []string{"--max-depth=1"}},
		{"members", `{"a":1,"b":2}`, "too_large", []string{"--max-object-members=1"}},
		{"string", `"é"`, "too_large", []string{"--max-string-bytes=1"}},
		{"canonical", "{}\n", "not_canonical", []string{"--check"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runCanonTest(t, tt.input, append([]string{"jcs"}, tt.flags...)...)
			var ce *canonCommandError
			if out != "" || !errors.As(err, &ce) || ce.code != 1 || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

func TestCanonAllNoncharacters(t *testing.T) {
	var values []rune
	for r := rune(0xfdd0); r <= 0xfdef; r++ {
		values = append(values, r)
	}
	for plane := 0; plane <= 16; plane++ {
		values = append(values, rune(plane<<16|0xfffe), rune(plane<<16|0xffff))
	}
	for _, r := range values {
		out, err := runCanonTest(t, "\""+string(r)+"\"", "jcs")
		if out != "" || err == nil || !strings.Contains(err.Error(), "noncharacter") {
			t.Fatalf("U+%X: out=%q err=%v", r, out, err)
		}
	}
}

func TestCanonVectorAndDigest(t *testing.T) {
	const input = `{"b":2,"a":"é<"}`
	const canonical = `{"a":"é<","b":2}`
	sum := sha256.Sum256([]byte(canonical))
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	digest, err := runCanonTest(t, input, "jcs", "--digest")
	if err != nil || digest != wantDigest+"\n" {
		t.Fatalf("digest=%q err=%v", digest, err)
	}
	vector, err := runCanonTest(t, input, "jcs", "--vector")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(vector, "\n") {
		t.Fatal("vector lacks newline")
	}
	if err := canon.Check(canon.JCS, []byte(strings.TrimSuffix(vector, "\n")), canon.DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Length    int    `json:"byte_length"`
		Canonical string `json:"canonical"`
		Digest    string `json:"digest"`
		Scheme    string `json:"scheme"`
	}
	if err := json.Unmarshal([]byte(vector), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Length != len(canonical) || decoded.Canonical != canonical || decoded.Digest != wantDigest || decoded.Scheme != string(canon.JCS) {
		t.Fatalf("vector=%s", vector)
	}
	info, err := runCanonTest(t, "", "info")
	if err != nil || !strings.Contains(info, canon.CorpusDigest) {
		t.Fatalf("info=%q err=%v", info, err)
	}
	if err := canon.Check(canon.JCS, []byte(strings.TrimSuffix(info, "\n")), canon.DefaultLimits()); err != nil {
		t.Fatal(err)
	}
}

func TestCanonNDJSONAndSets(t *testing.T) {
	for _, tt := range []struct {
		name, input, want, refusal string
		args                       []string
	}{
		{"ndjson", "{\"b\":2,\"a\":1}\n0\n", `[{"a":1,"b":2},0]`, "", []string{"--ndjson-array"}},
		{"empty stream", "", `[]`, "", []string{"--ndjson-array"}},
		{"exact large integer", "18446744073709551616\n", `[18446744073709552000]`, "", []string{"--ndjson-array"}},
		{"zero depth", "", "", "too_deep", []string{"--ndjson-array", "--max-depth=0"}},
		{"prefix", "0\ninvalid tail", `[0]`, "", []string{"--ndjson-array", "--head=1", "--max-bytes=2"}},
		{"short prefix", "0\n", "", "invalid_json", []string{"--ndjson-array", "--head=2"}},
		{"unterminated", "0", "", "invalid_json", []string{"--ndjson-array"}},
		{"blank", "0\n\n", "", "invalid_json", []string{"--ndjson-array"}},
		{"crlf", "0\r\n", "", "invalid_json", []string{"--ndjson-array"}},
		{"wrapper depth", "[0]\n", "", "too_deep", []string{"--ndjson-array", "--max-depth=1"}},
		{"input bound", "0\n1\n", "", "too_large", []string{"--ndjson-array", "--max-bytes=3"}},
		{"set", `[{"b":2},{"a":1}]`, `[{"a":1},{"b":2}]`, "", []string{"--sort-set"}},
		{"duplicate set", `[{"a":1,"b":2},{"b":2,"a":1}]`, "", "duplicate_set_element", []string{"--sort-set"}},
		{"not array", `{}`, "", "invalid_json", []string{"--sort-set"}},
		{"numeric byte order", `[2,10]`, `[10,2]`, "", []string{"--sort-set"}},
		{"sorted check", `[10,2]`, "", "", []string{"--sort-set", "--check"}},
		{"unsorted check", `[2,10]`, "", "not_canonical", []string{"--sort-set", "--check"}},
		{"ndjson set", "2\n10\n", `[10,2]`, "", []string{"--ndjson-array", "--sort-set"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runCanonTest(t, tt.input, append([]string{"jcs"}, tt.args...)...)
			if tt.refusal == "" {
				if err != nil || out != tt.want {
					t.Fatalf("out=%q err=%v", out, err)
				}
			} else if out != "" || err == nil || !strings.Contains(err.Error(), tt.refusal) {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

func TestCanonUsageAndFiles(t *testing.T) {
	for _, args := range [][]string{
		{}, {"bad"}, {"jcs", "--check", "--digest"}, {"jcs", "--digest", "--vector"}, {"jcs", "--check", "--vector"},
		{"jcs", "--check", "--ndjson-array"}, {"jcs", "--head=1"}, {"jcs", "--ndjson-array", "--head=0"},
		{"jcs", "--max-depth=-1"}, {"jcs", "--max-bytes=bad"}, {"jcs", "--unknown"}, {"jcs", "a", "b"}, {"info", "unknown"},
	} {
		out, err := runCanonTest(t, "{}", args...)
		var ce *canonCommandError
		if out != "" || !errors.As(err, &ce) || ce.code != 2 {
			t.Fatalf("args=%v out=%q err=%v", args, out, err)
		}
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte("{ }"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := runCanonTest(t, "ignored", "jcs", path)
	if err != nil || out != "{}" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	out, err = runCanonTest(t, "{}", "jcs", path+"-missing")
	var ce *canonCommandError
	if out != "" || !errors.As(err, &ce) || ce.code != 4 {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

type canonFaultWriter struct {
	err   error
	short bool
}

func (w canonFaultWriter) Write(b []byte) (int, error) {
	if w.short {
		return 0, nil
	}
	return 0, w.err
}

type canonFaultReader struct{}

func (canonFaultReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestCanonIOErrors(t *testing.T) {
	for _, tt := range []struct {
		writer io.Writer
		code   int
	}{
		{canonFaultWriter{err: syscall.EPIPE}, 0}, {canonFaultWriter{err: io.ErrClosedPipe}, 5}, {canonFaultWriter{short: true}, 5},
	} {
		err := (canonOptions{limits: canon.DefaultLimits()}).run(strings.NewReader("{}"), tt.writer)
		if tt.code == 0 && err != nil {
			t.Fatal(err)
		}
		if tt.code != 0 {
			var ce *canonCommandError
			if !errors.As(err, &ce) || ce.code != tt.code {
				t.Fatalf("err=%v", err)
			}
		}
	}
	for _, ndjson := range []bool{false, true} {
		var out bytes.Buffer
		err := (canonOptions{limits: canon.DefaultLimits(), ndjson: ndjson}).run(canonFaultReader{}, &out)
		var ce *canonCommandError
		if out.Len() != 0 || !errors.As(err, &ce) || ce.code != 4 {
			t.Fatalf("out=%q err=%v", out.String(), err)
		}
	}
}

func TestCanonProcessExitCodes(t *testing.T) {
	if os.Getenv("GONEAT_CANON_PROCESS_TEST") == "1" {
		rootCmd = newRootCommand()
		rootCmd.AddCommand(newCanonCommand())
		for i, arg := range os.Args {
			if arg == "--" {
				rootCmd.SetArgs(os.Args[i+1:])
				break
			}
		}
		Execute()
		os.Exit(0)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		args       []string
		input, out string
		code       int
	}{
		{[]string{"canon", "jcs"}, `{"b":2,"a":1}`, `{"a":1,"b":2}`, 0},
		{[]string{"canon", "jcs", "--check"}, "{}\n", "", 1},
		{[]string{"canon", "jcs", "--head=0"}, "{}", "", 2},
		{[]string{"canon", "jcs", "--unknown"}, "{}", "", 2},
		{[]string{"canon", "bad"}, "", "", 2},
		{[]string{"canon", "jcs", filepath.Join(t.TempDir(), "absent")}, "", "", 4},
	} {
		cmd := exec.Command(exe, append([]string{"-test.run=^TestCanonProcessExitCodes$", "--"}, tt.args...)...)
		cmd.Env = append(os.Environ(), "GONEAT_CANON_PROCESS_TEST=1")
		cmd.Stdin = strings.NewReader(tt.input)
		var out, diagnostic bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &diagnostic
		err := cmd.Run()
		code := 0
		if err != nil {
			var ee *exec.ExitError
			if !errors.As(err, &ee) {
				t.Fatal(err)
			}
			code = ee.ExitCode()
		}
		if code != tt.code || out.String() != tt.out || (tt.code == 0 && diagnostic.Len() != 0) {
			t.Fatalf("%v code=%d stdout=%q stderr=%q err=%v", tt.args, code, out.String(), diagnostic.String(), err)
		}
	}
}
