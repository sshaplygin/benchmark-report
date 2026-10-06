// Package reproduction creates and verifies offline presentation replay bundles.
package reproduction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/internal/output"
	"github.com/sshaplygin/benchmark-report/schemas"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
)

const ManifestName = "reproduction.json"
const ComparisonName = "replay-inputs/comparison.json"
const ConfigurationName = "replay-inputs/configuration.json"

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// Build retains the original comparison bytes and expanded configuration.
func Build(c model.Comparison, cfg config.Config, comparison []byte, report []output.Entry, version string) ([]output.Entry, error) {
	cfgBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	cfgBytes = append(cfgBytes, '\n')
	entries := append([]output.Entry(nil), report...)
	entries = append(entries, output.Entry{Path: ComparisonName, Data: comparison}, output.Entry{Path: ConfigurationName, Data: cfgBytes})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	manifest := model.Reproduction{SchemaVersion: 1, Generator: model.Tool{Name: "benchreport", Version: version}, Platform: model.Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}, Base: c.Base, Head: c.Head, ConfigurationSHA256: digest(cfgBytes), Files: []model.File{}, Replay: model.Replay{Render: []string{"benchreport render --input " + ComparisonName + " --config " + ConfigurationName + " --reproduction " + ManifestName + " --output-dir replayed"}}}
	for _, entry := range entries {
		role := "markdown"
		switch {
		case entry.Path == ComparisonName:
			role = "comparison"
		case entry.Path == ConfigurationName:
			role = "configuration"
		case cfg.Outputs.JSON != nil && entry.Path == *cfg.Outputs.JSON:
			role = "presentation"
		}
		manifest.Files = append(manifest.Files, model.File{Path: entry.Path, SHA256: digest(entry.Data), Role: role})
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
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
	return append(entries, output.Entry{Path: ManifestName, Data: data}), nil
}

// Validate verifies the complete inventory before the CLI opens dependent documents.
// Returned physical paths are protected against replacement by the replay writer.
func Validate(path, inputPath, configPath, version string) (model.Reproduction, []string, error) {
	return validate(path, inputPath, configPath, version, map[string][]byte{})
}

