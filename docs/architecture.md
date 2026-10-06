# Architecture

This document specifies the first release. Version 1 schemas and serialization rules are defined in [Contracts](contracts.md); normalization, comparison, configuration validation, rendering, and verified presentation replay are implemented. History export remains pending the stages in the [implementation plan](implementation-plan.md).

## Boundaries

The planned executable is `benchreport`. Its parsing, comparison, rendering, and export code is written in Go. A future report action installs the released binary and invokes it. The root publication action already accepts a completed Markdown file and delegates GitHub comment operations to sticky-pull-request-comment. Go consumers and Rust consumers use the same reporting interface.

The first release supports Linux and macOS on amd64 and arm64. Installation uses release assets and published SHA-256 checksums. It does not install Go in consumer workflows. The action version selects a matching binary version; consumers pin the action to a commit. The release process must verify that mapping.

Go statistical analysis uses a pinned upstream benchstat binary built from a fixed `golang.org/x/perf` revision and included in each platform release archive. Keep its version in the release manifest and reproduction metadata. Locate it beside `benchreport`; an explicit `--benchstat-path` may select another copy only if it matches the recorded version. Do not implement a replacement significance test. The main CLI works without benchstat unless statistical details are requested. Criterion comparison initially uses the point estimates in its logs and does not claim statistical significance between independent runs.

Suggested source boundaries are `cmd/benchreport` and packages for input, comparison, rendering, export, and configuration. Domain packages must not depend on GitHub environment variables or API clients. The CLI has no publication command or GitHub API client.

## Adapter boundary

An input adapter translates a benchmark framework's results into the normalized-run model. It owns source syntax, metric mapping, estimator identification, and parser diagnostics. Shared comparison, rendering, and export consume that model without importing parser implementations. Publication receives only the rendered file. Source-specific statistical analysis remains an explicit optional capability; benchstat is not a requirement for other adapters.

