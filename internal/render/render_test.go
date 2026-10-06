package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sshaplygin/benchmark-report/internal/adapters/criterion"
	"github.com/sshaplygin/benchmark-report/internal/adapters/gobench"
	"github.com/sshaplygin/benchmark-report/internal/compare"
	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"github.com/sshaplygin/benchmark-report/internal/input"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"github.com/sshaplygin/benchmark-report/schemas"
)

func exampleComparison(criterion bool) model.Comparison {
	cfg := config.Defaults()
	env := model.Environment{Toolchain: "go1.25.0", OS: "linux", Arch: "amd64", Runner: "benchmark-demo"}
	c := model.Comparison{SchemaVersion: 1, Base: model.Side{Revision: strings.Repeat("1", 40), Environment: env}, Head: model.Side{Revision: strings.Repeat("2", 40), Environment: env}, Policy: cfg.Comparison, Generator: model.Tool{Name: "benchreport", Version: "test"}, Rows: []model.ComparisonRow{}, Gate: "disabled"}
	if criterion {
		c.Base.Revision = strings.Repeat("3", 40)
		c.Head.Revision = strings.Repeat("4", 40)
		c.Base.Environment.Toolchain = "rust1.90.0"
		c.Head.Environment = c.Base.Environment
	}
	add := func(suite, pkg, name, metric, base, head, delta, reason, signal string) {
		id := model.Identity{Suite: suite, Package: pkg, Benchmark: name, Metric: metric}
		def := model.Definition{Unit: "ns/op", Direction: "lower", Estimator: "median"}
		if criterion {
			def.Estimator = "criterion-point-estimate"
		}
		switch metric {
		case "allocations":
			def.Unit = "allocs/op"
		case "throughput":
			def.Unit = "B/s"
			def.Direction = "higher"
		}
		measurement := func(value string) *model.Measurement {
			if value == "" {
				return nil
			}
			m := &model.Measurement{Key: id.Key(), Identity: id, Definition: def, Estimate: value}
			if criterion {
				m.Bounds = &model.Bounds{Lower: value, Upper: value}
			} else {
				m.Samples = make([]string, 10)
				for i := range m.Samples {
					m.Samples[i] = value
				}
			}
			return m
		}
		row := model.ComparisonRow{Key: id.Key(), Identity: id, Definition: def, Base: measurement(base), Head: measurement(head), Reason: reason, Signal: signal}
		if delta != "" {
			d := delta
			row.DeltaPercent = &d
		}
		c.Rows = append(c.Rows, row)
	}
	if criterion {
		add("client", "", "rows/decode", "time", "10000", "10000", "0", "comparable", "below_threshold")
		add("job", "", "rows/encode", "time", "4000", "4200", "5", "comparable", "below_threshold")
		add("skiff", "", "codec/decode", "time", "2000", "1500", "-25", "comparable", "improvement")
		add("yson", "", "codec/encode", "time", "100", "130", "30", "comparable", "regression")
		return c
	}
	packet, payload := "example.org/socket/engineio/packet", "example.org/socket/engineio/payload"
	add("go", packet, "BenchmarkDecoder-4", "time", "100", "120", "20", "comparable", "regression")
	add("go", packet, "BenchmarkEncoder-4", "time", "80", "60", "-25", "comparable", "improvement")
	add("go", payload, "BenchmarkB64Decoder-4", "time", "2000", "2100", "5", "comparable", "below_threshold")
	add("go", payload, "BenchmarkNewDecoder-4", "time", "", "900", "", "added", "not_comparable")
	add("go", payload, "BenchmarkRemovedDecoder-4", "time", "700", "", "", "removed", "not_comparable")
	add("go", payload, "BenchmarkZeroBaseline-4", "time", "0", "5", "", "zero_baseline", "not_comparable")
	add("go", packet, "BenchmarkDecoder-4", "allocations", "64", "48", "-25", "comparable", "improvement")
	add("go", packet, "BenchmarkEncoder-4", "allocations", "0", "2", "", "zero_baseline", "not_comparable")
	add("go", payload, "BenchmarkB64Decoder-4", "throughput", "100000000", "125000000", "25", "comparable", "improvement")
	return c
}
func assertPresentation(t *testing.T, p model.Presentation) {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	schema, err := schemas.Read("presentation")
	if err != nil {
		t.Fatal(err)
	}
	if err = contracts.ValidateSchema(schema, data); err != nil {
		t.Fatal(err)
	}
}
func golden(t *testing.T, name string, data []byte) {
	t.Helper()
	path := filepath.Join("testdata/golden", name+".md")
	if os.Getenv("UPDATE_RENDER_GOLDENS") == "1" {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(expected) != string(data) {
		t.Fatalf("golden %s differs:\n%s", name, data)
	}
}
func rowNames(p model.Presentation) []string {
	names := []string{}
	for _, g := range p.Groups {
		for _, r := range g.Rows {
			names = append(names, r.Identity.Benchmark)
		}
	}
	return names
}

func TestSyntheticProfiles(t *testing.T) {
	for _, name := range []string{"go", "compact", "allocations", "criterion", "empty"} {
		t.Run(name, func(t *testing.T) {
			cfg := config.Defaults()
			c := exampleComparison(name == "criterion")
			cfg.Report.Title = "Go benchmarks: base vs PR"
			cfg.Report.GroupBy = []string{"package"}
			switch name {
			case "compact":
				cfg.Report.Title = "Benchmark changes"
				cfg.Report.Columns = []string{"benchmark", "change", "signal"}
				cfg.Report.Sort = "regression"
				cfg.Report.MaxRows = "2"
				cfg.Report.Sections.Metadata = false
			case "allocations":
				cfg.Report.Title = "Allocation and throughput changes"
				cfg.Report.Metrics = []string{"allocations", "throughput"}
				cfg.Report.GroupBy = []string{"package", "metric"}
				cfg.Report.Sections.Metadata = false
			case "criterion":
				cfg.Report.Title = "Criterion benchmarks: base vs PR"
				cfg.Report.GroupBy = []string{"suite"}
				cfg.Report.Columns = []string{"benchmark", "base", "head", "change", "samples", "signal"}
			case "empty":
				cfg.Report.Title = "Benchmark comparison"
				cfg.Report.Include = []string{"does-not-exist"}
				cfg.Report.Sections.Metadata = false
			}
			p, md, err := Build(c, cfg)
			if err != nil {
				t.Fatal(err)
			}
			assertPresentation(t, p)
			golden(t, name, md)
			switch name {
			case "go":
				if p.Summary != (model.PresentationSummary{Total: 9, Selected: 6, Displayed: 6, Regression: 1, Improvement: 1, BelowThreshold: 1, NotComparable: 3}) {
					t.Fatal(p.Summary)
				}
				for _, value := range []string{"100 ns | 120 ns | +20.0%", "2 µs | 2.1 µs | +5.0%", "added", "removed", "zero baseline"} {
					if !strings.Contains(string(md), value) {
						t.Error("missing", value)
					}
				}
			case "compact":
				if !reflect.DeepEqual(rowNames(p), []string{"BenchmarkDecoder-4", "BenchmarkB64Decoder-4"}) || p.Summary.Omitted != 4 || p.Summary.Selected != 6 {
					t.Fatal(p)
				}
			case "allocations":
				if p.Summary.Selected != 3 || p.Summary.Improvement != 2 || p.Summary.NotComparable != 1 {
					t.Fatal(p.Summary)
				}
				if !strings.Contains(string(md), "100 MB/s | 125 MB/s | +25.0%") {
					t.Fatal(string(md))
				}
			case "criterion":
				if p.Summary.Selected != 4 || p.Summary.BelowThreshold != 2 {
					t.Fatal(p.Summary)
				}
				for _, g := range p.Groups {
					for _, r := range g.Rows {
						if r.Samples != "—" {
							t.Fatal(r)
						}
					}
				}
			case "empty":
				if p.Summary.Total != 9 || p.Summary.Selected != 0 || !strings.Contains(string(md), "comparison contains 9 measurements") {
					t.Fatal(p)
				}
			}
		})
	}
}

func TestSelectionOrderAndImmutability(t *testing.T) {
	c := exampleComparison(false)
	original, _ := json.Marshal(c)
	cfg := config.Defaults()
	cfg.Report.Include = []string{"Decoder", "Encoder"}
	cfg.Report.Exclude = []string{"Encoder"}
	cfg.Report.Sort = "regression"
	cfg.Report.MaxRows = "1"
	p, _, err := Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.Selected != 4 || p.Summary.Displayed != 1 || rowNames(p)[0] != "BenchmarkDecoder-4" {
		t.Fatal(p)
	}
	cfg = config.Defaults()
	cfg.Report.Missing = "hide"
	p, _, err = Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.Selected != 4 || p.Summary.NotComparable != 1 {
		t.Fatal(p.Summary)
	}
	// Exact changes that display as zero are still selected when unchanged hides.
	c.Rows[0].Head.Estimate = "100.001"
	d := "0"
	c.Rows[0].DeltaPercent = &d
	c.Rows[0].Signal = "below_threshold"
	c.Rows[1].Head.Estimate = c.Rows[1].Base.Estimate
	c.Rows[1].DeltaPercent = &d
	c.Rows[1].Signal = "below_threshold"
	cfg.Report.Unchanged = "hide"
	p, _, err = Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p.Summary.Selected != 3 {
		t.Fatal(p.Summary)
	}
	if !contains(rowNames(p), "BenchmarkDecoder-4") || contains(rowNames(p), "BenchmarkEncoder-4") {
		t.Fatal(rowNames(p))
	}
	c = exampleComparison(false)
	_, _, err = Build(c, config.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(c)
	if string(original) != string(after) {
		t.Fatal("comparison mutated")
	}
}

func TestEveryColumnAndUnits(t *testing.T) {
	cfg := config.Defaults()
	cfg.Report.Columns = []string{"benchmark", "metric", "samples", "signal", "change", "head", "base"}
	cfg.Report.Metrics = []string{"time", "allocations", "throughput"}
	cfg.Report.GroupBy = []string{}
	cfg.Report.Units = "canonical"
	p, md, err := Build(exampleComparison(false), cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertPresentation(t, p)
	if len(p.Groups) != 1 || len(p.Groups[0].Labels) != 0 || p.Summary.Selected != 9 {
		t.Fatal(p)
	}
	for _, value := range []string{"| Benchmark | Metric | Samples | Signal | Change | PR | Base |", "10 / 10", "100000000 B/s", "100 ns/op", "64 allocs/op"} {
		if !strings.Contains(string(md), value) {
			t.Error("missing", value)
		}
	}
	c := exampleComparison(false)
	c.Rows = c.Rows[:1]
	c.Rows[0].Base.Estimate = "999.9995"
	c.Rows[0].Head.Estimate = "1000"
	cfg.Report.Units = "auto"
	p, _, err = Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := p.Groups[0].Rows[0]
	if r.Base != "1 µs" || r.Head != "1 µs" {
		t.Fatal(r)
	}
	c.Rows[0].Base.Estimate = "0.0005"
	c.Rows[0].Head.Estimate = "0"
	p, _, err = Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r = p.Groups[0].Rows[0]
	if r.Base != "0.001 ns" || r.Head != "0 ns" {
		t.Fatal(r)
	}
}

func TestSectionsAndMandatoryDisclosures(t *testing.T) {
	c := exampleComparison(false)
	c.EnvironmentOverride = model.EnvironmentOverride{Allowed: true, MismatchedFields: []string{"runner"}}
	cfg := config.Defaults()
	cfg.Report.MaxRows = "1"
	cfg.Report.Sections = config.Sections{}
	cfg.Outputs.Markdown = nil
	p, md, err := Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertPresentation(t, p)
	text := string(md)
	for _, required := range []string{"Advisory thresholds", "No statistical analysis was requested", "Environment mismatch override", "5 omitted"} {
		if !strings.Contains(text, required) {
			t.Error("hidden required disclosure", required)
		}
	}
	if strings.Contains(text, "| Benchmark") || strings.Contains(text, "Base `") || strings.Contains(text, "**") {
		t.Fatal(text)
	}
	if len(p.Groups) != 1 || p.Summary.Selected != 6 {
		t.Fatal(p)
	}
	cfg.Report.Include = []string{"nothing"}
	p, md, err = Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "comparison contains 9 measurements") {
		t.Fatal(string(md))
	}
}

func TestEscapingAndFence(t *testing.T) {
	c := exampleComparison(false)
	c.Rows = c.Rows[:1]
	name := "Benchmarkpipe|`[]<script>!link(x)\\\n雪"
	c.Rows[0].Identity.Benchmark = name
	c.Rows[0].Key = c.Rows[0].Identity.Key()
	c.Rows[0].Identity.Package = "<pkg>|\nnext"
	c.Rows[0].Key = c.Rows[0].Identity.Key()
	cfg := config.Defaults()
	cfg.Report.Title = "Title\n# [click](x) <img>"
	cfg.Report.GroupBy = []string{"package"}
	p, md, err := Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if p.Groups[0].Rows[0].Identity.Benchmark != name {
		t.Fatal("JSON identity escaped prematurely")
	}
	text := string(md)
	if strings.Contains(text, "<script>") || strings.Contains(text, "<img>") || strings.Contains(text, "[click](x)") || strings.Contains(text, "pipe|") {
		t.Fatal(text)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "|") && strings.Count(line, "|") != 6 {
			t.Fatal("broken table", line)
		}
	}
	c.Policy.Statistics = "benchstat"
	cfg.Comparison = c.Policy
	cfg.Report.Sections.Benchstat = true
	c.Statistics = []model.Statistics{{Suite: "unsafe </summary>", Stdout: "```\n</details><script>escape</script>\n```\n", Stderr: "warning `stderr`\n"}}
	_, md, err = Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	text = string(md)
	if !strings.Contains(text, "````text\n```\n</details><script>") || strings.Contains(text, "<summary>Full benchstat results: unsafe </summary>") {
		t.Fatal("unsafe fence or summary", text)
	}
	if !strings.Contains(text, "Standard error:") {
		t.Fatal("lost stderr")
	}
	golden(t, "escaped", md)
}

func TestNestedGroupParentChanges(t *testing.T) {
	c := exampleComparison(false)
	c.Rows = c.Rows[:1]
	second := c.Rows[0]
	second.Identity.Suite = "second"
	second.Key = second.Identity.Key()
	c.Rows = append(c.Rows, second)
	cfg := config.Defaults()
	cfg.Report.GroupBy = []string{"suite", "package", "metric"}
	_, md, err := Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(md), "##### example.org/socket/engineio/packet") != 2 || strings.Count(string(md), "###### Time") != 2 {
		t.Fatal(string(md))
	}
}

