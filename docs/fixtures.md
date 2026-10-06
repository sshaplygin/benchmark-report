# Fixture evidence

These archived inputs and independently calculated expectations exercise the parsing, comparison, and rendering contracts. The owning contracts remain [Architecture](architecture.md) and [Configuration](configuration.md).

## Captured inputs

All captures were downloaded through authenticated `gh api` on 2026-10-03. No benchmarks were executed. `testdata/captured/*/artifact-*.json` records GitHub artifact identity, archive checksum, original member checksum, and stored checksum after trimming. Go inputs retain only `goos`, `goarch`, `pkg`, `cpu`, and benchmark lines; Criterion inputs remove Cargo download, compilation, finish, runner, and Gnuplot setup lines. Benchmark results, progress, warnings, bounds, and throughput lines remain verbatim. No measurement excerpts were sampled away. Paths in the checked-in manifests refer to the trimmed files. Their checksums must therefore use stored bytes.

### Go PR 15

[Run 37104301673](https://github.com/sshaplygin/go-socket.io/actions/runs/37104301673), artifact `11267366782`, contains the exact report associated with [comment 5966539113](https://github.com/sshaplygin/go-socket.io/pull/15#issuecomment-5966539113). Base is `79a393cf6e5e299fd4faa8e02225f2632e0ea6e8`; head is `89368c38fa1643cbac4478d34aa8d7c3ef38aff1`. The workflow at that head records `go test -mod=readonly -run '^$' -bench . -benchmem -count=10 ./...`, followed by `benchstat base.txt pr.txt`. Benchstat is pinned to `v0.0.0-20251023143056-3684bd442cc8`. The checked-in `toolchain-evidence.txt` excerpts resolve `stable` to Go 1.27.1, Linux amd64; the workflow uses `GOTOOLCHAIN=local` and `ubuntu-24.04`. Both revisions run sequentially in the same compare job. CPU metadata in the raw inputs is `INTEL(R) XEON(R) PLATINUM 8573C`.

Each side has 130 benchmark result lines: ten repetitions of 13 identities. Package counts are packet 2, payload 10, transport 1. Every result supplies time, bytes, and allocations, so normalization should yield 39 measurements per side, each with ten samples; no throughput measurement occurs. `comparison.txt` and `benchmark-report.md` are unmodified archived consumer output. They are evidence of the legacy report, not golden outputs for the new renderer.

Decoder-4's base middle timing pair is 60.90 and 61.39 ns, giving exact median 61.145 ns; head median is 61.73 ns. This checks retention of precision beyond the legacy displayed values.

### Four Criterion suites

[Run 37101710472](https://github.com/sshaplygin/ytsaurus-rs/actions/runs/37101710472), PR 92, uses base `7b6bba056a06a5adb46cc2e079764beca072a4d4` and head `c7caaa65bf2c7b9b7c3d339c06929a9046c27368`. Both revisions' `rust-toolchain.toml` pins 1.94.0; the checked-in `toolchain-evidence.txt` excerpts from each suite's install log report `1.94.0-x86_64-unknown-linux-gnu`. The workflow uses `ubuntu-24.04`, with base and head sequential on one VM per suite. Four suite jobs use different VMs, so the manifest runner describes the class, not one shared VM for all suites.

| Suite | Artifact | Package / bench target | Timing rows on each side | Inline-name rows on each side |
| --- | --- | --- | ---: | ---: |
| client | 11266632287 | ytsaurus-client / rows | 12 | 3 |
| job | 11266742155 | ytsaurus-job / job_throughput | 9 | 0 |
| skiff | 11266302700 | ytsaurus-skiff / codec_throughput | 3 | 0 |
| yson | 11266261855 | ytsaurus-yson / yson_benchmark | 4 | 0 |

Invocation for each row is `cargo bench --locked -p PACKAGE --bench TARGET -- --noplot`, with combined stdout/stderr captured. Normalize 28 timing measurements per side. Retain bounds and the middle estimate; do not infer raw samples or counts from progress text. Throughput and baseline-change lines do not create timing measurements. For example, base `Skiff codec throughput/encode_dynamic` has bounds 2.8763–3.0792 ms and estimate 2.9655 ms, hence 2,876,300 / 2,965,500 / 3,079,200 ns.

## Requirement-to-fixture index

Expected outcomes below were calculated from the stated numbers and contracts, independently of implementation output. Numerical cases are machine-readable in `testdata/handwritten/numerical.json`; they are test specifications, not versioned product documents.

| Requirement home | Fixture | Expected outcome |
| --- | --- | --- |
| [Normalization](architecture.md#normalization): repeated Go samples, even median, packages, sub-benchmarks, CPU suffix, zero allocation, throughput | `handwritten/go/valid.txt` | Three benchmark identities; ten metric identities. In package a, four samples have time median 2.5 ns/op, bytes 3 B/op, allocations 1.5 allocs/op, throughput 2,500,000 B/s. Package b's same sub-name and CPU suffixes remain distinct; its zero bytes/allocations remain measurements. |
| Normalization: inline/separate Criterion names, units, progress and baseline lines | `handwritten/criterion/valid.txt` | Four timing measurements: estimates 2, 2000, 2000000, 2000000000 ns/op; bounds each half/1.5 times estimate. No percentage-change or throughput metric. Samples unavailable. |
| Normalization: all consumer packages and suites | `captured/go-pr15`, `captured/criterion-four-suites` | Counts and precision traces above; separate suite identity retained. |
| Normalization: malformed, finite, nonnegative, unsupported metric | `handwritten/invalid/go-{truncated,nonfinite,negative,unsupported}.txt`, `criterion-truncated.txt` | Input failure (exit 1), with file and result line; no successful normalized output. |
| Normalization: duplicate definitions | `handwritten/invalid/criterion-duplicate.txt` | Fail duplicate estimate in one suite; repeated Go rows remain valid samples. |
| Normalization: empty input versus display selection | `handwritten/invalid/empty.txt`, [empty output example](output-examples.md#empty-selection) | Empty raw log fails; selecting zero existing comparison rows succeeds with explicit zero selected count. |
| Normalization: missing suite/file | `handwritten/invalid/missing-suite-manifest.json`, `missing-file-manifest.json` | Missing expected suite versus listed absent input file both fail; diagnostics name suite/file. Neither becomes a removed benchmark. |
| [Comparison](architecture.md#comparison): rounded classification and zero rules | `handwritten/numerical.json` | Literal expected deltas/signals in each case; rounding ties away from zero; finite zero-base exception explicit. |
| Comparison: added/removed and mismatch | `handwritten/comparability.json` | Listed expected reason or failure; environment override disclosed, metric mismatch never overridden. |
| [Selection](configuration.md#report-selection): filter precedence, stable calculation, row limits | `handwritten/selection.json` | Literal expected selected/order/omitted values; full classifications and gate unchanged. |
| [Reproduction](architecture.md#reproduction-artifacts): exact stored inputs | Captured manifests and artifact metadata | All referenced files exist; original and trimmed checksum roles distinct; no network needed to read captured files. |

The adapter and comparison tests use these expectations; [rendering goldens](../internal/render/testdata/golden/) and [CLI integration tests](../tests/) cover generated reports and replay. Stage acceptance is recorded in the implementation PR's completion record.
