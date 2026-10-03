package compare

import (
	"encoding/json"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"os"
	"strings"
	"testing"
)

func run(value, metric string) model.Run {
	identity := model.Identity{Suite: "suite", Package: "p", Benchmark: "BenchmarkX-4", Metric: metric}
	definition := model.Definition{Unit: "ns/op", Direction: "lower", Estimator: "median"}
	if metric == "throughput" {
		definition.Unit = "B/s"
		definition.Direction = "higher"
	}
	measurement := model.Measurement{Key: identity.Key(), Identity: identity, Definition: definition, Estimate: value, Samples: []string{value}}
	result := model.Run{Manifest: model.Manifest{SchemaVersion: 1, Revision: strings.Repeat("a", 40), Environment: model.Environment{Toolchain: "go1.25.0", OS: "linux", Arch: "amd64", Runner: "test"}, ExpectedSuites: []string{"suite"}, Suites: []model.Suite{{ID: "suite", Parser: model.Parser{Name: "go", Version: "1"}, Command: "go test -bench .", Files: []string{"raw.txt"}}}}, Inputs: []model.Input{{Suite: "suite", Path: "raw.txt", SHA256: strings.Repeat("0", 64)}}, Measurements: []model.Measurement{measurement}}
	if metric != "time" {
		timing := measurement
		timing.Identity.Metric = "time"
		timing.Key = timing.Identity.Key()
		timing.Definition = model.Definition{Unit: "ns/op", Direction: "lower", Estimator: "median"}
		timing.Estimate = "1"
		timing.Samples = []string{"1"}
		result.Measurements = append(result.Measurements, timing)
	}
	return result
}
func rowFor(t *testing.T, c model.Comparison, metric string) model.ComparisonRow {
	t.Helper()
	for _, row := range c.Rows {
		if row.Identity.Metric == metric {
			return row
		}
	}
	t.Fatal("missing metric", metric)
	return model.ComparisonRow{}
}
func TestHandCalculatedFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/handwritten/numerical.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			ID        string  `json:"id"`
			Base      string  `json:"base"`
			Head      string  `json:"head"`
			Direction string  `json:"direction"`
			Rounded   *string `json:"rounded_percent"`
			Signal    string  `json:"signal"`
			Reason    string  `json:"reason"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Cases {
		t.Run(test.ID, func(t *testing.T) {
			metric := "time"
			if test.Direction == "higher" {
				metric = "throughput"
			}
			c, err := Runs(run(test.Base, metric), run(test.Head, metric), config.Defaults().Comparison, false, "test")
			if err != nil {
				t.Fatal(err)
			}
			row := rowFor(t, c, metric)
			if row.Reason != test.Reason || row.Signal != test.Signal {
				t.Fatalf("%+v", row)
			}
			if (row.DeltaPercent == nil) != (test.Rounded == nil) {
				t.Fatal("incorrect availability")
			}
			if test.Rounded != nil && *row.DeltaPercent != *test.Rounded {
				t.Fatalf("want%s got%s", *test.Rounded, *row.DeltaPercent)
			}
		})
	}
}
func TestUnionGateAndOrder(t *testing.T) {
	base, head := run("100", "time"), run("120", "time")
	removed := base.Measurements[0]
	removed.Identity.Benchmark = "BenchmarkRemoved-4"
	removed.Key = removed.Identity.Key()
	base.Measurements = append(base.Measurements, removed)
	added := head.Measurements[0]
	added.Identity.Benchmark = "BenchmarkAdded-4"
	added.Key = added.Identity.Key()
	head.Measurements = append(head.Measurements, added)
	policy := config.Defaults().Comparison
	policy.FailOnRegression = true
	c, err := Runs(base, head, policy, false, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Rows) != 3 || c.Gate != "failed" {
		t.Fatal(c)
	}
	for i, row := range c.Rows {
		if i > 0 && c.Rows[i-1].Key >= row.Key {
			t.Fatal("not canonical order")
		}
		if row.Identity.Benchmark == "BenchmarkAdded-4" && (row.Reason != "added" || row.Base != nil || row.DeltaPercent != nil) {
			t.Fatal(row)
		}
		if row.Identity.Benchmark == "BenchmarkRemoved-4" && (row.Reason != "removed" || row.Head != nil || row.DeltaPercent != nil) {
			t.Fatal(row)
		}
	}
	policy.FailOnRegression = false
	c, err = Runs(base, head, policy, false, "test")
	if err != nil || c.Gate != "disabled" {
		t.Fatal(c.Gate, err)
	}
}
func TestEnvironmentAndSuiteCompatibility(t *testing.T) {
	base, head := run("1", "time"), run("1", "time")
	head.Environment.Toolchain = "different"
	head.Environment.Runner = "other"
	if _, err := Runs(base, head, config.Defaults().Comparison, false, "test"); err == nil {
		t.Fatal("allowed mismatch")
	}
	c, err := Runs(base, head, config.Defaults().Comparison, true, "test")
	if err != nil || !c.EnvironmentOverride.Allowed || strings.Join(c.EnvironmentOverride.MismatchedFields, ",") != "toolchain,runner" {
		t.Fatal(c.EnvironmentOverride, err)
	}
	head.ExpectedSuites = []string{"missing"}
	if _, err := Runs(base, head, config.Defaults().Comparison, true, "test"); err == nil {
		t.Fatal("allowed incomplete suite")
	}
}
func TestTamperedEvidenceAndDefinition(t *testing.T) {
	for _, mutate := range []func(*model.Run){func(r *model.Run) { r.Measurements[0].Estimate = "2" }, func(r *model.Run) { r.Measurements[0].Definition.Unit = "B/op" }, func(r *model.Run) { r.Measurements[0].Definition.Direction = "higher" }, func(r *model.Run) { r.Measurements[0].Definition.Estimator = "criterion-point-estimate" }, func(r *model.Run) { r.Measurements[0].Samples = []string{"1", "2", "3", "4"} }} {
		base, head := run("1", "time"), run("1", "time")
		mutate(&head)
		if _, err := Runs(base, head, config.Defaults().Comparison, true, "test"); err == nil {
			t.Fatal("allowed incompatible evidence")
		}
	}
}
func TestPrecisionAndAsymmetricThresholds(t *testing.T) {
	policy := config.Defaults().Comparison
	policy.RegressionPercent = json.Number("6.3")
	policy.ImprovementPercent = json.Number("10")
	policy.PercentDecimals = 1
	for _, test := range []struct{ head, signal string }{{"170", "regression"}, {"150", "below_threshold"}} {
		c, err := Runs(run("160", "time"), run(test.head, "time"), policy, false, "test")
		if err != nil || c.Rows[0].Signal != test.signal {
			t.Fatal(c, err)
		}
	}
	policy.PercentDecimals = 0
	c, err := Runs(run("160", "time"), run("170", "time"), policy, false, "test")
	if err != nil || *c.Rows[0].DeltaPercent != "6" || c.Rows[0].Signal != "below_threshold" {
		t.Fatal(c, err)
	}
}
