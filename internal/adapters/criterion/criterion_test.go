package criterion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/model"
)

func parseText(text string) ([]model.Measurement, error) {
	return Parse(model.Suite{ID: "suite"}, "input.txt", []byte(text))
}

func TestHandwritten(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/handwritten/criterion/valid.txt")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Parse(model.Suite{ID: "suite"}, "valid.txt", data)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{"short": "2", "long/group/benchmark/name": "2000", "millisecond": "2000000", "second": "2000000000"}
	if len(rows) != len(expected) {
		t.Fatalf("got %d rows", len(rows))
	}
	for _, r := range rows {
		if r.Estimate != expected[r.Identity.Benchmark] {
			t.Errorf("%s: %s", r.Identity.Benchmark, r.Estimate)
		}
		if len(r.Samples) != 0 || r.Bounds == nil {
			t.Errorf("invented samples or missing bounds: %#v", r)
		}
		if r.Definition != (model.Definition{Unit: "ns/op", Direction: "lower", Estimator: "criterion-point-estimate"}) {
			t.Errorf("definition: %#v", r.Definition)
		}
		if r.Identity.Package != "" || r.Identity.Metric != "time" || r.Identity.Suite != "suite" || r.Key != r.Identity.Key() {
			t.Errorf("identity: %#v", r)
		}
	}
}

func TestCapturedSuites(t *testing.T) {
	expected := map[string]int{"client": 12, "job": 9, "skiff": 3, "yson": 4}
	var allKeys = map[string]bool{}
	for suite, count := range expected {
		for _, side := range []string{"base", "pr"} {
			t.Run(suite+"/"+side, func(t *testing.T) {
				path := filepath.Join("../../../testdata/captured/criterion-four-suites", "criterion-main-vs-pr-"+suite, side, "benchmarks.txt")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				rows, err := Parse(model.Suite{ID: suite}, path, data)
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != count {
					t.Fatalf("got %d estimates, expected %d", len(rows), count)
				}
				for _, r := range rows {
					if len(r.Samples) != 0 || r.Bounds == nil || r.Identity.Package != "" {
						t.Fatalf("bad measurement %#v", r)
					}
					if side == "base" {
						if allKeys[r.Key] {
							t.Fatal("suite identity collision")
						}
						allKeys[r.Key] = true
					}
					if suite == "skiff" && side == "base" && r.Identity.Benchmark == "Skiff codec throughput/encode_dynamic" {
						if r.Estimate != "2965500" || r.Bounds.Lower != "2876300" || r.Bounds.Upper != "3079200" {
							t.Fatalf("precision lost: %#v", r)
						}
					}
				}
				if suite == "client" {
					for name, want := range map[string]string{"read/read_table/1000": "244470", "read/read_table/10000": "1592800", "read/read_table/100000": "16179000"} {
						found := false
						for _, r := range rows {
							if r.Identity.Benchmark == name {
								found = true
								if side == "base" && r.Estimate != want {
									t.Errorf("inline estimate %s: %s", name, r.Estimate)
								}
							}
						}
						if !found {
							t.Errorf("missing inline %s", name)
						}
					}
				}
			})
		}
	}
	if len(allKeys) != 28 {
		t.Fatalf("expected 28 complete base identities, got %d", len(allKeys))
	}
}

