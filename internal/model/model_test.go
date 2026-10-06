package model

import (
	"encoding/json"
	"testing"
	"unicode/utf8"
)

func FuzzIdentity(f *testing.F) {
	f.Add("suite/α", "pkg", "Benchmark<x>/child-4", "time")
	f.Add("a|b", "", "line\n\u2028", "throughput")
	f.Fuzz(func(t *testing.T, suite, pkg, benchmark, metric string) {
		for _, v := range []string{suite, pkg, benchmark, metric} {
			if !utf8.ValidString(v) {
				t.Skip()
			}
		}
		id := Identity{Suite: suite, Package: pkg, Benchmark: benchmark, Metric: metric}
		var decoded []string
		if err := json.Unmarshal([]byte(id.Key()), &decoded); err != nil {
			t.Fatal(err)
		}
		want := []string{suite, pkg, benchmark, metric}
		if len(decoded) != 4 {
			t.Fatal(decoded)
		}
		for i := range want {
			if want[i] != decoded[i] {
				t.Fatal("identity loss")
			}
		}
	})
}
