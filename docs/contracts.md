# Version 1 contracts

This document defines serialization, file layout, tool identity, and the CLI. [Architecture](architecture.md) defines normalization and comparison; [Configuration](configuration.md) defines policy and selection.

## Schemas and validation

[schemas](../schemas/) contains JSON Schema Draft 2020-12 documents and [examples/contracts](../examples/contracts/) contains their examples. Internal document `schema_version` is `1`; incompatible changes require a new version. Native GitHub action string maps are versioned by the pinned action commit, and external history arrays have no schema-version field.

Readers reject unknown fields, duplicate keys at every depth, trailing JSON content, invalid UTF-8, and unpaired Unicode surrogate escapes. Validation never fetches schemas from the network. Run `go test ./...` or `go run ./cmd/contractcheck schemas/configuration.schema.json examples/benchmark-report.json`; `contractcheck` is a development command separate from `benchreport`.

Semantic validation checks canonical identity, suite coverage and uniqueness, metric/unit mapping, estimate bounds, estimator evidence, regular expressions, and configuration interactions. Comparison loading verifies stored arithmetic and evidence. Examples with synthetic hashes and version labels demonstrate shape; captured fixtures provide runnable inputs.

## Exact values and identities

Nonnegative measurements, samples, and bounds are canonical decimal strings: `0` or a positive value with an integer part and an optional fractional part ending in a nonzero digit. Serialized values reject exponents, leading zeros, trailing fractional zeros, plus signs, non-finite values, and negative zero. Adapters accept finite nonnegative source decimal/exponent notation and canonicalize it.

Source numeric tokens and canonical values are limited to 4096 characters; exponents outside -10000 through 10000 fail before allocation. Computation uses exact decimal/rational arithmetic. Configuration thresholds remain JSON numbers decoded from their original decimal text, without binary floating-point conversion.

`delta_percent` stores the percentage rounded to `policy.percent_decimals`, with ties away from zero; its canonical string omits insignificant trailing zeros and normalizes negative zero. The unrounded percentage is rederived exactly from the two estimates as a rational; recurring fractions are never approximated in a serialized “raw” value. `null` denotes an unavailable percentage. Reason is `comparable`, `zero_baseline`, `added`, or `removed`. Added/removed rows have precisely one null side; zero-baseline has base zero and head positive. Invalid metric definitions fail the command; they cannot become removed or silently omitted rows.

Identity is `(suite, package, benchmark, metric)`. The canonical key is Go `encoding/json.Marshal` of these four strings as an array, without whitespace: HTML characters are escaped as `\u003c`, `\u003e`, `\u0026`; U+2028/U+2029 are escaped; other non-ASCII characters remain UTF-8. Strings are not Unicode-normalized. Control characters use the standard Go JSON escapes. Keys sort by UTF-8 bytes.

Filters and history names use this key. Go identity retains the complete raw benchmark token, including `Benchmark`, sub-benchmarks, and CPU suffix. Display labels do not alter identity. Criterion package is the empty string and names retain their complete source text.

| Metric | Canonical unit | Direction | Estimator |
| --- | --- | --- | --- |
| `time` | `ns/op` | `lower` | Go `median`; Criterion `criterion-point-estimate` |
| `bytes` | `B/op` | `lower` | Go `median` |
| `allocations` | `allocs/op` | `lower` | Go `median` |
| `throughput` | `B/s` | `higher` | Go `median` |

