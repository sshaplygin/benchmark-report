# Configuration

This document owns the version 1 configuration contract; configuration loading and validation are implemented, rendering is implemented and history export remains a later stage. The first release uses JSON to allow strict decoding and schema validation without a YAML parser. The [complete example](../examples/benchmark-report.json) selects a package-oriented Markdown report and history export.

## Loading and validation

Configuration has `schema_version: 1`. Unknown fields, duplicate keys, invalid enum values, invalid regular expressions, unsupported schema versions, and wrong value types are errors. JSON integers follow mathematical JSON Schema semantics: `1`, `1.0`, and `1e0` represent the same integer. Comparison thresholds retain exact decimal source text; tokens and canonical threshold values have the [4096-character defensive limit](contracts.md#exact-values-and-identities), with source exponents limited to -10000 through 10000. Defaults apply only to absent fields. An empty string is not an absent path. `benchreport config validate` must validate without reading benchmark files or using the network.

Configuration paths resolve relative to its file. The output directory is selected by CLI. Configured filenames must be relative, stay within that directory, and be unique across all enabled outputs after Unicode NFC normalization and case folding. File/directory prefix collisions follow the same rule. The fixed reproduction paths in [Contracts](contracts.md#reproduction-layout) are reserved and cannot be configured as report or enabled history filenames. Reject traversal and symlink escapes when writing. Tokens, repository credentials, and executable commands are not configuration fields.

CLI flags select input files, the output directory, and explicitly documented operational overrides. They do not silently override comparison policy. Every comparison records the effective policy; every reproduction bundle records the full effective configuration.

## Comparison policy

| Field | Default | Allowed values and effect |
| --- | --- | --- |
| `comparison.regression_percent` | `20` | Finite decimal greater than zero; advisory regression boundary |
| `comparison.improvement_percent` | `20` | Finite decimal greater than zero; advisory improvement boundary |
| `comparison.percent_decimals` | `1` | Integer 0 through 4; precision used for both displayed deltas and advisory classification |
| `comparison.fail_on_regression` | `false` | Boolean; enables exit code 2 from `compare` |
| `comparison.statistics` | `"none"` | `none` or `benchstat`; benchstat requires Go input and the pinned tool |

The exact comparison and zero-baseline behavior is defined in [Architecture](architecture.md#comparison). Benchstat details may coexist with advisory classifications. There is no option to relabel a threshold result as statistical significance.

## Report selection

| Field | Default | Allowed values and effect |
| --- | --- | --- |
| `report.title` | `"Benchmark comparison"` | Nonempty plain text |
| `report.metrics` | `["time"]` | Nonempty unique subset of `time`, `bytes`, `allocations`, `throughput` |
| `report.include` | `[]` | Regular expressions over the canonical measurement key; empty means all |
| `report.exclude` | `[]` | Regular expressions over the same key; exclusion wins |
| `report.group_by` | `["suite", "package"]` | Ordered unique subset of `suite`, `package`, `metric`; empty means one table |
| `report.columns` | `["benchmark", "base", "head", "change", "signal"]` | Ordered unique subset of these values plus `metric`, `samples`; `benchmark` is required |
| `report.sort` | `"name"` | `name` or `regression`; the latter orders regressions, below-threshold rows, improvements, then non-comparable rows, with full identity as tie-break |
| `report.unchanged` | `"show"` | `show` or `hide`; unchanged means equal estimates, not a rounded zero delta |
| `report.missing` | `"show"` | `show` or `hide`; affects added and removed rows |
| `report.max_rows` | `0` | Nonnegative integer; zero means no user limit, otherwise a global limit after sorting |
| `report.units` | `"auto"` | `auto` or `canonical`; auto uses one readable unit for both values in each row |

The canonical key is the JSON array serialization of `[suite, package, benchmark, metric]` with no insignificant whitespace. Filters use Go regular expressions and case-sensitive matching. JSON encoding avoids collisions between names that contain separators. Examples and `normalize` diagnostics must expose the key so users can test filters.

Apply selection in this order: metrics, include, exclude, unchanged/missing visibility, sort, row limit, grouping. Summary counts describe selected rows before the row limit and display the selected/total measurement counts. A row limit shows how many selected rows were omitted. A report with no selected rows says so and still records total input counts. Filtering does not create an empty-input success during normalization.

When `metric` is absent from both grouping and columns, more than one selected metric is invalid. Empty package groups are omitted for Criterion. If a source does not provide a sample count, render an em dash rather than inventing one.

## Sections and formats

| Field | Default | Allowed values and effect |
| --- | --- | --- |
| `report.sections.metadata` | `true` | Revision identities and measurement environment |
| `report.sections.summary` | `true` | Advisory and comparability counts |
| `report.sections.tables` | `true` | Selected measurement tables |
| `report.sections.benchstat` | `false` | Expandable full statistical output; requires `comparison.statistics: "benchstat"` |
| `outputs.markdown` | `"report.md"` | Relative filename or `null` to disable |
| `outputs.json` | `"report.json"` | Relative filename or `null` to disable |

Markdown uses GitHub-flavored tables and `details` for optional statistical output. `report.json` is a versioned presentation model containing selected rows, section settings, display values, and summary counts. It is distinct from the complete `comparison.json` calculation artifact. At least one output format must be enabled.

Mandatory disclosures survive section switches: environment mismatch overrides, omitted-row counts, and the distinction between advisory signals and statistical evidence. Arbitrary templates, raw HTML injection, PDF, and standalone HTML rendering are not first-release features. Requests for those formats should extend the renderer contract without changing the comparison model.

## History export

| Field | Default | Allowed values and effect |
| --- | --- | --- |
| `history.enabled` | `false` | Boolean; enables export for github-action-benchmark |
| `history.metrics` | `["time"]` | Same metric vocabulary as report selection |
| `history.include` | `[]` | Regular expressions over canonical keys |
| `history.exclude` | `[]` | Exclusion wins |
| `history.smaller_file` | `"benchmark-smaller.json"` | Relative output filename |
| `history.bigger_file` | `"benchmark-bigger.json"` | Relative output filename |

History selection is independent of report selection. Hiding a row in a PR comment must not remove that metric from its historical series. Export only nonempty direction groups and return their paths explicitly; remove obsolete generated direction files in a reused output directory so a caller cannot upload stale data. If enabled selection yields no metrics, return an error.

Exported `name` is the canonical measurement key. Units and estimator definitions stay fixed for a series. Changing display precision or report titles does not change exported measurements. The [integration contract](architecture.md#history-integration) owns the caller's action settings and series separation.

## Example profiles

Use the checked-in example for Go timing tables and full benchstat details. For Criterion, set `comparison.statistics` to `none` and `report.sections.benchstat` to false. For a machine-readable-only report, set `outputs.markdown` to null. For allocation analysis, select `allocations` and `bytes` and include `metric` in grouping or columns.

These are configuration changes to the same CLI. No consumer-specific renderer or configuration interpreter is required.
