package gobench

import (
	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"os"
	"strings"
	"testing"
)

func parse(t *testing.T, text string) []model.Measurement {
	t.Helper()
	rows, err := Parse(model.Suite{ID: "go"}, "fixture.txt", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
func TestHandCalculated(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/handwritten/go/valid.txt")
	if err != nil {
		t.Fatal(err)
	}
	rows := parse(t, string(data))
	if len(rows) != 10 {
		t.Fatalf("got%d measurements", len(rows))
	}
	want := map[string]string{"time": "2.5", "bytes": "3", "allocations": "1.5", "throughput": "2500000"}
	for _, r := range rows {
		if r.Identity.Package == "example.org/a" {
			if r.Estimate != want[r.Identity.Metric] || len(r.Samples) != 4 {
				t.Fatalf("bad median: %+v", r)
			}
		}
		if r.Identity.Benchmark != "BenchmarkSame/sub-4" && r.Identity.Benchmark != "BenchmarkSame/sub-8" {
			t.Fatal("lost identity", r.Identity)
		}
	}
}
func TestCaptured(t *testing.T) {
	for _, side := range []string{"base", "pr"} {
		data, err := os.ReadFile("../../../testdata/captured/go-pr15/go-benchmarks/" + side + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		rows := parse(t, string(data))
		if len(rows) != 39 {
			t.Fatalf("%s got%d rows", side, len(rows))
		}
		packages := map[string]bool{}
		for _, r := range rows {
			packages[r.Identity.Package] = true
			if len(r.Samples) != 10 {
				t.Fatalf("%s %s samples=%d", side, r.Key, len(r.Samples))
			}
		}
		if len(packages) != 3 {
			t.Fatal("package identity lost")
		}
	}
}
func TestInvalid(t *testing.T) {
	for _, fixture := range []string{"go-negative", "go-nonfinite", "go-truncated", "go-unsupported", "empty"} {
		data, err := os.ReadFile("../../../testdata/handwritten/invalid/" + fixture + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Parse(model.Suite{ID: "s"}, fixture+".txt", data); err == nil {
			t.Fatal("accepted", fixture)
		}
	}
	for _, text := range []string{
		"BenchmarkX-4 1 2 ns/op\n",
		"pkg: p\nBenchmarkX-4 0 2 ns/op\n",
		"pkg: p\nBenchmarkX-4 x 2 ns/op\n",
		"pkg: p\nBenchmarkX-4 1 2 ns/op 3 ns/op\n",
		"pkg: p\nBenchmarkX-4 1 2 B/op\n",
		"pkg: p\nBenchmarkX-4 1 2 ns/op\nBenchmarkIncomplete-4\n",
		"pkg: p\nBenchmarkX-4 1 2 ns/op\nFAIL\tp\n",
		"pkg: p\nBenchmarkX-4 1 2 ns/op 3 B/op\nBenchmarkX-4 1 2 ns/op\n",
		"pkg: p\nBenchmarkX-4 1 2 ns/op trailing\n",
	} {
		if _, err := Parse(model.Suite{ID: "s"}, "bad.txt", []byte(text)); err == nil || !strings.Contains(err.Error(), "bad.txt:") {
			t.Fatalf("invalid accepted or no location %q %v", text, err)
		}
	}
}
func TestProgressAndScientific(t *testing.T) {
	rows := parse(t, "goos: linux\npkg: p\nBenchmarkX\nBenchmarkX-4 1 1.25e2 ns/op 0 B/op\nPASS\nok p\n")
	for _, r := range rows {
		if r.Identity.Metric == "time" && r.Estimate != "125" {
			t.Fatal(r)
		}
	}
}
func FuzzParse(f *testing.F) {
	for _, seed := range []string{"pkg: p\nBenchmarkX-4 1 2 ns/op\n", "pkg: p\nBenchmarkX\n", "pkg: p\nBenchmarkX-4 1 NaN ns/op\n", "pkg: p\nBenchmarkX-4 1 1e5000 ns/op\n", "pkg: p\nBenchmarkParent\nBenchmarkParent/child\nBenchmarkParent/child-4 10 1 ns/op\n"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		rows, err := Parse(model.Suite{ID: "suite|µ"}, "fuzz.txt", []byte(text))
		if err != nil {
			return
		}
		for _, r := range rows {
			if r.Key != r.Identity.Key() {
				t.Fatal("noncanonical key")
			}
			n, err := decimal.Normalize(r.Estimate)
			if err != nil || n != r.Estimate {
				t.Fatalf("noncanonical estimate %s: %v", r.Estimate, err)
			}
			if len(r.Samples) == 0 {
				t.Fatal("lost samples")
			}
		}
	})
}

func TestVerboseParentHeadings(t *testing.T) {
	rows := parse(t, "pkg: p\nBenchmarkParent\nBenchmarkParent/child\nBenchmarkParent/child-4 10 1 ns/op\n")
	if len(rows) != 1 || rows[0].Identity.Benchmark != "BenchmarkParent/child-4" {
		t.Fatal(rows)
	}
}
