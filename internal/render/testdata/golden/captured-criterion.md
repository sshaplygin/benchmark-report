### Benchmark comparison

Base `7b6bba056a06` → PR `c7caaa65bf2c`

Environment: linux amd64, runner github-hosted:ubuntu-24.04, toolchain rust1.94.0 for both revisions.

Criterion values are point estimates; sample counts are unavailable.

**28 of 28 measurements selected · 0 regressions · 0 improvements · 28 below threshold · 0 not comparable**

Time is lower-is-better.

Advisory thresholds: 20% regression, 20% improvement. No statistical analysis was requested.

#### client

| Benchmark | Base | PR | Change | Signal | Samples |
| --- | ---: | ---: | ---: | --- | --- |
| read/read&#95;table/1000 | 244.47 µs | 246.23 µs | +0.7% | ℹ️ below threshold | — |
| read/read&#95;table/10000 | 1.593 ms | 1.595 ms | +0.2% | ℹ️ below threshold | — |
| read/read&#95;table/100000 | 16.179 ms | 16.49 ms | +1.9% | ℹ️ below threshold | — |
| read/read&#95;table&#95;rows/1000 | 569.65 µs | 569.43 µs | 0.0% | ℹ️ below threshold | — |
| read/read&#95;table&#95;rows/10000 | 4.834 ms | 4.861 ms | +0.6% | ℹ️ below threshold | — |
| read/read&#95;table&#95;rows/100000 | 52.794 ms | 51.407 ms | -2.6% | ℹ️ below threshold | — |
| write/encode&#95;then&#95;write&#95;table/1000 | 279.23 µs | 275.98 µs | -1.2% | ℹ️ below threshold | — |
| write/encode&#95;then&#95;write&#95;table/10000 | 1.771 ms | 1.993 ms | +12.6% | ℹ️ below threshold | — |
| write/encode&#95;then&#95;write&#95;table/100000 | 17.349 ms | 17.668 ms | +1.8% | ℹ️ below threshold | — |
| write/write&#95;table&#95;rows/1000 | 290.4 µs | 285.18 µs | -1.8% | ℹ️ below threshold | — |
| write/write&#95;table&#95;rows/10000 | 1.943 ms | 1.91 ms | -1.7% | ℹ️ below threshold | — |
| write/write&#95;table&#95;rows/100000 | 17.948 ms | 17.818 ms | -0.7% | ℹ️ below threshold | — |

#### job

| Benchmark | Base | PR | Change | Signal | Samples |
| --- | ---: | ---: | ---: | --- | --- |
| Skiff job throughput/skiff&#95;dynamic | 49.979 ms | 48.54 ms | -2.9% | ℹ️ below threshold | — |
| YSON job throughput/parse&#95;borrowed | 66.722 ms | 65.449 ms | -1.9% | ℹ️ below threshold | — |
| YSON job throughput/parse&#95;dynamic | 94.804 ms | 92.239 ms | -2.7% | ℹ️ below threshold | — |
| YSON job throughput/parse&#95;owned | 70.025 ms | 68.578 ms | -2.1% | ℹ️ below threshold | — |
| YSON job throughput/pass&#95;through | 20.783 ms | 20.719 ms | -0.3% | ℹ️ below threshold | — |
| YSON vs Skiff dynamic encoding/skiff&#95;dynamic | 21.642 ms | 22.341 ms | +3.2% | ℹ️ below threshold | — |
| YSON vs Skiff dynamic encoding/yson&#95;dynamic | 77.853 ms | 77.602 ms | -0.3% | ℹ️ below threshold | — |
| YSON vs Skiff dynamic job API/skiff&#95;dynamic | 50.582 ms | 48.673 ms | -3.8% | ℹ️ below threshold | — |
| YSON vs Skiff dynamic job API/yson&#95;dynamic | 97.809 ms | 98.563 ms | +0.8% | ℹ️ below threshold | — |

#### skiff

| Benchmark | Base | PR | Change | Signal | Samples |
| --- | ---: | ---: | ---: | --- | --- |
| Skiff codec throughput/decode&#95;dynamic | 32.842 ms | 30.971 ms | -5.7% | ℹ️ below threshold | — |
| Skiff codec throughput/encode&#95;dynamic | 2.966 ms | 2.989 ms | +0.8% | ℹ️ below threshold | — |
| Skiff codec throughput/validate&#95;and&#95;skip | 3.414 ms | 3.381 ms | -1.0% | ℹ️ below threshold | — |

#### yson

| Benchmark | Base | PR | Change | Signal | Samples |
| --- | ---: | ---: | ---: | --- | --- |
| YSON Throughput/Deserialize Binary | 7.786 ms | 7.768 ms | -0.2% | ℹ️ below threshold | — |
| YSON Throughput/Deserialize Text | 9.808 ms | 9.835 ms | +0.3% | ℹ️ below threshold | — |
| YSON Throughput/Serialize Binary | 1.057 ms | 1.051 ms | -0.6% | ℹ️ below threshold | — |
| YSON Throughput/Serialize Text | 3.495 ms | 3.621 ms | +3.6% | ℹ️ below threshold | — |
