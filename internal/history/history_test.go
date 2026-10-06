package history

import (
	"encoding/json"
	"github.com/sshaplygin/benchmark-report/internal/adapters/criterion"
	"github.com/sshaplygin/benchmark-report/internal/adapters/gobench"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func goRun(t *testing.T) model.Run {
	t.Helper()
	source, err := os.ReadFile("../../testdata/handwritten/go/valid.txt")
	if err != nil {
		t.Fatal(err)
	}
	suite := model.Suite{ID: "go", Parser: model.Parser{Name: "go", Version: "1"}, Command: "go test -bench .", Files: []string{"raw.txt"}}
	measurements, err := gobench.Parse(suite, "raw.txt", source)
	if err != nil {
		t.Fatal(err)
	}
	return model.Run{Manifest: model.Manifest{SchemaVersion: 1, Revision: strings.Repeat("a", 40), Environment: model.Environment{Toolchain: "go1.25.0", OS: "linux", Arch: "amd64", Runner: "fixture"}, ExpectedSuites: []string{"go"}, Suites: []model.Suite{suite}}, Inputs: []model.Input{{Suite: "go", Path: "raw.txt", SHA256: strings.Repeat("0", 64)}}, Measurements: measurements}
}
func allHistory() config.History {
	h := config.Defaults().History
	h.Enabled = true
	h.Metrics = []string{"time", "bytes", "allocations", "throughput"}
	return h
}
func TestHandCalculatedAbsoluteValues(t *testing.T) {
	run := goRun(t)
	files, err := Build(run, allHistory())
	if err != nil {
		t.Fatal(err)
	}
	var smaller, bigger []Measurement
	if err = json.Unmarshal(files.Smaller, &smaller); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(files.Bigger, &bigger); err != nil {
		t.Fatal(err)
	}
	if len(smaller) != 9 || len(bigger) != 1 {
		t.Fatalf("got%d/%d", len(smaller), len(bigger))
	}
	expected := map[string]string{"time": "2.5", "bytes": "3", "allocations": "1.5", "throughput": "2500000"}
	for _, entry := range append(smaller, bigger...) {
		var id []string
		if err = json.Unmarshal([]byte(entry.Name), &id); err != nil {
			t.Fatal(err)
		}
		if id[1] == "example.org/a" && string(entry.Value) != expected[id[3]] {
			t.Fatal(entry)
		}
		if entry.Range != "" || !strings.Contains(entry.Extra, `"estimator":"median"`) {
			t.Fatal(entry)
		}
	}
	if !strings.Contains(string(files.Smaller), `"value": 2.5`) {
		t.Fatal("numeric value was stringified")
	}
}
func TestIndependentFiltersAndDeterminism(t *testing.T) {
	run := goRun(t)
	cfg := config.Defaults()
	cfg.History = allHistory()
	cfg.History.Include = []string{`example.org/a`}
	cfg.History.Exclude = []string{`throughput`}
	first, err := Build(run, cfg.History)
	if err != nil {
		t.Fatal(err)
	}
	if first.Bigger != nil {
		t.Fatal("exclude did not win")
	}
	cfg.Report.Title = "Other"
	cfg.Report.Metrics = []string{"throughput"}
	cfg.Report.Exclude = []string{".*"}
	cfg.Report.MaxRows = "1"
	second, err := Build(run, cfg.History)
	if err != nil || string(first.Smaller) != string(second.Smaller) {
		t.Fatal("presentation affected export", err)
	}
	cfg.History.Exclude = []string{".*"}
	if _, err := Build(run, cfg.History); err == nil {
		t.Fatal("empty selection accepted")
	}
	cfg.History.Enabled = false
	if _, err := Build(run, cfg.History); err == nil {
		t.Fatal("disabled history accepted")
	}
}
func TestCriterionBounds(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "run.json")
	if err := input.Normalize("criterion", "../../testdata/captured/criterion-four-suites/head-manifest.json", out, criterion.Parse); err != nil {
		t.Fatal(err)
	}
	run, err := input.LoadRun(out)
	if err != nil {
		t.Fatal(err)
	}
	h := config.Defaults().History
	h.Enabled = true
	files, err := Build(run, h)
	if err != nil {
		t.Fatal(err)
	}
	var entries []Measurement
	if err = json.Unmarshal(files.Smaller, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 28 || files.Bigger != nil {
		t.Fatal("lost suite measurements")
	}
	for i, entry := range entries {
		measurement := run.Measurements[i]
		if entry.Name != measurement.Key || string(entry.Value) != measurement.Estimate || entry.Range != measurement.Bounds.Lower+".."+measurement.Bounds.Upper+" ns/op" {
			t.Fatal("different absolute estimate/bounds", entry)
		}
		if !strings.Contains(entry.Extra, `"criterion-point-estimate"`) {
			t.Fatal(entry)
		}
	}
}
func TestBinary64HistoryRoundTrip(t *testing.T) {
	for _, value := range []string{"0", "0.1", "61.145", "9007199254740992", "100000000000000000000"} {
		if err := compatibleNumber(value); err != nil {
			t.Fatal("ordinary value rejected", value, err)
		}
	}
	for _, value := range []string{"9007199254740993", "0.1000000000000000000000000000000001", "1" + strings.Repeat("0", 309), "0." + strings.Repeat("0", 323) + "1"} {
		if err := compatibleNumber(value); err == nil {
			t.Fatal("corrupt history value accepted", value)
		}
	}
	run := goRun(t)
	for i := range run.Measurements {
		if run.Measurements[i].Identity.Metric == "time" {
			run.Measurements[i].Estimate = "9007199254740993"
			run.Measurements[i].Samples = make([]string, len(run.Measurements[i].Samples))
			for j := range run.Measurements[i].Samples {
				run.Measurements[i].Samples[j] = "9007199254740993"
			}
		}
	}
	if _, err := Build(run, allHistory()); err == nil || !strings.Contains(err.Error(), `measurement ["go"`) {
		t.Fatal("missing identity on numeric incompatibility", err)
	}
}
