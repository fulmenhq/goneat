package cmd

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/3leaps/canonprims/canon"
	"github.com/spf13/cobra"
)

// Canon commands have a pipe-oriented exit contract, independent of the
// assessment command's result codes. All diagnostics stay on stderr.
type canonCommandError struct {
	code int
	err  error
}

func (e *canonCommandError) Error() string { return e.err.Error() }
func (e *canonCommandError) Unwrap() error { return e.err }

func canonError(code int, err error) error { return &canonCommandError{code: code, err: err} }
func canonUsage(message string) error      { return canonError(2, errors.New(message)) }
func canonRefuse(code canon.Code) error    { return canonError(1, canon.Refusal{Code: code}) }

type canonOptions struct {
	check, digest, vector, ndjson, sortSet bool
	head                                   int
	limits                                 canon.Limits
}

func newCanonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "canon", Short: "Canonical JSON bytes, digests, and checks (RFC 8785)",
		SilenceUsage: true, SilenceErrors: true,
		// Override the root hook: data-only commands need no package managers,
		// scanner initialization or logger configuration.
		PersistentPreRun: func(*cobra.Command, []string) { signal.Ignore(syscall.SIGPIPE) },
		RunE:             func(*cobra.Command, []string) error { return canonUsage("choose canon jcs or canon info") },
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return canonError(2, err) })
	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) { _, _ = fmt.Fprint(c.OutOrStdout(), c.UsageString()) })
	options := canonOptions{limits: canon.DefaultLimits()}
	jcs := &cobra.Command{
		Use: "jcs [file|-]", Short: "Canonicalize strict I-JSON using RFC 8785",
		SilenceUsage: true, SilenceErrors: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return canonUsage("canon jcs accepts at most one input file")
			}
			return nil
		},
		RunE: func(c *cobra.Command, args []string) error {
			if err := options.validate(c.Flags().Changed("head")); err != nil {
				return err
			}
			input := c.InOrStdin()
			if len(args) == 1 && args[0] != "-" {
				file, err := os.Open(args[0]) // #nosec G304 -- explicit CLI input file
				if err != nil {
					return canonError(4, err)
				}
				defer func() { _ = file.Close() }()
				input = file
			}
			return options.run(input, c.OutOrStdout())
		},
	}
	f := jcs.Flags()
	f.BoolVar(&options.check, "check", false, "Require byte-identical canonical input; write nothing")
	f.BoolVar(&options.digest, "digest", false, "Write the canonical SHA-256 digest")
	f.BoolVar(&options.vector, "vector", false, "Write canonical bytes, length, and digest as a JSON vector")
	f.BoolVar(&options.ndjson, "ndjson-array", false, "Convert strict LF-terminated NDJSON to a canonical array")
	f.IntVar(&options.head, "head", 0, "Select the first N NDJSON lines (requires N >= 1)")
	f.BoolVar(&options.sortSet, "sort-set", false, "Sort array elements by canonical bytes; refuse duplicates")
	f.IntVar(&options.limits.MaxBytes, "max-bytes", options.limits.MaxBytes, "Maximum input bytes")
	f.IntVar(&options.limits.MaxDepth, "max-depth", options.limits.MaxDepth, "Maximum container nesting depth")
	f.IntVar(&options.limits.MaxObjectMembers, "max-object-members", options.limits.MaxObjectMembers, "Maximum members per object")
	f.IntVar(&options.limits.MaxStringBytes, "max-string-bytes", options.limits.MaxStringBytes, "Maximum decoded bytes per string")
	info := &cobra.Command{
		Use: "info [scheme]", Short: "Describe the canonicalization scheme and conformance corpus",
		SilenceUsage: true, SilenceErrors: true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return canonUsage("canon info accepts at most one scheme")
			}
			return nil
		},
		RunE: func(c *cobra.Command, args []string) error {
			scheme := canon.JCS
			if len(args) == 1 {
				scheme = canon.Scheme(args[0])
			}
			info, err := canon.Info(scheme)
			if err != nil {
				return canonError(2, err)
			}
			return writeCanonMetadata(c.OutOrStdout(), map[string]any{
				"scheme": info.Name, "spec": info.Spec, "corpus": info.Corpus, "corpus_digest": info.CorpusDigest,
			})
		},
	}
	cmd.AddCommand(jcs, info)
	return cmd
}

