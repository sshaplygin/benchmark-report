<!-- go-socket.io:benchmark-comparison -->
## 📊 Go benchmarks: base vs PR

🔍 Base `79a393cf6e5e` → PR `89368c38fa16`

**📊 13 benchmark(s) compared · ⚠️ 0 regression(s) at ≥20% · ✅ 0 improvement(s) at ≥20%**

ℹ️ Time is lower-is-better. Values are medians of repeated measurements. Both revisions ran on the same GitHub-hosted VM and Go toolchain; ±20% is an advisory marker, not a merge gate. Markers use the displayed percentage, not a statistical significance test.

### engineio/packet

| Benchmark | base | PR | change | signal |
| --- | ---: | ---: | ---: | --- |
| Decoder-4 | 61.15 ns | 61.73 ns | +1.0% | ℹ️ below threshold |
| Encoder-4 | 25.54 ns | 25.29 ns | -1.0% | ℹ️ below threshold |

### engineio/payload

| Benchmark | base | PR | change | signal |
| --- | ---: | ---: | ---: | --- |
| B64Decoder-4 | 2.432 µs | 2.405 µs | -1.1% | ℹ️ below threshold |
| B64Encoder-4 | 1.999 µs | 1.969 µs | -1.5% | ℹ️ below threshold |
| BinaryDecoder-4 | 2.346 µs | 2.329 µs | -0.7% | ℹ️ below threshold |
| BinaryEncoder-4 | 1.272 µs | 1.237 µs | -2.8% | ℹ️ below threshold |
| ReadBinaryLen-4 | 131.6 ns | 129.1 ns | -1.9% | ℹ️ below threshold |
| ReadStringLen-4 | 134.8 ns | 131.2 ns | -2.6% | ℹ️ below threshold |
| StringDecoder-4 | 1.925 µs | 1.883 µs | -2.2% | ℹ️ below threshold |
| StringEncoder-4 | 1.269 µs | 1.212 µs | -4.5% | ℹ️ below threshold |
| WriteBinaryLen-4 | 141.8 ns | 140.9 ns | -0.7% | ℹ️ below threshold |
| WriteStringLen-4 | 142.5 ns | 138.9 ns | -2.5% | ℹ️ below threshold |

### engineio/transport

| Benchmark | base | PR | change | signal |
| --- | ---: | ---: | ---: | --- |
| ConnParameters-4 | 729.7 ns | 719 ns | -1.5% | ℹ️ below threshold |

<details>
<summary>Full benchstat results: timings, allocations and statistical comparisons</summary>

