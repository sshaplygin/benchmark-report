package main

import (
	"flag"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/adapters/criterion"
	"github.com/sshaplygin/benchmark-report/internal/adapters/gobench"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"io"
	"os"
)

var version = "0.1.0-dev"

const help = `benchreport normalizes recorded Go and Criterion benchmarks offline.
Usage:
  benchreport normalize --parser go|criterion --manifest FILE --out FILE
  benchreport --version
  benchreport --help
Only normalize is implemented at this stage. Diagnostics go to stderr.
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
