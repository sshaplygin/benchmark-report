package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamples(t *testing.T) {
	for _, name := range []string{"input-manifest", "normalized-run", "comparison", "presentation", "reproduction", "configuration"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../../examples/contracts", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := Validate(filepath.Join("../../schemas", name+".schema.json"), data); err != nil {
				t.Fatal(err)
			}
		})
	}
	data, err := os.ReadFile("../../examples/benchmark-report.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate("../../schemas/configuration.schema.json", data); err != nil {
		t.Fatal(err)
	}
}
func TestRejectInvalid(t *testing.T) {
	for _, data := range []string{
		`{"schema_version":1,"schema_version":1}`,
		`{"schema_version":1,"report":{"title":"a","title":"b"}}`,
		`{"schema_version":1,"unknown":true}`,
		`{"schema_version":2}`,
		`{"schema_version":1,"report":{"metrics":["cpu"]}}`,
		`{"schema_version":1,"report":{"include":["["]}}`,
		`{"schema_version":1,"outputs":{"markdown":null,"json":null}}`,
		`{"schema_version":1,"outputs":{"markdown":"../escape.md"}}`,
		`{"schema_version":1,"outputs":{"markdown":"x/./same","json":"x/same"}}`,
		`{"schema_version":1,"outputs":{"markdown":"reproduction.json"}}`,
		`{"schema_version":1,"outputs":{"json":"replay-inputs/comparison.json"}}`,
		`{"schema_version":1,"history":{"enabled":true,"bigger_file":"replay-inputs"}}`,
		`{"schema_version":1} {}`,
		`{"schema_version":1,"report":{"metrics":["time","bytes"]}}`,
		`{"schema_version":1,"report":{"sections":{"benchstat":true}}}`,
		`{"schema_version":1,"outputs":{"markdown":"report.json"}}`,
		`{"schema_version":1,"history":{"enabled":true,"smaller_file":"report.md"}}`,
		`{"schema_version":1,"outputs":{"markdown":"C:\\escape.md"}}`,
	} {
		if err := Validate("../../schemas/configuration.schema.json", []byte(data)); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}
func TestCanonicalKey(t *testing.T) {
	got := Key("a/b", "", "<x>\u2028", "time")
	if got != `["a/b","","\u003cx\u003e\u2028","time"]` {
		t.Fatal(got)
	}
}
func TestRejectIdentityMismatch(t *testing.T) {
	data, err := os.ReadFile("../../examples/contracts/normalized-run.json")
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(data), `"benchmark": "BenchmarkSmall-4"`, `"benchmark": "other"`, 1)
	if err := Validate("../../schemas/normalized-run.schema.json", []byte(bad)); err == nil {
		t.Fatal("accepted mismatched key")
	}
}

func TestRejectNormalizedRelations(t *testing.T) {
	data, err := os.ReadFile("../../examples/contracts/normalized-run.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range [][2]string{{`"expected_suites": [
    "example"`, `"expected_suites": [
    "missing"`}, {`"unit": "ns/op"`, `"unit": "B/op"`}, {`"samples": [`, `"unrecognized_samples": [`}, {`"name": "go"`, `"name": "criterion"`}, {`"suite": "example"`, `"suite": "missing"`}, {`"estimate": "2.5"`, `"estimate": "2.50"`}} {
		bad := strings.Replace(string(data), change[0], change[1], 1)
		if bad == string(data) {
			t.Fatal("mutation did not apply", change)
		}
		if err := Validate("../../schemas/normalized-run.schema.json", []byte(bad)); err == nil {
			t.Fatalf("accepted mutation %v", change)
		}
	}
}