func TestHugeRowLimitsAndPolicyMismatch(t *testing.T) {
	for _, value := range []json.Number{"0", "1e1000000", "184467440737095516160000", "9.0e0"} {
		cfg := config.Defaults()
		cfg.Report.MaxRows = value
		p, _, err := Build(exampleComparison(false), cfg)
		if err != nil {
			t.Fatal(value, err)
		}
		if p.Summary.Displayed != 6 {
			t.Fatal(value, p.Summary)
		}
	}
	for _, value := range []json.Number{"1.0", "1e0", "10e-1"} {
		cfg := config.Defaults()
		cfg.Report.MaxRows = value
		p, _, err := Build(exampleComparison(false), cfg)
		if err != nil {
			t.Fatal(value, err)
		}
		if p.Summary.Displayed != 1 {
			t.Fatal(value, p.Summary)
		}
	}
	cfg := config.Defaults()
	cfg.Comparison.RegressionPercent = "10"
	if _, _, err := Build(exampleComparison(false), cfg); err == nil {
		t.Fatal("policy changed")
	}
	cfg = config.Defaults()
	cfg.Report.Metrics = []string{"time", "throughput"}
	if _, _, err := Build(exampleComparison(false), cfg); err == nil {
		t.Fatal("metrics ambiguous")
	}
	cfg = config.Defaults()
	cfg.Report.Include = []string{"["}
	if _, _, err := Build(exampleComparison(false), cfg); err == nil {
		t.Fatal("invalid regex")
	}
}

