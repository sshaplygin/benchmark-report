// Package contracts validates versioned JSON documents without parsing benchmark logs.
package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const SchemaVersion = 1
const ParserVersion = "1"
const BenchstatVersion = "v0.0.0-20251023143056-3684bd442cc8"

// Decode rejects duplicate object keys at every depth before schema validation.
func Decode(data []byte) (any, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("invalid UTF-8 JSON")
	}
	if err := validateSurrogates(data); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	v, err := value(d, "$")
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON content")
	}
	return v, nil
}
func value(d *json.Decoder, path string) (any, error) {
	t, err := d.Token()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return t, nil
	}
	switch delim {
	case '{':
		out := map[string]any{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, err
			}
			key := k.(string)
			if _, ok := out[key]; ok {
				return nil, fmt.Errorf("%s.%s: duplicate field", path, key)
			}
			v, err := value(d, path+"."+key)
			if err != nil {
				return nil, err
			}
			out[key] = v
		}
		_, err = d.Token()
		return out, err
	case '[':
		out := []any{}
		for d.More() {
			v, err := value(d, fmt.Sprintf("%s[%d]", path, len(out)))
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		_, err = d.Token()
		return out, err
	}
	return nil, fmt.Errorf("%s: unexpected delimiter", path)
}

// Validate validates a document against a local schema, with no network access.
func Validate(schemaPath string, data []byte) error {

	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	return ValidateSchema(raw, data)
}

// ValidateSchema validates against an embedded or explicitly supplied schema document.
func ValidateSchema(raw, data []byte) error {
	v, err := Decode(data)
	if err != nil {
		return err
	}
	schemaDoc, err := Decode(raw)
	if err != nil {
		return err
	}
	c := jsonschema.NewCompiler()
	if err = c.AddResource("urn:benchreport:schema", schemaDoc); err != nil {
		return err
	}
	schema, err := c.Compile("urn:benchreport:schema")
	if err != nil {
		return err
	}
	if err = schema.Validate(v); err != nil {
		return err
	}
	return semantic(v)
}
func semantic(v any) error {
	switch x := v.(type) {
	case []any:
		for _, v := range x {
			if err := semantic(v); err != nil {
				return err
			}
		}
	case map[string]any:
		if err := documentRules(x); err != nil {
			return err
		}
		if id, ok := x["identity"].(map[string]any); ok {
			key := Key(id["suite"].(string), id["package"].(string), id["benchmark"].(string), id["metric"].(string))
			if x["key"] != key {
				return fmt.Errorf("key: must equal canonical identity %s", key)
			}
		}
		for _, field := range []string{"include", "exclude"} {
			if patterns, ok := x[field].([]any); ok {
				for _, p := range patterns {
					if _, err := regexp.Compile(p.(string)); err != nil {
						return fmt.Errorf("%s: %w", field, err)
					}
				}
			}
		}
		if metrics, ok := x["metrics"].([]any); ok && len(metrics) > 1 && x["columns"] != nil && x["group_by"] != nil {
			has := func(v any) bool {
				for _, a := range v.([]any) {
					if a == "metric" {
						return true
					}
				}
				return false
			}
			if !has(x["columns"]) && !has(x["group_by"]) {
				return fmt.Errorf("report: multiple metrics require metric column or grouping")
			}
		}
		if outputs, ok := x["outputs"].(map[string]any); ok {
			a, aa := outputs["markdown"]
			b, bb := outputs["json"]
			if aa && bb && a == nil && b == nil {
				return fmt.Errorf("outputs: at least one format required")
			}
		}
		for _, v := range x {
			if err := semantic(v); err != nil {
				return err
			}
		}
	}
	return nil
}

// Key uses Go JSON escaping, including HTML and U+2028/U+2029, with no whitespace.
func Key(suite, pkg, benchmark, metric string) string {
	data, _ := json.Marshal([]string{suite, pkg, benchmark, metric})
	return strings.TrimSpace(string(data))
}

