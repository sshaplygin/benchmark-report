// Package history exports absolute measurements for github-action-benchmark.
package history

import (
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"math"
	"regexp"
	"sort"
	"strconv"
)

type Measurement struct {
	Name  string      `json:"name"`
	Unit  string      `json:"unit"`
	Value json.Number `json:"value"`
	Range string      `json:"range,omitempty"`
	Extra string      `json:"extra"`
}
type Files struct {
	Smaller []byte
	Bigger  []byte
}

// Build selects independently of presentation and preserves exact canonical estimate digits.
func Build(run model.Run, cfg config.History) (Files, error) {
	var files Files
	if !cfg.Enabled {
		return files, fmt.Errorf("history.enabled must be true to export")
	}
	if err := model.ValidateRun(run); err != nil {
		return files, err
	}
	metrics := map[string]bool{}
	for _, m := range cfg.Metrics {
		switch m {
		case "time", "bytes", "allocations", "throughput":
		default:
			return files, fmt.Errorf("history.metrics: unsupported metric %q", m)
		}
		if metrics[m] {
			return files, fmt.Errorf("history.metrics: duplicate metric %q", m)
		}
		metrics[m] = true
	}
	if len(metrics) == 0 {
		return files, fmt.Errorf("history.metrics must not be empty")
	}
	compile := func(patterns []string) ([]*regexp.Regexp, error) {
		result := []*regexp.Regexp{}
		for _, pattern := range patterns {
			r, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			result = append(result, r)
		}
		return result, nil
	}
	include, err := compile(cfg.Include)
	if err != nil {
		return files, fmt.Errorf("history.include: %w", err)
	}
	exclude, err := compile(cfg.Exclude)
	if err != nil {
		return files, fmt.Errorf("history.exclude: %w", err)
	}
	match := func(patterns []*regexp.Regexp, key string) bool {
		for _, pattern := range patterns {
			if pattern.MatchString(key) {
				return true
			}
		}
		return false
	}
	rows := append([]model.Measurement(nil), run.Measurements...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	smaller, bigger := []Measurement{}, []Measurement{}
	for _, m := range rows {
		if !metrics[m.Identity.Metric] || len(include) > 0 && !match(include, m.Key) || match(exclude, m.Key) {
			continue
		}
		if err := compatibleNumber(m.Estimate); err != nil {
			return files, fmt.Errorf("measurement %s: cannot export estimate %s: %w", m.Key, m.Estimate, err)
		}
		// Metadata is serialized text accepted by the external action, not a custom object.
		metadata := struct {
			Estimator   string            `json:"estimator"`
			Environment model.Environment `json:"environment"`
		}{m.Definition.Estimator, run.Environment}
		extra, err := json.Marshal(metadata)
		if err != nil {
			return files, err
		}
		entry := Measurement{Name: m.Key, Unit: m.Definition.Unit, Value: json.Number(m.Estimate), Extra: string(extra)}
		if m.Bounds != nil {
			entry.Range = m.Bounds.Lower + ".." + m.Bounds.Upper + " " + m.Definition.Unit
		}
		if m.Definition.Direction == "lower" {
			smaller = append(smaller, entry)
		} else {
			bigger = append(bigger, entry)
		}
	}
	if len(smaller) == 0 && len(bigger) == 0 {
		return files, fmt.Errorf("history selection contains no measurements")
	}
	encode := func(rows []Measurement) ([]byte, error) {
		if len(rows) == 0 {
			return nil, nil
		}
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	}
	files.Smaller, err = encode(smaller)
	if err != nil {
		return files, err
	}
	files.Bigger, err = encode(bigger)
	return files, err
}
func compatibleNumber(source string) error {
	number, err := strconv.ParseFloat(source, 64)
	if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
		return fmt.Errorf("value exceeds finite binary64 history range")
	}
	original, err := decimal.Parse(source)
	if err != nil {
		return err
	}
	roundtrip, err := decimal.Parse(strconv.FormatFloat(number, 'g', -1, 64))
	if err != nil {
		return err
	}
	if original.Cmp(roundtrip) != 0 {
		return fmt.Errorf("value changes during binary64 shortest-decimal history round trip")
	}
	return nil
}
