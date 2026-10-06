package compare

import (
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/schemas"
	"reflect"
)

// Validate checks calculation evidence without reading or changing raw benchmark files.
func Validate(c model.Comparison) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	schema, err := schemas.Read("comparison")
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return err
	}
	if err = config.ValidatePolicy(c.Policy); err != nil {
		return err
	}
	seen := map[string]bool{}
	suites := map[string]bool{}
	sideSuites := []map[string]bool{{}, {}}
	counts := []map[string]int{{}, {}}
	timeKeys := []map[string]bool{{}, {}}
	regression := false
	for _, row := range c.Rows {
		if seen[row.Key] {
			return fmt.Errorf("rows: duplicate measurement %s", row.Key)
		}
		seen[row.Key] = true
		suites[row.Identity.Suite] = true
		for side, m := range []*model.Measurement{row.Base, row.Head} {
			if m == nil {
				continue
			}
			if m.Key != row.Key || m.Identity != row.Identity || m.Definition != row.Definition {
				return fmt.Errorf("measurement %s: row and side identity/definition disagree", row.Key)
			}
			sideSuites[side][m.Identity.Suite] = true
			if _, err := decimal.Parse(m.Estimate); err != nil {
				return fmt.Errorf("measurement %s: %w", row.Key, err)
			}
			if m.Definition.Estimator == "criterion-point-estimate" && m.Identity.Package != "" {
				return fmt.Errorf("measurement %s: Criterion package must be empty", row.Key)
			}
			if m.Definition.Estimator == "median" {
				if err := model.ValidateMeasurement(m); err != nil {
					return err
				}
				benchmark := contracts.Key(m.Identity.Suite, m.Identity.Package, m.Identity.Benchmark, "")
				if n, ok := counts[side][benchmark]; ok && n != len(m.Samples) {
					return fmt.Errorf("measurement %s: inconsistent repeated metric sample counts", row.Key)
				}
				counts[side][benchmark] = len(m.Samples)
				if m.Identity.Metric == "time" {
					timeKeys[side][benchmark] = true
				}
			}
		}
		calculated, err := Row(row.Base, row.Head, c.Policy)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(calculated, row) {
			return fmt.Errorf("measurement %s: stored reason, delta, or signal disagrees with exact evidence and policy", row.Key)
		}
		if row.Signal == "regression" {
			regression = true
		}
	}
	for side, set := range sideSuites {
		if len(set) != len(suites) {
			return fmt.Errorf("comparison: side %d has missing suite evidence", side)
		}
		for key := range counts[side] {
			if !timeKeys[side][key] {
				return fmt.Errorf("comparison: Go benchmark %s missing timing evidence", key)
			}
		}
	}
	mismatches := []string{}
	for _, field := range []struct{ name, a, b string }{{"toolchain", c.Base.Environment.Toolchain, c.Head.Environment.Toolchain}, {"os", c.Base.Environment.OS, c.Head.Environment.OS}, {"arch", c.Base.Environment.Arch, c.Head.Environment.Arch}, {"runner", c.Base.Environment.Runner, c.Head.Environment.Runner}} {
		if field.a != field.b {
			mismatches = append(mismatches, field.name)
		}
	}
	if !reflect.DeepEqual(mismatches, c.EnvironmentOverride.MismatchedFields) || len(mismatches) > 0 && !c.EnvironmentOverride.Allowed {
		return fmt.Errorf("comparison: environment override disclosure disagrees with recorded environments")
	}
	gate := "disabled"
	if c.Policy.FailOnRegression {
		gate = "passed"
		if regression {
			gate = "failed"
		}
	}
	if c.Gate != gate {
		return fmt.Errorf("comparison: gate disagrees with complete classifications")
	}
	if c.Policy.Statistics == "none" {
		if c.Statistics != nil {
			return fmt.Errorf("comparison: statistics present with policy none")
		}
	} else {
		if len(c.Statistics) != len(suites) {
			return fmt.Errorf("comparison: missing per-suite statistical evidence")
		}
		statSuites := map[string]bool{}
		for _, s := range c.Statistics {
			if !suites[s.Suite] || statSuites[s.Suite] {
				return fmt.Errorf("comparison: duplicate or unexpected statistical suite")
			}
			statSuites[s.Suite] = true
			if s.Tool.Name != "benchstat" || s.Tool.Version != contracts.BenchstatVersion {
				return fmt.Errorf("comparison: incompatible recorded benchstat version")
			}
			haveBase, haveHead := false, false
			for _, in := range s.Inputs {
				if in.Suite != s.Suite {
					return fmt.Errorf("comparison: statistical input suite mismatch")
				}
				if in.Side == "base" {
					haveBase = true
				} else {
					haveHead = true
				}
			}
			if !haveBase || !haveHead {
				return fmt.Errorf("comparison: statistical invocation requires both sides")
			}
		}
		for _, r := range c.Rows {
			if r.Definition.Estimator != "median" {
				return fmt.Errorf("comparison: benchstat requires Go measurements")
			}
		}
	}
	return nil
}
