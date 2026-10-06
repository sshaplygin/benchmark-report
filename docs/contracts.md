# Version 1 contracts

This document freezes serialization and implementation choices left open by [Architecture](architecture.md) and [Configuration](configuration.md). It specifies the planned CLI; only the development validator is executable at stage 1.

## Schemas and validation

[schemas](../schemas/) contains JSON Schema Draft 2020-12 documents for input manifests, normalized runs, comparisons, presentation, reproduction, and configuration. [examples/contracts](../examples/contracts/) contains one example per schema. All objects reject unknown fields; readers additionally reject duplicate keys at every depth and trailing JSON content. `schema_version` is exactly `1`; incompatible changes require a new version. Run `go test ./...` or `go run ./cmd/contractcheck schemas/configuration.schema.json examples/benchmark-report.json`. The validator is a development tool, not a released `benchreport` command. Validation never fetches schemas from the network.

Structural schemas cannot express every relation. Shared validation additionally checks canonical identity, suite coverage and uniqueness, metric/unit mapping, estimate bounds, required estimator evidence, Go regular expressions, and effective configuration interactions. Input file existence, checksums, median arithmetic, comparison arithmetic, symlink containment, file/directory output conflicts, and reproduction inventory completeness are checked by the commands that consume those documents in later stages. Examples use synthetic hashes and version labels; they demonstrate shape, not runnable reproduction bundles.

## Exact values and identities

Nonnegative measurements, samples, and bounds are canonical decimal strings: `0` or a positive integer with an optional fractional part ending in a nonzero digit. Exponents, leading zeros, trailing fractional zeros, plus signs, non-finite values, and negative zero are rejected. Computation uses exact decimal/rational arithmetic. Configuration thresholds remain JSON numbers, decoded from their original decimal text without conversion through binary floating point.

`delta_percent` stores the percentage rounded to `policy.percent_decimals`, with ties away from zero; its canonical string omits insignificant trailing zeros and normalizes negative zero. The unrounded percentage is rederived exactly from the two estimates as a rational; recurring fractions are never approximated in a serialized “raw” value. `null` denotes an unavailable percentage. Reason is `comparable`, `zero_baseline`, `added`, or `removed`. Added/removed rows have precisely one null side; zero-baseline has base zero and head positive. Invalid metric definitions fail the command; they cannot become removed or silently omitted rows.

Identity is `(suite, package, benchmark, metric)`. The canonical key is Go `encoding/json.Marshal` of these four strings as an array, without whitespace: HTML characters are escaped as `\u003c`, `\u003e`, `\u0026`; U+2028/U+2029 are escaped; other non-ASCII characters remain UTF-8. Strings are not Unicode-normalized. Control characters use the standard Go JSON escapes. Keys sort by UTF-8 bytes. This rule applies to filters and history names. Go identity retains the complete raw benchmark token, including `Benchmark`, sub-benchmarks, and CPU suffix. Display may strip the initial literal `Benchmark`. Criterion package is the empty string and names retain their complete source text.

| Metric | Canonical unit | Direction | Estimator |
| --- | --- | --- | --- |
| `time` | `ns/op` | `lower` | Go `median`; Criterion `criterion-point-estimate` |
| `bytes` | `B/op` | `lower` | Go `median` |
| `allocations` | `allocs/op` | `lower` | Go `median` |
| `throughput` | `B/s` | `higher` | Go `median` |

Go samples are retained, including repeated rows; median uses the arithmetic mean of the middle pair for even counts. Go `MB/s` means decimal megabytes and converts by exactly 1,000,000. Criterion timing keeps lower/point/upper estimates converted to nanoseconds, no invented samples. Criterion `thrpt:` and percentage-change lines are recognized ancillary output and ignored for first-release timing; malformed supported timing fails. Unknown Go metric units fail with source location.

## Metadata and files

Revision is a full lowercase 40-digit Git SHA. Environment stores recorded toolchain, OS, architecture, and runner description. Compatibility compares these four fields exactly; mismatch fails unless `--allow-environment-mismatch` is set and the differing field names are recorded. The override never permits metric-definition mismatch. Base/head expected-suite sets must match, irrespective of their order. Manifest suite IDs are unique and exactly cover `expected_suites`; normalized runs additionally contain measurements for every suite. Missing file, missing suite, and empty suite fail before comparison. A removed benchmark is possible only between two complete runs.

Each suite records parser name (`go` or `criterion`), parser contract version (`"1"`), benchmark invocation, and ordered raw file references. The command is provenance text and is never executed. Version 1 parser readers reject unknown versions rather than silently upgrading. Normalized documents retain suite/parser metadata and per-input SHA-256 values. Manifest paths resolve relative to the manifest; normalization rebases suite file references and input paths relative to the normalized output file. Comparisons resolve them from that file, never the working directory. Bundle creation rebases references again and rejects paths outside its root. No filesystem absolute path is required for portable bundles.

