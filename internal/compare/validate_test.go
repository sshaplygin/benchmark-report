package compare

import (
	"encoding/json"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"os"
	"testing"
)

func artifact(t *testing.T) model.Comparison {
	t.Helper()
	data, err := os.ReadFile("../../examples/contracts/comparison.json")
	if err != nil {
		t.Fatal(err)
	}
	var c model.Comparison
	if err = json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	return c
}
func TestStoredEvidenceValidation(t *testing.T) {
	if err := Validate(artifact(t)); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []func(*model.Comparison){func(c *model.Comparison) { s := "19"; c.Rows[0].DeltaPercent = &s }, func(c *model.Comparison) { c.Rows[0].Signal = "below_threshold" }, func(c *model.Comparison) { c.Gate = "failed" }, func(c *model.Comparison) { c.Rows[0].Head.Estimate = "4" }, func(c *model.Comparison) { c.Rows[0].Base.Identity.Benchmark = "other" }, func(c *model.Comparison) { c.Rows = append(c.Rows, c.Rows[0]) }, func(c *model.Comparison) { c.Head.Environment.OS = "different" }, func(c *model.Comparison) { c.Policy.Statistics = "benchstat" }, func(c *model.Comparison) {
		c.Rows[0].Head = nil
		c.Rows[0].Reason = "removed"
		c.Rows[0].Signal = "not_comparable"
		c.Rows[0].DeltaPercent = nil
	}} {
		c := artifact(t)
		mutation(&c)
		if err := Validate(c); err == nil {
			t.Fatal("tampered artifact accepted")
		}
	}
}
