### Go benchmarks: base vs PR

Base `111111111111` → PR `222222222222`

Environment: linux amd64, runner benchmark-demo, toolchain go1.25.0 for both revisions.

Go values are medians of repeated measurements.

**6 of 9 measurements selected · 1 regression · 1 improvement · 1 below threshold · 3 not comparable**

Time is lower-is-better.

Advisory thresholds: 20% regression, 20% improvement. No statistical analysis was requested.

#### example.org/socket/engineio/packet

| Benchmark | Base | PR | Change | Signal |
| --- | ---: | ---: | ---: | --- |
| Decoder-4 | 100 ns | 120 ns | +20.0% | ⚠️ regression |
| Encoder-4 | 80 ns | 60 ns | -25.0% | ✅ improved |

#### example.org/socket/engineio/payload

| Benchmark | Base | PR | Change | Signal |
| --- | ---: | ---: | ---: | --- |
| B64Decoder-4 | 2 µs | 2.1 µs | +5.0% | ℹ️ below threshold |
| NewDecoder-4 | — | 900 ns | — | added |
| RemovedDecoder-4 | 700 ns | — | — | removed |
| ZeroBaseline-4 | 0 ns | 5 ns | — | zero baseline |
