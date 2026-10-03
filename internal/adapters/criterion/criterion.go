// Package criterion adapts Criterion console timing estimates to the shared model.
package criterion

import (
	"bufio"
	"bytes"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/sshaplygin/benchmark-report/internal/decimal"
	"github.com/sshaplygin/benchmark-report/internal/model"
)

var timingMarker = regexp.MustCompile(`(^|[\t ]+)time\s*:`)
var truncatedTiming = regexp.MustCompile(`(^|[\t ]+)time\s*\[`)

// Parse reads one suite log. Criterion supplies bounds and a point estimate,
// but this adapter does not infer sample data from harness progress.
func Parse(suite model.Suite, path string, data []byte) ([]model.Measurement, error) {
	fail := func(line int, message string) ([]model.Measurement, error) {
		return nil, fmt.Errorf("%s:%d: suite %q: %s", path, line, suite.ID, message)
	}
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 4096), 4*1024*1024)
	var rows []model.Measurement
	seen := map[string]bool{}
	announced := map[string]int{}
	name := ""
	lineNumber := 0
	for scan.Scan() {
		lineNumber++
		if !utf8.ValidString(scan.Text()) {
			return fail(lineNumber, "invalid UTF-8 log")
		}
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Benchmarking ") {
			candidate := strings.TrimPrefix(line, "Benchmarking ")
			for _, suffix := range []string{": Warming up for ", ": Collecting ", ": Analyzing"} {
				if i := strings.LastIndex(candidate, suffix); i >= 0 {
					candidate = candidate[:i]
					break
				}
			}
			if candidate == "" {
				return fail(lineNumber, "empty benchmark name in progress line")
			}
			name = candidate
			if _, ok := announced[name]; !ok && !seen[name] {
				announced[name] = lineNumber
			}
			continue
		}
		markers := timingMarker.FindAllStringIndex(line, -1)
		if len(markers) > 0 {
			marker := markers[len(markers)-1]
			inline := strings.TrimSpace(line[:marker[0]])
			if inline != "" {
				name = inline
			}
			if name == "" {
				return fail(lineNumber, "timing estimate has no benchmark name")
			}
			if seen[name] {
				return fail(lineNumber, fmt.Sprintf("duplicate timing estimate for benchmark %q", name))
			}
			values, err := parseBounds(strings.TrimSpace(line[marker[1]:]))
			if err != nil {
				return fail(lineNumber, fmt.Sprintf("benchmark %q: %v", name, err))
			}
			identity := model.Identity{Suite: suite.ID, Package: "", Benchmark: name, Metric: "time"}
			rows = append(rows, model.Measurement{Key: identity.Key(), Identity: identity, Definition: model.Definition{Unit: "ns/op", Direction: "lower", Estimator: "criterion-point-estimate"}, Estimate: values[1], Bounds: &model.Bounds{Lower: values[0], Upper: values[2]}})
			seen[name] = true
			delete(announced, name)
			name = ""
			continue
		}
		if truncatedTiming.MatchString(line) {
			return fail(lineNumber, "incomplete timing estimate")
		}
		if ancillary(line) {
			continue
		}
		// Unrecognized harness output is harmless. A separate-line name is used
		// only when a subsequent supported timing line supplies its estimate.
		if _, active := announced[name]; !active {
			name = line
		}
	}
	if err := scan.Err(); err != nil {
		return fail(lineNumber+1, fmt.Sprintf("read log: %v", err))
	}
	if len(announced) > 0 {
		firstName := ""
		firstLine := lineNumber + 1
		for n, l := range announced {
			if l < firstLine || l == firstLine && n < firstName {
				firstName = n
				firstLine = l
			}
		}
		return fail(firstLine, fmt.Sprintf("benchmark %q has no complete timing estimate", firstName))
	}
	if name == "time" {
		return fail(lineNumber, "incomplete timing estimate")
	}
	if len(rows) == 0 {
		return fail(1, "log contains no timing measurements")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	return rows, nil
}

func ancillary(line string) bool {
	for _, prefix := range []string{"thrpt:", "change:", "time change:", "thrpt change:", "Found ", "Warning:", "Performance has ", "No change in performance", "Updating ", "Downloading ", "Downloaded ", "Compiling ", "Finished ", "Running ", "Gnuplot not found"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	// Outlier classifications are indented rows such as "2 (10.00%) high severe".
	return strings.Contains(line, "%) ") && (strings.HasSuffix(line, " mild") || strings.HasSuffix(line, " severe"))
}

func parseBounds(text string) ([3]string, error) {
	var result [3]string
	if !strings.HasPrefix(text, "[") || !strings.HasSuffix(text, "]") {
		return result, fmt.Errorf("malformed timing interval; expected [lower unit estimate unit upper unit]")
	}
	fields := strings.Fields(text[1 : len(text)-1])
	if len(fields) != 6 {
		return result, fmt.Errorf("malformed timing interval; expected three values and units")
	}
	var exact [3]*big.Rat
	for i := 0; i < 3; i++ {
		value := fields[2*i]
		unit := fields[2*i+1]
		number, err := decimal.Parse(value)
		if err != nil {
			return result, fmt.Errorf("invalid finite nonnegative timing value %q: %v", value, err)
		}
		scale := int64(0)
		switch unit {
		case "ns":
			scale = 1
		case "us", "µs":
			scale = 1000
		case "ms":
			scale = 1000000
		case "s":
			scale = 1000000000
		default:
			return result, fmt.Errorf("unsupported timing unit %q", unit)
		}
		number.Mul(number, new(big.Rat).SetInt64(scale))
		exact[i] = number
		result[i], err = decimal.String(number)
		if err != nil {
			return result, err
		}
	}
	if exact[0].Cmp(exact[1]) > 0 || exact[1].Cmp(exact[2]) > 0 {
		return result, fmt.Errorf("timing bounds must satisfy lower <= estimate <= upper")
	}
	return result, nil
}
