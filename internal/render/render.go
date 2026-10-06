// Package render selects and displays an existing comparison without changing it.
package render

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"

	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/internal/model"
)

// Build creates both presentation models in memory. The caller owns output files.
func Build(comparison model.Comparison, cfg config.Config) (model.Presentation, []byte, error) {
	var p model.Presentation
	if !config.PolicyEqual(comparison.Policy, cfg.Comparison) {
		return p, nil, fmt.Errorf("render comparison policy differs from recorded policy")
	}
	includes, err := patterns(cfg.Report.Include)
	if err != nil {
		return p, nil, fmt.Errorf("report.include: %w", err)
	}
	excludes, err := patterns(cfg.Report.Exclude)
	if err != nil {
		return p, nil, fmt.Errorf("report.exclude: %w", err)
	}
	selected := []model.ComparisonRow{}
	for _, row := range comparison.Rows {
		if !contains(cfg.Report.Metrics, row.Identity.Metric) || len(includes) > 0 && !matches(includes, row.Key) || matches(excludes, row.Key) {
			continue
		}
		if cfg.Report.Unchanged == "hide" && row.Base != nil && row.Head != nil && row.Base.Estimate == row.Head.Estimate {
			continue
		}
		if cfg.Report.Missing == "hide" && (row.Reason == "added" || row.Reason == "removed") {
			continue
		}
		selected = append(selected, row)
	}
	metrics := map[string]bool{}
	for _, row := range selected {
		metrics[row.Identity.Metric] = true
	}
	if len(metrics) > 1 && !contains(cfg.Report.GroupBy, "metric") && !contains(cfg.Report.Columns, "metric") {
		return p, nil, fmt.Errorf("multiple selected metrics require metric grouping or column")
	}
	sort.Slice(selected, func(i, j int) bool {
		if cfg.Report.Sort == "regression" && rank(selected[i].Signal) != rank(selected[j].Signal) {
			return rank(selected[i].Signal) < rank(selected[j].Signal)
		}
		return selected[i].Key < selected[j].Key
	})
	sections := cfg.Report.Sections
	p = model.Presentation{SchemaVersion: 1, Title: cfg.Report.Title, Columns: append([]string{}, cfg.Report.Columns...), Base: comparison.Base, Head: comparison.Head, Sections: model.PresentationSections{Metadata: sections.Metadata, Summary: sections.Summary, Tables: sections.Tables, Benchstat: sections.Benchstat}, Summary: model.PresentationSummary{Total: len(comparison.Rows), Selected: len(selected)}, Disclosures: []string{}, Groups: []model.PresentationGroup{}}
	for _, row := range selected {
		switch row.Signal {
		case "regression":
			p.Summary.Regression++
		case "improvement":
			p.Summary.Improvement++
		case "below_threshold":
			p.Summary.BelowThreshold++
		case "not_comparable":
			p.Summary.NotComparable++
		}
	}
	limit, err := rowLimit(string(cfg.Report.MaxRows), len(selected))
	if err != nil {
		return p, nil, err
	}
	selected = selected[:limit]
	p.Summary.Displayed = len(selected)
	p.Summary.Omitted = p.Summary.Selected - p.Summary.Displayed
	for _, metric := range []string{"time", "bytes", "allocations", "throughput"} {
		if metrics[metric] {
			direction := "lower"
			if metric == "throughput" {
				direction = "higher"
			}
			verb := "is"
			if metric == "bytes" || metric == "allocations" {
				verb = "are"
			}
			p.Disclosures = append(p.Disclosures, metricLabel(metric)+" "+verb+" "+direction+"-is-better.")
		}
	}
	p.Disclosures = append(p.Disclosures, advisory(comparison.Policy))
	if len(comparison.EnvironmentOverride.MismatchedFields) > 0 {
		p.Disclosures = append(p.Disclosures, "Environment mismatch override enabled for: "+strings.Join(comparison.EnvironmentOverride.MismatchedFields, ", ")+". This comparison is advisory.")
	}
	if p.Summary.Omitted > 0 {
		p.Disclosures = append(p.Disclosures, fmt.Sprintf("Showing %d of %d selected measurements; %d omitted by the configured row limit.", p.Summary.Displayed, p.Summary.Selected, p.Summary.Omitted))
	}
	index := map[string]int{}
	for _, row := range selected {
		base, head, err := displayValues(row, cfg.Report.Units)
		if err != nil {
			return p, nil, fmt.Errorf("measurement %s: %w", row.Key, err)
		}
		labels := []model.PresentationLabel{}
		for _, field := range cfg.Report.GroupBy {
			value := ""
			switch field {
			case "suite":
				value = row.Identity.Suite
			case "package":
				value = row.Identity.Package
			case "metric":
				value = row.Identity.Metric
			}
			if field == "package" && value == "" {
				continue
			}
			labels = append(labels, model.PresentationLabel{Field: field, Value: value})
		}
		key, _ := json.Marshal(labels)
		at, ok := index[string(key)]
		if !ok {
			at = len(p.Groups)
			index[string(key)] = at
			p.Groups = append(p.Groups, model.PresentationGroup{Labels: labels, Rows: []model.PresentationRow{}})
		}
		change, err := displayChange(row.DeltaPercent, cfg.Comparison.PercentDecimals)
		if err != nil {
			return p, nil, err
		}
		p.Groups[at].Rows = append(p.Groups[at].Rows, model.PresentationRow{Key: row.Key, Identity: row.Identity, Base: base, Head: head, Change: change, Signal: row.Signal, Samples: sampleText(row)})
	}
	if sections.Benchstat && (comparison.Policy.Statistics != "benchstat" || len(comparison.Statistics) == 0) {
		return p, nil, fmt.Errorf("benchstat section requires recorded benchstat analysis")
	}
	return p, markdown(p, comparison), nil
}
func patterns(values []string) ([]*regexp.Regexp, error) {
	result := []*regexp.Regexp{}
	for _, value := range values {
		pattern, err := regexp.Compile(value)
		if err != nil {
			return nil, err
		}
		result = append(result, pattern)
	}
	return result, nil
}
func matches(patterns []*regexp.Regexp, key string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(key) {
			return true
		}
	}
	return false
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func rank(signal string) int {
	switch signal {
	case "regression":
		return 0
	case "below_threshold":
		return 1
	case "improvement":
		return 2
	default:
		return 3
	}
}
func advisory(policy model.Policy) string {
	evidence := "No statistical analysis was requested."
	if policy.Statistics == "benchstat" {
		evidence = "Benchstat results are separate from these advisory signals."
	}
	return fmt.Sprintf("Advisory thresholds: %s%% regression, %s%% improvement. %s", policy.RegressionPercent, policy.ImprovementPercent, evidence)
}
func sampleText(row model.ComparisonRow) string {
	one := func(m *model.Measurement) string {
		if m == nil || len(m.Samples) == 0 {
			return "—"
		}
		return fmt.Sprint(len(m.Samples))
	}
	a, b := one(row.Base), one(row.Head)
	if a == "—" && b == "—" {
		return "—"
	}
	return a + " / " + b
}
func displayChange(value *string, places int) (string, error) {
	if value == nil {
		return "—", nil
	}
	n, ok := new(big.Rat).SetString(*value)
	if !ok {
		return "", fmt.Errorf("invalid recorded delta %q", *value)
	}
	text := n.FloatString(places)
	if n.Sign() > 0 {
		text = "+" + text
	}
	return text + "%", nil
}

