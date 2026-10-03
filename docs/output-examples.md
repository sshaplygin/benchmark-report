# Output examples

These examples show the proposed output of Benchmark Report. All measurements, revisions, and environments below are synthetic. They are design examples, not results from either consumer repository or output from a released binary.

Each configuration block is a complete configuration with unspecified fields taking their [documented defaults](configuration.md). The calculation rules remain in [Architecture](architecture.md#comparison). The examples do not define additional settings or output schemas.

## Go timing report

The example run contains nine measurement identities: six timings, two allocation measurements, and one throughput measurement. Timing estimates are medians of ten samples on each available side. The following configuration selects the six timings and groups them by package.

```json
{
  "schema_version": 1,
  "report": {
    "title": "Go benchmarks: base vs PR",
    "group_by": ["package"]
  }
}
```

Expected `report.md` rendering:

### Go benchmarks: base vs PR

Base `111111111111` → PR `222222222222`

Suite: `go`. Environment: Linux amd64, runner `benchmark-demo`, Go 1.25.0 for both revisions. Both revisions ran on the same runner. Values are medians of repeated measurements.

**6 of 9 measurements selected · 1 regression · 1 improvement · 1 below threshold · 3 not comparable**

Time is lower-is-better. Signals use a 20% advisory threshold and are not statistical significance tests.

#### example.org/socket/engineio/packet

| Benchmark | Base | PR | Change | Signal |
| --- | ---: | ---: | ---: | --- |
| Decoder-4 | 100 ns | 120 ns | +20.0% | ⚠️ regression |
| Encoder-4 | 80 ns | 60 ns | -25.0% | ✅ improved |

#### example.org/socket/engineio/payload

| Benchmark | Base | PR | Change | Signal |
| --- | ---: | ---: | ---: | --- |
| B64Decoder-4 | 2.0 µs | 2.1 µs | +5.0% | ℹ️ below threshold |
| NewDecoder-4 | — | 900 ns | — | added |
| RemovedDecoder-4 | 700 ns | — | — | removed |
| ZeroBaseline-4 | 0 ns | 5 ns | — | zero baseline |

The optional benchstat section is disabled in this example. With the [Go example configuration](../examples/benchmark-report.json), the report also includes an expandable section containing the actual, unmodified benchstat output. The [existing consumer report](https://github.com/sshaplygin/go-socket.io/pull/15#issuecomment-5966539113) demonstrates that section. No p-values are inferred from the synthetic timing estimates here.

## Compact report

This configuration renders the same Go comparison with fewer columns, no metadata section, and a two-row limit. Grouping by package keeps benchmark ownership visible.

```json
{
  "schema_version": 1,
  "report": {
    "title": "Benchmark changes",
    "group_by": ["package"],
    "columns": ["benchmark", "change", "signal"],
    "sort": "regression",
    "max_rows": 2,
    "sections": {
      "metadata": false
    }
  }
}
```

Expected rendering:

### Benchmark changes

**6 of 9 measurements selected · 1 regression · 1 improvement · 1 below threshold · 3 not comparable**

Showing 2 of 6 selected measurements; 4 omitted by the configured row limit.

Time is lower-is-better. Signals use a 20% advisory threshold and are not statistical significance tests.

#### example.org/socket/engineio/packet

| Benchmark | Change | Signal |
| --- | ---: | --- |
| Decoder-4 | +20.0% | ⚠️ regression |

#### example.org/socket/engineio/payload

| Benchmark | Change | Signal |
| --- | ---: | --- |
| B64Decoder-4 | +5.0% | ℹ️ below threshold |

The improvement is outside the row limit because the configured ordering places below-threshold rows before improvements. Summary counts still cover all selected measurements. The full comparison artifact retains all nine identities.

## Allocation and throughput report

This configuration selects the other three measurements from the same Go run. Separate metric sections expose the opposite improvement directions.

```json
{
  "schema_version": 1,
  "report": {
    "title": "Allocation and throughput changes",
    "metrics": ["allocations", "throughput"],
    "group_by": ["package", "metric"],
    "sections": {
      "metadata": false
    }
  }
}
```

Expected rendering:

### Allocation and throughput changes

**3 of 9 measurements selected · 0 regressions · 2 improvements · 0 below threshold · 1 not comparable**

Signals use a 20% advisory threshold and are not statistical significance tests.

#### example.org/socket/engineio/packet

##### Allocations

Lower is better.

| Benchmark | Base | PR | Change | Signal |
| --- | ---: | ---: | ---: | --- |
| Decoder-4 | 64 allocs/op | 48 allocs/op | -25.0% | ✅ improved |
| Encoder-4 | 0 allocs/op | 2 allocs/op | — | zero baseline |

#### example.org/socket/engineio/payload

##### Throughput

Higher is better.

| Benchmark | Base | PR | Change | Signal |
| --- | ---: | ---: | ---: | --- |
| B64Decoder-4 | 100 MB/s | 125 MB/s | +25.0% | ✅ improved |

## Criterion report

This independent example contains one timing measurement in each of four suites. The values are Criterion point estimates. The captured logs do not provide sample counts to this adapter, so the Samples column is unavailable.

```json
{
  "schema_version": 1,
  "report": {
    "title": "Criterion benchmarks: base vs PR",
    "group_by": ["suite"],
    "columns": ["benchmark", "base", "head", "change", "samples", "signal"]
  }
}
```

Expected rendering:

### Criterion benchmarks: base vs PR

Base `333333333333` → PR `444444444444`

Environment: Linux amd64, runner class `benchmark-demo`, Rust 1.90.0 for both revisions. Each suite's base and PR ran on the same runner. Values are Criterion point estimates.

**4 of 4 measurements selected · 1 regression · 1 improvement · 2 below threshold · 0 not comparable**

Time is lower-is-better. Signals use a 20% advisory threshold. Statistical significance between these runs was not evaluated.

#### client

| Benchmark | Base | PR | Change | Samples | Signal |
| --- | ---: | ---: | ---: | --- | --- |
| rows/decode | 10 µs | 10 µs | 0.0% | — | ℹ️ below threshold |

#### job

| Benchmark | Base | PR | Change | Samples | Signal |
| --- | ---: | ---: | ---: | --- | --- |
| rows/encode | 4.0 µs | 4.2 µs | +5.0% | — | ℹ️ below threshold |

#### skiff

| Benchmark | Base | PR | Change | Samples | Signal |
| --- | ---: | ---: | ---: | --- | --- |
| codec/decode | 2.0 µs | 1.5 µs | -25.0% | — | ✅ improved |

#### yson

| Benchmark | Base | PR | Change | Samples | Signal |
| --- | ---: | ---: | ---: | --- | --- |
| codec/encode | 100 ns | 130 ns | +30.0% | — | ⚠️ regression |

## History JSON

For this export example, the Go head measurements above are treated as a completed primary-branch run. The configuration selects two measurements for a small history example; selection is independent of the Markdown profiles above.

```json
{
  "schema_version": 1,
  "history": {
    "enabled": true,
    "metrics": ["time", "throughput"],
    "include": [
      "^\\[\"go\",\"example\\.org/socket/engineio/packet\",\"Decoder-4\",\"time\"\\]$",
      "^\\[\"go\",\"example\\.org/socket/engineio/payload\",\"B64Decoder-4\",\"throughput\"\\]$"
    ]
  }
}
```

Expected `benchmark-smaller.json`, passed to `customSmallerIsBetter`:

```json
[
  {
    "name": "[\"go\",\"example.org/socket/engineio/packet\",\"Decoder-4\",\"time\"]",
    "unit": "ns/op",
    "value": 120
  }
]
```

Expected `benchmark-bigger.json`, passed to `customBiggerIsBetter`:

```json
[
  {
    "name": "[\"go\",\"example.org/socket/engineio/payload\",\"B64Decoder-4\",\"throughput\"]",
    "unit": "MB/s",
    "value": 125
  }
]
```

These are absolute head values, not the `+20.0%` and `+25.0%` deltas shown in the reports. History series names and publication behavior are covered by the [integration contract](architecture.md#history-integration).

## Empty selection

Applying `report.include: ["does-not-exist"]` to the Go example is a valid display filter. With the default title and metadata disabled, the expected report body is:

### Benchmark comparison

**0 of 9 measurements selected**

No measurements match the report selection. The comparison contains 9 measurements.

## Invalid input

If an expected Criterion suite log is missing, normalization fails before producing a report. An illustrative diagnostic is:

```text
benchreport: base-inputs.json: suite "skiff": missing log "base/skiff.txt"
```

The command exits with code 1. It does not publish a three-suite report that appears to cover all four suites. This differs from the valid empty display selection above.