func TestCapturedGolden(t *testing.T) {
	for _, name := range []string{"go", "criterion"} {
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			adapter := input.Adapter(gobench.Parse)
			bundle := "go-pr15"
			if name == "criterion" {
				adapter = criterion.Parse
				bundle = "criterion-four-suites"
			}
			var runs [2]model.Run
			for i, side := range []string{"base", "head"} {
				path := filepath.Join(out, side+".json")
				if err := input.Normalize(name, "../../testdata/captured/"+bundle+"/"+side+"-manifest.json", path, adapter); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(data, &runs[i]); err != nil {
					t.Fatal(err)
				}
			}
			cfg := config.Defaults()
			cfg.Report.GroupBy = []string{"suite", "package"}
			cfg.Report.Columns = append(cfg.Report.Columns, "samples")
			c, err := compare.Runs(runs[0], runs[1], cfg.Comparison, false, "test")
			if err != nil {
				t.Fatal(err)
			}
			p, md, err := Build(c, cfg)
			if err != nil {
				t.Fatal(err)
			}
			assertPresentation(t, p)
			golden(t, "captured-"+name, md)
			total, selected := 39, 13
			if name == "criterion" {
				total, selected = 28, 28
			}
			if p.Summary.Total != total || p.Summary.Selected != selected {
				t.Fatal(p.Summary)
			}
		})
	}
}

