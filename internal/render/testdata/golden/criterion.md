### Criterion benchmarks: base vs PR

Base `333333333333` → PR `444444444444`

Environment: linux amd64, runner benchmark-demo, toolchain rust1.90.0 for both revisions.

Criterion values are point estimates; sample counts are unavailable.

**4 of 4 measurements selected · 1 regression · 1 improvement · 2 below threshold · 0 not comparable**

Time is lower-is-better.

Advisory thresholds: 20% regression, 20% improvement. No statistical analysis was requested.

#### client

| Benchmark | Base | PR | Change | Samples | Signal |
| --- | ---: | ---: | ---: | --- | --- |
| rows/decode | 10 µs | 10 µs | 0.0% | — | ℹ️ below threshold |

#### job

| Benchmark | Base | PR | Change | Samples | Signal |
| --- | ---: | ---: | ---: | --- | --- |
| rows/encode | 4 µs | 4.2 µs | +5.0% | — | ℹ️ below threshold |

#### skiff

| Benchmark | Base | PR | Change | Samples | Signal |
| --- | ---: | ---: | ---: | --- | --- |
| codec/decode | 2 µs | 1.5 µs | -25.0% | — | ✅ improved |

#### yson

| Benchmark | Base | PR | Change | Samples | Signal |
| --- | ---: | ---: | ---: | --- | --- |
| codec/encode | 100 ns | 130 ns | +30.0% | — | ⚠️ regression |
