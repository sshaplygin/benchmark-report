// Package input loads manifests and orchestrates adapters without benchmark execution.
package input

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/schemas"
	"os"
	"path/filepath"
	"sort"
)

type Adapter func(model.Suite, string, []byte) ([]model.Measurement, error)

// LoadManifest strictly validates the embedded version 1 schema before decoding.
func LoadManifest(path string) (model.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		return m, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}
func ParseManifest(data []byte) (model.Manifest, error) {
	var m model.Manifest

	schema, err := schemas.Read("input-manifest")
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return m, err
	}
	if err = contracts.Unmarshal(data, &m); err != nil {
		return m, err
	}
	return m, nil
}

// Normalize validates all files, rebases raw references to out, and atomically writes one run.
func Normalize(parser, manifestPath, out string, adapter Adapter) error {
	if parser != "go" && parser != "criterion" {
		return fmt.Errorf("unsupported parser %q", parser)
	}
	if adapter == nil {
		return fmt.Errorf("adapter unavailable for %q", parser)
	}
	manifestAbs, err := filepath.Abs(manifestPath)
	if err != nil {
		return err
	}
	outAbs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	// Resolve directory symlinks before relative-path calculation (e.g. /var on macOS).
	outputParent, err := filepath.EvalSymlinks(filepath.Dir(outAbs))
	if err != nil {
		return fmt.Errorf("output directory %s: %w", filepath.Dir(outAbs), err)
	}
	outAbs = filepath.Join(outputParent, filepath.Base(outAbs))
	manifestParent, err := filepath.EvalSymlinks(filepath.Dir(manifestAbs))
	if err != nil {
		return fmt.Errorf("manifest directory: %w", err)
	}
	manifestAbs = filepath.Join(manifestParent, filepath.Base(manifestAbs))
	if err := distinctOutput(outAbs, manifestAbs); err != nil {
		return err
	}
	m, err := LoadManifest(manifestAbs)
	if err != nil {
		return err
	}
	run := model.Run{Manifest: m, Inputs: []model.Input{}, Measurements: []model.Measurement{}}
	// Separate copy avoids mutating caller-visible metadata while preserving recorded file order.
	run.Suites = append([]model.Suite(nil), m.Suites...)
	all := map[string]*model.Measurement{}
	for i, suite := range m.Suites {
		if suite.Parser.Name != parser {
			return fmt.Errorf("%s: suites[%d].parser.name: %q does not match --parser %q", manifestPath, i, suite.Parser.Name, parser)
		}
		rebased := make([]string, 0, len(suite.Files))
		suiteRows := 0
		metricSets := map[string]string{}
		for _, reference := range suite.Files {
			source := reference
			if !filepath.IsAbs(source) {
				source = filepath.Join(filepath.Dir(manifestAbs), filepath.FromSlash(source))
			}
			source, err = filepath.Abs(source)
			if err != nil {
				return err
			}
			source, err = filepath.EvalSymlinks(source)
			if err != nil {
				return fmt.Errorf("suite %q input %s: %w", suite.ID, source, err)
			}
			if err := distinctOutput(outAbs, source); err != nil {
				return err
			}
			data, err := os.ReadFile(source)
			if err != nil {
				return fmt.Errorf("suite %q input %s: %w", suite.ID, source, err)
			}
			rows, err := adapter(suite, source, data)
			if err != nil {
				return fmt.Errorf("suite %q: %w", suite.ID, err)
			}
			if len(rows) == 0 {
				return fmt.Errorf("suite %q input %s: no measurements", suite.ID, source)
			}
			suiteRows += len(rows)
			relative, err := filepath.Rel(filepath.Dir(outAbs), source)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			rebased = append(rebased, relative)
			digest := sha256.Sum256(data)
			run.Inputs = append(run.Inputs, model.Input{Suite: suite.ID, Path: relative, SHA256: hex.EncodeToString(digest[:])})
			// A repeated Go benchmark must expose the same metric set in every file.
			sets := map[string][]string{}
			for _, row := range rows {
				key := contracts.Key(row.Identity.Suite, row.Identity.Package, row.Identity.Benchmark, "")
				sets[key] = append(sets[key], row.Identity.Metric)
			}
			for k, v := range sets {
				sort.Strings(v)
				encoded, _ := json.Marshal(v)
				set := string(encoded)
				if old, ok := metricSets[k]; ok && old != set {
					return fmt.Errorf("%s: repeated benchmark %s has inconsistent metric set", source, k)
				}
				metricSets[k] = set
			}
			for _, row := range rows {
				if row.Identity.Suite != suite.ID {
					return fmt.Errorf("%s: adapter returned unexpected suite", source)
				}
				if old, ok := all[row.Key]; ok {
					if parser != "go" {
						return fmt.Errorf("%s: duplicate measurement %s across input files", source, row.Key)
					}
					if old.Definition != row.Definition {
						return fmt.Errorf("%s: incompatible repeated measurement %s", source, row.Key)
					}
					old.Samples = append(old.Samples, row.Samples...)
					old.Estimate, err = decimal.Median(old.Samples)
					if err != nil {
						return fmt.Errorf("%s: %w", source, err)
					}
				} else {
					copy := row
					all[row.Key] = &copy
				}
			}
		}
		if suiteRows == 0 {
			return fmt.Errorf("suite %q: no measurements", suite.ID)
		}
		run.Suites[i].Files = rebased
	}
	for _, row := range all {
		run.Measurements = append(run.Measurements, *row)
	}
	sort.Slice(run.Measurements, func(i, j int) bool { return run.Measurements[i].Key < run.Measurements[j].Key })
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	schema, err := schemas.Read("normalized-run")
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return fmt.Errorf("normalized output: %w", err)
	}
	return atomicWrite(outAbs, data)
}
func distinctOutput(out, input string) error {
	if out == input {
		return fmt.Errorf("output %s would overwrite an input", out)
	}
	a, ae := os.Stat(out)
	b, be := os.Stat(input)
	if ae == nil && be == nil && os.SameFile(a, b) {
		return fmt.Errorf("output %s would overwrite an input", out)
	}
	return nil
}
func atomicWrite(path string, data []byte) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("running executable: %w", err)
	}
	if err = distinctOutput(path, executable); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output %s is a symlink", path)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".benchreport-*.tmp")
	if err != nil {
		return fmt.Errorf("output %s: %w", path, err)
	}
	temporary := f.Name()
	defer func() { _ = os.Remove(temporary) }() // Cleanup must not replace the write error.
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporary, path); err != nil {
		return fmt.Errorf("output %s: %w", path, err)
	}
	return nil
}

// ResolveReference anchors document-relative paths at the physical parent directory.
// Resolve symlinks before filepath.Join cleans parent components.
func ResolveReference(document, reference string) (string, error) {
	if filepath.IsAbs(reference) {
		return filepath.Clean(reference), nil
	}
	absolute, err := filepath.Abs(document)
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.FromSlash(reference)), nil
}