Go `MB/s` converts to canonical `B/s` by exactly 1,000,000. Estimator and source-parsing behavior are defined in [Normalization](architecture.md#normalization).

## Metadata and files

Revision is a full lowercase 40-digit Git SHA. Environment stores recorded toolchain, OS, architecture, and runner description. Compatibility compares these four fields exactly; mismatch fails unless `--allow-environment-mismatch` is set and the differing field names are recorded. The override never permits metric-definition mismatch. Base/head expected-suite sets must match, irrespective of their order. Manifest suite IDs are unique and exactly cover `expected_suites`; normalized runs additionally contain measurements for every suite. Missing file, missing suite, and empty suite fail before comparison. A removed benchmark is possible only between two complete runs.

Each suite records parser name (`go` or `criterion`), parser contract version (`"1"`), benchmark invocation, and ordered raw file references. The command is provenance text and is never executed. Readers reject unknown parser versions. Normalized documents retain suite/parser metadata and per-input SHA-256 values.

Manifest paths resolve relative to the manifest. Normalization rebases suite file references and input paths relative to the normalized output file; comparison resolves them from that file, never the working directory. Bundle creation rebases references again and rejects paths outside its root.

Readers resolve a document's parent directory through filesystem symlinks before joining relative references. Cleaning the path first can select a different file on systems with aliases such as macOS `/var`. Portable bundles do not require absolute filesystem paths.

## Tools and boundaries

The Go module is `github.com/sshaplygin/benchmark-report` and requires Go 1.25.0 or later. Package responsibilities are defined in [Boundaries](architecture.md#boundaries).

Benchstat is pinned to `golang.org/x/perf/cmd/benchstat@v0.0.0-20251023143056-3684bd442cc8` (commit `3684bd442cc85a615905c090d30b3a23d16e35d9`, module requires Go 1.24). Discovery uses `--benchstat-path FILE` when supplied, otherwise the binary beside `benchreport`. The CLI verifies Go build metadata before execution and rejects missing, unreadable, or mismatched tools. It does not install or download them. Default `statistics: none` needs no executable.

Reproduction evidence records executable SHA-256, version, ordered arguments, stdout, and stderr.

Benchstat runs separately for each suite, in canonical suite order, with base inputs before head inputs and each side's manifest file order preserved. Equal benchmark/package names across suites are not pooled. Recorded input checksums must match before invocation.

Each raw file is one argument, `base=ABSOLUTE_PATH` or `head=ABSOLUTE_PATH`, repeating the side label for every file on that side. The pinned [CLI](https://github.com/golang/perf/blob/3684bd442cc85a615905c090d30b3a23d16e35d9/cmd/benchstat/main.go#L318) and [file reader](https://github.com/golang/perf/blob/3684bd442cc85a615905c090d30b3a23d16e35d9/benchfmt/files.go#L11) preserve these labels, pooling samples into one base and one head column. Arguments record the invocation; input provenance retains each file and checksum. Paths are process arguments, never shell text.

## CLI contract

| Command | Inputs | Successful outputs |
| --- | --- | --- |
| `normalize --parser go\|criterion --manifest FILE --out FILE` | Manifest and all its suite logs; selected parser must match suite metadata | One complete normalized run |
| `compare --base FILE --head FILE [--config FILE] --out FILE [--allow-environment-mismatch] [--benchstat-path FILE]` | Two complete normalized runs and optional configuration; archived raw files if statistics requested | Complete comparison, even when gate fails |
| `render --input FILE [--config FILE] --output-dir DIR [--reproduction FILE]` | Comparison and optional configuration | Enabled Markdown/presentation outputs and render-only reproduction manifest |
| `export --input FILE [--config FILE] --output-dir DIR` | One normalized run and enabled history configuration | Nonempty direction files with explicit paths as stdout JSON |
| `report` (flags below) | Both manifests/raw logs and effective configuration | Full bundle and paths/gate stdout JSON; gate failure returns 0 |
| `replay --reproduction FILE --output-dir DIR [--benchstat-path FILE]` | Verified full bundle and matching tools | Fresh verified full bundle and paths/gate stdout JSON |
| `config validate --config FILE` | Configuration only | No generated files |

All commands have `--help` and `--version`, returning 0 without inputs. Missing/unknown flags and unsupported parsers return 1. Diagnostics go to stderr and generated outputs go to requested files. Commands do not execute benchmarks, use GitHub credentials, or require network access.

Omitting `--config` uses defaults; a supplied missing file is an error. Configuration cannot invoke commands or read environment secrets.

Exit codes are 0 for success, 1 for invalid input or execution failure, and 2 for an enabled regression gate failure from `compare`, after writing its complete artifact. `report` and `replay` return the gate status without failing on regressions. Failed commands leave no complete-looking output. Output paths cannot replace input documents, raw inputs, the running generator, or the selected benchstat executable. [Compatibility](compatibility.md) lists tested formats and native targets.

Render compares the effective comparison policy with the stored policy after defaults expand; threshold equality compares numeric values rather than lexical forms such as `20` and `20.0`. Display settings do not change policy.

Changes display at the policy's precision. Measurement display rounds to at most three fractional decimal places, with ties away from zero and trailing zeros removed. Auto time selects the largest of s/ms/µs/ns for which the larger nonzero side is at least one unit; auto bytes and throughput use decimal GB/MB/kB/B with the same rule. Both sides use one unit. Values under one canonical unit retain three decimals; zero is `0`. Canonical mode uses the canonical unit with the same rounding. Display strings do not change stored values.

Markdown escapes benchmark names, revision labels, and metadata for their table, text, and code contexts. User-supplied values never become shell code.

## Reproduction layout

Standalone `render` writes `reproduction.json` at the output-directory root, plus `replay-inputs/comparison.json` and `replay-inputs/configuration.json` containing the original comparison and effective configuration. Enabled rendered files keep their configured paths. The inventory includes these inputs and reports using root-relative paths; it excludes itself.

The recorded render instruction reads the two `replay-inputs` files and writes to `replayed/`. The manifest records binary/platform identity but has no raw logs or verification instruction. Complete statistics remain in the comparison; the presentation manifest has `statistics: null` and never resolves or executes their historical raw paths.

`reproduction.json`, the complete `replay-inputs/` and `replayed/` subtrees, and any configured file that would be their parent are reserved against all enabled report/history outputs. Full-bundle paths are listed [below](#full-report-and-calculation-replay).

`render --reproduction reproduction.json` requires explicit `--input` and `--config` matching the bundle inventory. Before parsing those inputs, it validates every inventory member and checksum, generator version, configuration digest, metadata, enabled-output completeness, and path containment. It consumes the verified in-memory input snapshots. Source platform metadata permits cross-platform replay with the matching generator version. Recorded replay strings are instructions for the user; the CLI never evaluates them or invokes a shell.

Output names are relative to the selected output directory and cannot traverse outside it. They must be unique and cannot be file/directory prefixes after Unicode NFC normalization and case folding, even on case-sensitive filesystems. Reserved names follow the same rule.

Writers validate destinations, stage all bytes, prevent symlink escapes with rooted filesystem operations, and roll back installed outputs if commit fails. A rollback failure retains recovery copies and reports their location.

## History serialization and outputs

`export` requires `history.enabled: true`; disabled history or an empty enabled selection fails with exit 1 and no stdout result. Successful exports print one JSON object, for example `{"schema_version":1,"files":{"smaller":"/absolute/out/benchmark-smaller.json"}}`. `files.smaller` and `files.bigger` are physical absolute paths and appear only for nonempty direction groups. Callers consume those keys rather than guessing filenames. Export reads the normalized run and configuration, with no benchmark execution, raw-log reads, or benchstat requirement.

History arrays store `name` as the canonical key, `unit` as the canonical metric unit, and `value` as a JSON number retaining the complete canonical estimate digits. `extra` is a string containing compact JSON with estimator and recorded environment; Criterion `range` is a string of the form `lower..upper ns/op`. No unavailable metric becomes zero, and display precision never changes exported values. Entries sort by canonical identity.

The upstream JavaScript action uses binary64 numbers. Export rejects overflow, nonzero underflow, and estimates that change when parsed to binary64 and formatted to the shortest round-trippable decimal, comparing the resulting decimal exactly. Normal values such as `0.1` and `61.145` are supported; `9007199254740993` and excessive fractional precision fail with the measurement key. This history-only compatibility restriction does not change normalized values, comparison calculations, or report rendering.

Nonempty files and deletion of empty direction files form one transaction. Cleanup touches only the two currently configured history filenames; it never searches for older names, removes directories, or deletes unrelated files. Changing configured filenames may leave earlier files in place, so callers must use the returned paths. Validation and protection cover writes and deletions, including normalized/configuration documents, referenced raw paths, and the running generator. A failed commit restores preexisting files using the output rollback rules.

## Full report and calculation replay

The full reporting command is:

```text
report --parser go|criterion --base-manifest FILE --head-manifest FILE [--config FILE] --output-dir DIR [--allow-environment-mismatch] [--benchstat-path FILE] [--artifact-url URL] [--comment-header HEADER]
```

Header defaults to `benchmark-report`. The command runs normalization, comparison, rendering, and optional history export. A failed gate still returns 0 so the caller can publish before applying it.

Report and replay stdout follow [report-result.schema.json](../schemas/report-result.schema.json), for example `{"schema_version":1,"files":{"comparison":"/absolute/comparison.json","reproduction":"/absolute/reproduction.json"},"gate":"disabled"}`. Gate is `disabled`, `passed`, or `failed`. Optional file keys are `markdown`, `presentation`, `comment`, `history_smaller`, and `history_bigger`; every returned path is physical and absolute.

Full bundles reserve `comparison.json`, `comment.md`, and the entire `raw/`, `manifests/`, `normalized/`, and `statistics/` namespaces in addition to presentation replay reservations.

| Files | Contents |
| --- | --- |
| `manifests/base.json`, `manifests/head.json` | Archived manifests with rebased file references |
| `normalized/base.json`, `normalized/head.json` | Complete normalized runs |
| `replay-inputs/configuration.json` | Effective configuration |
| `raw/{base,head}/{suite-index}/{file-index}.log` | Raw logs; indices have at least three digits with zero padding |
| `statistics/{suite-index}.txt` | Optional complete benchstat stdout |
| `comparison.json`, `reproduction.json` | Calculation and inventory |
| Configured report/history paths, optional `comment.md` | Generated outputs |

Suite IDs sort lexically; each suite retains source file ordering. The optional strict `report` object in the reproduction manifest records parser, comment header, and artifact URL, distinguishing full bundles from presentation bundles. Its `replay.verify` instruction invokes `replay`; presentation bundles omit that field. Relative replay instructions run from the bundle root with matching binaries installed beforehand.

`replay --reproduction FILE --output-dir DIR [--benchstat-path FILE]` verifies all inventory checksums and containment before reading dependent snapshots. It re-normalizes archived raw inputs, computes every comparison again, and requires equivalent calculations and byte-identical normalized, presentation, comment, and history outputs. It never evaluates recorded command strings. The exact generator version must match. Pinned benchstat is required only when originally selected: tool identity and statistical evidence must match; the executable checksum must also match on the original platform. Different-platform verified pinned binaries may differ in checksum and absolute invocation prefixes. Archived tool provenance and original platform remain preserved in replay outputs. No tools are downloaded implicitly.

Full report and replay output directories must be absent or empty. This prevents stale unlisted raw/statistical/comment files from entering uploaded bundles; arbitrary existing files are never cleaned. Both operations commit all outputs transactionally and protect source documents, referenced raw logs, configuration, and executable files. JSON-only reports omit comments. Shortened comments record the operational header/URL and never change the complete Markdown or comparison artifact. `render --reproduction` also accepts full bundles with `--input comparison.json` and the archived effective configuration.
