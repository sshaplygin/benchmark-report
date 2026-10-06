package input

import (
	"encoding/json"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRunExactEvidence(t *testing.T) {
	source, err := os.ReadFile("../../examples/contracts/normalized-run.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "run.json")
	write(t, path, source)
	loaded, err := LoadRun(path)
	if err != nil || loaded.Measurements[0].Estimate != "2.5" {
		t.Fatal(loaded, err)
	}
	mathematical := strings.Replace(string(source), `"schema_version": 1`, `"schema_version": 1.0`, 1)
	write(t, path, []byte(mathematical))
	if _, err := LoadRun(path); err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(source), `"estimate": "2.5"`, `"estimate": "2.6"`, 1)
	write(t, path, []byte(tampered))
	if _, err := LoadRun(path); err == nil || !strings.Contains(err.Error(), "median") {
		t.Fatal("forged estimate accepted", err)
	}
}
func TestNormalizedMetricEvidenceConsistency(t *testing.T) {
	data, err := os.ReadFile("../../examples/contracts/normalized-run.json")
	if err != nil {
		t.Fatal(err)
	}
	var run model.Run
	if err = json.Unmarshal(data, &run); err != nil {
		t.Fatal(err)
	}
	bytes := run.Measurements[0]
	bytes.Identity.Metric = "bytes"
	bytes.Key = bytes.Identity.Key()
	bytes.Definition.Unit = "B/op"
	bytes.Samples = []string{"0"}
	bytes.Estimate = "0"
	run.Measurements = append(run.Measurements, bytes)
	if err := ValidateRun(run); err == nil || !strings.Contains(err.Error(), "sample counts") {
		t.Fatal("missing repeated sample evidence accepted", err)
	}
}
func TestWriteDocumentProtectsSymlinkParent(t *testing.T) {
	physical := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(alias, "missing-raw.json")
	out := filepath.Join(physical, "missing-raw.json")
	if err := WriteDocument("configuration", map[string]any{"schema_version": 1}, out, []string{protected}); err == nil {
		t.Fatal("protected absent raw file alias ignored")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("output left", err)
	}
}
