package sbom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	sourceMaxEntries = 100_000
	sourceMaxBytes   = int64(2 << 30)
)

type sourceLimits struct {
	entries int
	bytes   int64
}

type capturedEntry struct {
	Name      string      `json:"path"`
	Directory bool        `json:"directory"`
	Size      int64       `json:"size"`
	Mode      fs.FileMode `json:"mode"`
	ModTime   string      `json:"modified_at"`
	SHA256    string      `json:"sha256,omitempty"`
	info      fs.FileInfo
}

// sourceCapture is an invocation-owned, reproducible captured-byte subject.
// It is not an atomic filesystem snapshot or a hostile-writer boundary.
type sourceCapture struct {
	original string
	path     string
	root     *os.Root
	manifest []capturedEntry
	digest   string
	excludes []string
	limits   sourceLimits
	created  fs.FileInfo
	cleaned  bool
}

func captureSource(ctx context.Context, target, temporaryParent string, opts SourceOptions, limits sourceLimits) (result *sourceCapture, retErr error) {
	if limits.entries <= 0 || limits.bytes <= 0 {
		return nil, fmt.Errorf("invalid source SBOM capture limits")
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	selected, err := os.Lstat(target)
	if err != nil {
		return nil, fmt.Errorf("source SBOM target %s: %w", target, err)
	}
	if !selected.IsDir() {
		return nil, fmt.Errorf("source SBOM target must be a non-symlink directory: %s", target)
	}
	original, err := os.OpenRoot(target)
	if err != nil {
		return nil, fmt.Errorf("open source SBOM root: %w", err)
	}
	var capture *sourceCapture
	defer func() {
		retErr = errors.Join(retErr, original.Close())
		if retErr != nil && capture != nil {
			retErr = errors.Join(retErr, capture.cleanup())
			result = nil
		}
	}()
	opened, err := original.Stat(".")
	if err != nil || !os.SameFile(selected, opened) {
		return nil, fmt.Errorf("source SBOM root changed while opening %s", target)
	}
	// Fail on invalid root policy/force syntax before copying the subject. The
	// captured copies are parsed again below, so selection still describes the
	// reconciled captured state rather than a stale preflight policy read.
	if !opts.NoIgnore {
		for _, name := range []string{".gitignore", ".goneatignore"} {
			info, err := original.Lstat(name)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("source SBOM ignore %s: %w", name, err)
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("source SBOM ignore %s must be a regular non-symlink file", name)
			}
			if info.Size() > limits.bytes {
				return nil, fmt.Errorf("source SBOM ignore %s exceeds capture byte limit", name)
			}
		}
		if _, err := loadSourcePatterns(original.FS(), opts); err != nil {
			return nil, err
		}
	}
	for _, value := range opts.ForceInclude {
		if _, err := sourceForcePath(value); err != nil {
			return nil, err
		}
	}
	parent, err := filepath.Abs(temporaryParent)
	if err != nil {
		return nil, err
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return nil, fmt.Errorf("resolve source SBOM staging parent: %w", err)
	}
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return nil, err
	}
	if containedSourcePath(realTarget, parent) {
		return nil, fmt.Errorf("source SBOM staging directory must be outside target %s", target)
	}
	temporary, err := os.MkdirTemp(parent, "goneat-source-sbom-")
	if err != nil {
		return nil, fmt.Errorf("create private source SBOM capture: %w", err)
	}
	capture = &sourceCapture{original: target, path: temporary, limits: limits}
	capture.created, err = os.Lstat(temporary)
	if err != nil {
		return nil, err
	}
	if err := makeSourcePrivate(temporary); err != nil {
		return nil, err
	}
	privateInfo, err := os.Lstat(temporary)
	if err != nil {
		return nil, err
	}
	if err := verifySourcePrivacy(temporary, privateInfo, true); err != nil {
		return nil, err
	}
	capture.root, err = os.OpenRoot(temporary)
	if err != nil {
		return nil, err
	}
	capture.manifest, err = scanSourceTree(ctx, original, capture.root, limits)
	if err != nil {
		return nil, err
	}
	reconciled, err := scanSourceTree(ctx, original, nil, limits)
	if err != nil {
		return nil, fmt.Errorf("source SBOM reconciliation: %w", err)
	}
	if err := compareSourceManifests(capture.manifest, reconciled, true); err != nil {
		return nil, err
	}
	current, err := os.Lstat(target)
	if err != nil || !os.SameFile(selected, current) {
		return nil, fmt.Errorf("source SBOM root pathname changed during capture: %s", target)
	}
	if err := capture.verify(ctx); err != nil {
		return nil, err
	}
	patterns, err := loadSourcePatterns(capture.root.FS(), opts)
	if err != nil {
		return nil, err
	}
	var selection []sourceEntry
	for _, entry := range capture.manifest {
		if entry.Name != "." {
			selection = append(selection, sourceEntry{name: entry.Name, directory: entry.Directory})
		}
	}
	capture.excludes, err = compileSourceExcludes(selection, patterns, opts)
	if err != nil {
		return nil, err
	}
	manifestJSON, err := json.Marshal(capture.manifest)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(manifestJSON)
	capture.digest = hex.EncodeToString(hash[:])
	return capture, nil
}

