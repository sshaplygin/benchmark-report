package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAndPartial(t *testing.T) {
	c, err := Parse([]byte(`{"schema_version":1,"comparison":{"regression_percent":2e1},"report":{"title":"Custom"},"outputs":{"markdown":null}}`))
	if err != nil {
		t.Fatal(err)
	}
	defaults := Defaults()
	if c.Report.Title != "Custom" || c.Outputs.Markdown != nil || c.Outputs.JSON == nil || c.Report.Sections.Tables != true || c.History.Enabled || c.Comparison.RegressionPercent != "2e1" || !PolicyEqual(c.Comparison, defaults.Comparison) {
		t.Fatal(c)
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(encoded)
	if err != nil || !PolicyEqual(c.Comparison, again.Comparison) {
		t.Fatal(err)
	}
}
func TestDecimalPolicyEquality(t *testing.T) {
	a, b := Defaults().Comparison, Defaults().Comparison
	a.RegressionPercent = "1"
	b.RegressionPercent = "1.0"
	a.ImprovementPercent = "0.000000000000000000000001"
	b.ImprovementPercent = "1e-24"
	if !PolicyEqual(a, b) {
		t.Fatal("lexical comparison")
	}
	b.ImprovementPercent = "1.000000000000000000000001e-24"
	if PolicyEqual(a, b) {
		t.Fatal("precision lost")
	}
}
func TestInvalid(t *testing.T) {
	for _, raw := range []string{`{"schema_version":1,"schema_version":1}`, `{"schema_version":1,"report":{"include":["["]}}`, `{"schema_version":1,"comparison":{"regression_percent":0}}`, `{"schema_version":1,"comparison":{"improvement_percent":-1}}`, `{"schema_version":1,"comparison":{"regression_percent":"20"}}`, `{"schema_version":1,"comparison":{"percent_decimals":5}}`, `{"schema_version":1,"report":{"metrics":["time","bytes"]}}`, `{"schema_version":1,"report":{"sections":{"benchstat":true}}}`, `{"schema_version":1,"outputs":{"markdown":null,"json":null}}`, `{"schema_version":1,"outputs":{"markdown":"report.json"}}`, `{"schema_version":1,"outputs":{"markdown":"replayed"}}`, `{"schema_version":1,"report":{"unknown":true}}`, `{"schema_version":1,"history":{"enabled":true,"smaller_file":"report.md"}}`} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatal("accepted", raw)
		}
	}
}
func TestMissingSuppliedConfig(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("missing file became defaults")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
}

func TestMathematicalIntegerNotation(t *testing.T) {
	for _, raw := range []string{`{"schema_version":1.0,"comparison":{"percent_decimals":1e0},"report":{"max_rows":2.0}}`, `{"schema_version":1e0,"report":{"max_rows":10000000000000000000000000000000000000000}}`} {
		c, err := Parse([]byte(raw))
		if err != nil {
			t.Fatal(raw, err)
		}
		if c.SchemaVersion != 1 || c.Comparison.PercentDecimals != 1 {
			t.Fatal(c)
		}
	}
}
