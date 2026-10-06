package input

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/sshaplygin/benchmark-report/internal/adapters/gobench"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T, files []string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	m := model.Manifest{SchemaVersion: 1, Revision: strings.Repeat("a", 40), Environment: model.Environment{Toolchain: "go1.25.0", OS: "linux", Arch: "amd64", Runner: "test"}, ExpectedSuites: []string{"go"}, Suites: []model.Suite{{ID: "go", Parser: model.Parser{Name: "go", Version: "1"}, Command: "go test -bench .", Files: files}}}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "manifest.json")
	write(t, path, data)
	return dir, path
}
func TestMergeRebaseAndChecksums(t *testing.T) {
	dir, manifest := fixture(t, []string{"a.txt", "b.txt"})
	a := []byte("pkg: p\nBenchmarkX-4 1 1 ns/op\nBenchmarkX-4 1 2 ns/op\n")
	b := []byte("pkg: p\nBenchmarkX-4 1 3 ns/op\nBenchmarkX-4 1 4 ns/op\n")
	write(t, filepath.Join(dir, "a.txt"), a)
	write(t, filepath.Join(dir, "b.txt"), b)
	outputDir := t.TempDir()
	out := filepath.Join(outputDir, "normalized.json")
	if err := Normalize("go", manifest, out, gobench.Parse); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var run model.Run
	if err = json.Unmarshal(data, &run); err != nil {
		t.Fatal(err)
	}
	if len(run.Measurements) != 1 || run.Measurements[0].Estimate != "2.5" || len(run.Measurements[0].Samples) != 4 {
		t.Fatal(run.Measurements)
	}
	for i, source := range [][]byte{a, b} {
		ref := run.Inputs[i]
		resolved := filepath.Join(outputDir, filepath.FromSlash(ref.Path))
		got, err := os.ReadFile(resolved)
		if err != nil || string(got) != string(source) {
			t.Fatal("unresolvable path", resolved, err)
		}
		sum := sha256.Sum256(source)
		if ref.SHA256 != hex.EncodeToString(sum[:]) || run.Suites[0].Files[i] != ref.Path {
			t.Fatal("bad provenance", ref)
		}
	}
	if err := Normalize("go", manifest, out, gobench.Parse); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(out)
	if string(data) != string(again) {
		t.Fatal("nondeterministic output")
	}
}
func TestFailuresLeaveNoOutput(t *testing.T) {
	for _, test := range []struct {
		name     string
		files    []string
		contents []string
		parser   string
	}{{"missing", []string{"missing.txt"}, nil, "go"}, {"empty", []string{"a.txt"}, []string{"PASS\n"}, "go"}, {"mismatch", []string{"a.txt"}, []string{"pkg: p\nBenchmarkX-4 1 1 ns/op\n"}, "criterion"}, {"truncated", []string{"a.txt"}, []string{"pkg: p\nBenchmarkX-4 1\n"}, "go"}, {"different metric sets", []string{"a.txt", "b.txt"}, []string{"pkg: p\nBenchmarkX-4 1 1 ns/op\n", "pkg: p\nBenchmarkX-4 1 1 ns/op 1 B/op\n"}, "go"}, {"aliased files", []string{"a.txt", "./a.txt"}, []string{"pkg: p\nBenchmarkX-4 1 1 ns/op\n"}, "go"}} {
		t.Run(test.name, func(t *testing.T) {
			dir, manifest := fixture(t, test.files)
			for i, data := range test.contents {
				write(t, filepath.Join(dir, test.files[i]), []byte(data))
			}
			out := filepath.Join(dir, "out.json")
			if err := Normalize(test.parser, manifest, out, gobench.Parse); err == nil {
				t.Fatal("accepted bad input")
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatal("output left", err)
			}
			temps, _ := filepath.Glob(filepath.Join(dir, ".benchreport-*.tmp"))
			if len(temps) != 0 {
				t.Fatal("temporary files left")
			}
		})
	}
}
func TestOutputCannotOverwriteInput(t *testing.T) {
	dir, manifest := fixture(t, []string{"a.txt"})
	source := filepath.Join(dir, "a.txt")
	data := []byte("pkg: p\nBenchmarkX-4 1 1 ns/op\n")
	write(t, source, data)
	for _, out := range []string{manifest, source} {
		if err := Normalize("go", manifest, out, gobench.Parse); err == nil {
			t.Fatal("input overwrite allowed")
		}
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Link(source, link); err != nil {
		t.Fatal(err)
	}
	if err := Normalize("go", manifest, link, gobench.Parse); err == nil {
		t.Fatal("hardlink overwrite allowed")
	}
	target := filepath.Join(dir, "target.json")
	write(t, target, []byte("old"))
	symlink := filepath.Join(dir, "symlink.json")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	if err := Normalize("go", manifest, symlink, gobench.Parse); err == nil {
		t.Fatal("symlink output allowed")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Fatal("modified target")
	}
}
func TestStrictManifest(t *testing.T) {
	dir, manifest := fixture(t, []string{"a.txt"})
	out := filepath.Join(dir, "out.json")
	for _, text := range []string{`{"schema_version":1,"schema_version":1}`, `{"schema_version":1,"unknown":true}`} {
		write(t, manifest, []byte(text))
		if err := Normalize("go", manifest, out, gobench.Parse); err == nil || !strings.Contains(err.Error(), manifest) {
			t.Fatal("missing manifest diagnostic", err)
		}
	}
}

func TestRebaseThroughDirectorySymlink(t *testing.T) {
	dir, manifest := fixture(t, []string{"a.txt"})
	source := filepath.Join(dir, "a.txt")
	write(t, source, []byte("pkg: p\nBenchmarkX-4 1 1 ns/op\n"))
	physical := t.TempDir()
	aliasParent := t.TempDir()
	alias := filepath.Join(aliasParent, "directory-link")
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(alias, "out.json")
	if err := Normalize("go", manifest, out, gobench.Parse); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var run model.Run
	if err = json.Unmarshal(data, &run); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveReference(out, run.Inputs[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(resolved)
	if err != nil || len(got) == 0 {
		t.Fatal("unresolvable symlink-relative reference", resolved, err)
	}
}
