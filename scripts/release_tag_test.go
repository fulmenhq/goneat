package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for release-tag.sh. They generate throwaway keys in a temporary gpg
// homedir and use a local bare repository as origin; no network and no real
// signing key are involved. The fixture repository carries copies of the
// release scripts and its own committed pin, anchors (derived by decernor) and
// tagger identity. Skipped when git, gpg or decernor is unavailable, and
// failed instead when GONEAT_REQUIRE_CRYPTO_TOOLS=1.

const (
	testTag         = "v1.2.3"
	testTaggerName  = "Release Tagger"
	testTaggerEmail = "tagger@example.test"
)

type releaseKeys struct {
	home    string
	primary string
	signA   string
	signB   string
	foreign string
}

// releaseTagTools are the external tools the release-tag tests need.
var releaseTagTools = []string{"git", "gpg", "gpgconf", "bash", "python3"}

// releaseTagScripts are copied into each fixture repository and committed, so
// the scripts operate on the fixture exactly as they do on a goneat checkout.
var releaseTagScripts = []string{
	"release-tag.sh",
	"release-decernor.sh",
	"release-verify-anchors.sh",
	"validate-release-anchors.sh",
	"verify-pinned-tag.sh",
	"release-insert-anchors.sh",
	"install-release-anchors.sh",
}

// testDecernor locates the checker for the tests. The release scripts never
// search PATH; the tests may, and pass the resolved absolute path explicitly.
func testDecernor(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("GONEAT_DECERNOR_BIN")
	if bin == "" {
		found, err := exec.LookPath("decernor")
		if err != nil {
			if os.Getenv("GONEAT_REQUIRE_CRYPTO_TOOLS") == "1" {
				t.Fatalf("decernor unavailable and GONEAT_REQUIRE_CRYPTO_TOOLS=1 (set GONEAT_DECERNOR_BIN)")
			}
			t.Skip("decernor unavailable (set GONEAT_DECERNOR_BIN)")
		}
		bin = found
	}
	resolved, err := filepath.EvalSymlinks(bin)
	if err != nil {
		t.Fatalf("resolve decernor: %v", err)
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		t.Fatalf("Abs decernor: %v", err)
	}
	return abs
}

// requireTools skips the test when a tool is missing, or fails it when
// GONEAT_REQUIRE_CRYPTO_TOOLS=1 (set by the CI job that must run these tests).
func requireTools(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			if os.Getenv("GONEAT_REQUIRE_CRYPTO_TOOLS") == "1" {
				t.Fatalf("%s unavailable and GONEAT_REQUIRE_CRYPTO_TOOLS=1: %v", tool, err)
			}
			t.Skipf("%s unavailable: %v", tool, err)
		}
	}
}

func run(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	out, err := runErr(dir, env, name, args...)
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return out
}

func runErr(dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// cleanEnv drops ambient GONEAT_*, GNUPGHOME, and GIT_* variables so the
// operator's release environment can never leak into a test.
func cleanEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		key := kv[:strings.IndexByte(kv, '=')]
		if strings.HasPrefix(key, "GONEAT_") || strings.HasPrefix(key, "GIT_") || key == "GNUPGHOME" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
}

func fingerprints(t *testing.T, home, selector string) []string {
	t.Helper()
	out := run(t, "", cleanEnv(), "gpg", "--homedir", home, "--batch", "--with-colons", "--fixed-list-mode", "--list-keys", selector)
	var fprs []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) > 9 && fields[0] == "fpr" {
			fprs = append(fprs, fields[9])
		}
	}
	return fprs
}