func containedSourcePath(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// All paths are relative to already-opened roots. Root confines traversal;
// Lstat and handle/path checks reject links rather than silently following them.
func scanSourceTree(ctx context.Context, root, destination *os.Root, limits sourceLimits) ([]capturedEntry, error) {
	var entries []capturedEntry
	var total int64
	err := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("source SBOM unreadable or incomplete path %q: %w", name, walkErr)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(entries) >= limits.entries {
			return fmt.Errorf("source SBOM capture exceeds %d entries (including excluded content)", limits.entries)
		}
		info, err := root.Lstat(name)
		if err != nil {
			return fmt.Errorf("source SBOM stat %q: %w", name, err)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("source SBOM rejects symlink or special file %q", name)
		}
		// A root-run test or privileged process must not silently claim coverage
		// of a subject that explicitly has no read/traversal permission bits.
		if info.Mode().Perm()&0o444 == 0 || (info.IsDir() && info.Mode().Perm()&0o111 == 0) {
			return fmt.Errorf("source SBOM unreadable path %q", name)
		}
		captured := capturedEntry{Name: name, Directory: info.IsDir(), Mode: info.Mode(), ModTime: info.ModTime().UTC().Format(time.RFC3339Nano), info: info}
		if info.IsDir() {
			if destination != nil && name != "." {
				if err := destination.Mkdir(name, 0o700); err != nil {
					return fmt.Errorf("source SBOM stage directory %q: %w", name, err)
				}
			}
		} else {
			if info.Size() > limits.bytes-total {
				return fmt.Errorf("source SBOM capture exceeds %d bytes at %q (including excluded content)", limits.bytes, name)
			}
			captured.Size = info.Size()
			captured.SHA256, err = copySourceFile(ctx, root, destination, name, info, limits.bytes-total)
			if err != nil {
				return err
			}
			total += captured.Size
		}
		entries = append(entries, captured)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

type sourceContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r sourceContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func copySourceFile(ctx context.Context, root, destination *os.Root, name string, before fs.FileInfo, remaining int64) (_ string, retErr error) {
	file, err := root.Open(name)
	if err != nil {
		return "", fmt.Errorf("source SBOM open %q: %w", name, err)
	}
	defer func() { retErr = errors.Join(retErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !sameSourceFile(before, opened) {
		return "", fmt.Errorf("source SBOM path %q changed while opening", name)
	}
	hash := sha256.New()
	var writer io.Writer = hash
	if destination != nil {
		out, err := destination.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return "", fmt.Errorf("source SBOM stage %q: %w", name, err)
		}
		defer func() { retErr = errors.Join(retErr, out.Close()) }()
		writer = io.MultiWriter(out, hash)
	}
	copied, err := io.Copy(writer, io.LimitReader(sourceContextReader{ctx: ctx, r: file}, remaining+1))
	if err != nil {
		return "", fmt.Errorf("source SBOM copy/hash %q: %w", name, err)
	}
	after, err := file.Stat()
	if err != nil || copied != before.Size() || !sameSourceFile(before, after) {
		return "", fmt.Errorf("source SBOM file %q changed during copy/hash", name)
	}
	pathInfo, err := root.Lstat(name)
	if err != nil || !sameSourceFile(before, pathInfo) {
		return "", fmt.Errorf("source SBOM pathname %q changed during copy/hash", name)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sameSourceFile(a, b fs.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func compareSourceManifests(expected, actual []capturedEntry, identity bool) error {
	if len(expected) != len(actual) {
		return fmt.Errorf("source SBOM subject changed: manifest entry count %d != %d", len(expected), len(actual))
	}
	for i, want := range expected {
		got := actual[i]
		if want.Name != got.Name || want.Directory != got.Directory || want.Size != got.Size || want.SHA256 != got.SHA256 ||
			(identity && !sameSourceFile(want.info, got.info)) {
			return fmt.Errorf("source SBOM subject changed at %q", want.Name)
		}
	}
	return nil
}

func (capture *sourceCapture) verify(ctx context.Context) error {
	info, err := os.Lstat(capture.path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("source SBOM private capture root changed: %s", capture.path)
	}
	if err := verifySourcePrivacy(capture.path, info, true); err != nil {
		return err
	}
	opened, err := capture.root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		return fmt.Errorf("source SBOM private capture pathname changed: %s", capture.path)
	}
	actual, err := scanSourceTree(ctx, capture.root, nil, capture.limits)
	if err != nil {
		return err
	}
	for _, entry := range actual {
		if err := verifySourcePrivacy(filepath.Join(capture.path, filepath.FromSlash(entry.Name)), entry.info, entry.Name == "."); err != nil {
			return err
		}
	}
	return compareSourceManifests(capture.manifest, actual, false)
}

func (capture *sourceCapture) cleanup() error {
	if capture.cleaned {
		return nil
	}
	current, identityErr := os.Lstat(capture.path)
	if identityErr == nil && (capture.created == nil || !os.SameFile(capture.created, current) || !current.IsDir()) {
		identityErr = fmt.Errorf("private capture pathname no longer identifies the invocation-owned directory")
	}
	var closeErr error
	if capture.root != nil {
		closeErr = capture.root.Close()
		capture.root = nil
	}
	if identityErr != nil {
		return fmt.Errorf("source SBOM cleanup refused changed pathname; private data may remain at or have moved from %s: %w", capture.path, errors.Join(identityErr, closeErr))
	}
	// This exact directory was created by this invocation; never remove a user
	// target, output parent, or any broader temporary parent.
	removeErr := os.RemoveAll(capture.path)
	if err := errors.Join(closeErr, removeErr); err != nil {
		return fmt.Errorf("source SBOM cleanup failed; private data may remain at %s: %w", capture.path, err)
	}
	capture.cleaned = true
	return nil
}

// Conservative budgets account for both argument encoding and inherited
// environment. They intentionally fail below OS maxima rather than truncating.
func checkSourceArguments(command string, args, environment []string, goos string) error {
	if goos == "windows" {
		units := 1
		for _, arg := range append([]string{command}, args...) {
			// Worst-case quoting doubles backslashes/quotes, plus separators.
			units += 2*len(utf16.Encode([]rune(arg))) + 3
		}
		envUnits := 1
		for _, value := range environment {
			envUnits += len(utf16.Encode([]rune(value))) + 1
		}
		if units > 30_000 || envUnits > 30_000 {
			return fmt.Errorf("source SBOM exceeds conservative Windows argument/environment limit; narrow the source target")
		}
		return nil
	}
	bytes := 8192 // Reserve process/pointer/alignment overhead.
	for _, value := range append(append([]string{command}, args...), environment...) {
		bytes += len(value) + 1 + 8
	}
	if bytes > 128*1024 {
		return fmt.Errorf("source SBOM exceeds conservative argument/environment limit; narrow the source target")
	}
	return nil
}
