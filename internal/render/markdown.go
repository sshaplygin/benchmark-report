package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sshaplygin/benchmark-report/internal/model"
)

// plain escapes every Markdown delimiter and HTML opener in user text. Entities
// keep table cells on one source line, including embedded newlines and pipes.
func plain(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\\', '|', '`', '*', '_', '[', ']', '#', '!', '(', ')', '~', '\n', '\r', '\t':
			fmt.Fprintf(&b, "&#%d;", r)
		default:
			if r < 32 || r == 127 {
				fmt.Fprintf(&b, "&#%d;", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
func metricLabel(metric string) string {
	switch metric {
	case "time":
		return "Time"
	case "bytes":
		return "Bytes"
	case "allocations":
		return "Allocations"
	case "throughput":
		return "Throughput"
	}
	return metric
}
func markdown(p model.Presentation, comparison model.Comparison) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n\n", plain(p.Title))
	if p.Sections.Metadata {
		fmt.Fprintf(&b, "Base `%s` → PR `%s`\n\n", p.Base.Revision[:min(12, len(p.Base.Revision))], p.Head.Revision[:min(12, len(p.Head.Revision))])
		environment := func(e model.Environment) string {
			return plain(e.OS) + " " + plain(e.Arch) + ", runner " + plain(e.Runner) + ", toolchain " + plain(e.Toolchain)
		}
		if p.Base.Environment == p.Head.Environment {
			fmt.Fprintf(&b, "Environment: %s for both revisions.\n\n", environment(p.Base.Environment))
		} else {
			fmt.Fprintf(&b, "Base environment: %s. Head environment: %s.\n\n", environment(p.Base.Environment), environment(p.Head.Environment))
		}
		estimators := map[string]bool{}
		for _, row := range comparison.Rows {
			estimators[row.Definition.Estimator] = true
		}
		if estimators["median"] {
			b.WriteString("Go values are medians of repeated measurements.\n\n")
		}
		if estimators["criterion-point-estimate"] {
			b.WriteString("Criterion values are point estimates; sample counts are unavailable.\n\n")
		}
	}
	if p.Sections.Summary {
		fmt.Fprintf(&b, "**%d of %d measurements selected", p.Summary.Selected, p.Summary.Total)
		if p.Summary.Selected > 0 {
			regression, improvement := "regressions", "improvements"
			if p.Summary.Regression == 1 {
				regression = "regression"
			}
			if p.Summary.Improvement == 1 {
				improvement = "improvement"
			}
			fmt.Fprintf(&b, " · %d %s · %d %s · %d below threshold · %d not comparable", p.Summary.Regression, regression, p.Summary.Improvement, improvement, p.Summary.BelowThreshold, p.Summary.NotComparable)
		}
		b.WriteString("**\n\n")
	}
	for _, disclosure := range p.Disclosures {
		b.WriteString(plain(disclosure))
		b.WriteString("\n\n")
	}
	if p.Summary.Selected == 0 {
		fmt.Fprintf(&b, "No measurements match the report selection. The comparison contains %d measurements.\n\n", p.Summary.Total)
	}
	if p.Sections.Tables {
		sources := map[string]model.ComparisonRow{}
		for _, row := range comparison.Rows {
			sources[row.Key] = row
		}
		previous := []model.PresentationLabel{}
		for _, group := range p.Groups {
			for i, label := range group.Labels {
				samePrefix := i < len(previous)
				for parent := 0; samePrefix && parent <= i; parent++ {
					samePrefix = group.Labels[parent] == previous[parent]
				}
				if samePrefix {
					continue
				}
				text := label.Value
				if label.Field == "metric" {
					text = metricLabel(text)
				}
				fmt.Fprintf(&b, "%s %s\n\n", strings.Repeat("#", 4+i), plain(text))
			}
			previous = group.Labels
			b.WriteString("|")
			for _, column := range p.Columns {
				label := strings.ToUpper(column[:1]) + column[1:]
				if column == "head" {
					label = "PR"
				}
				if column == "signal" {
					label = "Signal"
				}
				fmt.Fprintf(&b, " %s |", label)
			}
			b.WriteString("\n|")
			for _, column := range p.Columns {
				align := "---"
				if column == "base" || column == "head" || column == "change" {
					align = "---:"
				}
				fmt.Fprintf(&b, " %s |", align)
			}
			b.WriteByte('\n')
			for _, row := range group.Rows {
				source := sources[row.Key]
				b.WriteString("|")
				for _, column := range p.Columns {
					value := ""
					switch column {
					case "benchmark":
						value = row.Identity.Benchmark
						if source.Definition.Estimator == "median" {
							value = strings.TrimPrefix(value, "Benchmark")
						}
					case "base":
						value = row.Base
					case "head":
						value = row.Head
					case "change":
						value = row.Change
					case "metric":
						value = metricLabel(row.Identity.Metric)
					case "samples":
						value = row.Samples
					case "signal":
						switch row.Signal {
						case "regression":
							value = "⚠️ regression"
						case "improvement":
							value = "✅ improved"
						case "below_threshold":
							value = "ℹ️ below threshold"
						case "not_comparable":
							value = strings.ReplaceAll(source.Reason, "_", " ")
						}
					}
					fmt.Fprintf(&b, " %s |", plain(value))
				}
				b.WriteByte('\n')
			}
			b.WriteByte('\n')
		}
	}
	if p.Sections.Benchstat {
		statistics := append([]model.Statistics(nil), comparison.Statistics...)
		sort.Slice(statistics, func(i, j int) bool { return statistics[i].Suite < statistics[j].Suite })
		for _, record := range statistics {
			b.WriteString("<details>\n<summary>Full benchstat results: ")
			b.WriteString(plain(record.Suite))
			b.WriteString("</summary>\n\n")
			fenced(&b, record.Stdout)
			if record.Stderr != "" {
				b.WriteString("Standard error:\n\n")
				fenced(&b, record.Stderr)
			}
			b.WriteString("</details>\n\n")
		}
	}
	return []byte(strings.TrimRight(b.String(), "\n") + "\n")
}
func fenced(b *strings.Builder, text string) {
	longest, current := 0, 0
	for _, r := range text {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	b.WriteString(fence + "text\n")
	b.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString(fence + "\n\n")
}