func newReleaseKeys(t *testing.T) releaseKeys {
	t.Helper()
	// Short path: gpg-agent sockets live in the homedir and macOS limits
	// socket paths to ~104 bytes, which t.TempDir() can exceed.
	home, err := os.MkdirTemp("/tmp", "rtag-gpg-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() {
		_, _ = runErr("", cleanEnv(), "gpgconf", "--homedir", home, "--kill", "all")
		_ = os.RemoveAll(home)
	})
	gpg := func(args ...string) {
		base := []string{"--homedir", home, "--batch", "--pinentry-mode", "loopback", "--passphrase", ""}
		run(t, "", cleanEnv(), "gpg", append(base, args...)...)
	}

	gpg("--quick-gen-key", testTaggerName+" <"+testTaggerEmail+">", "ed25519", "cert", "never")
	primary := fingerprints(t, home, testTaggerEmail)[0]
	gpg("--quick-add-key", primary, "ed25519", "sign", "never")
	gpg("--quick-add-key", primary, "ed25519", "sign", "never")
	fprs := fingerprints(t, home, primary)
	if len(fprs) != 3 {
		t.Fatalf("expected primary + 2 subkeys, got %v", fprs)
	}
	gpg("--quick-gen-key", "Foreign Signer <foreign@example.test>", "ed25519", "sign", "never")
	foreign := fingerprints(t, home, "foreign@example.test")[0]

	return releaseKeys{home: home, primary: fprs[0], signA: fprs[1], signB: fprs[2], foreign: foreign}
}

type releaseRepo struct {
	t        *testing.T
	keys     releaseKeys
	origin   string
	clone    string
	script   string
	decernor string
	vars     map[string]string
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// syntheticMinisignPub is a structurally valid minisign public key for tests.
func syntheticMinisignPub(fill byte) []byte {
	blob := append([]byte("Ed"), bytes.Repeat([]byte{fill}, 40)...)
	return []byte("untrusted comment: synthetic public key\n" + base64.StdEncoding.EncodeToString(blob) + "\n")
}

func newReleaseRepo(t *testing.T, keys releaseKeys) *releaseRepo {
	t.Helper()
	decernor := testDecernor(t)
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	clone := filepath.Join(root, "clone")
	env := cleanEnv()
	run(t, root, env, "git", "init", "--quiet", "--bare", "--initial-branch=main", origin)
	run(t, root, env, "git", "clone", "--quiet", origin, clone)
	run(t, clone, env, "git", "config", "user.name", "Ambient Committer")
	run(t, clone, env, "git", "config", "user.email", "ambient@example.test")
	run(t, clone, env, "git", "config", "commit.gpgsign", "false")
	run(t, clone, env, "git", "checkout", "--quiet", "-b", "main")
	writeFile(t, filepath.Join(clone, "VERSION"), []byte(testTag+"\n"), 0o600)
	for _, name := range releaseTagScripts {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		writeFile(t, filepath.Join(clone, "scripts", name), data, 0o755)
	}
	// The committed pin: the primary plus exactly the signA signing subkey.
	pin := run(t, "", cleanEnv(), "gpg", "--homedir", keys.home, "--batch", "--armor",
		"--export-options", "export-minimal", "--export", keys.signA+"!")
	writeFile(t, filepath.Join(clone, "docs", "security", "release-signing-keys.asc"), []byte(pin), 0o644)
	writeFile(t, filepath.Join(clone, "docs", "security", "goneat-minisign.pub"), syntheticMinisignPub(7), 0o644)
	writeFile(t, filepath.Join(clone, "config", "release", "tagger-identity.txt"),
		[]byte(testTaggerName+" <"+testTaggerEmail+">\n"), 0o644)
	run(t, clone, env, "git", "add", "-A")
	run(t, clone, env, "git", "commit", "--quiet", "-m", "init")
	run(t, clone, append(cleanEnv(), "GONEAT_DECERNOR_BIN="+decernor), "bash", "scripts/release-insert-anchors.sh")
	run(t, clone, env, "git", "add", "keys")
	run(t, clone, env, "git", "commit", "--quiet", "-m", "anchors")
	run(t, clone, env, "git", "push", "--quiet", "origin", "main")

	return &releaseRepo{
		t: t, keys: keys, origin: origin, clone: clone,
		script: filepath.Join(clone, "scripts", "release-tag.sh"), decernor: decernor,
		vars: map[string]string{
			"GONEAT_RELEASE_TAG":  testTag,
			"GONEAT_DECERNOR_BIN": decernor,
			"GONEAT_PGP_KEY_ID":   keys.signA + "!",
			"GONEAT_GPG_HOMEDIR":  keys.home,
			"GONEAT_TAGGER_NAME":  testTaggerName,
			"GONEAT_TAGGER_EMAIL": testTaggerEmail,
		},
	}
}

// commitAndPush records a change to the fixture on main and origin/main.
func (r *releaseRepo) commitAndPush(message string) {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "--quiet", "-m", message)
	r.git("push", "--quiet", "origin", "main")
}

