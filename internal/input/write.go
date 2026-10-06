package input

import (
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/schemas"
	"path/filepath"
)

// WriteDocument validates JSON and replaces out atomically, protecting supplied inputs.
func WriteDocument(schemaName string, document any, out string, protected []string) error {
	absolute, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return fmt.Errorf("output directory: %w", err)
	}
	absolute = filepath.Join(parent, filepath.Base(absolute))
	for _, source := range protected {
		sourceAbs, err := filepath.Abs(source)
		if err != nil {
			return err
		}
		physical, err := filepath.EvalSymlinks(sourceAbs)
		if err == nil {
			sourceAbs = physical
		} else if physicalParent, parentErr := filepath.EvalSymlinks(filepath.Dir(sourceAbs)); parentErr == nil {
			sourceAbs = filepath.Join(physicalParent, filepath.Base(sourceAbs))
		}
		if err = distinctOutput(absolute, sourceAbs); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	schema, err := schemas.Read(schemaName)
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return fmt.Errorf("%s output: %w", schemaName, err)
	}
	return atomicWrite(absolute, data)
}
