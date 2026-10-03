package statistics

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/sshaplygin/benchmark-report/internal/adapters/gobench"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
)

func makeRun(t *testing.T, root string, values map[string][]string) (string, model.Run) {
	t.Helper()
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	document := filepath.Join(root, "normalized.json")
	run := model.Run{}
	// Deliberately reverse suite order; Analyze must sort invocations.
	for _, id := range []string{"z suite", "a suite"} {
		contents, ok := values[id]
		if !ok {
			continue
		}
		suite := model.Suite{ID: id, Parser: model.Parser{Name: "go", Version: "1"}}
		for i, content := range contents {
			name := id + " file " + string(rune('0'+i)) + "=literal.txt"
			if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			suite.Files = append(suite.Files, name)
			run.Inputs = append(run.Inputs, model.Input{Suite: id, Path: name, SHA256: hash([]byte(content))})
		}
		run.Suites = append(run.Suites, suite)
	}
	return document, run
}
func hash(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func benchmark(value string, n int) string {
	return "goos: linux\ngoarch: amd64\npkg: example.org/same\n" + strings.Repeat("BenchmarkSame-4 10 "+value+" ns/op 0 B/op 0 allocs/op\n", n)
}
func tool(t *testing.T) string {
	t.Helper()
	path := os.Getenv("BENCHREPORT_TEST_BENCHSTAT")
	if path == "" {
		t.Skip("real benchstat integration: set BENCHREPORT_TEST_BENCHSTAT to the pinned go-install binary; tests never download tools")
	}
	return path
}

func TestPinnedMetadata(t *testing.T) {
	good := debug.BuildInfo{Path: commandPath, Main: debug.Module{Path: modulePath, Version: Version}}
	if err := validateBuildInfo(&good); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"command", "module", "version", "main-replacement", "dependency-replacement"} {
		t.Run(name, func(t *testing.T) {
			bad := good
			switch name {
			case "command":
				bad.Path = "other"
			case "module":
				bad.Main.Path = "other"
			case "version":
				bad.Main.Version = "(devel)"
			case "main-replacement":
				bad.Main.Replace = &debug.Module{Path: "evil"}
			case "dependency-replacement":
				bad.Deps = []*debug.Module{{Path: "dependency", Replace: &debug.Module{Path: "evil"}}}
			}
			if err := validateBuildInfo(&bad); err == nil {
				t.Fatal("accepted wrong/replaced metadata")
			}
		})
	}
}

func TestRejectToolBeforeExecution(t *testing.T) {
	root := t.TempDir()
	bp, b := makeRun(t, filepath.Join(root, "base"), map[string][]string{"a suite": {benchmark("1", 10)}})
	hp, h := makeRun(t, filepath.Join(root, "head"), map[string][]string{"a suite": {benchmark("2", 10)}})
	for _, kind := range []string{"missing", "unreadable-buildinfo", "wrong-go-buildinfo", "directory"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(root, kind)
			if kind == "wrong-go-buildinfo" {
				var err error
				path, err = os.Executable()
				if err != nil {
					t.Fatal(err)
				}
			}
			if kind == "unreadable-buildinfo" {
				if err := os.WriteFile(path, []byte("#!/bin/sh\necho SHOULD_NOT_EXECUTE\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			records, err := Analyze(bp, hp, b, h, path)
			if err == nil || records != nil {
				t.Fatalf("accepted invalid tool: %#v %v", records, err)
			}
			if strings.Contains(err.Error(), "SHOULD_NOT_EXECUTE") {
				t.Fatal("unverified tool executed")
			}
		})
	}
}

func TestRawValidation(t *testing.T) {
	for _, kind := range []string{"modified", "missing", "checksum-missing", "checksum-duplicate", "criterion", "different-suite", "no-files"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			bp, b := makeRun(t, filepath.Join(root, "base"), map[string][]string{"a suite": {benchmark("1", 10)}})
			hp, h := makeRun(t, filepath.Join(root, "head"), map[string][]string{"a suite": {benchmark("2", 10)}})
			raw := filepath.Join(filepath.Dir(bp), b.Suites[0].Files[0])
			switch kind {
			case "modified":
				if err := os.WriteFile(raw, []byte(benchmark("99", 10)), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(raw); err != nil {
					t.Fatal(err)
				}
			case "checksum-missing":
				b.Inputs = nil
			case "checksum-duplicate":
				b.Inputs = append(b.Inputs, b.Inputs[0])
			case "criterion":
				b.Suites[0].Parser.Name = "criterion"
			case "different-suite":
				h.Suites[0].ID = "other"
			case "no-files":
				b.Suites[0].Files = nil
			}
			records, err := Analyze(bp, hp, b, h, filepath.Join(root, "nonexistent-tool"))
			if err == nil || records != nil {
				t.Fatalf("accepted invalid raw: %#v %v", records, err)
			}
			if strings.Contains(err.Error(), "nonexistent-tool") {
				t.Fatalf("validated tool before raw suite: %v", err)
			}
		})
	}
}

