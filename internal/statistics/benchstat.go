// Package statistics executes verified, pinned upstream Go statistical tooling.
package statistics

import (
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"

	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
)

const Version = contracts.BenchstatVersion
const modulePath = "golang.org/x/perf"
const commandPath = modulePath + "/cmd/benchstat"

// Analyze verifies all archived Go files and the tool before invoking benchstat.
// Inputs retain source-document-relative references; arguments record the exact
// absolute paths passed to the executable, with explicit base/head labels.
func Analyze(basePath, headPath string, base, head model.Run, executable string) ([]model.Statistics, error) {
	planned, err := prepare(basePath, headPath, base, head)
	if err != nil {
		return nil, err
	}
	executable, err = ResolveExecutable(executable)
	if err != nil {
		return nil, err
	}
	info, err := buildinfo.ReadFile(executable)
	if err != nil {
		return nil, fmt.Errorf("benchstat %s: unreadable Go build metadata: %w", executable, err)
	}
	if err = validateBuildInfo(info); err != nil {
		return nil, fmt.Errorf("benchstat %s: %w", executable, err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		return nil, fmt.Errorf("benchstat %s: %w", executable, err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	for i := range planned {
		evidence := &planned[i]
		evidence.Tool = model.Tool{Name: "benchstat", Version: Version}
		evidence.ExecutableSHA256 = digest
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(executable, evidence.Arguments...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		evidence.Stdout = stdout.String()
		evidence.Stderr = stderr.String()
		if err != nil {
			return nil, fmt.Errorf("benchstat suite %q: %w; stderr: %s", evidence.Suite, err, evidence.Stderr)
		}
	}
	return planned, nil
}

// ResolveExecutable finds the physical explicit tool or sibling benchstat path.
// Callers can protect this file from output writes before analysis or publication.
func ResolveExecutable(explicit string) (string, error) {
	if explicit == "" {
		self, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("locate benchreport executable: %w", err)
		}
		self, err = filepath.EvalSymlinks(self)
		if err != nil {
			return "", fmt.Errorf("locate benchreport executable: %w", err)
		}
		explicit = filepath.Join(filepath.Dir(self), "benchstat")
	}
	absolute, err := filepath.Abs(explicit)
	if err != nil {
		return "", err
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("benchstat %s: %w", absolute, err)
	}
	stat, err := os.Stat(physical)
	if err != nil {
		return "", fmt.Errorf("benchstat %s: %w", physical, err)
	}
	if !stat.Mode().IsRegular() {
		return "", fmt.Errorf("benchstat %s: not a regular executable file", physical)
	}
	return physical, nil
}

func validateBuildInfo(info *debug.BuildInfo) error {
	if info.Path != commandPath || info.Main.Path != modulePath || info.Main.Version != Version {
		return fmt.Errorf("expected %s@%s, found command %q module %q version %q", commandPath, Version, info.Path, info.Main.Path, info.Main.Version)
	}
	if info.Main.Replace != nil {
		return fmt.Errorf("replacement benchstat module is not supported")
	}
	for _, dep := range info.Deps {
		if dep.Replace != nil {
			return fmt.Errorf("replacement dependency %q is not supported", dep.Path)
		}
	}
	return nil
}

func prepare(basePath, headPath string, base, head model.Run) ([]model.Statistics, error) {
	baseSuites, err := goSuites(base)
	if err != nil {
		return nil, fmt.Errorf("base: %w", err)
	}
	headSuites, err := goSuites(head)
	if err != nil {
		return nil, fmt.Errorf("head: %w", err)
	}
	if len(baseSuites) != len(headSuites) {
		return nil, fmt.Errorf("benchstat base/head suite coverage differs")
	}
	ids := make([]string, 0, len(baseSuites))
	for id := range baseSuites {
		if _, ok := headSuites[id]; !ok {
			return nil, fmt.Errorf("benchstat head missing suite %q", id)
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	records := make([]model.Statistics, 0, len(ids))
	for _, id := range ids {
		record := model.Statistics{Suite: id, Arguments: []string{}, Inputs: []model.StatInput{}}
		for _, side := range []struct {
			name, path string
			run        model.Run
			suite      model.Suite
		}{{"base", basePath, base, baseSuites[id]}, {"head", headPath, head, headSuites[id]}} {
			for _, reference := range side.suite.Files {
				digest := ""
				matches := 0
				for _, in := range side.run.Inputs {
					if in.Suite == id && in.Path == reference {
						digest = in.SHA256
						matches++
					}
				}
				if matches != 1 {
					return nil, fmt.Errorf("benchstat %s suite %q input %q: expected one checksum record, found %d", side.name, id, reference, matches)
				}
				source, err := input.ResolveReference(side.path, reference)
				if err != nil {
					return nil, fmt.Errorf("benchstat %s suite %q input %q: %w", side.name, id, reference, err)
				}
				source, err = filepath.EvalSymlinks(source)
				if err != nil {
					return nil, fmt.Errorf("benchstat %s suite %q input %q: %w", side.name, id, reference, err)
				}
				data, err := os.ReadFile(source)
				if err != nil {
					return nil, fmt.Errorf("benchstat %s suite %q input %q: %w", side.name, id, reference, err)
				}
				if actual := fmt.Sprintf("%x", sha256.Sum256(data)); actual != digest {
					return nil, fmt.Errorf("benchstat %s suite %q input %q: SHA-256 mismatch: expected %s, found %s", side.name, id, reference, digest, actual)
				}
				record.Inputs = append(record.Inputs, model.StatInput{Side: side.name, Suite: id, Path: reference, SHA256: digest})
				record.Arguments = append(record.Arguments, side.name+"="+source)
			}
		}
		records = append(records, record)
	}
	return records, nil
}

func goSuites(run model.Run) (map[string]model.Suite, error) {
	if len(run.Suites) == 0 {
		return nil, fmt.Errorf("benchstat requires nonempty Go suites")
	}
	result := map[string]model.Suite{}
	for _, suite := range run.Suites {
		if suite.Parser.Name != "go" || suite.Parser.Version != "1" {
			return nil, fmt.Errorf("benchstat requires Go parser version 1; suite %q has %q version %q", suite.ID, suite.Parser.Name, suite.Parser.Version)
		}
		if len(suite.Files) == 0 {
			return nil, fmt.Errorf("suite %q has no raw input files", suite.ID)
		}
		if _, exists := result[suite.ID]; exists {
			return nil, fmt.Errorf("duplicate suite %q", suite.ID)
		}
		result[suite.ID] = suite
	}
	return result, nil
}
