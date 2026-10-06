package reproduction

import (
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/compare"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/history"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/internal/output"
	"github.com/sshaplygin/benchmark-report/schemas"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

const FullComparisonName = "comparison.json"

func BuildFull(c model.Comparison, cfg config.Config, entries []output.Entry, files []model.File, version string, report model.ReportInvocation) ([]output.Entry, error) {
	inventory := map[string][]byte{}
	for _, entry := range entries {
		inventory[entry.Path] = entry.Data
	}
	for i := range files {
		data, ok := inventory[files[i].Path]
		if !ok {
			return nil, fmt.Errorf("full bundle missing %s", files[i].Path)
		}
		files[i].SHA256 = digest(data)
	}
	m := model.Reproduction{Report: &report, SchemaVersion: 1, Generator: model.Tool{Name: "benchreport", Version: version}, Platform: model.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}, Base: c.Base, Head: c.Head, ConfigurationSHA256: digest(inventory[ConfigurationName]), Files: files, Statistics: c.Statistics, Replay: model.Replay{Render: []string{"benchreport render --input comparison.json --config " + ConfigurationName + " --reproduction reproduction.json --output-dir replayed"}, Verify: []string{"benchreport replay --reproduction reproduction.json --output-dir replayed"}}}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	schema, err := schemas.Read("reproduction")
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return nil, err
	}
	if err = checkFull(m, c, cfg, inventory); err != nil {
		return nil, err
	}
	return append(entries, output.Entry{Path: ManifestName, Data: data}), nil
}
func ValidateFull(manifestPath, version string) (model.Reproduction, []string, map[string][]byte, error) {
	root := filepath.Dir(manifestPath)
	m, protected, snapshots, err := ValidateSnapshot(manifestPath, filepath.Join(root, FullComparisonName), filepath.Join(root, ConfigurationName), version)
	if err == nil && m.Report == nil {
		err = fmt.Errorf("replay requires a full calculation bundle from report")
	}
	return m, protected, snapshots, err
}
func member(document, reference string) (string, error) {
	if path.IsAbs(reference) || strings.Contains(reference, "\\") {
		return "", fmt.Errorf("bundle reference must be relative")
	}
	resolved := path.Clean(path.Join(path.Dir(document), reference))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "", fmt.Errorf("bundle reference escapes inventory")
	}
	return resolved, nil
}
func checkFull(m model.Reproduction, c model.Comparison, cfg config.Config, inventory map[string][]byte) error {
	if m.Report == nil {
		return fmt.Errorf("full bundle report invocation missing")
	}
	if !reflect.DeepEqual(m.Statistics, c.Statistics) {
		return fmt.Errorf("full bundle statistics disagree with comparison")
	}
	expected := map[string]string{FullComparisonName: "comparison", ConfigurationName: "configuration", "manifests/base.json": "manifest", "manifests/head.json": "manifest", "normalized/base.json": "normalized", "normalized/head.json": "normalized"}
	runs := map[string]model.Run{}
	for _, side := range []string{"base", "head"} {
		manifestName := "manifests/" + side + ".json"
		manifest, err := input.ParseManifest(inventory[manifestName])
		if err != nil {
			return fmt.Errorf("%s: %w", manifestName, err)
		}
		normalizedName := "normalized/" + side + ".json"
		run, err := input.ParseRun(inventory[normalizedName])
		if err != nil {
			return fmt.Errorf("%s: %w", normalizedName, err)
		}
		meta := run.Manifest
		if manifest.Revision != meta.Revision || !reflect.DeepEqual(manifest.Environment, meta.Environment) || !reflect.DeepEqual(manifest.ExpectedSuites, meta.ExpectedSuites) || len(manifest.Suites) != len(meta.Suites) {
			return fmt.Errorf("full bundle manifest/normalized metadata mismatch")
		}
		rawHashes := map[string]string{}
		for i, suite := range manifest.Suites {
			if suite.Parser.Name != m.Report.Parser {
				return fmt.Errorf("full bundle parser mismatch")
			}
			normalizedSuite := meta.Suites[i]
			if suite.ID != normalizedSuite.ID || suite.Parser != normalizedSuite.Parser || suite.Command != normalizedSuite.Command || len(suite.Files) != len(normalizedSuite.Files) {
				return fmt.Errorf("full bundle suite metadata mismatch")
			}
			for j, reference := range suite.Files {
				raw, err := member(manifestName, reference)
				if err != nil {
					return err
				}
				data, ok := inventory[raw]
				if !ok {
					return fmt.Errorf("full bundle raw dependency missing %s", raw)
				}
				expected[raw] = "raw"
				rawHashes[raw] = digest(data)
				normalizedRaw, err := member(normalizedName, normalizedSuite.Files[j])
				if err != nil || normalizedRaw != raw {
					return fmt.Errorf("full bundle normalized raw reference mismatch")
				}
			}
		}
		if len(run.Inputs) != len(rawHashes) {
			return fmt.Errorf("full bundle raw checksum inventory mismatch")
		}
		for _, in := range run.Inputs {
			raw, err := member(normalizedName, in.Path)
			if err != nil || rawHashes[raw] != in.SHA256 {
				return fmt.Errorf("full bundle raw input checksum mismatch")
			}
		}
		runs[side] = run
	}
	calculated, err := compare.Runs(runs["base"], runs["head"], c.Policy, c.EnvironmentOverride.Allowed, c.Generator.Version)
	if err != nil {
		return err
	}
	calculated.Statistics = c.Statistics
	if !reflect.DeepEqual(calculated, c) {
		return fmt.Errorf("full bundle comparison differs from archived normalized evidence")
	}
	if cfg.Outputs.Markdown != nil {
		expected[*cfg.Outputs.Markdown] = "markdown"
		expected["comment.md"] = "comment"
	}
	if cfg.Outputs.JSON != nil {
		expected[*cfg.Outputs.JSON] = "presentation"
	}
	if cfg.History.Enabled {
		files, err := history.Build(runs["head"], cfg.History)
		if err != nil {
			return err
		}
		if files.Smaller != nil {
			expected[cfg.History.SmallerFile] = "history_smaller"
			if string(files.Smaller) != string(inventory[cfg.History.SmallerFile]) {
				return fmt.Errorf("full bundle history values mismatch")
			}
		}
		if files.Bigger != nil {
			expected[cfg.History.BiggerFile] = "history_bigger"
			if string(files.Bigger) != string(inventory[cfg.History.BiggerFile]) {
				return fmt.Errorf("full bundle history values mismatch")
			}
		}
	}
	for i, stat := range c.Statistics {
		name := fmt.Sprintf("statistics/%03d.txt", i)
		expected[name] = "statistics"
		if string(inventory[name]) != stat.Stdout {
			return fmt.Errorf("full bundle statistical output mismatch")
		}
		for _, raw := range stat.Inputs {
			reference, err := member(FullComparisonName, raw.Path)
			if err != nil || digest(inventory[reference]) != raw.SHA256 || expected[reference] != "raw" {
				return fmt.Errorf("full bundle statistical raw dependency mismatch")
			}
		}
	}
	if len(expected) != len(m.Files) {
		return fmt.Errorf("full bundle incomplete or unexpected inventory")
	}
	for _, file := range m.Files {
		if expected[file.Path] != file.Role {
			return fmt.Errorf("full bundle unexpected role/path %s", file.Path)
		}
	}
	return nil
}