func TestRealPinnedGroupingIsolationAndDirect(t *testing.T) {
	executable := tool(t)
	root := t.TempDir()
	bp, b := makeRun(t, filepath.Join(root, "base directory"), map[string][]string{"z suite": {benchmark("200", 5), benchmark("200", 5)}, "a suite": {benchmark("100", 5), benchmark("100", 5)}})
	hp, h := makeRun(t, filepath.Join(root, "head directory"), map[string][]string{"z suite": {benchmark("100", 5), benchmark("100", 5)}, "a suite": {benchmark("200", 5), benchmark("200", 5)}})
	records, err := Analyze(bp, hp, b, h, executable)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Suite != "a suite" || records[1].Suite != "z suite" {
		t.Fatal(records)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range records {
		if len(r.Arguments) != 4 || len(r.Inputs) != 4 {
			t.Fatal(r)
		}
		if r.Tool != (model.Tool{Name: "benchstat", Version: Version}) || r.ExecutableSHA256 != hash(binary) {
			t.Fatal("tool provenance differs")
		}
		for n, in := range r.Inputs {
			side := "base"
			doc := bp
			if n >= 2 {
				side = "head"
				doc = hp
			}
			if in.Side != side || in.Suite != r.Suite || !strings.HasPrefix(r.Arguments[n], side+"=") {
				t.Fatal("input order", r)
			}
			p, err := input.ResolveReference(doc, in.Path)
			if err != nil {
				t.Fatal(err)
			}
			p, err = filepath.EvalSymlinks(p)
			if err != nil {
				t.Fatal(err)
			}
			if r.Arguments[n] != side+"="+p {
				t.Fatal("actual arguments differ")
			}
		}
		if !strings.Contains(r.Stdout, "n=10") || strings.Contains(r.Stdout, "n=5") {
			t.Fatalf("multi-file samples not pooled: %s", r.Stdout)
		}
		change := "+100.00%"
		if i == 1 {
			change = "-50.00%"
		}
		if !strings.Contains(r.Stdout, change) {
			t.Fatalf("suites pooled or inverted: %s", r.Stdout)
		}
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(executable, r.Arguments...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		if stdout.String() != r.Stdout || stderr.String() != r.Stderr {
			t.Fatal("direct invocation differs")
		}
	}
}

func TestCapturedGoDirect(t *testing.T) {
	executable := tool(t)
	out := t.TempDir()
	var runs [2]model.Run
	var paths [2]string
	for i, side := range []string{"base", "head"} {
		paths[i] = filepath.Join(out, side+".json")
		if err := input.Normalize("go", "../../testdata/captured/go-pr15/"+side+"-manifest.json", paths[i], gobench.Parse); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(paths[i])
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, &runs[i]); err != nil {
			t.Fatal(err)
		}
	}
	records, err := Analyze(paths[0], paths[1], runs[0], runs[1], executable)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Suite != "go" {
		t.Fatal(records)
	}
	for _, pkg := range []string{"engineio/packet", "engineio/payload", "engineio/transport"} {
		if !strings.Contains(records[0].Stdout, pkg) {
			t.Fatal("missing package", pkg)
		}
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(executable, records[0].Arguments...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != records[0].Stdout || stderr.String() != records[0].Stderr {
		t.Fatal("archived direct invocation differs")
	}
}

func TestSiblingDiscovery(t *testing.T) {
	// The test executable ordinarily has no sibling benchstat. Explicit discovery
	// is verified by real integration; discovery here must not fall back to PATH.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(filepath.Dir(self), "benchstat")
	if _, err = os.Stat(expected); err == nil {
		t.Skip("test binary unexpectedly has a sibling benchstat")
	}
	if _, err = ResolveExecutable(""); err == nil {
		t.Fatal("missing sibling should fail")
	}
}