func TestNamesAndInterleaving(t *testing.T) {
	text := "Benchmarking long/name\nBenchmarking long/name: Warming up for 3 s\nignored harness message\nWarning: sample warning\nlong/name\nBenchmarking long/name: Analyzing\n time: [1 ns 2 ns 3 ns]\n change: [-10% 0% 10%]\n thrpt: [1 GiB/s 2 GiB/s 3 GiB/s]\nFound 1 outliers among 10 measurements (10.00%)\n  1 (10.00%) high severe\n"
	rows, err := parseText(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Identity.Benchmark != "long/name" {
		t.Fatalf("%#v", rows)
	}
	rows, err = parseText("name | `雪` <a> [x]\t time: [0 ns 1 ns 2 ns]\n")
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Identity.Benchmark != "name | `雪` <a> [x]" {
		t.Fatal(rows)
	}
	if rows[0].Key != contracts.Key("suite", "", "name | `雪` <a> [x]", "time") {
		t.Fatal(rows[0].Key)
	}
	// Identity remains suite-specific for equal source names.
	other, err := Parse(model.Suite{ID: "other"}, "other.txt", []byte("long/name time: [1 ns 2 ns 3 ns]\n"))
	if err != nil {
		t.Fatal(err)
	}
	same, err := parseText("long/name time: [1 ns 2 ns 3 ns]\n")
	if err != nil {
		t.Fatal(err)
	}
	if same[0].Key == other[0].Key {
		t.Fatal("suite identity lost")
	}
}

func TestSeparateNamesEndingInTime(t *testing.T) {
	for _, name := range []string{"time", "event time"} {
		rows, err := parseText(name + "\n time: [1 ns 2 ns 3 ns]\n")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Identity.Benchmark != name {
			t.Fatalf("name changed: %#v", rows)
		}
	}
}

func TestExactUnitsAndExponents(t *testing.T) {
	rows, err := parseText("mixed time: [1e-3 ms +1.25 us .000002 s]\nzero time: [0 ns 0.000 ns 0e9 ns]\n")
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Estimate != "1250" || *rows[0].Bounds != (model.Bounds{Lower: "1000", Upper: "2000"}) {
		t.Fatal(rows[0])
	}
	if rows[1].Estimate != "0" || *rows[1].Bounds != (model.Bounds{Lower: "0", Upper: "0"}) {
		t.Fatal(rows[1])
	}
}

func TestInvalid(t *testing.T) {
	valid := "valid time: [1 ns 2 ns 3 ns]\n"
	cases := map[string]string{
		"no-name":              "time: [1 ns 2 ns 3 ns]\n",
		"truncated-inline":     valid + "broken time: [1 ns 2 ns\n",
		"truncated-separate":   valid + "broken\n time: [1 ns 2 ns\n",
		"missing-colon":        valid + "broken time [1 ns 2 ns 3 ns]\n",
		"bare-timing":          valid + "time\n",
		"announced-unfinished": valid + "Benchmarking unfinished\nBenchmarking unfinished: Analyzing\n",
		"earlier-unfinished":   "Benchmarking unfinished\n" + valid,
		"duplicate":            valid + valid,
		"nonfinite":            "bad time: [1 ns NaN ns 3 ns]\n",
		"infinite":             "bad time: [1 ns 2 ns Inf ns]\n",
		"negative":             "bad time: [-1 ns 2 ns 3 ns]\n",
		"unsupported-unit":     "bad time: [1 ps 2 ps 3 ps]\n",
		"reversed-lower":       "bad time: [3 ns 2 ns 4 ns]\n",
		"reversed-upper":       "bad time: [1 ns 4 ns 3 ns]\n",
		"invalid-exponent":     "bad time: [1 ns 1e10001 ns 3 ns]\n",
		"extra-value":          "bad time: [1 ns 2 ns 3 ns 4 ns]\n",
		"empty":                "PASS\nNo measurements\n",
		"invalid-utf8":         string([]byte{0xff}),
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			rows, err := parseText(text)
			if err == nil {
				t.Fatalf("unexpected success %#v", rows)
			}
			if rows != nil {
				t.Fatal("partial result returned")
			}
			if !strings.Contains(err.Error(), "input.txt:") || !strings.Contains(err.Error(), `suite "suite"`) {
				t.Fatal(err)
			}
		})
	}
	for _, name := range []string{"criterion-truncated.txt", "criterion-duplicate.txt", "empty.txt"} {
		p := "../../../testdata/handwritten/invalid/" + name
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Parse(model.Suite{ID: "suite"}, p, data); err == nil {
			t.Fatal("accepted", name)
		}
	}
}

func TestDeterministic(t *testing.T) {
	first, err := parseText("z time: [1 ns 2 ns 3 ns]\na time: [2 ns 3 ns 4 ns]\n")
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseText("a time: [2 ns 3 ns 4 ns]\nz time: [1 ns 2 ns 3 ns]\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("input order changes output")
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "samples") {
		t.Fatal("sample count invented")
	}
}

func FuzzParse(f *testing.F) {
	for _, text := range []string{"short time: [1 ns 2 ns 3 ns]\n", "long/name\n time: [1 us 2 µs 3 us]\n", "Benchmarking unfinished\n", "valid time: [0 ns 1 ns 2 ns]\nbroken time: [1 ns\n", "mixed time: [1e-3 ms +1.25 us .000002 s]\n", "valid time: [1 ns 2 ns 3 ns]\nbroken time [1 ns 2 ns 3 ns]\n"} {
		f.Add([]byte(text))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		rows, err := Parse(model.Suite{ID: "fuzz"}, "fuzz.txt", data)
		if err != nil {
			if rows != nil {
				t.Fatal("partial rows on failure")
			}
			return
		}
		seen := map[string]bool{}
		for _, r := range rows {
			if r.Key != r.Identity.Key() || seen[r.Key] || r.Bounds == nil || len(r.Samples) != 0 {
				t.Fatalf("invalid row %#v", r)
			}
			seen[r.Key] = true
		}
	})
}
