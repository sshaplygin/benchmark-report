// Package gobench normalizes standard go test benchmark text.
package gobench

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

var positiveInteger = regexp.MustCompile(`^[0-9]+$`)

type metric struct {
	name, unit, direction string
	factor                int64
}

var units = map[string]metric{"ns/op": {"time", "ns/op", "lower", 1}, "B/op": {"bytes", "B/op", "lower", 1}, "allocs/op": {"allocations", "allocs/op", "lower", 1}, "MB/s": {"throughput", "B/s", "higher", 1000000}}

// Parse preserves repeated sample order; all recognized malformed results fail.
func Parse(suite model.Suite, path string, data []byte) ([]model.Measurement, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s: invalid UTF-8", path)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	pkg := ""
	line := 0
	rows := map[string]*model.Measurement{}
	counts := map[string]int{}
	headings := map[string]bool{}
	fail := func(message string) ([]model.Measurement, error) {
		return nil, fmt.Errorf("%s:%d: %s", path, line, message)
	}
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			continue
		}
		if strings.HasPrefix(raw, "pkg:") {
			pkg = strings.TrimSpace(strings.TrimPrefix(raw, "pkg:"))
			if pkg == "" {
				return fail("empty package path")
			}
			continue
		}
		if raw == "FAIL" || strings.HasPrefix(raw, "FAIL\t") || strings.HasPrefix(raw, "FAIL ") || strings.HasPrefix(raw, "--- FAIL:") || strings.HasPrefix(raw, "panic:") || strings.HasPrefix(raw, "fatal error:") {
			return fail("benchmark harness failed")
		}
		if !strings.HasPrefix(fields[0], "Benchmark") {
			continue
		}
		// Verbose harness headings and indented sub-benchmark names have no result yet.
		if len(fields) == 1 {
			headings[pkg+"\x00"+fields[0]] = true
			continue
		}
		if pkg == "" {
			return fail("benchmark result has no pkg: package boundary")
		}
		if len(fields) < 4 || (len(fields)-2)%2 != 0 {
			return fail("truncated benchmark result; expected iteration count and value/unit pairs")
		}
		if !positiveInteger.MatchString(fields[1]) {
			return fail("invalid iteration count")
		}
		count, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || count == 0 {
			return fail("iteration count must be positive")
		}
		benchmarkKey := pkg + "\x00" + fields[0]
		counts[benchmarkKey]++
		for name := range headings {
			if name == benchmarkKey || strings.HasPrefix(benchmarkKey, name+"-") || strings.HasPrefix(benchmarkKey, name+"/") {
				delete(headings, name)
			}
		}
		seen := map[string]bool{}
		for i := 2; i < len(fields); i += 2 {
			sourceUnit := fields[i+1]
			spec, ok := units[sourceUnit]
			if !ok {
				return fail(fmt.Sprintf("unsupported metric unit %q", sourceUnit))
			}
			if seen[spec.name] {
				return fail(fmt.Sprintf("duplicate metric %q within benchmark result", spec.name))
			}
			seen[spec.name] = true
			sample, err := decimal.Scale(fields[i], spec.factor)
			if err != nil {
				return fail(fmt.Sprintf("%s: %v", sourceUnit, err))
			}
			id := model.Identity{Suite: suite.ID, Package: pkg, Benchmark: fields[0], Metric: spec.name}
			key := id.Key()
			m := rows[key]
			if m == nil {
				m = &model.Measurement{Key: key, Identity: id, Definition: model.Definition{Unit: spec.unit, Direction: spec.direction, Estimator: "median"}}
				rows[key] = m
			}
			m.Samples = append(m.Samples, sample)
		}
		if !seen["time"] {
			return fail("benchmark result missing ns/op timing")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s:%d: %w", path, line+1, err)
	}
	if len(headings) > 0 {
		return nil, fmt.Errorf("%s:%d: incomplete benchmark heading without result", path, line)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: no benchmark measurements", path)
	}
	result := make([]model.Measurement, 0, len(rows))
	for _, m := range rows {
		if len(m.Samples) != counts[m.Identity.Package+"\x00"+m.Identity.Benchmark] {
			return nil, fmt.Errorf("%s:%d: repeated benchmark has inconsistent metric set", path, line)
		}
		estimate, err := decimal.Median(m.Samples)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		m.Estimate = estimate
		result = append(result, *m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}
