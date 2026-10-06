package input

import (
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/schemas"
	"os"
	"strings"
)

// LoadRun reads a complete normalized run without requiring archived raw files.
func LoadRun(path string) (model.Run, error) {
	var run model.Run
	data, err := os.ReadFile(path)
	if err != nil {
		return run, fmt.Errorf("%s: %w", path, err)
	}
	schema, err := schemas.Read("normalized-run")
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return run, fmt.Errorf("%s: %w", path, err)
	}
	if err = contracts.Unmarshal(data, &run); err != nil {
		return run, fmt.Errorf("%s: %w", path, err)
	}
	if err = ValidateRun(run); err != nil {
		return run, fmt.Errorf("%s: %w", path, err)
	}
	return run, nil
}

// ValidateRun checks source evidence in addition to structural contract validation.
func ValidateRun(run model.Run) error {
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