func (r *releaseRepo) env() []string {
	env := cleanEnv()
	for k, v := range r.vars {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func (r *releaseRepo) runScript(cmd string) (string, error) {
	return runErr(r.clone, r.env(), "bash", r.script, cmd)
}

func (r *releaseRepo) git(args ...string) string {
	r.t.Helper()
	return run(r.t, r.clone, cleanEnv(), "git", args...)
}

func (r *releaseRepo) localTagExists() bool {
	return strings.TrimSpace(r.git("tag", "--list", testTag)) != ""
}

func (r *releaseRepo) remoteTagExists() bool {
	return strings.TrimSpace(r.git("ls-remote", "--tags", "origin", "refs/tags/"+testTag)) != ""
}

// signWith creates the release tag by hand, signed by an arbitrary key with the
// declared tagger identity, to exercise verify against the wrong signer.
func (r *releaseRepo) signWith(keySelector string) {
	r.t.Helper()
	r.signTagWith(testTag, keySelector)
}

func (r *releaseRepo) signTagWith(tag, keySelector string) {
	r.t.Helper()
	env := append(cleanEnv(),
		"GNUPGHOME="+r.keys.home,
		"GIT_COMMITTER_NAME="+testTaggerName,
		"GIT_COMMITTER_EMAIL="+testTaggerEmail,
	)
	run(r.t, r.clone, env, "git", "-c", "gpg.program=gpg", "tag", "-s", "-u", keySelector, tag, "-m", "Release "+tag)
}

// advanceOrigin pushes a new commit to origin from a second clone, leaving the
// release clone behind origin/main.
func (r *releaseRepo) advanceOrigin() {
	r.t.Helper()
	other := filepath.Join(r.t.TempDir(), "other")
	env := cleanEnv()
	run(r.t, "", env, "git", "clone", "--quiet", r.origin, other)
	run(r.t, other, env, "git", "-c", "user.name=Other", "-c", "user.email=other@example.test",
		"commit", "--quiet", "--allow-empty", "-m", "moved on")
	run(r.t, other, env, "git", "push", "--quiet", "origin", "main")
}

func expectFail(t *testing.T, out string, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected failure containing %q, got success:\n%s", want, out)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("expected output containing %q, got:\n%s", want, out)
	}
	if strings.Contains(out, "verified: ") || strings.Contains(out, "created signed local tag") {
		t.Fatalf("failure path printed a success line:\n%s", out)
	}
}

func TestReleaseTagGuards(t *testing.T) {
	requireTools(t, releaseTagTools...)
	keys := newReleaseKeys(t)

	for _, name := range []string{"GONEAT_RELEASE_TAG", "GONEAT_DECERNOR_BIN", "GONEAT_PGP_KEY_ID", "GONEAT_GPG_HOMEDIR", "GONEAT_TAGGER_NAME", "GONEAT_TAGGER_EMAIL"} {
		t.Run("missing "+name, func(t *testing.T) {
			r := newReleaseRepo(t, keys)
			r.vars[name] = ""
			out, err := r.runScript("create")
			expectFail(t, out, err, name+" is required")
			if r.localTagExists() {
				t.Fatal("tag created despite missing variable")
			}
		})
	}

	cases := []struct {
		name  string
		setup func(r *releaseRepo)
		want  string
	}{
		{"tag does not match VERSION", func(r *releaseRepo) { r.vars["GONEAT_RELEASE_TAG"] = "v9.9.9" }, "does not match VERSION"},
		{"dirty tree", func(r *releaseRepo) {
			if err := os.WriteFile(filepath.Join(r.clone, "VERSION"), []byte(testTag+"\n\n"), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
		}, "working tree is not clean"},
		{"not on main", func(r *releaseRepo) { r.git("checkout", "--quiet", "-b", "other") }, "release tags are created from main"},
		{"HEAD ahead of origin/main", func(r *releaseRepo) { r.git("commit", "--quiet", "--allow-empty", "-m", "local only") }, "does not match origin/main"},
		{"tag exists locally", func(r *releaseRepo) { r.git("tag", testTag) }, "already exists locally"},
		{"tag exists on origin", func(r *releaseRepo) {
			r.git("push", "--quiet", "origin", "HEAD:refs/tags/"+testTag)
		}, "already exists on origin"},
		{"tagger email not a uid", func(r *releaseRepo) { r.vars["GONEAT_TAGGER_EMAIL"] = "someone@example.test" }, "not a uid email"},
		{"short key id rejected", func(r *releaseRepo) { r.vars["GONEAT_PGP_KEY_ID"] = keys.signA[32:] }, "must be a 40-hex fingerprint or 16-hex long key id"},
		{"email selector rejected", func(r *releaseRepo) { r.vars["GONEAT_PGP_KEY_ID"] = testTaggerEmail }, "must be a 40-hex fingerprint or 16-hex long key id"},
		{"unknown key", func(r *releaseRepo) { r.vars["GONEAT_PGP_KEY_ID"] = strings.Repeat("A", 40) }, "does not match any key"},
		{"unforced subkey rejected", func(r *releaseRepo) { r.vars["GONEAT_PGP_KEY_ID"] = keys.signA }, "names a subkey"},
		{"relative checker path", func(r *releaseRepo) { r.vars["GONEAT_DECERNOR_BIN"] = "decernor" }, "absolute executable regular file"},
		{"unprefixed checker variable ignored", func(r *releaseRepo) {
			r.vars["GONEAT_DECERNOR_BIN"] = ""
			r.vars["DECERNOR_BIN"] = r.decernor
		}, "GONEAT_DECERNOR_BIN is required"},
		{"tagger differs from committed identity", func(r *releaseRepo) { r.vars["GONEAT_TAGGER_NAME"] = "Someone Else" }, "do not match the committed tagger identity"},
		{"forced subkey is not the committed signing subkey", func(r *releaseRepo) { r.vars["GONEAT_PGP_KEY_ID"] = keys.signB + "!" }, "not the committed signing subkey"},
		{"committed anchors disagree with the pin", func(r *releaseRepo) {
			path := filepath.Join(r.clone, "keys", "expected-fingerprints.txt")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			lines := strings.SplitN(string(data), "\n", 2)
			writeFile(t, path, []byte(lines[0]+"\nminisign "+strings.Repeat("0", 64)+"\n"), 0o644)
			r.commitAndPush("edit anchors")
		}, "do not verify against their anchors"},
		{"committed pin replaced without new anchors", func(r *releaseRepo) {
			writeFile(t, filepath.Join(r.clone, "docs", "security", "goneat-minisign.pub"), syntheticMinisignPub(9), 0o644)
			r.commitAndPush("swap minisign pin")
		}, "do not verify against their anchors"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newReleaseRepo(t, keys)
			tc.setup(r)
			out, err := r.runScript("create")
			expectFail(t, out, err, tc.want)
			if tc.name != "tag exists locally" && tc.name != "tag exists on origin" && r.localTagExists() {
				t.Fatal("tag created despite failed guard")
			}
		})
	}

	t.Run("tagger email failure lists key uids", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		r.vars["GONEAT_TAGGER_EMAIL"] = "someone@example.test"
		out, _ := r.runScript("create")
		if !strings.Contains(out, testTaggerName+" <"+testTaggerEmail+">") {
			t.Fatalf("uid list missing from output:\n%s", out)
		}
	})

	t.Run("ambiguous selector", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		// A real 64-bit key id collision cannot be generated, so a gpg shim
		// reports two keys that share the selected long id.
		shimDir := t.TempDir()
		longID := keys.signA[24:]
		listing := strings.Join([]string{
			"pub:u:255:22:" + keys.primary[24:] + ":1:::u:::cC::::::23::0:",
			"fpr:::::::::" + keys.primary + ":",
			"sub:u:255:22:" + longID + ":1::::::s::::::23:",
			"fpr:::::::::" + keys.signA + ":",
			"pub:u:255:22:" + longID + ":1:::u:::scSC::::::23::0:",
			"fpr:::::::::" + strings.Repeat("B", 24) + longID + ":",
		}, "\n")
		shim := "#!/usr/bin/env bash\ncat <<'EOF'\n" + listing + "\nEOF\n"
		if err := os.WriteFile(filepath.Join(shimDir, "gpg"), []byte(shim), 0o700); err != nil {
			t.Fatalf("WriteFile shim: %v", err)
		}
		r.vars["GONEAT_PGP_KEY_ID"] = longID + "!"
		env := r.env()
		for i, kv := range env {
			if strings.HasPrefix(kv, "PATH=") {
				env[i] = "PATH=" + shimDir + ":" + kv[len("PATH="):]
			}
		}
		out, err := runErr(r.clone, env, "bash", r.script, "create")
		expectFail(t, out, err, "is ambiguous")
		if r.localTagExists() {
			t.Fatal("tag created despite ambiguous selector")
		}
	})
}

