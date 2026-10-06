package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/model"
)

func markerBytes(header string) int {
	return len("\n<!-- Sticky Pull Request Comment" + header + " -->")
}
func largeRows(count, width int) model.Comparison {
	c := exampleComparison(false)
	prototype := c.Rows[0]
	c.Rows = []model.ComparisonRow{}
	for i := 0; i < count; i++ {
		row := prototype
		row.Identity.Benchmark = fmt.Sprintf("BenchmarkRow%05d", i) + strings.Repeat("雪", width)
		row.Key = row.Identity.Key()
		left, right := *prototype.Base, *prototype.Head
		left.Identity, left.Key = row.Identity, row.Key
		right.Identity, right.Key = row.Identity, row.Key
		row.Base = &left
		row.Head = &right
		c.Rows = append(c.Rows, row)
	}
	return c
}
func checkCommentBudget(t *testing.T, body []byte, header string) {
	t.Helper()
	if !utf8.Valid(body) || len(body)+markerBytes(header) > CommentByteBudget {
		t.Fatalf("invalid comment: %d body + %d marker bytes", len(body), markerBytes(header))
	}
}

func TestCommentFullUnchangedAndBoundary(t *testing.T) {
	c := exampleComparison(false)
	cfg := config.Defaults()
	_, full, err := Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Comment(c, cfg, "benchmark-report", "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, full) {
		t.Fatal("fitting report changed")
	}
	for _, header := range []string{"x", strings.Repeat("a", 100)} {
		cfg.Report.Title = "X"
		_, original, err := Build(c, cfg)
		if err != nil {
			t.Fatal(err)
		}
		available := CommentByteBudget - markerBytes(header)
		cfg.Report.Title = strings.Repeat("A", available-len(original)+1)
		_, full, err = Build(c, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if len(full) != available {
			t.Fatal("boundary fixture", len(full), available)
		}
		result, err = Comment(c, cfg, header, "")
		if err != nil {
			t.Fatal(err)
		}
		checkCommentBudget(t, result, header)
		if !bytes.Equal(result, full) {
			t.Fatal("exact boundary changed")
		}
		cfg.Report.Title = strings.Repeat("A", CommentByteBudget)
		if _, err = Comment(c, cfg, header, "https://example.org/artifact"); err == nil || !strings.Contains(err.Error(), "fixed content") {
			t.Fatal("over-budget fixed content accepted", err)
		}
	}
	for _, header := range []string{"", strings.Repeat("a", 101), "雪", "header space", "<tag>", "x\n"} {
		if _, err = Comment(c, config.Defaults(), header, ""); err == nil {
			t.Fatal("invalid header", header)
		}
	}
}

var omissions = regexp.MustCompile(`Showing ([0-9]+) of ([0-9]+) selected measurements; ([0-9]+) omitted \(([0-9]+) by the configured row limit, ([0-9]+) by the comment byte budget\)\.`)

func omissionCounts(t *testing.T, body []byte) []int {
	t.Helper()
	matches := omissions.FindStringSubmatch(html.UnescapeString(string(body)))
	if len(matches) != 6 {
		t.Fatalf("missing omission counts: %s", body[:min(len(body), 1000)])
	}
	counts := []int{}
	for _, v := range matches[1:] {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		counts = append(counts, n)
	}
	return counts
}

func TestCommentRowPrefixConfiguredLimitAndImmutability(t *testing.T) {
	c := largeRows(100, 1000)
	cfg := config.Defaults()
	cfg.Report.MaxRows = "50"
	cfg.Report.GroupBy = []string{"package"}
	original, _ := json.Marshal(c)
	_, full, err := Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(full) < CommentByteBudget {
		t.Fatal("fixture too short")
	}
	result, err := Comment(c, cfg, "benchmark-report", "https://example.org/artifacts/run?name=(bench)&view=full")
	if err != nil {
		t.Fatal(err)
	}
	checkCommentBudget(t, result, "benchmark-report")
	counts := omissionCounts(t, result)
	shown, total, omitted, configured, budget := counts[0], counts[1], counts[2], counts[3], counts[4]
	if shown <= 0 || shown >= 50 || total != 100 || configured != 50 || budget != 50-shown || omitted != 100-shown {
		t.Fatal(counts)
	}
	if strings.Count(string(result), "| Row") != shown {
		t.Fatal("disclosure and table count differ")
	}
	for i := 0; i < shown; i++ {
		if !strings.Contains(string(result), fmt.Sprintf("| Row%05d", i)) {
			t.Fatal("missing prefix row", i)
		}
	}
	if strings.Contains(string(result), fmt.Sprintf("| Row%05d", shown)) {
		t.Fatal("retained non-prefix row")
	}
	if !strings.Contains(string(result), "https://example.org/artifacts/run?name=%28bench%29&amp;view=full") {
		t.Fatal("unsafe or changed artifact link")
	}
	repeated, err := Comment(c, cfg, "benchmark-report", "https://example.org/artifacts/run?name=(bench)&view=full")
	if err != nil || !bytes.Equal(result, repeated) {
		t.Fatal("nondeterministic comment", err)
	}
	after, _ := json.Marshal(c)
	_, afterFull, err := Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) || !bytes.Equal(full, afterFull) {
		t.Fatal("full comparison/report changed")
	}
}

