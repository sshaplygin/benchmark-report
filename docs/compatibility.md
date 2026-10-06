# Compatibility

This matrix covers version `0.1.0`. The [report action](report-action.md#binary-mapping-and-installation) explains its action-to-binary mapping and runtime prerequisites. Coverage below describes tested formats and platforms, not every version of Go or Criterion.

## Inputs and documents

| Surface | Implemented coverage | Evidence and limits |
| --- | --- | --- |
| Go benchmark text | `ns/op`, `B/op`, `allocs/op`, and `MB/s`; repeated samples and package identity | Captured Go 1.27.1 output covers 13 benchmarks across three packages, with ten samples and three metrics per benchmark. Handwritten cases cover throughput and additional syntax. [Fixtures](fixtures.md) records exact provenance. |
| Criterion text | Inline or separate benchmark names; timing intervals in ns, µs/us, ms, and s | Captured Rust 1.94.0 output covers 28 timing estimates across four suites. Progress, baseline changes, and throughput annotations do not become timing measurements. This is tested text-format coverage, not a claim about all Criterion releases. |
| Internal JSON documents | Schema version `1` | [Contracts](contracts.md#schemas-and-validation) owns document validation and unsupported-version behavior. Native action string maps and external history arrays use their separately defined formats. |
| Go statistics | Bundled, pinned upstream benchstat | [Statistical evidence](contracts.md#tools-and-boundaries) defines identity verification and suite isolation. Criterion reports use point estimates and do not claim cross-run statistical significance. |

The [adapter boundary](architecture.md#adapter-boundary) describes format scope and future C++, C#, and JavaScript adapters; those adapters are not implemented.

## Native candidates and replay

| OS | Architecture | CI runner |
| --- | --- | --- |
| Linux | amd64 | `ubuntu-24.04` |
| Linux | arm64 | `ubuntu-24.04-arm` |
| macOS | arm64 | `macos-14` |
| macOS | amd64 | `macos-15-intel` |

The [CI package and replay jobs](../.github/workflows/ci.yml) build and test a native candidate on each platform, including installation checks and captured Go/Criterion report generation. A single pair of Linux-generated Go/Criterion bundles is replayed on all four platforms; complete Markdown, presentation JSON, and bounded comments must match byte for byte. This verifies those archived bundles across the matrix, not arbitrary operating systems or environments.

Version and statistical-tool requirements are defined by the [replay contract](contracts.md#full-report-and-calculation-replay).

## History profiles

[History integration](history.md) documents tested appends, environment/estimator profile isolation, and the external JSON-number compatibility boundary.
