// Package compare joins normalized identities and computes exact advisory changes.
package compare

import (
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/decimal"

	"github.com/sshaplygin/benchmark-report/internal/model"
	"math/big"
	"sort"
)

// Runs compares every input metric. Presentation and history filters are not inputs.
func Runs(base, head model.Run, policy model.Policy, allowEnvironmentMismatch bool, generatorVersion string) (model.Comparison, error) {
	var result model.Comparison
	if err := config.ValidatePolicy(policy); err != nil {
		return result, err
	}
	for _, side := range []struct {
		name string
		run  model.Run
	}{{"base", base}, {"head", head}} {
		if err := model.ValidateRun(side.run); err != nil {
			return result, fmt.Errorf("%s normalized run: %w", side.name, err)
		}
	}
	a, b := append([]string(nil), base.ExpectedSuites...), append([]string(nil), head.ExpectedSuites...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return result, fmt.Errorf("base/head expected suite sets differ")
	}
	for i := range a {
		if a[i] != b[i] {
			return result, fmt.Errorf("base/head expected suite sets differ")
		}
	}
	mismatch := []string{}
	for _, field := range []struct{ name, a, b string }{{"toolchain", base.Environment.Toolchain, head.Environment.Toolchain}, {"os", base.Environment.OS, head.Environment.OS}, {"arch", base.Environment.Arch, head.Environment.Arch}, {"runner", base.Environment.Runner, head.Environment.Runner}} {
		if field.a != field.b {
			mismatch = append(mismatch, field.name)
		}
	}
	if len(mismatch) > 0 && !allowEnvironmentMismatch {
		return result, fmt.Errorf("base/head environment mismatch in %v; use --allow-environment-mismatch for disclosed advisory comparison", mismatch)
	}
	result = model.Comparison{SchemaVersion: 1, Base: model.Side{Revision: base.Revision, Environment: base.Environment}, Head: model.Side{Revision: head.Revision, Environment: head.Environment}, Generator: model.Tool{Name: "benchreport", Version: generatorVersion}, Policy: policy, EnvironmentOverride: model.EnvironmentOverride{Allowed: allowEnvironmentMismatch, MismatchedFields: mismatch}, Rows: []model.ComparisonRow{}, Gate: "disabled"}
	bases, heads := map[string]*model.Measurement{}, map[string]*model.Measurement{}
	keys := map[string]bool{}
	for _, m := range base.Measurements {
		copy := m
		bases[m.Key] = &copy
		keys[m.Key] = true
	}
	for _, m := range head.Measurements {
		copy := m
		heads[m.Key] = &copy
		keys[m.Key] = true
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	hasRegression := false
	for _, key := range ordered {

		row, err := Row(bases[key], heads[key], policy)
		if err != nil {
			return model.Comparison{}, err
		}
		if row.Signal == "regression" {
			hasRegression = true
		}
		result.Rows = append(result.Rows, row)
	}
	if policy.FailOnRegression {
		result.Gate = "passed"
		if hasRegression {
			result.Gate = "failed"
		}
	}
	return result, nil
}
func classify(rounded, direction string, policy model.Policy) string {
	n, _ := new(big.Rat).SetString(rounded)
	if direction == "higher" {
		n.Neg(n)
	}
	regression, _ := decimal.Parse(string(policy.RegressionPercent))
	improvement, _ := decimal.Parse(string(policy.ImprovementPercent))
	if n.Sign() > 0 && n.Cmp(regression) >= 0 {
		return "regression"
	}
	if n.Sign() < 0 && new(big.Rat).Abs(n).Cmp(improvement) >= 0 {
		return "improvement"
	}
	return "below_threshold"
}

// Row computes one identity join without consulting presentation settings.
func Row(left, right *model.Measurement, policy model.Policy) (model.ComparisonRow, error) {
	if left == nil && right == nil {
		return model.ComparisonRow{}, fmt.Errorf("measurement has no sides")
	}

	source := left
	if source == nil {
		source = right
	}
	row := model.ComparisonRow{Key: source.Key, Identity: source.Identity, Definition: source.Definition, Base: left, Head: right, Signal: "not_comparable"}
	switch {
	case left == nil:
		row.Reason = "added"
	case right == nil:
		row.Reason = "removed"
	default:
		if left.Definition != right.Definition {
			return model.ComparisonRow{}, fmt.Errorf("measurement %s: incompatible unit, direction, or estimator", source.Key)
		}
		baseline, err := decimal.Parse(left.Estimate)
		if err != nil {
			return model.ComparisonRow{}, err
		}
		current, err := decimal.Parse(right.Estimate)
		if err != nil {
			return model.ComparisonRow{}, err
		}
		if baseline.Sign() == 0 && current.Sign() != 0 {
			row.Reason = "zero_baseline"
		} else {
			row.Reason = "comparable"
			percentage := new(big.Rat)
			if baseline.Sign() != 0 {
				percentage.Quo(current, baseline)
				percentage.Sub(percentage, big.NewRat(1, 1))
				percentage.Mul(percentage, big.NewRat(100, 1))
			}
			rounded, err := decimal.RoundSigned(percentage, policy.PercentDecimals)
			if err != nil {
				return model.ComparisonRow{}, fmt.Errorf("measurement %s: %w", source.Key, err)
			}
			row.DeltaPercent = &rounded
			row.Signal = classify(rounded, source.Definition.Direction, policy)

		}
	}
	return row, nil
}