func displayValues(row model.ComparisonRow, units string) (string, string, error) {
	factor := int64(1)
	label := row.Definition.Unit
	if units == "auto" {
		var candidates []struct {
			factor int64
			label  string
		}
		switch row.Identity.Metric {
		case "time":
			candidates = []struct {
				factor int64
				label  string
			}{{1000000000, "s"}, {1000000, "ms"}, {1000, "µs"}, {1, "ns"}}
		case "bytes":
			candidates = []struct {
				factor int64
				label  string
			}{{1000000000, "GB/op"}, {1000000, "MB/op"}, {1000, "kB/op"}, {1, "B/op"}}
		case "throughput":
			candidates = []struct {
				factor int64
				label  string
			}{{1000000000, "GB/s"}, {1000000, "MB/s"}, {1000, "kB/s"}, {1, "B/s"}}
		}
		largest := new(big.Rat)
		for _, m := range []*model.Measurement{row.Base, row.Head} {
			if m != nil {
				n, err := decimal.Parse(m.Estimate)
				if err != nil {
					return "", "", err
				}
				if n.Cmp(largest) > 0 {
					largest = n
				}
			}
		}
		for _, candidate := range candidates {
			if largest.Cmp(new(big.Rat).SetInt64(candidate.factor)) >= 0 || candidate.factor == 1 {
				factor, label = candidate.factor, candidate.label
				break
			}
		}
	}
	one := func(m *model.Measurement) (string, error) {
		if m == nil {
			return "—", nil
		}
		n, err := decimal.Parse(m.Estimate)
		if err != nil {
			return "", err
		}
		n.Quo(n, new(big.Rat).SetInt64(factor))
		text, err := decimal.RoundSigned(n, 3)
		if err != nil {
			return "", err
		}
		return text + " " + label, nil
	}
	a, err := one(row.Base)
	if err != nil {
		return "", "", err
	}
	b, err := one(row.Head)
	return a, b, err
}
