### Allocation and throughput changes

**3 of 9 measurements selected · 0 regressions · 2 improvements · 0 below threshold · 1 not comparable**

Allocations are lower-is-better.

Throughput is higher-is-better.

Advisory thresholds: 20% regression, 20% improvement. No statistical analysis was requested.

#### example.org/socket/engineio/packet

##### Allocations

| Benchmark | Base | PR | Change | Signal |
| --- | ---: | ---: | ---: | --- |
| Decoder-4 | 64 allocs/op | 48 allocs/op | -25.0% | ✅ improved |
| Encoder-4 | 0 allocs/op | 2 allocs/op | — | zero baseline |

#### example.org/socket/engineio/payload

##### Throughput

| Benchmark | Base | PR | Change | Signal |
| --- | ---: | ---: | ---: | --- |
| B64Decoder-4 | 100 MB/s | 125 MB/s | +25.0% | ✅ improved |
