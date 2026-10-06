package reportaction_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// parseOutputs accepts complete multiline declarations and preserves their values
// byte for byte, including trailing newlines. Other output syntax is rejected.
func parseOutputs(text string) (map[string]string, error) {
	result := map[string]string{}
	namePattern := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	delimiterPattern := regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	for text != "" {
		declaration, remaining, ok := strings.Cut(text, "\n")
		if !ok {
			return nil, fmt.Errorf("unterminated declaration")
		}
		name, delimiter, ok := strings.Cut(declaration, "<<")
		if !ok || !namePattern.MatchString(name) || !delimiterPattern.MatchString(delimiter) {
			return nil, fmt.Errorf("invalid declaration %q", declaration)
		}
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("duplicate output %q", name)
		}
		value, next, ok := strings.Cut(remaining, "\n"+delimiter+"\n")
		if !ok {
			return nil, fmt.Errorf("unterminated output %q", name)
		}
		result[name] = value
		text = next
	}
	return result, nil
}

func TestOutputParsing(t *testing.T) {
	valid := "report-path<<marker\npath\n\nmarker\ngate<<other\nfailed\nother\n"
	values, err := parseOutputs(valid)
	if err != nil || values["report-path"] != "path\n" || values["gate"] != "failed" {
		t.Fatalf("newline preservation: %v %v", values, err)
	}
	for _, bad := range []string{"gate=passed\n", "gate<<\npassed\n\n", "gate<<x\npassed", "gate<<x\npassed\nx\ngate<<y\nfailed\ny\n", "gate<<x\npassed\nx\ninjected=value\n", "bad name<<x\nvalue\nx\n"} {
		if _, err := parseOutputs(bad); err == nil {
			t.Errorf("accepted malformed outputs %q", bad)
		}
	}
}