// These cases preserve useful consumer reporter regressions while applying the
// version 1 three-fractional-decimal display contract. The legacy reporter
// used four significant digits, so 0.012345 ns becomes 0.012 ns here instead
// of its legacy 0.01235 ns.
func TestConsumerTimingRegressions(t *testing.T) {
	for _, tc := range []struct{ value, want string }{{"80.04", "80.04 ns"}, {"2117", "2.117 µs"}, {"2127.5", "2.128 µs"}, {"2000000", "2 ms"}, {"3000000000", "3 s"}, {"0", "0 ns"}, {"0.012345", "0.012 ns"}} {
		c := exampleComparison(false)
		c.Rows = c.Rows[:1]
		c.Rows[0].Base.Estimate = tc.value
		c.Rows[0].Head.Estimate = tc.value
		p, _, err := Build(c, config.Defaults())
		if err != nil {
			t.Fatal(err)
		}
		r := p.Groups[0].Rows[0]
		if r.Base != tc.want || r.Head != tc.want {
			t.Errorf("%s: got %s/%s want %s", tc.value, r.Base, r.Head, tc.want)
		}
	}
}

func TestConsumerPackageOwnershipRegression(t *testing.T) {
	c := exampleComparison(false)
	c.Rows = c.Rows[:1]
	second := c.Rows[0]
	second.Identity.Package = "example.org/socket/engineio/payload"
	second.Key = second.Identity.Key()
	second.Base = &model.Measurement{Identity: second.Identity, Key: second.Key, Estimate: "200", Definition: second.Definition, Samples: []string{"200"}}
	second.Head = second.Base
	c.Rows = append([]model.ComparisonRow{second}, c.Rows...)
	cfg := config.Defaults()
	cfg.Report.GroupBy = []string{"package"}
	p, md, err := Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertPresentation(t, p)
	text := string(md)
	packet := strings.Index(text, "#### example.org/socket/engineio/packet")
	payload := strings.Index(text, "#### example.org/socket/engineio/payload")
	if packet < 0 || payload <= packet || strings.Count(text, "| Benchmark | Base | PR | Change | Signal |") != 2 {
		t.Fatal(text)
	}
	if !strings.Contains(text[packet:payload], "| Decoder-4 | 100 ns |") || strings.Contains(text[packet:payload], "| Decoder-4 | 200 ns |") || !strings.Contains(text[payload:], "| Decoder-4 | 200 ns |") {
		t.Fatal("crossed package identities", text)
	}
}