## Tools and boundaries

The module is `github.com/sshaplygin/benchmark-report`, minimum Go 1.25.0. `internal/contracts` owns version definitions, JSON validation and canonical identity. Later packages separate adapters (`internal/adapters/go`, `internal/adapters/criterion`), comparison (`internal/compare`), rendering (`internal/render`), history (`internal/history`), configuration (`internal/config`), and reproduction (`internal/reproduction`). `cmd/benchreport` will wire them together; those directories are created when their stage implements them. No parser imports renderer or publisher code.

Benchstat is pinned to `golang.org/x/perf/cmd/benchstat@v0.0.0-20251023143056-3684bd442cc8` (commit `3684bd442cc85a615905c090d30b3a23d16e35d9`, module requires Go 1.24). This matches the recorded Go consumer workflow. Offline discovery uses explicit `compare --benchstat-path FILE` when supplied, otherwise the `benchstat` binary beside `benchreport`. Verify Go build metadata module version before execution; absent/unreadable/wrong metadata or a different pin is an error when requested. Never install or download it implicitly. Default `statistics: none` needs no executable. Record executable SHA-256, version, ordered arguments, stdout and stderr in reproduction evidence.

Run benchstat separately for each suite, in canonical suite order, with base inputs before head inputs and each side's manifest file order preserved. Store each invocation separately; equal benchmark/package names across suites must never be pooled. Recorded file checksums must match before invocation. Advisory policy remains independent of statistics.

## Planned CLI help

| Command | Inputs | Successful outputs |
| --- | --- | --- |
| `normalize --parser go\|criterion --manifest FILE --out FILE` | Manifest and all its suite logs; selected parser must match suite metadata | One complete normalized run |
| `compare --base FILE --head FILE [--config FILE] --out FILE [--allow-environment-mismatch] [--benchstat-path FILE]` | Two complete normalized runs and optional configuration; archived raw files if statistics requested | Complete comparison, even when gate fails |
| `render --input FILE [--config FILE] --output-dir DIR` | Comparison and optional configuration | Enabled Markdown/presentation outputs and render-only reproduction manifest |
| `export --input FILE [--config FILE] --output-dir DIR` | One normalized run and enabled history configuration | Nonempty direction files with explicit paths |
| `config validate --config FILE` | Configuration only | No generated files |

All commands have `--help` and `--version`; help/version return 0 without inputs. Missing/unknown flags and unsupported parsers return 1. Regular diagnostics go to stderr; generated outputs go to requested files. None executes benchmarks or uses GitHub credentials. None requires network access. Exit codes are 0 success, 1 invalid input/execution failure, and 2 enabled regression gate failure from `compare` after its complete artifact is written. Failed commands leave no complete-looking output. The planned release targets Linux/macOS amd64/arm64.

Default comparison fields are expanded before rendering checks equality against stored policy. Report/history display changes do not alter policy. Render uses the display precision from policy for change; measurement display rounds to at most three fractional decimal places, ties away from zero, trimming trailing zeros. Auto time selects the largest of s/ms/µs/ns for which the larger nonzero side is at least one unit; auto bytes and throughput use decimal GB/MB/kB/B with the same rule. Both sides use one unit. Values under one canonical unit retain three decimals; zero is `0`. Canonical mode uses the canonical unit with the same rounding. These are display strings only.

Render-only reproduction records comparison, effective configuration and rendered outputs; its `replay.verify` is omitted because the comparison alone does not retain raw log references. Complete calculation bundles are assembled by the later report-action orchestration from retained manifest/raw/normalized paths; they include both base/head normalize commands followed by compare, and a `replay.verify` sequence. The CLI never pretends a render-only bundle can verify normalization. Relative commands run from the bundle root. A matching binary and any pinned benchstat must be installed beforehand.

## Reproduction layout

Standalone `render` writes `reproduction.json` at the output-directory root, plus `replay-inputs/comparison.json` and `replay-inputs/configuration.json` containing the original comparison and fully expanded effective configuration. Enabled rendered files keep their configured paths. The manifest inventory uses paths relative to this root and includes those inputs and all generated report files; it excludes itself. Its render command reads the two `replay-inputs` files and writes to `replayed/`, avoiding overwriting the archived bundle. The manifest records the binary/platform identity; no raw log files or verify command are invented.

`reproduction.json`, the complete `replay-inputs/` subtree, and any path that would make either reserved path a child of a configured file are reserved against all enabled report/history outputs. Full calculation bundles add a `raw/`, `manifests/`, and `normalized/` inventory under the report-action orchestration's artifact directory; that orchestration selects its destination independently of standalone render. Raw suite file names are rewritten deterministically to indexed files within each side and suite, preserving input ordering. Full bundles carry render and base/head normalize-plus-compare commands; their inventory is checked before replay. Output writers validate symlink containment and all file/directory collisions before making changes.