func TestReleaseTagSignatureChecks(t *testing.T) {
	requireTools(t, releaseTagTools...)
	keys := newReleaseKeys(t)

	t.Run("forced key rejects sibling subkey of same primary", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		r.signWith(keys.signB + "!")
		out, err := r.runScript("verify")
		expectFail(t, out, err, "not the selected key")
	})

	t.Run("forced key rejects foreign key", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		r.signWith(keys.foreign)
		out, err := r.runScript("verify")
		expectFail(t, out, err, "not the selected key")
	})

	t.Run("unforced primary rejects foreign key", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		r.vars["GONEAT_PGP_KEY_ID"] = keys.primary
		r.signWith(keys.foreign)
		out, err := r.runScript("verify")
		expectFail(t, out, err, "was signed under primary")
	})

	t.Run("operator-allowed subkey outside the pin fails the pinned check", func(t *testing.T) {
		// signB is a live signing subkey of the operator's primary, so the
		// operator keyring accepts it; the committed pin names only signA.
		r := newReleaseRepo(t, keys)
		r.vars["GONEAT_PGP_KEY_ID"] = keys.primary
		r.signWith(keys.signB + "!")
		out, err := r.runScript("verify")
		// The operator check passes (and prints its verified line); the
		// pinned check must then fail the command.
		if err == nil || !strings.Contains(out, "signature is invalid under the pin") ||
			!strings.Contains(out, "does not verify against the pin committed") {
			t.Fatalf("expected the pinned check to refuse, got err=%v:\n%s", err, out)
		}

		out, err = r.runScript("push")
		if err == nil || !strings.Contains(out, "does not verify against the pin committed") {
			t.Fatalf("expected push to refuse, got err=%v:\n%s", err, out)
		}
		if r.remoteTagExists() {
			t.Fatal("push published a tag signed outside the pin")
		}
	})

	t.Run("tampered tag fails", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		r.signWith(keys.signA + "!")
		obj := r.git("cat-file", "tag", testTag)
		tampered := strings.Replace(obj, "Release "+testTag, "Release v1.2.4", 1)
		cmd := exec.Command("git", "hash-object", "-t", "tag", "-w", "--stdin")
		cmd.Dir = r.clone
		cmd.Env = cleanEnv()
		cmd.Stdin = strings.NewReader(tampered)
		sha, err := cmd.Output()
		if err != nil {
			t.Fatalf("hash-object: %v", err)
		}
		r.git("update-ref", "refs/tags/"+testTag, strings.TrimSpace(string(sha)))
		out, err := r.runScript("verify")
		expectFail(t, out, err, "git verify-tag failed")

		out, err = r.runScript("push")
		expectFail(t, out, err, "git verify-tag failed")
		if r.remoteTagExists() {
			t.Fatal("push published a tag that failed verification")
		}
	})

	t.Run("verbatim message without trailing newline fails", func(t *testing.T) {
		// Regression: with --cleanup=verbatim and no trailing newline, git
		// appends the signature to the message line and cannot find it.
		r := newReleaseRepo(t, keys)
		env := append(cleanEnv(),
			"GNUPGHOME="+keys.home,
			"GIT_COMMITTER_NAME="+testTaggerName,
			"GIT_COMMITTER_EMAIL="+testTaggerEmail,
		)
		run(t, r.clone, env, "git", "-c", "gpg.program=gpg", "tag", "-s", "-u", keys.signA+"!",
			"--cleanup=verbatim", testTag, "-m", "Release "+testTag)
		if obj := r.git("cat-file", "-p", testTag); !strings.Contains(obj, "Release "+testTag+"-----BEGIN PGP SIGNATURE-----") {
			t.Fatalf("fixture did not reproduce the joined signature layout:\n%s", obj)
		}
		out, err := r.runScript("verify")
		expectFail(t, out, err, "git verify-tag failed")

		out, err = r.runScript("push")
		expectFail(t, out, err, "git verify-tag failed")
		if r.remoteTagExists() {
			t.Fatal("push published a tag that failed verification")
		}
	})

	t.Run("unsigned annotated tag fails", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		env := append(cleanEnv(), "GIT_COMMITTER_NAME="+testTaggerName, "GIT_COMMITTER_EMAIL="+testTaggerEmail)
		run(t, r.clone, env, "git", "tag", "-a", testTag, "-m", "Release "+testTag)
		out, err := r.runScript("verify")
		expectFail(t, out, err, "git verify-tag failed")
	})

	t.Run("lightweight tag fails", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		r.git("tag", testTag)
		out, err := r.runScript("verify")
		expectFail(t, out, err, "is not an annotated tag object")
	})

	t.Run("tag object for another version fails", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		r.signTagWith("v1.2.4", keys.signA+"!")
		r.git("update-ref", "refs/tags/"+testTag, strings.TrimSpace(r.git("rev-parse", "refs/tags/v1.2.4")))
		out, err := r.runScript("push")
		expectFail(t, out, err, "records a different tag name")
		if r.remoteTagExists() {
			t.Fatal("push published a tag that failed verification")
		}
	})
}

