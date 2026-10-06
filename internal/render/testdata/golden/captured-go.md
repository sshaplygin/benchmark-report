### Benchmark comparison

Base `79a393cf6e5e` → PR `89368c38fa16`

Environment: linux amd64, runner github-hosted:ubuntu-24.04, toolchain go1.27.1 for both revisions.

Go values are medians of repeated measurements.

**13 of 39 measurements selected · 0 regressions · 0 improvements · 13 below threshold · 0 not comparable**

Time is lower-is-better.

Advisory thresholds: 20% regression, 20% improvement. No statistical analysis was requested.

#### go

##### github.com/sshaplygin/go-socket.io/engineio/packet

| Benchmark | Base | PR | Change | Signal | Samples |
| --- | ---: | ---: | ---: | --- | --- |
| Decoder-4 | 61.145 ns | 61.73 ns | +1.0% | ℹ️ below threshold | 10 / 10 |
| Encoder-4 | 25.535 ns | 25.29 ns | -1.0% | ℹ️ below threshold | 10 / 10 |

##### github.com/sshaplygin/go-socket.io/engineio/payload

| Benchmark | Base | PR | Change | Signal | Samples |
| --- | ---: | ---: | ---: | --- | --- |
| B64Decoder-4 | 2.432 µs | 2.405 µs | -1.1% | ℹ️ below threshold | 10 / 10 |
| B64Encoder-4 | 1.999 µs | 1.969 µs | -1.5% | ℹ️ below threshold | 10 / 10 |
| BinaryDecoder-4 | 2.346 µs | 2.329 µs | -0.7% | ℹ️ below threshold | 10 / 10 |
| BinaryEncoder-4 | 1.272 µs | 1.237 µs | -2.8% | ℹ️ below threshold | 10 / 10 |
| ReadBinaryLen-4 | 131.55 ns | 129.05 ns | -1.9% | ℹ️ below threshold | 10 / 10 |
| ReadStringLen-4 | 134.75 ns | 131.2 ns | -2.6% | ℹ️ below threshold | 10 / 10 |
| StringDecoder-4 | 1.925 µs | 1.883 µs | -2.2% | ℹ️ below threshold | 10 / 10 |
| StringEncoder-4 | 1.269 µs | 1.212 µs | -4.5% | ℹ️ below threshold | 10 / 10 |
| WriteBinaryLen-4 | 141.8 ns | 140.85 ns | -0.7% | ℹ️ below threshold | 10 / 10 |
| WriteStringLen-4 | 142.45 ns | 138.9 ns | -2.5% | ℹ️ below threshold | 10 / 10 |

##### github.com/sshaplygin/go-socket.io/engineio/transport

| Benchmark | Base | PR | Change | Signal | Samples |
| --- | ---: | ---: | ---: | --- | --- |
| ConnParameters-4 | 729.7 ns | 718.95 ns | -1.5% | ℹ️ below threshold | 10 / 10 |
