# Implementation plan

Deliver a reusable Go CLI and action that satisfy the [architecture](architecture.md) and [configuration](configuration.md) contracts. This plan defines delivery order and review evidence. It does not authorize changes to the consumer repositories or publication of releases.

The root publication wrapper is implemented against sticky-pull-request-comment and accepts existing Markdown reports. Its contract and current verification scope are in [Publication](publication.md). The generator stages below remain planned; stage 6 covers integration and release acceptance of the wrapper, not a new GitHub API client.

## Adapter scope for the PoC

The PoC implements only Go benchmark text and Rust Criterion logs. Other input formats are deferred. Their absence does not block PoC acceptance or the initial release described by this plan.

Keep the [adapter boundary](architecture.md#adapter-boundary) open to additional formats. For the PoC, this requires separation between input parsing and the shared comparison and rendering code. It does not require a plugin system, public adapter SDK, placeholder parsers, additional runtime dependencies, or fixtures for deferred frameworks.

### TODO after the PoC

- [ ] Evaluate a C++ adapter for [Google Benchmark JSON](https://google.github.io/benchmark/user_guide.html#output-formats), preserving CPU time and elapsed time as distinct metrics.
- [ ] Evaluate a .NET adapter for [BenchmarkDotNet JSON exports](https://benchmarkdotnet.org/articles/configs/exporters.html), preserving runtime, job, and benchmark parameter identity.
- [ ] Evaluate a JS/TS adapter for [Tinybench](https://github.com/tinylibs/tinybench), with an explicit export format and supported version range.
- [ ] Design a public `benchreport-json` input format for custom producers. The internal normalized-run document is not yet a stable public ingestion API.

Each future adapter needs its own format mapping, compatibility policy, fixtures, and acceptance criteria before implementation. These TODOs are references for later work, not additional stages or commitments for the PoC.

## Review process

Each stage is submitted with its diff, requirement links, verification commands and results, and any unresolved defects. Reviewer agents examine the stage independently of the implementing agent's summary. They inspect relevant code and fixtures and reproduce the checks listed below. No review stage is complete solely because another agent reports success.

A reviewer returns `accept` or `request changes`, with file references, the violated contract, a reproducer, and the required correction. Calculation errors, silent omissions, invalid successful outputs, credential exposure, and unreproducible reports block acceptance. Optional improvements are recorded separately. If a contract is ambiguous, resolve it in its owning document before approving code that depends on it.

The implementing agent addresses blocking findings and resubmits affected checks. Accepted earlier stages are reopened only when later changes invalidate their evidence. Stages are sequential; the parser stage may split Go and Criterion work after stage 1 has frozen the shared model. Reviewer roles below specify expertise, not a requirement to run agents concurrently.

## Stage 1 Contracts and fixtures

**Work**

- Initialize the Go module, choose and record the minimum supported Go version, and establish the package boundaries.
- Freeze versioned schemas for input manifests, normalized runs, comparisons, presentation JSON, reproduction manifests, and configuration.
- Resolve exact decimal serialization, canonical identity encoding, parser metadata, benchstat version, and executable discovery.
- Capture representative logs from both consumers, recording revision, invocation, toolchain, suite, and origin. Exclude credentials and unrelated logs.
- Add hand-calculated small fixtures alongside captured inputs. Create a requirement-to-fixture index with expected outcomes independent of implementation output.

**Definition of done**

Schemas, examples, CLI help specification, fixture provenance, and the requirement index agree. Every first-release command has defined inputs, outputs, exit statuses, and offline behavior. No executable behavior is represented as already released.

**Reviewer acceptance**

- The contract reviewer validates all example documents against the schemas and rejects unknown and duplicate fields.
- The benchmark reviewer traces a repeated Go benchmark and a Criterion estimate through the proposed model without losing suite identity, estimator, or units.
- The reviewer can distinguish missing input, missing suite, removed benchmark, and unsupported metric from the schema alone.
- The shared model represents metric units, direction, and estimator without requiring a Go package or Go sample data for every source. Adding a parser does not require a separate renderer or publisher.
- All unresolved questions that affect parsing or output compatibility are closed before stage 2.

## Stage 2 Input adapters

**Work**

- Implement strict manifest loading and normalized-run serialization.
- Implement both adapters against the [normalization contract](architecture.md#normalization).
- Add diagnostics for unsupported, malformed, duplicate, incomplete, and empty inputs.
- Preserve sample data, input checksums, metadata, and deterministic measurement ordering.

**Definition of done**

Both adapters normalize captured consumer logs and the expected valid fixtures. Invalid fixtures fail with actionable locations. No benchmark execution or GitHub access occurs during normalization.

**Reviewer acceptance**

- The parser reviewer independently checks row counts and units against raw logs, including all four original Criterion suites and the three Go packages from PR 15 when those artifacts are available.
- Go cases cover repeated samples, an even sample count, sub-benchmarks, multiple packages, equal names in distinct packages, CPU suffixes, zero allocations, and throughput.
- Criterion cases cover short inline names, long names, progress output, mixed time units, and baseline percentage lines.
- Negative tests cover missing suite files, truncated measurements, non-finite values, duplicate definitions, and logs with no measurements.
- Fuzz tests exercise parsing and identity construction; regression seeds for discovered defects are committed.
- Parser-specific handling stays behind the adapter boundary. Deferred parser names are rejected as unsupported; they are not advertised as implemented or treated as empty successful inputs.

## Stage 3 Comparison and statistical details

**Work**

- Implement the identity join, median calculation, unit handling, comparability reasons, decimal deltas, and advisory classifications.
- Implement environment compatibility checks and the explicit override disclosure.
- Integrate the pinned benchstat executable for Go inputs and capture its complete output and invocation.
- Implement optional regression gating with a complete comparison artifact available on exit code 2.

**Definition of done**

The comparison artifact contains all input measurement identities and the evidence needed by renderers. Calculation and policy tests use independently calculated expected values. Statistical results are not derived from advisory thresholds.

**Reviewer acceptance**

- The numerical reviewer verifies `[1, 2, 3, 4]` has median `2.5`, and baseline 160 versus head 170 produces `+6.3%` at one decimal.
- Tests exercise both signs at threshold boundaries, rounded negative zero, exact zero changes, a zero baseline, added/removed rows, and direction reversal for throughput.
- Equivalent values in different supported input units yield the same result after normalization.
- Changing report filters leaves comparison classifications and gate status unchanged.
- A direct invocation of the pinned benchstat on the same ordered raw inputs matches the attached output.
- Missing or incompatible statistical tooling fails clearly when requested; the default comparison works without it.

## Stage 4 Configurable rendering and offline replay

**Work**

- Implement configuration validation, selection, section controls, Markdown rendering, and presentation JSON.
- Implement context-specific escaping, output path checks, deterministic sorting, and atomic writes.
- Produce reproduction manifests and the local replay instructions carried in artifacts.
- Add golden reports for the existing Go layout and a grouped Criterion layout.

**Definition of done**

All documented configuration fields have meaningful behavior and rejection tests. The example configuration renders a complete report. Archived inputs can regenerate the report on Linux and macOS without a token or network connection.

**Reviewer acceptance**

- The report reviewer verifies every row against the comparison artifact and the selection pipeline in the configuration contract.
- Configuration cases cover JSON-only output, allocation tables, include/exclude precedence, hidden unchanged rows, missing rows, global row limits, and empty selection.
- Changing columns, grouping, or title changes presentation without changing calculations or exported measurements.
- Names containing pipes, backticks, brackets, HTML, newlines, and Unicode do not break table structure or create unintended markup.
- Traversal paths, colliding filenames, symlink escapes, and partial writes are rejected or prevented as specified.
- The same reproduction bundle yields byte-identical Markdown and presentation JSON on both supported operating systems. Review rendered Markdown as well as source snapshots.

## Stage 5 History export

**Work**

- Implement the [history integration](architecture.md#history-integration) and its independent metric filters.
- Generate the two direction-specific files and expose only files with measurements.
- Add an integration fixture pinned to a tested github-action-benchmark commit.
- Provide a primary-branch workflow example with Pages prerequisites, required permissions, and serialized history writes.

**Definition of done**

The pinned external action accepts exported data and updates a test history. The exported values match the normalized run and corresponding report estimates. Empty groups and reused output directories cannot upload stale metrics.

**Reviewer acceptance**

- The integration reviewer checks the external parser accepts the JSON, including optional metadata.
- Time, memory, and allocation metrics go to smaller-is-better; Go throughput goes to bigger-is-better.
- A display-only configuration change preserves exported names, units, and values.
- Two successive primary-branch runs append history points; different environment or estimator profiles use separate configured series.
- The workflow cannot publish PR measurements into primary-branch history and produces no duplicate summary or alert comment.

## Stage 6 Comment publication and action packaging

**Work**

- Integrate the report generator with the root publication wrapper, following the [publication contract](publication.md). Delegate comment operations to the pinned upstream action; do not implement a Go publisher.
- Have the renderer supply a shortened comment linked to the complete artifact when necessary; the publication wrapper rejects over-budget files.
- Package the planned report action and its binary installer, checksums, and version mapping. Keep publication usable with a pre-rendered file and no generator installation.
- Define action inputs and outputs as a schema-backed contract. Include reproduction artifact paths and the comparison gate status.
- Provide workflow control flow that publishes reports before applying an enabled regression gate to the final job status.

**Definition of done**

Local wrapper tests and summary-only CI pass. An authorized trial PR verifies upstream create/update integration. Packaged generator binaries run on all four target platforms. Consumer workflows require no Go or Python installation for reporting or publication. API failures fail the publishing step; skipped publication has an explicit status.

**Reviewer acceptance**

- The publication reviewer verifies the upstream SHA and its input mapping, then tests create, update, unchanged content, distinct headers, and insufficient permissions in an authorized trial. Upstream API internals are not reimplemented or given a separate mock test suite here.
- Wrapper tests cover missing, blank, invalid UTF-8, symlinked, and oversized reports; filenames containing glob characters still resolve to exactly one file.
- The renderer shortens oversized Unicode content on valid boundaries, with an accurate omitted-row count and a working artifact link. The complete artifact stays intact; the wrapper rejects a comment that still exceeds its budget.
- The workflow reviewer confirms PR execution cannot access the publication token and the publisher does not execute checked-out PR code.
- Fork and Dependabot examples finish with summary and caller-uploaded artifacts without attempting comments. Concurrent report workflows use the documented serialization key; documentation does not claim an atomic stale-head check.
- The release reviewer corrupts a downloaded asset and confirms installation fails; version mismatches cannot silently select another binary.

## Stage 7 Consumer migration trial

**Work**

- Prepare separate proposed migrations for `go-socket.io` and `ytsaurus-rs` using the shared action and per-repository configuration.
- Preserve benchmark commands, toolchains, and same-runner base/head execution. Choose stable sticky headers and apply the [legacy-comment migration](publication.md#migrating-existing-comments).
- Replace formatting and publication scripts only after comparisons against archived reports pass.
- Record intentional behavior changes, including the Criterion inline-name parser fix, with before/after fixtures.
- Document rollback to the previous pinned workflows and artifacts.

**Definition of done**

Both proposed migrations have verified artifacts and reviewer evidence. Trial execution in external repositories and comment posting require authorization for those actions; preparing local patches and replaying archived artifacts does not. Existing scripts remain available until acceptance of each migration.

**Reviewer acceptance**

- The consumer reviewer reproduces the Go report from PR 15 and a Criterion report from all configured suites, checking numeric equivalence and documented presentation differences.
- In an authorized trial, a second run updates the sticky-managed comment. Existing legacy comments follow the documented one-time migration policy; ID preservation is required only when that migration mode is explicitly chosen.
- Go change detection and Rust suite selection retain their existing scope. Toolchain equality is enforced by the actual commands, not only claimed in report text.
- A failed suite cannot produce an apparently complete report; an added or removed benchmark remains visible under the configured policy.
- The reviewer executes the rollback instructions in a disposable checkout and verifies the old workflow references are restored.

## Stage 8 Release acceptance

**Work**

- Run the release test matrix, Go static analysis, race tests where applicable, action YAML validation, and schema/example validation.
- Verify release archives, license and dependency notices, checksums, and pinned benchstat and external action references.
- Replace proposed-interface notices with supported-version documentation only for implemented features.
- Publish a compatibility matrix covering Go output, Criterion output, operating systems, architectures, and artifact schemas.

**Definition of done**

The release evidence links every accepted stage to its verification results and remaining nonblocking limitations. A fresh checkout can build the CLI, and the proposed release assets can replay both consumer bundles. No required acceptance item is deferred to a future release.

**Reviewer acceptance**

- The release reviewer follows the README from a clean machine without undocumented prerequisites.
- The example configuration validates against the released schema, and command examples match the released CLI help.
- Documentation contains no claims of unsupported formats, cross-run statistical guarantees, or identical performance on repeated benchmark execution.
- The reviewer verifies a primary-branch export and a PR report use the same estimator for each metric.
- Release publication and consumer rollout occur only after the concrete artifacts have passed review and the user has authorized those external actions.

## Completion record

Maintain one stage checklist in the implementation PR or issue. Record stage, implementation commit, reviewer verdict, evidence links, and open defects. Do not copy requirements or test logs into multiple documents. Contract changes belong in Architecture or Configuration; delivery status belongs in that checklist.