func documentRules(x map[string]any) error {
	if expected, ok := x["expected_suites"].([]any); ok {
		want := map[string]bool{}
		for _, v := range expected {
			want[v.(string)] = true
		}
		seen := map[string]bool{}
		suiteParsers := map[string]string{}
		suiteFiles := map[string]map[string]bool{}
		for _, v := range x["suites"].([]any) {
			id := v.(map[string]any)["id"].(string)
			if seen[id] || !want[id] {
				return fmt.Errorf("suites: duplicate or unexpected suite %q", id)
			}
			seen[id] = true
			spec := v.(map[string]any)
			suiteParsers[id] = spec["parser"].(map[string]any)["name"].(string)
			suiteFiles[id] = map[string]bool{}
			for _, f := range spec["files"].([]any) {
				suiteFiles[id][f.(string)] = true
			}
		}
		if len(seen) != len(want) {
			return fmt.Errorf("suites: expected suite missing")
		}
		if values, ok := x["measurements"].([]any); ok {
			covered := map[string]bool{}
			keys := map[string]bool{}
			for _, v := range values {
				m := v.(map[string]any)
				id := m["identity"].(map[string]any)
				suite := id["suite"].(string)
				if !want[suite] {
					return fmt.Errorf("measurements: unexpected suite %q", suite)
				}
				covered[suite] = true
				def := m["definition"].(map[string]any)
				if suiteParsers[suite] == "criterion" && (id["package"] != "" || def["estimator"] != "criterion-point-estimate") {
					return fmt.Errorf("measurements: Criterion suite requires timing point estimator and empty package")
				}
				if suiteParsers[suite] == "go" && def["estimator"] != "median" {
					return fmt.Errorf("measurements: Go suite requires median estimator")
				}
				k := m["key"].(string)
				if keys[k] {
					return fmt.Errorf("measurements: duplicate key %s", k)
				}
				keys[k] = true
			}
			inputsSeen := map[string]map[string]bool{}
			for _, input := range x["inputs"].([]any) {
				in := input.(map[string]any)
				suite := in["suite"].(string)
				path := in["path"].(string)
				if !suiteFiles[suite][path] {
					return fmt.Errorf("inputs: unexpected suite or path")
				}
				if inputsSeen[suite] == nil {
					inputsSeen[suite] = map[string]bool{}
				}
				if inputsSeen[suite][path] {
					return fmt.Errorf("inputs: duplicate path")
				}
				inputsSeen[suite][path] = true
			}
			for suite, files := range suiteFiles {
				if len(inputsSeen[suite]) != len(files) {
					return fmt.Errorf("inputs: suite file inventory incomplete")
				}
			}
			if len(covered) != len(want) {
				return fmt.Errorf("measurements: empty suite")
			}
		}
	}
	if id, ok := x["identity"].(map[string]any); ok && x["definition"] != nil {
		def := x["definition"].(map[string]any)
		units := map[string]string{"time": "ns/op", "bytes": "B/op", "allocations": "allocs/op", "throughput": "B/s"}
		if def["unit"] != units[id["metric"].(string)] {
			return fmt.Errorf("definition: metric unit mismatch")
		}
		if est, ok := x["estimate"].(string); ok {
			number := func(v string) *big.Rat { n, _ := new(big.Rat).SetString(v); return n }
			e := number(est)
			if bounds, ok := x["bounds"].(map[string]any); ok {
				if number(bounds["lower"].(string)).Cmp(e) > 0 || number(bounds["upper"].(string)).Cmp(e) < 0 {
					return fmt.Errorf("bounds: must contain estimate")
				}
			}
			if def["estimator"] == "median" && x["samples"] == nil {
				return fmt.Errorf("samples: median requires samples")
			}
			if def["estimator"] == "criterion-point-estimate" && (id["metric"] != "time" || x["samples"] != nil || x["bounds"] == nil) {
				return fmt.Errorf("definition: Criterion timing requires bounds and no invented samples")
			}
		}
	}
	if _, ok := x["schema_version"]; ok && x["revision"] == nil && x["rows"] == nil && x["groups"] == nil && x["files"] == nil {
		get := func(m map[string]any, k string, d any) any {
			if v, ok := m[k]; ok {
				return v
			}
			return d
		}
		report, _ := x["report"].(map[string]any)
		policy, _ := x["comparison"].(map[string]any)
		sections, _ := report["sections"].(map[string]any)
		if get(sections, "benchstat", false) == true && get(policy, "statistics", "none") != "benchstat" {
			return fmt.Errorf("report.sections.benchstat requires comparison.statistics benchstat")
		}
		metrics := get(report, "metrics", []any{"time"}).([]any)
		has := func(v any) bool {
			for _, a := range v.([]any) {
				if a == "metric" {
					return true
				}
			}
			return false
		}
		if len(metrics) > 1 && !has(get(report, "columns", []any{"benchmark", "base", "head", "change", "signal"})) && !has(get(report, "group_by", []any{"suite", "package"})) {
			return fmt.Errorf("report: multiple metrics require metric column or grouping")
		}
		outputs, _ := x["outputs"].(map[string]any)
		names := map[string]bool{}
		add := func(v any) error {
			if v == nil {
				return nil
			}
			n := v.(string)
			if n == "reproduction.json" || strings.HasPrefix(n, "reproduction.json/") || n == "replay-inputs" || strings.HasPrefix(n, "replay-inputs/") {
				return fmt.Errorf("outputs: reserved reproduction path %q", n)
			}
			if names[n] {
				return fmt.Errorf("outputs: filename collision %q", n)
			}
			names[n] = true
			return nil
		}
		if err := add(get(outputs, "markdown", "report.md")); err != nil {
			return err
		}
		if err := add(get(outputs, "json", "report.json")); err != nil {
			return err
		}
		history, _ := x["history"].(map[string]any)
		if get(history, "enabled", false) == true {
			if err := add(get(history, "smaller_file", "benchmark-smaller.json")); err != nil {
				return err
			}
			if err := add(get(history, "bigger_file", "benchmark-bigger.json")); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateSurrogates prevents encoding/json from replacing invalid identity escapes.
func validateSurrogates(data []byte) error {
	inside := false
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			inside = !inside
			continue
		}
		if !inside || data[i] != '\\' {
			continue
		}
		i++
		if i >= len(data) {
			break
		}
		if data[i] != 'u' {
			continue
		}
		if i+4 >= len(data) {
			break
		}
		code, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil {
			continue
		}
		i += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return fmt.Errorf("JSON string: unpaired low surrogate")
		}
		if code >= 0xd800 && code <= 0xdbff {
			if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
				return fmt.Errorf("JSON string: unpaired high surrogate")
			}
			low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return fmt.Errorf("JSON string: invalid surrogate pair")
			}
			i += 6
		}
	}
	return nil
}

// Unmarshal decodes a schema-validated document into typed structs, accepting
// mathematical JSON integers such as 1.0 and 1e0 while preserving decimal policy text.
func Unmarshal(data []byte, target any) error {
	value, err := Decode(data)
	if err != nil {
		return err
	}
	var normalize func(any) error
	normalize = func(value any) error {
		switch v := value.(type) {
		case []any:
			for _, child := range v {
				if err := normalize(child); err != nil {
					return err
				}
			}
		case map[string]any:
			for key, child := range v {
				if key == "schema_version" || key == "percent_decimals" {
					number, ok := child.(json.Number)
					if ok {
						n, ok := new(big.Rat).SetString(string(number))
						if !ok || !n.IsInt() {
							return fmt.Errorf("%s: integer required", key)
						}
						v[key] = json.Number(n.Num().String())
					}
				}
				if err := normalize(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = normalize(value); err != nil {
		return err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(canonical, target)
}
