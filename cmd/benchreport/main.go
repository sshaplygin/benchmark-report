package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/adapters/criterion"
	"github.com/sshaplygin/benchmark-report/internal/adapters/gobench"
	"github.com/sshaplygin/benchmark-report/internal/compare"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/history"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/internal/output"
	"github.com/sshaplygin/benchmark-report/internal/pipeline"
	"github.com/sshaplygin/benchmark-report/internal/render"
	"github.com/sshaplygin/benchmark-report/internal/reproduction"
	"github.com/sshaplygin/benchmark-report/internal/statistics"
	"github.com/sshaplygin/benchmark-report/schemas"
	"io"
	"os"
	"path/filepath"
)

var version = "0.1.0-dev"

const help = `benchreport normalizes and compares recorded benchmarks offline.
Usage:
  benchreport normalize --parser go|criterion --manifest FILE --out FILE
  benchreport compare --base FILE --head FILE [--config FILE] --out FILE [--allow-environment-mismatch] [--benchstat-path FILE]
  benchreport render --input FILE [--config FILE] --output-dir DIR [--reproduction FILE]
  benchreport export --input FILE [--config FILE] --output-dir DIR
  benchreport report --parser go|criterion --base-manifest FILE --head-manifest FILE [--config FILE] --output-dir DIR [--allow-environment-mismatch] [--benchstat-path FILE] [--artifact-url URL] [--comment-header HEADER]
  benchreport replay --reproduction FILE --output-dir DIR [--benchstat-path FILE]
  benchreport config validate --config FILE
  benchreport --version
  benchreport --help
Diagnostics go to stderr. compare writes its complete artifact before gate exit 2.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, help)
		return 1
	}
	if len(args) == 1 {
		switch args[0] {
		case "--help", "-h", "help":
			fmt.Fprint(stdout, help)
			return 0
		case "--version", "version":
			fmt.Fprintf(stdout, "benchreport %s\n", version)
			return 0
		}
	}
	if len(args) == 2 && args[1] == "--version" {
		fmt.Fprintf(stdout, "benchreport %s\n", version)
		return 0
	}
	if args[0] == "report" || args[0] == "replay" {
		return runPipeline(args[0], args[1:], stdout, stderr)
	}
	if args[0] == "export" {
		return runExport(args[1:], stdout, stderr)
	}
	if args[0] == "render" {
		return runRender(args[1:], stdout, stderr)
	}
	if args[0] == "compare" {
		return runCompare(args[1:], stdout, stderr)
	}
	if args[0] == "config" {
		return runConfig(args[1:], stdout, stderr)
	}
	if args[0] != "normalize" {
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 1
	}
	if len(args) == 2 && (args[1] == "--version") {
		fmt.Fprintf(stdout, "benchreport %s\n", version)
		return 0
	}
	flags := flag.NewFlagSet("normalize", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stdout, "Usage: benchreport normalize --parser go|criterion --manifest FILE --out FILE\n")
	}
	parser := flags.String("parser", "", "input parser: go or criterion")
	manifest := flags.String("manifest", "", "version 1 input manifest")
	out := flags.String("out", "", "normalized output file")
	if err := flags.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if flags.NArg() != 0 || *parser == "" || *manifest == "" || *out == "" {
		fmt.Fprintln(stderr, "normalize requires --parser, --manifest, --out and no positional arguments")
		return 1
	}
	adapter := input.Adapter(nil)
	switch *parser {
	case "go":
		adapter = gobench.Parse
	case "criterion":
		adapter = criterion.Parse
	default:
		fmt.Fprintf(stderr, "unsupported parser %q\n", *parser)
		return 1
	}
	if err := input.Normalize(*parser, *manifest, *out, adapter); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func parseFlags(flags *flag.FlagSet, args []string) int {
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	return -1
}
func explicitEmpty(flags *flag.FlagSet, name string, value string) bool {
	found := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found && value == ""
}
func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, "Usage: benchreport config validate --config FILE")
		return 0
	}
	if len(args) == 0 || args[0] != "validate" {
		fmt.Fprintln(stderr, "Usage: benchreport config validate --config FILE")
		return 1
	}
	if len(args) == 2 && args[1] == "--version" {
		fmt.Fprintf(stdout, "benchreport %s\n", version)
		return 0
	}
	flags := flag.NewFlagSet("config validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprintln(stdout, "Usage: benchreport config validate --config FILE") }
	path := flags.String("config", "", "version 1 configuration")
	if code := parseFlags(flags, args[1:]); code >= 0 {
		return code
	}
	if *path == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "config validate requires --config FILE and no positional arguments")
		return 1
	}
	if _, err := config.Load(*path); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
func runCompare(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "benchreport %s\n", version)
		return 0
	}
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stdout, "Usage: benchreport compare --base FILE --head FILE [--config FILE] --out FILE [--allow-environment-mismatch] [--benchstat-path FILE]")
	}
	basePath := flags.String("base", "", "normalized base run")
	headPath := flags.String("head", "", "normalized head run")
	configPath := flags.String("config", "", "optional version 1 configuration")
	out := flags.String("out", "", "comparison output file")
	allow := flags.Bool("allow-environment-mismatch", false, "permit and disclose environment mismatch")
	benchstatPath := flags.String("benchstat-path", "", "explicit matching pinned benchstat executable")
	if code := parseFlags(flags, args); code >= 0 {
		return code
	}
	if flags.NArg() != 0 || *basePath == "" || *headPath == "" || *out == "" || explicitEmpty(flags, "config", *configPath) || explicitEmpty(flags, "benchstat-path", *benchstatPath) {
		fmt.Fprintln(stderr, "compare requires --base, --head, --out and nonempty supplied paths")
		return 1
	}
	base, err := input.LoadRun(*basePath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	head, err := input.LoadRun(*headPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	artifact, err := compare.Runs(base, head, cfg.Comparison, *allow, version)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	protected := []string{*basePath, *headPath}
	if *configPath != "" {
		protected = append(protected, *configPath)
	}
	for _, side := range []struct {
		path string
		run  model.Run
	}{{*basePath, base}, {*headPath, head}} {
		for _, raw := range side.run.Inputs {
			resolved, err := input.ResolveReference(side.path, raw.Path)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			protected = append(protected, resolved)
		}
	}
	if cfg.Comparison.Statistics == "benchstat" {
		tool, err := statistics.ResolveExecutable(*benchstatPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		protected = append(protected, tool)
		artifact.Statistics, err = statistics.Analyze(*basePath, *headPath, base, head, *benchstatPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		outputParent, err := input.ResolveReference(*out, ".")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for i := range artifact.Statistics {
			for j := range artifact.Statistics[i].Inputs {
				record := &artifact.Statistics[i].Inputs[j]
				sourceDocument := *basePath
				if record.Side == "head" {
					sourceDocument = *headPath
				}
				source, err := input.ResolveReference(sourceDocument, record.Path)
				if err != nil {
					fmt.Fprintln(stderr, err)
					return 1
				}
				relative, err := filepath.Rel(outputParent, source)
				if err != nil {
					fmt.Fprintln(stderr, err)
					return 1
				}
				record.Path = filepath.ToSlash(relative)
			}
		}
	}
	if err := input.WriteDocument("comparison", artifact, *out, protected); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if artifact.Gate == "failed" {
		return 2
	}
	return 0
}

func runRender(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "benchreport %s\n", version)
		return 0
	}
	flags := flag.NewFlagSet("render", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stdout, "Usage: benchreport render --input FILE [--config FILE] --output-dir DIR [--reproduction FILE]")
	}
	path := flags.String("input", "", "complete comparison artifact")
	configPath := flags.String("config", "", "optional version 1 configuration")
	directory := flags.String("output-dir", "", "output bundle directory")
	replayPath := flags.String("reproduction", "", "verify existing bundle before rendering")
	if code := parseFlags(flags, args); code >= 0 {
		return code
	}
	if *path == "" || *directory == "" || flags.NArg() != 0 || explicitEmpty(flags, "config", *configPath) || explicitEmpty(flags, "reproduction", *replayPath) || *replayPath != "" && *configPath == "" {
		fmt.Fprintln(stderr, "render requires --input, --output-dir and nonempty supplied paths; --reproduction requires --config")
		return 1
	}
	protected := []string{*path}
	if *configPath != "" {
		protected = append(protected, *configPath)
	}
	var comparisonBytes []byte
	var cfg config.Config
	var err error
	if *replayPath != "" {
		_, inventory, snapshots, e := reproduction.ValidateSnapshot(*replayPath, *path, *configPath, version)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		protected = append(protected, inventory...)
		comparisonBytes = snapshots["comparison"]
		cfg, err = config.Parse(snapshots["configuration"])
	} else {
		comparisonBytes, err = os.ReadFile(*path)
		if err == nil {
			cfg, err = config.Load(*configPath)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	c, err := input.ParseComparison(comparisonBytes)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !config.PolicyEqual(c.Policy, cfg.Comparison) {
		fmt.Fprintln(stderr, "render configuration comparison policy differs from recorded policy")
		return 1
	}
	presentation, markdown, err := render.Build(c, cfg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	entries := []output.Entry{}
	if cfg.Outputs.Markdown != nil {
		entries = append(entries, output.Entry{Path: *cfg.Outputs.Markdown, Data: markdown})
	}
	if cfg.Outputs.JSON != nil {
		data, e := json.MarshalIndent(presentation, "", "  ")
		if e == nil {
			schema, e2 := schemas.Read("presentation")
			e = e2
			if e == nil {
				e = contracts.ValidateSchema(schema, data)
			}
		}
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		entries = append(entries, output.Entry{Path: *cfg.Outputs.JSON, Data: append(data, '\n')})
	}
	entries, err = reproduction.Build(c, cfg, comparisonBytes, entries, version)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = output.Commit(*directory, entries, protected); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runExport(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "benchreport %s\n", version)
		return 0
	}
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stdout, "Usage: benchreport export --input FILE [--config FILE] --output-dir DIR\nRequires history.enabled. Success prints JSON containing only nonempty files.smaller/files.bigger absolute paths.")
	}
	path := flags.String("input", "", "normalized run")
	configPath := flags.String("config", "", "optional version 1 configuration")
	directory := flags.String("output-dir", "", "history output directory")
	if code := parseFlags(flags, args); code >= 0 {
		return code
	}
	if *path == "" || *directory == "" || flags.NArg() != 0 || explicitEmpty(flags, "config", *configPath) {
		fmt.Fprintln(stderr, "export requires --input, --output-dir and nonempty supplied paths")
		return 1
	}
	run, err := input.LoadRun(*path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	files, err := history.Build(run, cfg.History)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	protected := []string{*path}
	if *configPath != "" {
		protected = append(protected, *configPath)
	}
	for _, raw := range run.Inputs {
		resolved, err := input.ResolveReference(*path, raw.Path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		protected = append(protected, resolved)
	}
	entries := []output.Entry{}
	remove := []string{}
	if files.Smaller != nil {
		entries = append(entries, output.Entry{Path: cfg.History.SmallerFile, Data: files.Smaller})
	} else {
		remove = append(remove, cfg.History.SmallerFile)
	}
	if files.Bigger != nil {
		entries = append(entries, output.Entry{Path: cfg.History.BiggerFile, Data: files.Bigger})
	} else {
		remove = append(remove, cfg.History.BiggerFile)
	}
	if err = output.Transaction(*directory, entries, remove, protected); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	absolute, err := filepath.Abs(*directory)
	if err == nil {
		absolute, err = filepath.EvalSymlinks(absolute)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result := struct {
		SchemaVersion int               `json:"schema_version"`
		Files         map[string]string `json:"files"`
	}{SchemaVersion: 1, Files: map[string]string{}}
	if files.Smaller != nil {
		result.Files["smaller"] = filepath.Join(absolute, filepath.FromSlash(cfg.History.SmallerFile))
	}
	if files.Bigger != nil {
		result.Files["bigger"] = filepath.Join(absolute, filepath.FromSlash(cfg.History.BiggerFile))
	}
	if err = json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runPipeline(command string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stdout, help) }
	directory := flags.String("output-dir", "", "empty full-bundle directory")
	tool := flags.String("benchstat-path", "", "pinned benchstat executable")
	var opts pipeline.Options
	opts.Version = version
	var manifest *string
	if command == "report" {
		flags.StringVar(&opts.Parser, "parser", "", "go or criterion")
		flags.StringVar(&opts.BaseManifest, "base-manifest", "", "base input manifest")
		flags.StringVar(&opts.HeadManifest, "head-manifest", "", "head input manifest")
		flags.StringVar(&opts.ConfigPath, "config", "", "configuration")
		flags.StringVar(&opts.ArtifactURL, "artifact-url", "", "full artifact URL for shortened comments")
		flags.StringVar(&opts.CommentHeader, "comment-header", "benchmark-report", "sticky comment header")
		flags.BoolVar(&opts.AllowEnvironmentMismatch, "allow-environment-mismatch", false, "disclose environment mismatch")
	} else {
		manifest = flags.String("reproduction", "", "full calculation bundle manifest")
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if flags.NArg() != 0 || *directory == "" || (command == "report" && (opts.Parser == "" || opts.BaseManifest == "" || opts.HeadManifest == "")) || (command == "replay" && *manifest == "") {
		fmt.Fprintln(stderr, "missing required pipeline flags or unexpected positional arguments")
		return 1
	}
	opts.BenchstatPath = *tool
	var result pipeline.Result
	var err error
	if command == "report" {
		result, err = pipeline.Run(opts, *directory)
	} else {
		result, err = pipeline.Replay(*manifest, *directory, *tool, version)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