func TestCommentGiantUnicodeRowAndMandatorySections(t *testing.T) {
	c := largeRows(1, 30000)
	cfg := config.Defaults()
	cfg.Report.Sections.Metadata = false
	cfg.Report.Sections.Summary = false
	c.EnvironmentOverride = model.EnvironmentOverride{Allowed: true, MismatchedFields: []string{"runner"}}
	result, err := Comment(c, cfg, "unicode", "https://example.org/full")
	if err != nil {
		t.Fatal(err)
	}
	checkCommentBudget(t, result, "unicode")
	if strings.Contains(string(result), "雪") || !strings.Contains(string(result), "Showing 0 of 1 selected") || !strings.Contains(string(result), "Environment mismatch override") || !strings.Contains(string(result), "Advisory thresholds:") {
		t.Fatal(string(result))
	}
	golden(t, "comment-zero-rows", result)
}

func TestCommentBenchstatOmissionAndDisabledTables(t *testing.T) {
	c := exampleComparison(false)
	cfg := config.Defaults()
	c.Policy.Statistics = "benchstat"
	cfg.Comparison = c.Policy
	cfg.Report.Sections.Benchstat = true
	c.Statistics = []model.Statistics{{Suite: "go", Stdout: strings.Repeat("雪```\n", 12000), Stderr: "stderr preserved only in full artifact\n"}}
	_, full, err := Build(c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Comment(c, cfg, "benchstat", "https://example.org/full")
	if err != nil {
		t.Fatal(err)
	}
	checkCommentBudget(t, result, "benchstat")
	if strings.Contains(string(result), "<details>") || strings.Contains(string(result), "stderr preserved") || strings.Contains(string(result), "by the comment byte budget") {
		t.Fatal("unexpected row/detail truncation", string(result))
	}
	if !strings.Contains(string(result), "Full benchstat details") || !strings.Contains(string(result), "| Decoder-4 |") {
		t.Fatal("missing notice or complete row")
	}
	_, after, err := Build(c, cfg)
	if err != nil || !bytes.Equal(full, after) {
		t.Fatal("full statistical artifact changed")
	}
	cfg.Report.Sections.Tables = false
	result, err = Comment(c, cfg, "benchstat", "https://example.org/full")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result), "Showing") || strings.Contains(string(result), "comment byte budget") || strings.Contains(string(result), "| Decoder") {
		t.Fatal("disabled tables treated as truncated rows", string(result))
	}
	golden(t, "comment-stats-only", result)
}

func TestCommentArtifactURLs(t *testing.T) {
	c := largeRows(1, 30000)
	cfg := config.Defaults()
	for _, value := range []string{"", "relative/path", "javascript:alert(1)", "https:///path", "file:///tmp/x", "https://user:password@example.org/path", "https://example.org/\npath", "https://example.org/\x00path"} {
		if _, err := Comment(c, cfg, "header", value); err == nil {
			t.Fatal("invalid artifact URL", value)
		}
	}
	value := `https://example.org/a)[<"']?v=[x]&q=雪`
	body, err := Comment(c, cfg, "header", value)
	if err != nil {
		t.Fatal(err)
	}
	checkCommentBudget(t, body, "header")
	if strings.Contains(string(body), `a)[<`) || !strings.Contains(string(body), "%29") || !strings.Contains(string(body), "%3C") || !strings.Contains(string(body), "&amp;q=%E9%9B%AA") {
		t.Fatal("unsafe link", string(body))
	}
}

func TestCommentRegressionPrefixBeforeGrouping(t *testing.T) {
	c := largeRows(20, 4000)
	cfg := config.Defaults()
	cfg.Report.Sort = "regression"
	cfg.Report.GroupBy = []string{"suite", "package"}
	for i := range c.Rows {
		row := &c.Rows[i]
		row.Identity.Suite = "a"
		if i%2 != 0 {
			row.Identity.Suite = "b"
		}
		row.Key = row.Identity.Key()
		row.Base.Identity, row.Base.Key = row.Identity, row.Key
		row.Head.Identity, row.Head.Key = row.Identity, row.Key
		if i < 5 {
			row.Signal = "improvement"
			delta := "-40"
			row.DeltaPercent = &delta
			row.Head.Estimate = "60"
			row.Head.Samples = make([]string, 10)
			for n := range row.Head.Samples {
				row.Head.Samples[n] = "60"
			}
		}
	}
	body, err := Comment(c, cfg, "grouped", "HTTPS://example.org/full")
	if err != nil {
		t.Fatal(err)
	}
	checkCommentBudget(t, body, "grouped")
	counts := omissionCounts(t, body)
	shown := counts[0]
	order := []int{6, 8, 10, 12, 14, 16, 18, 5, 7, 9, 11, 13, 15, 17, 19, 0, 2, 4, 1, 3}
	for _, i := range order[:shown] {
		if !strings.Contains(string(body), fmt.Sprintf("| Row%05d", i)) {
			t.Fatal("lost regression prefix row", i)
		}
	}
	for _, i := range order[shown:] {
		if strings.Contains(string(body), fmt.Sprintf("| Row%05d", i)) {
			t.Fatal("retained non-prefix row", i)
		}
	}
	if !strings.Contains(string(body), "15 regressions · 5 improvements") {
		t.Fatal("classification counts changed")
	}
}
