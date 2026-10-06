// Package model defines language-neutral version 1 benchmark documents.
package model

import (
	"encoding/json"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
)

type Environment struct {
	Toolchain string `json:"toolchain"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Runner    string `json:"runner"`
}
type Parser struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type Suite struct {
	ID      string   `json:"id"`
	Parser  Parser   `json:"parser"`
	Command string   `json:"command"`
	Files   []string `json:"files"`
}
type Manifest struct {
	SchemaVersion  int         `json:"schema_version"`
	Revision       string      `json:"revision"`
	Environment    Environment `json:"environment"`
	ExpectedSuites []string    `json:"expected_suites"`
	Suites         []Suite     `json:"suites"`
}
type Identity struct {
	Suite     string `json:"suite"`
	Package   string `json:"package"`
	Benchmark string `json:"benchmark"`
	Metric    string `json:"metric"`
}

func (i Identity) Key() string { return contracts.Key(i.Suite, i.Package, i.Benchmark, i.Metric) }

type Definition struct {
	Unit      string `json:"unit"`
	Direction string `json:"direction"`
	Estimator string `json:"estimator"`
}
type Bounds struct {
	Lower string `json:"lower"`
	Upper string `json:"upper"`
}
type Measurement struct {
	Key        string     `json:"key"`
	Identity   Identity   `json:"identity"`
	Definition Definition `json:"definition"`
	Estimate   string     `json:"estimate"`
	Samples    []string   `json:"samples,omitempty"`
	Bounds     *Bounds    `json:"bounds,omitempty"`
}
type Input struct {
	Suite  string `json:"suite"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Run struct {
	Manifest
	Inputs       []Input       `json:"inputs"`
	Measurements []Measurement `json:"measurements"`
}

// Policy is the complete effective comparison policy; thresholds retain decimal source text.
type Policy struct {
	RegressionPercent  json.Number `json:"regression_percent"`
	ImprovementPercent json.Number `json:"improvement_percent"`
	PercentDecimals    int         `json:"percent_decimals"`
	FailOnRegression   bool        `json:"fail_on_regression"`
	Statistics         string      `json:"statistics"`
}
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type Side struct {
	Revision    string      `json:"revision"`
	Environment Environment `json:"environment"`
}
type StatInput struct {
	Side   string `json:"side"`
	Suite  string `json:"suite"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Statistics struct {
	Tool             Tool        `json:"tool"`
	ExecutableSHA256 string      `json:"executable_sha256"`
	Suite            string      `json:"suite"`
	Arguments        []string    `json:"arguments"`
	Inputs           []StatInput `json:"inputs"`
	Stdout           string      `json:"stdout"`
	Stderr           string      `json:"stderr"`
}
type EnvironmentOverride struct {
	Allowed          bool     `json:"allowed"`
	MismatchedFields []string `json:"mismatched_fields"`
}
type ComparisonRow struct {
	Key          string       `json:"key"`
	Identity     Identity     `json:"identity"`
	Definition   Definition   `json:"definition"`
	Base         *Measurement `json:"base"`
	Head         *Measurement `json:"head"`
	DeltaPercent *string      `json:"delta_percent"`
	Reason       string       `json:"reason"`
	Signal       string       `json:"signal"`
}
type Comparison struct {
	SchemaVersion       int                 `json:"schema_version"`
	Base                Side                `json:"base"`
	Head                Side                `json:"head"`
	Generator           Tool                `json:"generator"`
	Policy              Policy              `json:"policy"`
	EnvironmentOverride EnvironmentOverride `json:"environment_override"`
	Rows                []ComparisonRow     `json:"rows"`
	Statistics          []Statistics        `json:"statistics"`
	Gate                string              `json:"gate"`
}