// ValidateSnapshot returns checksum-verified document bytes, preventing subsequent path rereads.
func ValidateSnapshot(path, inputPath, configPath, version string) (model.Reproduction, []string, map[string][]byte, error) {
	snapshots := map[string][]byte{}
	m, p, err := validate(path, inputPath, configPath, version, snapshots)
	return m, p, snapshots, err
}
func validate(path, inputPath, configPath, version string, snapshots map[string][]byte) (model.Reproduction, []string, error) {
	var manifest model.Reproduction
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, nil, fmt.Errorf("reproduction %s: %w", path, err)
	}
	schema, err := schemas.Read("reproduction")
	if err == nil {
		err = contracts.ValidateSchema(schema, data)
	}
	if err != nil {
		return manifest, nil, fmt.Errorf("reproduction %s: %w", path, err)
	}
	if err = contracts.Unmarshal(data, &manifest); err != nil {
		return manifest, nil, err
	}
	if manifest.Generator.Name != "benchreport" || manifest.Generator.Version != version {
		return manifest, nil, fmt.Errorf("reproduction: requires benchreport %s, installed %s", manifest.Generator.Version, version)
	}
	if manifest.Report == nil && (len(manifest.Replay.Verify) != 0 || manifest.Statistics != nil) {
		return manifest, nil, fmt.Errorf("reproduction: full calculation replay is not supported by standalone render")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return manifest, nil, err
	}
	root, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return manifest, nil, err
	}
	cap, err := os.OpenRoot(root)
	if err != nil {
		return manifest, nil, err
	}
	defer cap.Close()
	// Validate every lexical path and collision before opening an inventory member.
	seen := map[string]bool{}
	for _, file := range manifest.Files {
		if seen[norm.NFC.String(cases.Fold().String(file.Path))] {
			return manifest, nil, fmt.Errorf("reproduction: duplicate inventory path %q", file.Path)
		}
		seen[norm.NFC.String(cases.Fold().String(file.Path))] = true
		for _, segment := range strings.Split(file.Path, "/") {
			if segment == "" || segment == "." || segment == ".." {
				return manifest, nil, fmt.Errorf("reproduction: invalid inventory path %q", file.Path)
			}
		}
		if file.Path == ManifestName {
			return manifest, nil, fmt.Errorf("reproduction: manifest cannot inventory itself")
		}
	}
	for a := range seen {
		for b := range seen {
			if strings.HasPrefix(b, a+"/") {
				return manifest, nil, fmt.Errorf("reproduction: file/directory inventory collision")
			}
		}
	}
	protected := []string{absolute}
	comparison, configuration := "", ""
	for _, file := range manifest.Files {
		info, err := cap.Lstat(filepath.FromSlash(file.Path))
		if err != nil {
			return manifest, nil, fmt.Errorf("reproduction %s: %w", file.Path, err)
		}
		if !info.Mode().IsRegular() {
			return manifest, nil, fmt.Errorf("reproduction %s: expected regular file", file.Path)
		}
		bytes, err := cap.ReadFile(filepath.FromSlash(file.Path))
		if err != nil {
			return manifest, nil, fmt.Errorf("reproduction %s: %w", file.Path, err)
		}
		if digest(bytes) != file.SHA256 {
			return manifest, nil, fmt.Errorf("reproduction %s: checksum mismatch", file.Path)
		}
		physical, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			return manifest, nil, err
		}
		relative, err := filepath.Rel(root, physical)
		if err != nil || relative == ".." || strings.HasPrefix(relative, "../") {
			return manifest, nil, fmt.Errorf("reproduction: inventory path escapes bundle")
		}
		protected = append(protected, physical)
		snapshots[file.Path] = bytes
		snapshots[file.Role] = bytes
		switch file.Role {
		case "comparison":
			if comparison != "" {
				return manifest, nil, fmt.Errorf("reproduction: multiple comparison inputs")
			}
			comparison = physical
		case "configuration":
			if configuration != "" || file.SHA256 != manifest.ConfigurationSHA256 {
				return manifest, nil, fmt.Errorf("reproduction: configuration identity or digest mismatch")
			}
			configuration = physical
		}
	}
	resolve := func(path string) (string, error) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		return filepath.EvalSymlinks(absolute)
	}
	actualInput, err := resolve(inputPath)
	if err != nil {
		return manifest, nil, err
	}
	actualConfig, err := resolve(configPath)
	if err != nil {
		return manifest, nil, err
	}
	if comparison == "" || configuration == "" || actualInput != comparison || actualConfig != configuration {
		return manifest, nil, fmt.Errorf("reproduction: --input and --config must match inventoried comparison/configuration")
	}
	cfg, err := config.Parse(snapshots["configuration"])
	if err != nil {
		return manifest, nil, fmt.Errorf("reproduction configuration: %w", err)
	}
	c, err := input.ParseComparison(snapshots["comparison"])
	if err != nil {
		return manifest, nil, fmt.Errorf("reproduction comparison: %w", err)
	}
	if !reflect.DeepEqual(manifest.Base, c.Base) || !reflect.DeepEqual(manifest.Head, c.Head) {
		return manifest, nil, fmt.Errorf("reproduction: base/head metadata disagrees with comparison")
	}
	if !config.PolicyEqual(c.Policy, cfg.Comparison) {
		return manifest, nil, fmt.Errorf("reproduction: configuration policy disagrees with comparison")
	}
	if manifest.Report != nil {
		if err := checkFull(manifest, c, cfg, snapshots); err != nil {
			return manifest, nil, err
		}
		return manifest, protected, nil
	}
	expected := map[string]string{ComparisonName: "comparison", ConfigurationName: "configuration"}
	if cfg.Outputs.Markdown != nil {
		expected[*cfg.Outputs.Markdown] = "markdown"
	}
	if cfg.Outputs.JSON != nil {
		expected[*cfg.Outputs.JSON] = "presentation"
	}
	if len(expected) != len(manifest.Files) {
		return manifest, nil, fmt.Errorf("reproduction: incomplete or unexpected report inventory")
	}
	for _, file := range manifest.Files {
		if expected[file.Path] != file.Role {
			return manifest, nil, fmt.Errorf("reproduction: missing or inconsistent inventoried output %q", file.Path)
		}
	}
	return manifest, protected, nil
}