```text
goos: linux
goarch: amd64
pkg: github.com/sshaplygin/go-socket.io/engineio/packet
cpu: INTEL(R) XEON(R) PLATINUM 8573C
          │  base.txt   │               pr.txt               │
          │   sec/op    │   sec/op     vs base               │
Decoder-4   61.15n ± 3%   61.73n ± 2%       ~ (p=0.383 n=10)
Encoder-4   25.54n ± 2%   25.29n ± 2%  -0.96% (p=0.037 n=10)
geomean     39.51n        39.51n       -0.01%

          │  base.txt  │               pr.txt                │
          │    B/op    │    B/op     vs base                 │
Decoder-4   34.00 ± 0%   34.00 ± 0%       ~ (p=1.000 n=10) ¹
Encoder-4   2.000 ± 0%   2.000 ± 0%       ~ (p=1.000 n=10) ¹
geomean     8.246        8.246       +0.00%
¹ all samples are equal

          │  base.txt  │               pr.txt                │
          │ allocs/op  │ allocs/op   vs base                 │
Decoder-4   4.000 ± 0%   4.000 ± 0%       ~ (p=1.000 n=10) ¹
Encoder-4   2.000 ± 0%   2.000 ± 0%       ~ (p=1.000 n=10) ¹
geomean     2.828        2.828       +0.00%
¹ all samples are equal

pkg: github.com/sshaplygin/go-socket.io/engineio/payload
                 │  base.txt   │               pr.txt               │
                 │   sec/op    │   sec/op     vs base               │
StringDecoder-4    1.925µ ± 3%   1.883µ ± 2%  -2.18% (p=0.012 n=10)
B64Decoder-4       2.432µ ± 1%   2.405µ ± 1%       ~ (p=0.060 n=10)
BinaryDecoder-4    2.346µ ± 1%   2.329µ ± 1%  -0.70% (p=0.021 n=10)
StringEncoder-4    1.269µ ± 1%   1.212µ ± 2%  -4.49% (p=0.000 n=10)
B64Encoder-4       1.999µ ± 2%   1.969µ ± 1%  -1.50% (p=0.007 n=10)
BinaryEncoder-4    1.272µ ± 5%   1.236µ ± 2%  -2.79% (p=0.000 n=10)
WriteStringLen-4   142.4n ± 3%   138.9n ± 2%  -2.49% (p=0.020 n=10)
WriteBinaryLen-4   141.8n ± 4%   140.8n ± 3%       ~ (p=0.926 n=10)
ReadStringLen-4    134.8n ± 3%   131.2n ± 3%  -2.63% (p=0.015 n=10)
ReadBinaryLen-4    131.6n ± 3%   129.1n ± 4%       ~ (p=0.172 n=10)
geomean            646.1n        632.9n       -2.05%

                 │    base.txt    │                pr.txt                 │
                 │      B/op      │     B/op      vs base                 │
StringDecoder-4    4.164Ki ± 0%     4.164Ki ± 0%       ~ (p=0.526 n=10)
B64Decoder-4       6.184Ki ± 0%     6.184Ki ± 0%       ~ (p=0.628 n=10)
BinaryDecoder-4    4.164Ki ± 0%     4.164Ki ± 0%       ~ (p=0.628 n=10)
StringEncoder-4      0.000 ± 0%       0.000 ± 0%       ~ (p=1.000 n=10) ¹
B64Encoder-4       3.375Ki ± 0%     3.375Ki ± 0%       ~ (p=1.000 n=10) ¹
BinaryEncoder-4      0.000 ± 0%       0.000 ± 0%       ~ (p=1.000 n=10) ¹
WriteStringLen-4     0.000 ± 0%       0.000 ± 0%       ~ (p=1.000 n=10) ¹
WriteBinaryLen-4     0.000 ± 0%       0.000 ± 0%       ~ (p=1.000 n=10) ¹
ReadStringLen-4      0.000 ± 0%       0.000 ± 0%       ~ (p=1.000 n=10) ¹
ReadBinaryLen-4      0.000 ± 0%       0.000 ± 0%       ~ (p=1.000 n=10) ¹
geomean                         ²                 +0.00%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                 │   base.txt   │               pr.txt                │
                 │  allocs/op   │ allocs/op   vs base                 │
StringDecoder-4    4.000 ± 0%     4.000 ± 0%       ~ (p=1.000 n=10) ¹
B64Decoder-4       6.000 ± 0%     6.000 ± 0%       ~ (p=1.000 n=10) ¹
BinaryDecoder-4    4.000 ± 0%     4.000 ± 0%       ~ (p=1.000 n=10) ¹
StringEncoder-4    0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=10) ¹
B64Encoder-4       3.000 ± 0%     3.000 ± 0%       ~ (p=1.000 n=10) ¹
BinaryEncoder-4    0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=10) ¹
WriteStringLen-4   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=10) ¹
WriteBinaryLen-4   0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=10) ¹
ReadStringLen-4    0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=10) ¹
ReadBinaryLen-4    0.000 ± 0%     0.000 ± 0%       ~ (p=1.000 n=10) ¹
geomean                       ²               +0.00%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

pkg: github.com/sshaplygin/go-socket.io/engineio/transport
                 │  base.txt   │               pr.txt               │
                 │   sec/op    │   sec/op     vs base               │
ConnParameters-4   729.7n ± 2%   719.0n ± 0%  -1.47% (p=0.002 n=10)

                 │  base.txt  │             pr.txt             │
                 │    B/op    │    B/op     vs base            │
ConnParameters-4   152.0 ± 0%   152.0 ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal

                 │  base.txt  │             pr.txt             │
                 │ allocs/op  │ allocs/op   vs base            │
ConnParameters-4   3.000 ± 0%   3.000 ± 0%  ~ (p=1.000 n=10) ¹
¹ all samples are equal
```

</details>
