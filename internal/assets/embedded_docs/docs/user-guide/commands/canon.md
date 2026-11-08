# canon — Canonical JSON

`goneat canon` produces RFC 8785 JSON Canonicalization Scheme (JCS) bytes
using the pure-Go `github.com/3leaps/canonprims/canon` library. It needs no
external command or CGo runtime.

## Usage

```bash
printf '%s' '{"b": 2, "a": 1}' | goneat canon jcs
# Exact output: {"a":1,"b":2} (no trailing newline)
goneat canon jcs --check fixture.json
goneat canon jcs --digest fixture.json
goneat canon jcs --vector fixture.json
goneat canon info
goneat canon info jcs/rfc8785
```

`jcs [file|-]` reads one file or standard input (the default). It writes only
the requested result to stdout; diagnostics go to stderr. It does not probe
package managers or initialize scanners.

## Output modes

| Flag         | Result                                                     |
| ------------ | ---------------------------------------------------------- |
| No mode flag | Canonical UTF-8 bytes, without a trailing newline          |
| `--check`    | No output; success only for byte-identical canonical input |
| `--digest`   | `sha256:<lowercase-hex>` followed by LF                    |
| `--vector`   | Canonical JSON object followed by LF                       |

The vector has `byte_length`, `canonical` (a JSON-escaped string containing
the canonical bytes), `digest`, and `scheme` (`jcs/rfc8785`). The byte length
counts UTF-8 bytes, and the digest hashes those bytes without a newline.
`info` emits canonical JSON plus LF with `scheme`, `spec`, `corpus`, and
`corpus_digest` describing the library's conformance corpus.

`--check`, `--digest`, and `--vector` are mutually exclusive. Even a trailing
newline makes otherwise canonical input fail `--check`.

## NDJSON arrays and sets

```bash
goneat canon jcs --ndjson-array journal.ndjson
goneat canon jcs --ndjson-array --head 100 --digest journal.ndjson
goneat canon jcs --sort-set members.json
goneat canon jcs --ndjson-array --sort-set --vector members.ndjson
```

`--ndjson-array` canonicalizes each LF-terminated line and wraps the results
in an array. Blank or whitespace-only lines, raw carriage returns (including
CRLF), and unterminated final lines are refused. An empty stream produces
`[]`. Escaped `\r` and `\n` inside JSON strings remain valid.

`--head N` requires `--ndjson-array` and an explicitly positive N. It selects
exactly the first N lines and refuses a shorter stream. Later lines are not
validated or included in the byte budget; buffered input may read ahead.
Without `--head`, all lines are selected. `--check` cannot be combined with
`--ndjson-array`.

`--sort-set` requires a top-level array. It sorts elements by their canonical
UTF-8 bytes, refusing canonically equal elements with `duplicate_set_element`.
Ordering is bytewise, not numeric: `[2,10]` becomes `[10,2]`. It composes with
NDJSON, digest, vector, and check modes. With `--check`, the input must already
be canonical and in set order. Array/set transformations are explicit;
ordinary JCS preserves array order.

## Limits and input refusals

Defaults follow canonprims v0.1.4:

| Flag                   | Default           | Meaning                                                         |
| ---------------------- | ----------------- | --------------------------------------------------------------- |
| `--max-bytes`          | 16777216 (16 MiB) | Input bytes; selected NDJSON lines include their LF terminators |
| `--max-depth`          | 128               | Container nesting; a scalar is depth 0                          |
| `--max-object-members` | 1048576           | Members per object                                              |
| `--max-string-bytes`   | 16777216 (16 MiB) | Decoded UTF-8 bytes per string, including member names          |

Limits must be nonnegative. Zero permits none of that resource, rather than
disabling the limit. The maximum machine integer is not accepted for
`--max-bytes`, which needs one extra byte for overflow detection. Reads are
bounded to that byte budget plus one sentinel byte. NDJSON's generated array
counts toward the depth limit; generated punctuation and canonical number
spelling do not count toward the input-byte limit.

The library refuses invalid JSON or UTF-8, duplicate object members, lone
surrogates, Unicode noncharacters, non-finite numbers, integers outside its
binary64 domain, and inputs exceeding limits. Refusal identifiers include
`invalid_json`, `invalid_utf8`, `duplicate_member`, `lone_surrogate`,
`noncharacter`, `non_finite_number`, `integer_outside_binary64`, `too_deep`,
`too_large`, and `not_canonical`.

## Exit codes

| Code | Meaning                                                           |
| ---- | ----------------------------------------------------------------- |
| 0    | Success; also an output pipe closed early by its reader (`EPIPE`) |
| 1    | Input refusal or failed canonicality/set check                    |
| 2    | Usage or flag error                                               |
| 4    | Input read/open failure                                           |
| 5    | Output failure other than `EPIPE`                                 |

Input is validated before output is written. An output failure may leave a
partial result; consumers should check the exit status.
