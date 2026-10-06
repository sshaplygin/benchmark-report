# Proposed YTsaurus Rust migration

This patch targets consumer HEAD `c7caaa65bf2c7b9b7c3d339c06929a9046c27368` with a clean working tree. Shared action pin, release prerequisites, and rollout boundaries are recorded in [Migration proposals](../../docs/migrations.md).

## Consumer mapping

The existing `pull_request` trigger, path exclusions, four parallel suites, `ubuntu-24.04` runners, per-PR concurrency, Rust cache workspaces, and fourteen-day raw artifact retention remain. Each suite still runs base then PR on one VM with the exact commands below.

| Suite | Package | Bench target | Command |
| --- | --- | --- | --- |
| yson | ytsaurus-yson | yson_benchmark | `cargo bench --locked -p ytsaurus-yson --bench yson_benchmark -- --noplot` |
| skiff | ytsaurus-skiff | codec_throughput | `cargo bench --locked -p ytsaurus-skiff --bench codec_throughput -- --noplot` |
| job | ytsaurus-job | job_throughput | `cargo bench --locked -p ytsaurus-job --bench job_throughput -- --noplot` |
| client | ytsaurus-client | rows | `cargo bench --locked -p ytsaurus-client --bench rows -- --noplot` |

The checked-in toolchain is Rust `1.94.0`. The workflow retains its policy of using the PR's pinned compiler for both revisions. It now enforces that policy with `RUSTUP_TOOLCHAIN`: previously the base command could resolve the base checkout's own `rust-toolchain.toml`. This changes compiler selection only when the revisions request different toolchains.

`scripts/benchmark_report_inputs.py` is an input producer, not a formatter. Its record mode saves the actual compiler identity; assembly requires all four nonempty logs and matching recorded environments. It emits one explicit suite inventory per revision with exact commands and physical log paths. Python remains a consumer producer dependency, run through pinned setup-uv and `uv run --no-project --no-python-downloads` using the runner's preinstalled interpreter. The shared report and publisher actions require no Python or Go runtime. Existing Python CI applies ruff and ty to this helper.

The read-only aggregate job uses the PR checkout's producer and `.github/benchmark-report.json`, generates a full report and bounded comment, and uploads the complete replay inventory. A migration PR can therefore provide these new files without a separate base bootstrap. This job has only read permissions, like the measurement jobs that already execute PR Cargo code; publication remains isolated. Failed or missing suites prevent aggregation. The old `compare_criterion.py` and `post_benchmark_comment.py` files remain unchanged but have no workflow invocations.

A separate job has `pull-requests: write`, downloads only report data, appends the full summary once, and publishes the bounded comment. It checks out no consumer code and executes no downloaded program. Fork and Dependabot PRs retain summaries and artifacts while the publisher skips comments. The advisory policy remains ±20%, with the regression gate disabled; if enabled later, its failure applies after artifact upload and publication. The stable comment header is `benchmark-report-criterion`.

Configuration, artifact replay, fork handling, and comment sizing are defined by the existing [configuration](../../docs/configuration.md), [report action](../../docs/report-action.md), and [publication](../../docs/publication.md) contracts.

## Local evidence

Verification used the archived [four-suite fixture](../../testdata/captured/criterion-four-suites) and its [provenance](../../docs/fixtures.md).

The legacy parser finds 25 point estimates per revision: client 9, job 9, skiff 3, yson 4. All 25 estimates exactly match the new normalized nanosecond values when the legacy source decimal and unit are converted using decimal arithmetic. Floating-point intermediate rounding in the old Python implementation is not used as the equality oracle. The new parser finds 28 estimates per revision because it also accepts the client's three inline name-and-time lines:

| Complete benchmark name | Legacy result | Base ns/op | Head ns/op |
| --- | --- | ---: | ---: |
| read/read_table/1000 | omitted | 244470 | 246230 |
| read/read_table/10000 | omitted | 1592800 | 1595300 |
| read/read_table/100000 | omitted | 16179000 | 16490000 |

The legacy regex requires a timing line after the name; these names and timings occupy one line in both archived logs. No captured log was edited. Suite identity is retained instead of concatenating and pooling all names. Display order, grouped headings, canonical units, three-decimal timing displays, and disclosures follow the shared contract; Markdown is intentionally not byte-identical to the former formatter.

Checks passed in a disposable clone:

```sh
actionlint .github/workflows/benchmarks.yml
uvx ruff check scripts/benchmark_report_inputs.py
uvx ruff format --check scripts/benchmark_report_inputs.py
uvx ty check scripts/benchmark_report_inputs.py
```

The helper assembled manifests from copied captured logs plus fixture environment records. A locally built `benchreport report --parser criterion` accepted those manifests and the proposed configuration, producing comparison, presentation, Markdown, bounded comment, and reproduction outputs with `gate: disabled`. Removing the YSON base log caused assembly to fail. The current shared [CLI and action integration tests](../../tests/report-action.py) cover schema validation and replay behavior; fixture environment records used locally are not a new runner measurement.

The independent numeric oracle is [verify.py](verify.py). It imports the retained legacy parser, converts its source display decimals with `Decimal`, and compares every estimate and suite count against normalization. From this repository, reproduce it without consumer compilation:

```sh
go build -o /tmp/benchreport-stage7 ./cmd/benchreport
python3 migrations/ytsaurus-rs/verify.py --consumer /path/to/disposable/ytsaurus-rs --binary /tmp/benchreport-stage7
```

## Apply and rollback

Review and test in a disposable checkout first:

```sh
git checkout c7caaa65bf2c7b9b7c3d339c06929a9046c27368
git apply --check /path/to/benchmark-report/migrations/ytsaurus-rs/changes.patch
git apply /path/to/benchmark-report/migrations/ytsaurus-rs/changes.patch
```

Patch application and reversal were verified locally. `git apply --reverse changes.patch` restored the old workflow and removed both added files; the checkout was clean before reapplying. Use the absolute patch path for reversal, and check for subsequent edits before reversing. The retained Python scripts allow restoring the previous workflow without recovering deleted formatter code. Rust compilation tests and a production workflow trial remain consumer-side checks after approval and binary release availability.