func (o canonOptions) validate(headSet bool) error {
	if o.check && (o.digest || o.vector || o.ndjson) || o.digest && o.vector {
		return canonUsage("--check conflicts with --digest, --vector and --ndjson-array; --digest conflicts with --vector")
	}
	if headSet && (!o.ndjson || o.head < 1) {
		return canonUsage("--head requires --ndjson-array and N >= 1")
	}
	if o.limits.MaxBytes < 0 || o.limits.MaxBytes == int(^uint(0)>>1) || o.limits.MaxDepth < 0 || o.limits.MaxObjectMembers < 0 || o.limits.MaxStringBytes < 0 {
		return canonUsage("limits must be nonnegative; --max-bytes must leave room for an overflow sentinel")
	}
	return nil
}

func (o canonOptions) run(input io.Reader, output io.Writer) error {
	var raw, canonical []byte
	var err error
	if o.ndjson {
		canonical, err = o.readNDJSON(input)
	} else {
		raw, err = io.ReadAll(io.LimitReader(input, int64(o.limits.MaxBytes)+1))
		if err != nil {
			return canonError(4, err)
		}
		canonical, err = canon.Canonicalize(canon.JCS, raw, o.limits)
		if err != nil {
			return canonError(1, err)
		}
	}
	if err != nil {
		return err
	}
	if o.sortSet {
		canonical, err = sortCanonicalSet(canonical)
		if err != nil {
			return err
		}
	}
	if o.check {
		if !bytes.Equal(raw, canonical) {
			return canonRefuse(canon.NotCanonical)
		}
		return nil
	}
	if o.vector || o.digest {
		sum := sha256.Sum256(canonical)
		digest := "sha256:" + hex.EncodeToString(sum[:])
		if o.digest {
			return writeCanon(output, []byte(digest+"\n"))
		}
		return writeCanonMetadata(output, map[string]any{
			"byte_length": len(canonical), "canonical": string(canonical), "digest": digest, "scheme": string(canon.JCS),
		})
	}
	return writeCanon(output, canonical)
}

func (o canonOptions) readNDJSON(input io.Reader) ([]byte, error) {
	// Reserve one container level for the generated array. Validate original
	// tokens once: JCS number serialization can change exact integer spelling.
	if o.limits.MaxDepth < 1 {
		return nil, canonRefuse(canon.TooDeep)
	}
	lineLimits := o.limits
	lineLimits.MaxDepth--
	reader := bufio.NewReader(io.LimitReader(input, int64(o.limits.MaxBytes)+1))
	array := []byte{'['}
	used, count := 0, 0
	for o.head == 0 || count < o.head {
		line, err := reader.ReadBytes('\n')
		used += len(line)
		if used > o.limits.MaxBytes {
			return nil, canonRefuse(canon.TooLarge)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, canonError(4, err)
			}
			if len(line) != 0 || o.head > count {
				return nil, canonRefuse(canon.InvalidJSON)
			}
			break
		}
		line = line[:len(line)-1]
		if len(bytes.TrimSpace(line)) == 0 || bytes.ContainsRune(line, '\r') {
			return nil, canonRefuse(canon.InvalidJSON)
		}
		part, err := canon.Canonicalize(canon.JCS, line, lineLimits)
		if err != nil {
			return nil, canonError(1, err)
		}
		if count > 0 {
			array = append(array, ',')
		}
		array = append(array, part...)
		count++
	}
	return append(array, ']'), nil
}

func sortCanonicalSet(input []byte) ([]byte, error) {
	if len(input) == 0 || input[0] != '[' {
		return nil, canonRefuse(canon.InvalidJSON)
	}
	// Input has already passed the strict I-JSON parser. RawMessage preserves
	// number tokens and Unicode spellings without float64 decoding.
	var elements []json.RawMessage
	if err := json.Unmarshal(input, &elements); err != nil {
		return nil, canonError(1, err)
	}
	sort.Slice(elements, func(i, j int) bool { return bytes.Compare(elements[i], elements[j]) < 0 })
	out := []byte{'['}
	for i, element := range elements {
		if i > 0 {
			if bytes.Equal(elements[i-1], element) {
				return nil, canonError(1, errors.New("canon: duplicate_set_element"))
			}
			out = append(out, ',')
		}
		out = append(out, element...)
	}
	return append(out, ']'), nil
}

func writeCanonMetadata(output io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return canonError(5, err)
	}
	// Metadata may exceed the input limit due to JSON string escaping. It is
	// generated exclusively from admitted data, so use its own byte/string size.
	limits := canon.DefaultLimits()
	limits.MaxBytes, limits.MaxStringBytes = len(raw), len(raw)
	canonical, err := canon.Canonicalize(canon.JCS, raw, limits)
	if err != nil {
		return canonError(5, err)
	}
	return writeCanon(output, append(canonical, '\n'))
}

func writeCanon(output io.Writer, data []byte) error {
	n, err := output.Write(data)
	if errors.Is(err, syscall.EPIPE) {
		return nil
	}
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return canonError(5, err)
	}
	return nil
}
