package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/internal/output"
	"github.com/sshaplygin/benchmark-report/internal/reproduction"
)

// Replay reads only checksum-verified snapshots, never recorded shell commands.
func Replay(manifest, directory, tool, version string) (Result, error) {
	if err := fresh(directory); err != nil {
		return Result{}, err
	}
	m, protected, snapshots, err := reproduction.ValidateFull(manifest, version)
	if err != nil {
		return Result{}, err
	}
	workspace, err := os.MkdirTemp("", "benchreport-replay-")
	if err != nil {
		return Result{}, err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(workspace) // Temporary workspace cleanup is best effort.
	for _, file := range m.Files {
		target := filepath.Join(workspace, filepath.FromSlash(file.Path))
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return Result{}, err
		}
		if err = os.WriteFile(target, snapshots[file.Path], 0600); err != nil {
			return Result{}, err
		}
	}
	original, err := input.ParseComparison(snapshots[reproduction.FullComparisonName])
	if err != nil {
		return Result{}, err
	}
	b, err := Build(Options{Parser: m.Report.Parser, BaseManifest: filepath.Join(workspace, "manifests/base.json"), HeadManifest: filepath.Join(workspace, "manifests/head.json"), ConfigPath: filepath.Join(workspace, reproduction.ConfigurationName), BenchstatPath: tool, ArtifactURL: m.Report.ArtifactURL, CommentHeader: m.Report.CommentHeader, Version: version, AllowEnvironmentMismatch: original.EnvironmentOverride.Allowed})
	if err != nil {
		return Result{}, err
	}
	var calculated model.Comparison
	for _, entry := range b.Entries {
		if entry.Path == reproduction.FullComparisonName {
			calculated, err = input.ParseComparison(entry.Data)
		}
	}
	if err != nil {
		return Result{}, err
	}
	if len(original.Statistics) != len(calculated.Statistics) {
		return Result{}, fmt.Errorf("replay statistical suite count differs")
	}
	for i := range calculated.Statistics {
		old, new := original.Statistics[i], calculated.Statistics[i]
		if m.Platform.OS == runtime.GOOS && m.Platform.Arch == runtime.GOARCH && old.ExecutableSHA256 != new.ExecutableSHA256 {
			return Result{}, fmt.Errorf("replay benchstat executable checksum differs on original platform")
		}
		if len(old.Arguments) != len(old.Inputs) || len(new.Arguments) != len(new.Inputs) {
			return Result{}, fmt.Errorf("invalid statistical invocation")
		}
		for j, in := range old.Inputs {
			prefix := in.Side + "="
			for _, arg := range []string{old.Arguments[j], new.Arguments[j]} {
				if !strings.HasPrefix(arg, prefix) || !filepath.IsAbs(strings.TrimPrefix(arg, prefix)) || !strings.HasSuffix(filepath.ToSlash(arg), "/"+in.Path) {
					return Result{}, fmt.Errorf("invalid statistical input argument")
				}
			}
		}
		new.ExecutableSHA256 = old.ExecutableSHA256
		new.Arguments = old.Arguments
		if !reflect.DeepEqual(old, new) {
			return Result{}, fmt.Errorf("replay statistical evidence differs")
		}
	}
	calculated.Statistics = original.Statistics
	if !reflect.DeepEqual(original, calculated) {
		return Result{}, fmt.Errorf("replay calculation differs from archived comparison")
	}
	// Preserve the original tool provenance after proving fresh calculations.
	for i := range b.Entries {
		entry := &b.Entries[i]
		if entry.Path == reproduction.ManifestName {
			continue
		}
		if entry.Path == reproduction.FullComparisonName {
			entry.Data = snapshots[entry.Path]
			continue
		}
		if !reflect.DeepEqual(entry.Data, snapshots[entry.Path]) {
			return Result{}, fmt.Errorf("replay output differs: %s", entry.Path)
		}
	}
	data, err := encoded(m)
	if err != nil {
		return Result{}, err
	}
	for i := range b.Entries {
		if b.Entries[i].Path == reproduction.ManifestName {
			b.Entries[i].Data = data
		}
	}
	b.Protected = append(b.Protected, protected...)
	if err = output.Transaction(directory, b.Entries, nil, b.Protected); err != nil {
		return Result{}, err
	}
	absolute, err := filepath.Abs(directory)
	if err == nil {
		absolute, err = filepath.EvalSymlinks(absolute)
	}
	if err != nil {
		return Result{}, err
	}
	for key, name := range b.Result.Files {
		b.Result.Files[key] = filepath.Join(absolute, filepath.FromSlash(name))
	}
	return b.Result, nil
}
