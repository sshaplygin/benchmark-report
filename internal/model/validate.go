package model

import (
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/schemas"
	"strings"
)

// ValidateRun checks source evidence in addition to structural contract validation.
func ValidateRun(run Run) error {
	data, err := json.Marshal(run)
	if err != nil {
		return err
	}
	schema, err := schemas.Read("normalized-run")
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return err
	}
	counts := map[string]int{}
	times := map[string]bool{}
	goBenchmarks := map[string]bool{}
	for _, m := range run.Measurements {
		if m.Definition.Estimator != "median" {
			continue
		}
		if err := ValidateMeasurement(&m); err != nil {
			return err
		}
		k := contracts.Key(m.Identity.Suite, m.Identity.Package, m.Identity.Benchmark, "")
		if n, ok := counts[k]; ok && n != len(m.Samples) {
			return fmt.Errorf("measurement %s: inconsistent repeated metric sample counts", m.Key)
		}
		counts[k] = len(m.Samples)
		goBenchmarks[k] = true
		if m.Identity.Metric == "time" {
			times[k] = true
		}
	}
	for k := range goBenchmarks {
		if !times[k] {
			return fmt.Errorf("measurement %s: Go benchmark missing timing metric", k)
		}
	}
	return nil
}

// ValidateMeasurement checks exact Go estimate evidence after structural validation.
func ValidateMeasurement(m *Measurement) error {
	if m.Identity.Package == "" || !strings.HasPrefix(m.Identity.Benchmark, "Benchmark") {
		return fmt.Errorf("measurement %s: Go identity requires package and raw Benchmark prefix", m.Key)
	}
	median, err := decimal.Median(m.Samples)
	if err != nil {
		return fmt.Errorf("measurement %s: %w", m.Key, err)
	}
	if m.Estimate != median {
		return fmt.Errorf("measurement %s: estimate %s does not match sample median %s", m.Key, m.Estimate, median)
	}
	return nil
}