func TestReleaseTagPushGuards(t *testing.T) {
	requireTools(t, releaseTagTools...)
	keys := newReleaseKeys(t)

	created := func(t *testing.T) *releaseRepo {
		t.Helper()
		r := newReleaseRepo(t, keys)
		if out, err := r.runScript("create"); err != nil {
			t.Fatalf("create: %v\n%s", err, out)
		}
		return r
	}

	t.Run("origin already has the tag", func(t *testing.T) {
		r := created(t)
		r.git("push", "--quiet", "origin", "HEAD:refs/tags/"+testTag)
		out, err := r.runScript("push")
		expectFail(t, out, err, "already exists on origin")
	})

	t.Run("HEAD behind origin/main", func(t *testing.T) {
		r := created(t)
		r.advanceOrigin()
		out, err := r.runScript("push")
		expectFail(t, out, err, "does not match origin/main")
		if r.remoteTagExists() {
			t.Fatal("push published the tag from a stale main")
		}
	})

	t.Run("dirty tree", func(t *testing.T) {
		r := created(t)
		if err := os.WriteFile(filepath.Join(r.clone, "untracked"), []byte("x"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		out, err := r.runScript("push")
		expectFail(t, out, err, "working tree is not clean")
		if r.remoteTagExists() {
			t.Fatal("push published the tag from a dirty tree")
		}
	})

	t.Run("pushes only the release tag", func(t *testing.T) {
		r := created(t)
		r.git("tag", "stray-local-tag")
		before := r.git("ls-remote", "--heads", "origin")
		out, err := r.runScript("push")
		if err != nil {
			t.Fatalf("push: %v\n%s", err, out)
		}
		if !strings.Contains(out, "pushed "+testTag+" only") {
			t.Fatalf("push did not confirm the remote tag:\n%s", out)
		}
		if after := r.git("ls-remote", "--heads", "origin"); after != before {
			t.Fatalf("push moved branches:\nbefore %s\nafter %s", before, after)
		}
		tags := strings.TrimSpace(r.git("ls-remote", "--tags", "origin"))
		want := strings.TrimSpace(r.git("rev-parse", "refs/tags/"+testTag)) + "\trefs/tags/" + testTag + "\n" +
			strings.TrimSpace(r.git("rev-parse", "HEAD")) + "\trefs/tags/" + testTag + "^{}"
		if tags != want {
			t.Fatalf("origin tags = %q, want %q", tags, want)
		}
	})

	t.Run("remote override is ignored", func(t *testing.T) {
		r := created(t)
		decoy := filepath.Join(t.TempDir(), "decoy.git")
		run(t, "", cleanEnv(), "git", "init", "--quiet", "--bare", "--initial-branch=main", decoy)
		r.git("remote", "add", "decoy", decoy)
		r.vars["RELEASE_REMOTE"] = "decoy"
		r.vars["GONEAT_RELEASE_REMOTE"] = "decoy"
		out, err := r.runScript("push")
		if err != nil {
			t.Fatalf("push: %v\n%s", err, out)
		}
		if !r.remoteTagExists() {
			t.Fatal("tag was not pushed to origin")
		}
		if refs := strings.TrimSpace(r.git("ls-remote", "decoy")); refs != "" {
			t.Fatalf("push wrote to a remote other than origin:\n%s", refs)
		}
	})

	t.Run("push-to-remotes refuses a tag other than VERSION", func(t *testing.T) {
		r := created(t)
		pushScript, err := filepath.Abs("push-to-remotes.sh")
		if err != nil {
			t.Fatalf("Abs: %v", err)
		}
		out, err := runErr(r.clone, r.env(), "bash", pushScript, "v9.9.9")
		expectFail(t, out, err, "does not match VERSION")
		if r.remoteTagExists() {
			t.Fatal("push-to-remotes published a tag")
		}
	})
}

func TestReleaseTagHappyPath(t *testing.T) {
	requireTools(t, releaseTagTools...)
	keys := newReleaseKeys(t)

	t.Run("forced signing subkey: create, verify, push", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		out, err := r.runScript("create")
		if err != nil {
			t.Fatalf("create: %v\n%s", err, out)
		}
		if !strings.Contains(out, "signer:  "+keys.signA) || !strings.Contains(out, "primary: "+keys.primary) {
			t.Fatalf("verify output missing expected fingerprints:\n%s", out)
		}
		if got := r.git("cat-file", "-t", testTag); strings.TrimSpace(got) != "tag" {
			t.Fatalf("tag type = %q, want tag", got)
		}
		obj := r.git("cat-file", "-p", testTag)
		if !strings.Contains(obj, "\ntagger "+testTaggerName+" <"+testTaggerEmail+"> ") {
			t.Fatalf("tagger line does not carry the declared identity:\n%s", obj)
		}
		if !strings.Contains(obj, "\n\nRelease "+testTag+"\n-----BEGIN PGP SIGNATURE-----") {
			t.Fatalf("tag message/signature layout unexpected:\n%s", obj)
		}

		out, err = r.runScript("push")
		if err != nil {
			t.Fatalf("push: %v\n%s", err, out)
		}
		local := strings.TrimSpace(r.git("rev-parse", "refs/tags/"+testTag))
		remote := strings.Fields(r.git("ls-remote", "--tags", "origin", "refs/tags/"+testTag))[0]
		if local != remote {
			t.Fatalf("remote tag %s != local tag %s", remote, local)
		}
		heads := r.git("ls-remote", "--heads", "origin")
		if strings.Count(strings.TrimSpace(heads), "\n") != 0 {
			t.Fatalf("push touched branches:\n%s", heads)
		}
	})

	t.Run("forced long key id", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		r.vars["GONEAT_PGP_KEY_ID"] = strings.ToLower(keys.signA[24:]) + "!"
		out, err := r.runScript("create")
		if err != nil {
			t.Fatalf("create: %v\n%s", err, out)
		}
		if !strings.Contains(out, "signer:  "+keys.signA) {
			t.Fatalf("expected signer %s:\n%s", keys.signA, out)
		}
	})

	t.Run("unforced primary signs with the committed subkey", func(t *testing.T) {
		r := newReleaseRepo(t, keys)
		r.vars["GONEAT_PGP_KEY_ID"] = keys.primary
		out, err := r.runScript("create")
		if err != nil {
			t.Fatalf("create: %v\n%s", err, out)
		}
		if !strings.Contains(out, "signer:  "+keys.signA) {
			t.Fatalf("signer is not the committed signing subkey %s:\n%s", keys.signA, out)
		}
		if !strings.Contains(out, "signed by pinned subkey "+keys.signA) {
			t.Fatalf("pinned verification did not run:\n%s", out)
		}
	})
}