The PoC needs only the two adapters specified under [Normalization](#normalization). Use their shared boundary to accommodate later adapters without introducing a dynamic plugin loader or a public extension API. Additional metrics or framework semantics may require a versioned schema extension; do not claim compatibility before defining and testing that mapping. The [post-PoC TODO list](implementation-plan.md#todo-after-the-poc) records candidate formats.

## Data contracts

All JSON documents carry `schema_version: 1`, except the external action's array format. Readers reject unsupported versions and duplicate object keys. Input validation reports the file and field or line involved. Schemas and valid and invalid fixtures are stage 1 deliverables.

| Document | Required information |
| --- | --- |
| Run input manifest | Revision SHA; toolchain; OS and architecture; runner description; explicit expected suites; each suite's ID, raw log paths, parser version, and benchmark command |
| Normalized run | Manifest metadata; raw input checksums; measurements keyed by suite, package, benchmark, and metric; canonical unit; direction; estimator; raw samples when available |
| Comparison | Base/head identities; generator and statistical tool versions; effective comparison policy; union of measurement keys; estimates, delta, comparability reason, advisory classification, and separately stored statistical evidence |
| Reproduction manifest | File inventory with checksums; schema and binary versions; input metadata; configuration digest; benchstat version and arguments if used |

Paths inside input manifests resolve relative to the manifest. Suite IDs are explicit, not inferred from artifact directory names. Several logs may belong to one suite, but duplicate measurement definitions across files are errors unless the parser identifies them as repeated samples of the same benchmark. An expected suite with no measurements is an error.

A metric definition fixes its unit, direction, and estimator. Retain Go package paths and CPU suffixes in identity. Criterion suite IDs disambiguate equal benchmark names in different suites. A display label never changes identity. Different toolchains or execution environments across base/head fail compatibility validation; an explicit `--allow-environment-mismatch` override permits an advisory comparison and records the mismatched fields in the report. It never overrides a metric unit, direction, or estimator mismatch.

For measurements, accept only finite, nonnegative numbers. Reject malformed measurements that resemble a supported result; ignore known harness progress and unrelated log lines. Preserve sufficient decimal precision to implement the rounding contract without binary floating-point threshold errors. Define the serialized decimal representation in stage 1; the external action requires JSON numbers.

## Normalization

The Go adapter reads standard `go test -bench -benchmem` text, including package boundaries, sub-benchmarks, CPU suffixes, and repeated samples. First-release metrics are time, bytes per operation, allocations per operation, and throughput. Unsupported metric units produce a diagnostic and require an explicit future adapter extension. Preserve samples and use the median for display and history export; an even sample count uses the arithmetic mean of the middle pair.

The Criterion adapter handles both inline and separate-line benchmark names, timing units `ns`, `us`, `µs`, `ms`, and `s`, and interleaved progress output. Normalize timing estimates to nanoseconds. Keep lower and upper bounds as estimate metadata. Do not parse Criterion's percentage-change output as an absolute measurement. The first release exports Criterion timing only; throughput parsing and native Criterion JSON input are later extensions.

Neither adapter invents missing measurements. A missing suite is a failed input; a benchmark added or removed between complete runs is a valid comparison row. The parser must distinguish these cases.

## Comparison

Join measurements by full identity. Comparable rows require compatible metric definitions and values on both sides. The raw percentage change is `(head / base - 1) * 100`. Improvement direction comes from the metric, not from the sign alone.

When both values are zero, the delta is zero. A zero base and nonzero head has no finite percentage and is marked `zero_baseline`. Added and removed measurements receive their own reasons and no percentage. Reject incompatible definitions rather than silently dropping them.

Classify advisory changes using the percentage rounded to the configured precision, with decimal ties rounded away from zero. Normalize negative zero for display. A lower-is-better increase at or above the regression threshold is a regression; a decrease at or beyond the improvement threshold is an improvement. Reverse the classification for higher-is-better metrics. Other comparable rows are `below_threshold`. These labels do not assert statistical significance.

Presentation filters never change classification, the stored comparison, or a failure decision. An optional regression gate evaluates all comparable measurements in the input. It is disabled by default. Statistical results remain a separate field and, when requested, an attached full benchstat report. Advisory thresholds never stand in for p-values.

## CLI

| Command | Responsibility |
| --- | --- |
| `normalize --parser go\|criterion --manifest FILE --out FILE` | Validate the manifest, parse all expected inputs, and write one normalized run |
| `compare --base FILE --head FILE --config FILE --out FILE` | Validate compatibility, calculate comparisons, and optionally execute pinned benchstat against the recorded raw Go inputs |
| `render --input FILE --config FILE --output-dir DIR` | Render selected formats from an existing comparison without recalculation |
| `export --input FILE --config FILE --output-dir DIR` | Export one normalized run for the history action |
| `config validate --config FILE` | Validate configuration without benchmarks, network access, or filesystem writes |

`--config` is optional; omission uses documented defaults. A supplied missing configuration is an error. Resolve templates or output names only as specified by the [configuration contract](configuration.md); there are no arbitrary command hooks.

Exit codes are 0 for success, 1 for execution or input failure, and 2 for an enabled regression gate. `compare` writes a valid comparison before returning 2 so CI can still render and publish it. A rendering configuration with different comparison settings from those recorded in the comparison is rejected. Normal diagnostics go to stderr. A failed command must not leave an apparently complete output file; write to temporary files and rename after validation.

Output ordering and numeric formatting are deterministic. Do not insert a current timestamp into report content. Revision labels and tool metadata come from recorded inputs. The same binary, inputs, and configuration produce identical report bytes on the supported platforms.

## GitHub integration

Keep computation and publication in separate jobs. The planned `report` entry point orchestrates normalization, comparison, rendering, and reproduction artifacts from downloaded input manifests. It returns paths for the report, comparison, reproduction manifest, and any requested history export, plus a regression status. The root publication action owns optional job-summary output so callers do not append the same report twice.

The root action consumes a completed report and invokes a pinned sticky-pull-request-comment. The dependency owns comment lookup, pagination, author matching, and create/update requests. Do not add a parallel API client, custom retry layer, or comment discovery implementation. The implemented input contract and legacy-marker migration are defined in [Publication](publication.md).

Serialize the complete benchmark workflow per PR and report header with cancellation of superseded runs. The wrapper does not query current PR head state and cannot guarantee that an explicitly rerun old workflow will not replace a newer report. Strict freshness checking is deferred; disclose this limitation instead of promising it through sticky-pull-request-comment.

Benchmark execution jobs use read-only repository permissions and no publication secrets. The default fork behavior is summary plus artifacts. A privileged publisher must never execute PR-supplied binaries, actions, scripts, or configuration hooks. Cross-workflow publication for forks is outside the first release.

Report rendering must escape benchmark names, revision labels, and metadata for the selected context. No user-supplied value becomes shell code. Configuration cannot read environment secrets or invoke commands. The publication action rejects oversized comments rather than modifying supplied Markdown. The planned renderer will produce a shortened comment with deterministic row truncation, an omitted-row count, and a link to the complete artifact when needed. Its comment budget must include the upstream marker defined in Publication. Fail if the fixed content alone exceeds that budget.

## History integration

Export absolute measurements from primary-branch runs, not base/head deltas. Produce `benchmark-smaller.json` and `benchmark-bigger.json` as needed. Each contains entries with `name`, `unit`, and `value`; optional `range` and `extra` describe the estimator or bounds. Both exports use the same estimates as the local reports. Never convert an unavailable metric to zero.

The caller invokes `github-action-benchmark` with `customSmallerIsBetter` or `customBiggerIsBetter` and the matching file. Do not invoke it for an empty direction group. Configure `comment-always`, `comment-on-alert`, `summary-always`, and `fail-on-alert` as false. Use stable, distinct history suite names for different directions, execution environments, and estimator definitions. A change in environment or estimator starts a new series.

The consumer owns GitHub Pages setup, publication permissions, branch selection, and serialization of history writes. Examples must restrict history publication to pushes on the primary branch and prevent concurrent writers from losing updates. Keep this permission set separate from the PR benchmark job.

The external format is specified in [github-action-benchmark's documentation](https://github.com/benchmark-action/github-action-benchmark#readme). Its [action inputs](https://github.com/benchmark-action/github-action-benchmark/blob/master/action.yml) define publication switches. Pin the tested action commit in integration fixtures and release evidence.

## Reproduction artifacts

The planned report-action orchestration archives raw inputs, manifests, effective configuration, normalized runs, comparison JSON, complete rendered reports, benchstat output when enabled, and checksums. Standalone `render` produces a presentation replay bundle from its comparison and effective configuration; it does not claim to verify normalization. Standalone replay uses `render --reproduction FILE` to verify complete inventories and checksums before reading dependent inputs; it never executes recorded command strings. Fixed bundle paths and reserved names are defined in [Contracts](contracts.md#reproduction-layout). Rewrite archived file references to bundle-relative paths and reject references outside the bundle during replay. A replay command sequence in each artifact starts at rendering when only presentation is being reproduced and at normalization when calculation is being verified. Offline replay requires the matching platform release archive to be installed beforehand; no command downloads missing tools implicitly.

Record benchstat input ordering and suite boundaries. Missing archived dependencies produce an explicit error. No step silently fetches a baseline, infers a different revision, or replaces a recorded tool version with the latest release.
