package input

import (
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/compare"
	"github.com/sshaplygin/benchmark-report/internal/contracts"

	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/schemas"
	"os"
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

// ValidateRun checks normalized source evidence as well as the structural contract.
func ValidateRun(run model.Run) error { return model.ValidateRun(run) }

// LoadComparison validates complete stored computation before presentation.
func LoadComparison(path string) (model.Comparison, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Comparison{}, fmt.Errorf("%s: %w", path, err)
	}
	c, err := ParseComparison(data)
	if err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// ParseComparison validates a single in-memory snapshot of the comparison evidence.
func ParseComparison(data []byte) (model.Comparison, error) {
	var c model.Comparison
	schema, err := schemas.Read("comparison")
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return c, err
	}
	if err = contracts.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if err = compare.Validate(c); err != nil {
		return c, err
	}
	return c, nil
}
