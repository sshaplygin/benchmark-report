package reproduction

import (
	"encoding/json"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/internal/output"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../examples/contracts/comparison.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := input.ParseComparison(data)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	entries, err := Build(c, cfg, data, []output.Entry{{Path: "report.md", Data: []byte("markdown")}, {Path: "report.json", Data: []byte("{}")}}, "test-version")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := output.Commit(root, entries, nil); err != nil {
		t.Fatal(err)
	}
	return root
}
func verify(root string) (model.Reproduction, []string, error) {
	return Validate(filepath.Join(root, ManifestName), filepath.Join(root, ComparisonName), filepath.Join(root, ConfigurationName), "test-version")
}
func change(t *testing.T, root string, mutation func(*model.Reproduction)) {
	t.Helper()
	path := filepath.Join(root, ManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest model.Reproduction
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	mutation(&manifest)
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestVerifiedSnapshot(t *testing.T) {
	root := fixture(t)
	manifest, protected, snapshots, err := ValidateSnapshot(filepath.Join(root, ManifestName), filepath.Join(root, ComparisonName), filepath.Join(root, ConfigurationName), "test-version")
	if err != nil || len(protected) != 5 || len(snapshots["comparison"]) == 0 || manifest.Replay.Verify != nil {
		t.Fatal(manifest, protected, err)
	}
	if err := os.WriteFile(filepath.Join(root, ComparisonName), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := input.ParseComparison(snapshots["comparison"]); err != nil {
		t.Fatal("verified bytes lost", err)
	}
	if _, _, err := verify(root); err == nil {
		t.Fatal("tamper accepted")
	}
}
func TestRejectBrokenInventories(t *testing.T) {
	for _, mutation := range []func(*model.Reproduction){func(m *model.Reproduction) { m.Files = m.Files[:len(m.Files)-1] }, func(m *model.Reproduction) { m.Base.Revision = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" }, func(m *model.Reproduction) { m.Files = append(m.Files, m.Files[0]) }, func(m *model.Reproduction) { m.Files[0].Path = "../outside" }, func(m *model.Reproduction) {
		m.ConfigurationSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	}, func(m *model.Reproduction) { m.Generator.Version = "different" }} {
		root := fixture(t)
		change(t, root, mutation)
		if _, _, err := verify(root); err == nil {
			t.Fatal("bad inventory accepted")
		}
	}
}
func TestRejectMissingEnabledReportEvenValidMetadata(t *testing.T) {
	root := fixture(t)
	change(t, root, func(m *model.Reproduction) {
		kept := []model.File{}
		for _, f := range m.Files {
			if f.Role != "markdown" {
				kept = append(kept, f)
			}
		}
		m.Files = kept
	})
	if _, _, err := verify(root); err == nil {
		t.Fatal("missing enabled report accepted")
	}
}
func TestSymlinkInventoryEscape(t *testing.T) {
	root := fixture(t)
	outside := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(outside, []byte("markdown"), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "report.md")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Fatal(err)
	}
	if _, _, err := verify(root); err == nil {
		t.Fatal("symlink inventory accepted")
	}
}
