// Package config loads and expands the frozen version 1 configuration offline.
package config

import (
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/schemas"
	"os"
)

type Sections struct {
	Metadata  bool `json:"metadata"`
	Summary   bool `json:"summary"`
	Tables    bool `json:"tables"`
	Benchstat bool `json:"benchstat"`
}
type Report struct {
	Title     string      `json:"title"`
	Metrics   []string    `json:"metrics"`
	Include   []string    `json:"include"`
	Exclude   []string    `json:"exclude"`
	GroupBy   []string    `json:"group_by"`
	Columns   []string    `json:"columns"`
	Sort      string      `json:"sort"`
	Unchanged string      `json:"unchanged"`
	Missing   string      `json:"missing"`
	MaxRows   json.Number `json:"max_rows"`
	Units     string      `json:"units"`
	Sections  Sections    `json:"sections"`
}
type Outputs struct {
	Markdown *string `json:"markdown"`
	JSON     *string `json:"json"`
}
type History struct {
	Enabled     bool     `json:"enabled"`
	Metrics     []string `json:"metrics"`
	Include     []string `json:"include"`
	Exclude     []string `json:"exclude"`
	SmallerFile string   `json:"smaller_file"`
	BiggerFile  string   `json:"bigger_file"`
}
type Config struct {
	SchemaVersion int          `json:"schema_version"`
	Comparison    model.Policy `json:"comparison"`
	Report        Report       `json:"report"`
	Outputs       Outputs      `json:"outputs"`
	History       History      `json:"history"`
}

func Defaults() Config {
	markdown, js := "report.md", "report.json"
	return Config{SchemaVersion: 1, Comparison: model.Policy{RegressionPercent: json.Number("20"), ImprovementPercent: json.Number("20"), PercentDecimals: 1, Statistics: "none"}, Report: Report{Title: "Benchmark comparison", Metrics: []string{"time"}, Include: []string{}, Exclude: []string{}, GroupBy: []string{"suite", "package"}, Columns: []string{"benchmark", "base", "head", "change", "signal"}, Sort: "name", Unchanged: "show", Missing: "show", Units: "auto", MaxRows: json.Number("0"), Sections: Sections{Metadata: true, Summary: true, Tables: true}}, Outputs: Outputs{Markdown: &markdown, JSON: &js}, History: History{Metrics: []string{"time"}, Include: []string{}, Exclude: []string{}, SmallerFile: "benchmark-smaller.json", BiggerFile: "benchmark-bigger.json"}}
}

// Load uses defaults only when path is omitted; explicit callers validate empty paths themselves.
func Load(path string) (Config, error) {
	if path == "" {
		return Defaults(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	c, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}
func Parse(data []byte) (Config, error) {
	c := Defaults()
	schema, err := schemas.Read("configuration")
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return Config{}, err
	}
	if err = contracts.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	if err = ValidatePolicy(c.Comparison); err != nil {
		return Config{}, err
	}
	effective, err := json.Marshal(c)
	if err != nil {
		return Config{}, err
	}
	if err = contracts.ValidateSchema(schema, effective); err != nil {
		return Config{}, err
	}
	return c, nil
}
func ValidatePolicy(p model.Policy) error {
	for field, value := range map[string]json.Number{"regression_percent": p.RegressionPercent, "improvement_percent": p.ImprovementPercent} {
		n, err := decimal.Parse(string(value))
		if err != nil {
			return fmt.Errorf("comparison.%s: %w", field, err)
		}
		if n.Sign() <= 0 {
			return fmt.Errorf("comparison.%s: must be a positive finite decimal", field)
		}
	}
	if p.PercentDecimals < 0 || p.PercentDecimals > 4 {
		return fmt.Errorf("comparison.percent_decimals: must be 0 through 4")
	}
	if p.Statistics != "none" && p.Statistics != "benchstat" {
		return fmt.Errorf("comparison.statistics: unsupported value")
	}
	return nil
}

// PolicyEqual compares expanded policy semantically, so 1 and 1.0 are equivalent.
func PolicyEqual(a, b model.Policy) bool {
	if a.PercentDecimals != b.PercentDecimals || a.FailOnRegression != b.FailOnRegression || a.Statistics != b.Statistics {
		return false
	}
	for _, pair := range [][2]json.Number{{a.RegressionPercent, b.RegressionPercent}, {a.ImprovementPercent, b.ImprovementPercent}} {
		x, err := decimal.Parse(string(pair[0]))
		if err != nil {
			return false
		}
		y, err := decimal.Parse(string(pair[1]))
		if err != nil || x.Cmp(y) != 0 {
			return false
		}
	}
	return true
}
