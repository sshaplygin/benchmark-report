// Package pipeline orchestrates offline reporting without executing benchmarks.
package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/adapters/criterion"
	"github.com/sshaplygin/benchmark-report/internal/adapters/gobench"
	"github.com/sshaplygin/benchmark-report/internal/compare"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/history"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/internal/output"
	"github.com/sshaplygin/benchmark-report/internal/render"
	"github.com/sshaplygin/benchmark-report/internal/reproduction"
	"github.com/sshaplygin/benchmark-report/internal/statistics"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Options struct {
	Parser, BaseManifest, HeadManifest, ConfigPath, BenchstatPath, ArtifactURL, CommentHeader, Version string
	AllowEnvironmentMismatch                                                                           bool
}
type Result struct {
	SchemaVersion int               `json:"schema_version"`
	Files         map[string]string `json:"files"`
	Gate          string            `json:"gate"`
}
type Bundle struct {
	Entries           []output.Entry
	Remove, Protected []string
	Result            Result
}

func encoded(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
func Build(opts Options) (Bundle, error) {
	var bundle Bundle
	adapter := input.Adapter(nil)
	switch opts.Parser {
	case "go":
		adapter = gobench.Parse
	case "criterion":
		adapter = criterion.Parse
	default:
		return bundle, fmt.Errorf("unsupported parser %q", opts.Parser)
	}
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return bundle, err
	}
	if err = reserved(cfg); err != nil {
		return bundle, err
	}
	workspace, err := os.MkdirTemp("", "benchreport-full-")
	if err != nil {
		return bundle, err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(workspace) // Capture the path before resolving symlinks.
	workspace, err = filepath.EvalSymlinks(workspace)
	if err != nil {
		return bundle, err
	}
	roles := map[string]string{}
	write := func(name string, data []byte, role string) error {
		target := filepath.Join(workspace, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return err
		}
		roles[name] = role
		return nil
	}
	bundle.Protected = []string{opts.BaseManifest, opts.HeadManifest}
	if opts.ConfigPath != "" {
		bundle.Protected = append(bundle.Protected, opts.ConfigPath)
	}
	for _, side := range []struct{ name, source string }{{"base", opts.BaseManifest}, {"head", opts.HeadManifest}} {
		manifest, err := input.LoadManifest(side.source)
		if err != nil {
			return bundle, err
		}
		sort.Slice(manifest.Suites, func(i, j int) bool { return manifest.Suites[i].ID < manifest.Suites[j].ID })
		sort.Strings(manifest.ExpectedSuites)
		for i := range manifest.Suites {
			suite := &manifest.Suites[i]
			if suite.Parser.Name != opts.Parser {
				return bundle, fmt.Errorf("suite %s parser does not match --parser", suite.ID)
			}
			seen := map[string]bool{}
			for j, reference := range suite.Files {
				source, err := input.ResolveReference(side.source, reference)
				if err != nil {
					return bundle, err
				}
				physical, err := filepath.EvalSymlinks(source)
				if err != nil {
					return bundle, err
				}
				if seen[physical] {
					return bundle, fmt.Errorf("duplicate raw input in suite %s", suite.ID)
				}
				seen[physical] = true
				bytes, err := os.ReadFile(source)
				if err != nil {
					return bundle, fmt.Errorf("raw input %s: %w", source, err)
				}
				bundle.Protected = append(bundle.Protected, source)
				name := fmt.Sprintf("raw/%s/%03d/%03d.log", side.name, i, j)
				if err = write(name, bytes, "raw"); err != nil {
					return bundle, err
				}
				suite.Files[j] = "../" + name
			}
		}
		bytes, err := encoded(manifest)
		if err != nil {
			return bundle, err
		}
		manifestName := "manifests/" + side.name + ".json"
		if err = write(manifestName, bytes, "manifest"); err != nil {
			return bundle, err
		}
		normalized := "normalized/" + side.name + ".json"
		if err = os.MkdirAll(filepath.Join(workspace, "normalized"), 0755); err != nil {
			return bundle, err
		}
		if err = input.Normalize(opts.Parser, filepath.Join(workspace, manifestName), filepath.Join(workspace, normalized), adapter); err != nil {
			return bundle, err
		}
		roles[normalized] = "normalized"
	}
	basePath, headPath := filepath.Join(workspace, "normalized/base.json"), filepath.Join(workspace, "normalized/head.json")
	base, err := input.LoadRun(basePath)
	if err != nil {
		return bundle, err
	}
	head, err := input.LoadRun(headPath)
	if err != nil {
		return bundle, err
	}
	c, err := compare.Runs(base, head, cfg.Comparison, opts.AllowEnvironmentMismatch, opts.Version)
	if err != nil {
		return bundle, err
	}
	if cfg.Comparison.Statistics == "benchstat" {
		tool, err := statistics.ResolveExecutable(opts.BenchstatPath)
		if err != nil {
			return bundle, err
		}
		bundle.Protected = append(bundle.Protected, tool)
		c.Statistics, err = statistics.Analyze(basePath, headPath, base, head, tool)
		if err != nil {
			return bundle, err
		}
		for i := range c.Statistics {
			for j := range c.Statistics[i].Inputs {
				record := &c.Statistics[i].Inputs[j]
				document := basePath
				if record.Side == "head" {
					document = headPath
				}
				source, err := input.ResolveReference(document, record.Path)
				if err != nil {
					return bundle, err
				}
				relative, err := filepath.Rel(workspace, source)
				if err != nil {
					return bundle, err
				}
				record.Path = filepath.ToSlash(relative)
			}
			if err = write(fmt.Sprintf("statistics/%03d.txt", i), []byte(c.Statistics[i].Stdout), "statistics"); err != nil {
				return bundle, err
			}
		}
	}
	comparison, err := encoded(c)
	if err != nil {
		return bundle, err
	}
	if err = write(reproduction.FullComparisonName, comparison, "comparison"); err != nil {
		return bundle, err
	}
	cfgBytes, err := encoded(cfg)
	if err != nil {
		return bundle, err
	}
	if err = write(reproduction.ConfigurationName, cfgBytes, "configuration"); err != nil {
		return bundle, err
	}
	bundle.Result = Result{SchemaVersion: 1, Files: map[string]string{"comparison": reproduction.FullComparisonName, "reproduction": reproduction.ManifestName}, Gate: c.Gate}
	presentation, markdown, err := render.Build(c, cfg)
	if err != nil {
		return bundle, err
	}
	if cfg.Outputs.Markdown != nil {
		if err = write(*cfg.Outputs.Markdown, markdown, "markdown"); err != nil {
			return bundle, err
		}
		bundle.Result.Files["markdown"] = *cfg.Outputs.Markdown
		comment, err := render.Comment(c, cfg, opts.CommentHeader, opts.ArtifactURL)
		if err != nil {
			return bundle, err
		}
		if err = write("comment.md", comment, "comment"); err != nil {
			return bundle, err
		}
		bundle.Result.Files["comment"] = "comment.md"
	}
	if cfg.Outputs.JSON != nil {
		data, err := encoded(presentation)
		if err != nil {
			return bundle, err
		}
		if err = write(*cfg.Outputs.JSON, data, "presentation"); err != nil {
			return bundle, err
		}
		bundle.Result.Files["presentation"] = *cfg.Outputs.JSON
	}
	if cfg.History.Enabled {
		files, err := history.Build(head, cfg.History)
		if err != nil {
			return bundle, err
		}
		for _, group := range []struct {
			name, key, role string
			bytes           []byte
		}{{cfg.History.SmallerFile, "history_smaller", "history_smaller", files.Smaller}, {cfg.History.BiggerFile, "history_bigger", "history_bigger", files.Bigger}} {
			if group.bytes == nil {
				bundle.Remove = append(bundle.Remove, group.name)
				continue
			}
			if err = write(group.name, group.bytes, group.role); err != nil {
				return bundle, err
			}
			bundle.Result.Files[group.key] = group.name
		}
	}
	names := []string{}
	for name := range roles {
		names = append(names, name)
	}
	sort.Strings(names)
	fileRoles := []model.File{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(name)))
		if err != nil {
			return bundle, err
		}
		bundle.Entries = append(bundle.Entries, output.Entry{Path: name, Data: data})
		fileRoles = append(fileRoles, model.File{Path: name, Role: roles[name]})
	}
	bundle.Entries, err = reproduction.BuildFull(c, cfg, bundle.Entries, fileRoles, opts.Version, model.ReportInvocation{Parser: opts.Parser, CommentHeader: opts.CommentHeader, ArtifactURL: opts.ArtifactURL})
	return bundle, err
}
func Run(opts Options, directory string) (Result, error) {
	if err := fresh(directory); err != nil {
		return Result{}, err
	}
	bundle, err := Build(opts)
	if err != nil {
		return Result{}, err
	}
	if err = output.Transaction(directory, bundle.Entries, bundle.Remove, bundle.Protected); err != nil {
		return Result{}, err
	}
	absolute, err := filepath.Abs(directory)
	if err == nil {
		absolute, err = filepath.EvalSymlinks(absolute)
	}
	if err != nil {
		return Result{}, err
	}
	for key, path := range bundle.Result.Files {
		bundle.Result.Files[key] = filepath.Join(absolute, filepath.FromSlash(path))
	}
	return bundle.Result, nil
}
func reserved(cfg config.Config) error {
	names := []string{}
	if cfg.Outputs.Markdown != nil {
		names = append(names, *cfg.Outputs.Markdown)
	}
	if cfg.Outputs.JSON != nil {
		names = append(names, *cfg.Outputs.JSON)
	}
	if cfg.History.Enabled {
		names = append(names, cfg.History.SmallerFile, cfg.History.BiggerFile)
	}
	for _, name := range names {
		folded := norm.NFC.String(cases.Fold().String(name))
		for _, root := range []string{"raw", "manifests", "normalized", "statistics", "comment.md", "comparison.json"} {
			if folded == root || strings.HasPrefix(folded, root+"/") {
				return fmt.Errorf("report output %q collides with reserved full-bundle namespace %q", name, root)
			}
		}
	}
	return nil
}

func fresh(directory string) error {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("full bundle output directory must be absent or empty")
	}
	return nil
}