func TestReportAction(t *testing.T) {
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	binary := filepath.Join(root, "benchreport")
	validator := filepath.Join(root, "contractcheck")
	for _, build := range []struct{ path, pkg string }{{binary, "./cmd/benchreport"}, {validator, "./cmd/contractcheck"}} {
		command := exec.Command("go", "build", "-o", build.path, build.pkg)
		command.Dir = repo
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build: %v: %s", err, output)
		}
	}
	fake := filepath.Join(root, "fake")
	if err := os.Mkdir(fake, 0755); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"go", "python", "python3"} {
		if err := os.WriteFile(filepath.Join(fake, tool), []byte("#!/bin/sh\nprintf forbidden >> \"$TEST_FORBIDDEN\"\nexit 99\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	type testCase struct {
		name, parser, config, expected string
		overrides                      map[string]string
		custom, jsonOnly, failedGate   bool
	}
	cases := []testCase{
		{name: "captured Go schema-backed report", parser: "go"},
		{name: "captured Criterion schema-backed report", parser: "criterion"},
		{name: "JSON only", parser: "go", config: `{"schema_version":1,"outputs":{"markdown":null,"json":"report.json"}}`, jsonOnly: true},
		{name: "failed enabled gate still generates outputs", parser: "go", config: `{"schema_version":1,"comparison":{"regression_percent":0.01,"fail_on_regression":true}}`, failedGate: true},
		{name: "custom output names and directory retain trailing newlines", parser: "go", config: `{"schema_version":1,"outputs":{"markdown":"custom\nreport.md\n","json":"custom\npresentation.json\n"}}`, custom: true},
	}
	for _, value := range []string{"TRUE", "1", ""} {
		cases = append(cases, testCase{name: fmt.Sprintf("reject boolean %q", value), parser: "go", overrides: map[string]string{"ALLOW_ENVIRONMENT_MISMATCH": value}, expected: "must be true or false"})
	}
	for _, value := range []string{"", "bad header", strings.Repeat("x", 101), "name\ninjected=value"} {
		cases = append(cases, testCase{name: fmt.Sprintf("reject header %q", value), parser: "go", overrides: map[string]string{"COMMENT_HEADER": value}, expected: "Invalid comment-header"})
	}
	if len(cases) != 12 {
		t.Fatalf("expected twelve wrapper cases, got %d", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "github-output")
			forbidden := filepath.Join(dir, "forbidden")
			write := func(path string, data []byte) {
				t.Helper()
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			read := func(path string) []byte {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			write(outputPath, nil)
			fixture := "go-pr15"
			if tc.parser == "criterion" {
				fixture = "criterion-four-suites"
			}
			vars := map[string]string{"PATH": fake + string(os.PathListSeparator) + os.Getenv("PATH"), "REPORT_BINARY": binary, "REPORT_PARSER": tc.parser, "BASE_MANIFEST": filepath.Join(repo, "testdata/captured", fixture, "base-manifest.json"), "HEAD_MANIFEST": filepath.Join(repo, "testdata/captured", fixture, "head-manifest.json"), "RUNNER_TEMP": dir, "GITHUB_OUTPUT": outputPath, "TEST_FORBIDDEN": forbidden}
			cleared := map[string]bool{"REPORT_CONFIG": true, "REPORT_OUTPUT_DIR": true, "ALLOW_ENVIRONMENT_MISMATCH": true, "COMMENT_HEADER": true, "ARTIFACT_URL": true}
			if tc.config != "" {
				config := filepath.Join(dir, "config.json")
				write(config, []byte(tc.config))
				vars["REPORT_CONFIG"] = config
			}
			if tc.custom {
				vars["REPORT_OUTPUT_DIR"] = filepath.Join(dir, "bundle\ninjected-key=value\n")
			}
			for key, value := range tc.overrides {
				vars[key] = value
			}
			var env []string
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				_, overridden := vars[key]
				if !overridden && !cleared[key] {
					env = append(env, entry)
				}
			}
			for key, value := range vars {
				env = append(env, key+"="+value)
			}
			command := exec.Command("/bin/bash", filepath.Join(repo, "scripts/report.sh"))
			command.Dir = repo
			command.Env = env
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err := command.Run()
			if _, statErr := os.Stat(forbidden); !os.IsNotExist(statErr) {
				t.Fatal("runtime Go or Python invocation")
			}
			emitted := string(read(outputPath))
			if tc.expected != "" {
				if err == nil || !strings.Contains(stderr.String(), tc.expected) {
					t.Fatalf("expected %q failure, got %v: %s", tc.expected, err, stderr.String())
				}
				if emitted != "" {
					t.Fatalf("failure emitted outputs: %q", emitted)
				}
				return
			}
			if err != nil {
				t.Fatalf("wrapper: %v: %s", err, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("unexpected stdout: %s", stdout.String())
			}
			values, err := parseOutputs(emitted)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(values)
			if err != nil {
				t.Fatal(err)
			}
			outputs := filepath.Join(dir, "outputs.json")
			write(outputs, encoded)
			validate := func(schema, path string) {
				t.Helper()
				command := exec.Command(validator, filepath.Join(repo, "schemas", schema), path)
				if result, err := command.CombinedOutput(); err != nil {
					t.Fatalf("schema %s: %v: %s", schema, err, result)
				}
			}
			validate("report-action-outputs.schema.json", outputs)
			for _, document := range []struct{ key, schema string }{{"comparison-path", "comparison.schema.json"}, {"reproduction-path", "reproduction.schema.json"}, {"json-path", "presentation.schema.json"}} {
				if values[document.key] != "" {
					validate(document.schema, values[document.key])
				}
			}
			for _, key := range []string{"report-path", "json-path", "comparison-path", "reproduction-path", "comment-path"} {
				if values[key] != "" {
					info, err := os.Stat(values[key])
					if err != nil || !info.Mode().IsRegular() {
						t.Fatalf("missing file %s: %v", key, err)
					}
				}
			}
			if tc.custom {
				physical, err := filepath.EvalSymlinks(vars["REPORT_OUTPUT_DIR"])
				if err != nil {
					t.Fatal(err)
				}
				if values["artifact-path"] != physical || values["report-path"] != filepath.Join(physical, "custom\nreport.md\n") || values["json-path"] != filepath.Join(physical, "custom\npresentation.json\n") {
					t.Fatalf("newline paths changed: %v", values)
				}
			}
			if tc.jsonOnly && (values["report-path"] != "" || values["comment-path"] != "") {
				t.Fatalf("JSON only outputs: %v", values)
			}
			gate := "disabled"
			if tc.failedGate {
				gate = "failed"
			}
			if values["gate"] != gate {
				t.Fatalf("gate %s, expected %s", values["gate"], gate)
			}
		})
	}
}
