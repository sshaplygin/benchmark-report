# Configuration

Configuration is version 1 JSON. The [complete example](../examples/benchmark-report.json) selects Go timing tables, benchstat details, and history export.

## Loading and validation

The [shared validation rules](contracts.md#schemas-and-validation) apply, including rejection of unsupported versions and unknown or duplicate fields. Invalid field values and regular expressions fail validation. JSON integers follow mathematical JSON Schema semantics: `1`, `1.0`, and `1e0` represent the same integer. Thresholds follow the [exact numeric limits](contracts.md#exact-values-and-identities).

Defaults apply only to absent fields. An empty path is invalid. `benchreport config validate` reads no benchmark files, uses no network, and writes no output files.

The CLI selects the output directory. Configured filenames follow the [output path and reserved-name rules](contracts.md#reproduction-layout). Tokens, repository credentials, and executable commands are not configuration fields.

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

Filters use Go regular expressions and case-sensitive matching against the complete [canonical key](contracts.md#exact-values-and-identities).

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

Environment mismatch overrides, omitted-row counts, and the distinction between advisory signals and statistical evidence remain visible regardless of section switches. Arbitrary templates, raw HTML injection, PDF, and standalone HTML rendering are unsupported.

## History export

| Field | Default | Allowed values and effect |
| --- | --- | --- |
| `history.enabled` | `false` | Boolean; enables export for github-action-benchmark |
| `history.metrics` | `["time"]` | Same metric vocabulary as report selection |
| `history.include` | `[]` | Regular expressions over canonical keys |
| `history.exclude` | `[]` | Exclusion wins |
| `history.smaller_file` | `"benchmark-smaller.json"` | Relative output filename |
| `history.bigger_file` | `"benchmark-bigger.json"` | Relative output filename |

History selection is independent of report selection. Hiding a row in a PR report does not remove it from history. Export requires enabled history and a nonempty selection. [Contracts](contracts.md#history-serialization-and-outputs) defines returned paths, empty-direction cleanup, and numeric compatibility.

Display precision and report titles do not change exported measurements. [History](history.md) defines series profiles and caller action settings.

## Example profiles

Use the checked-in example for Go timing tables and full benchstat details. For Criterion, set `comparison.statistics` to `none` and `report.sections.benchstat` to false. For a machine-readable-only report, set `outputs.markdown` to null. For allocation analysis, select `allocations` and `bytes` and include `metric` in grouping or columns.

Comment header and artifact URL are operational [CLI inputs](contracts.md#full-report-and-calculation-replay), recorded for replay rather than comparison policy.
