// Package history_test exercises the unmodified pinned Node action offline.
package history_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sshaplygin/benchmark-report/internal/model"
)

const pin = "4322e5726e6334590d251fc4f92bec0efafc45dc"

type harness struct {
	t                                 *testing.T
	repo, directory, upstream, binary string
}

func (h harness) command(cwd string, env []string, args ...string) (string, error) {
	h.t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cwd
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("%v: %w\n%s\n%s", args, err, stdout.String(), stderr.String())
	}
	return stdout.String(), nil
}
func (h harness) run(cwd string, env []string, args ...string) string {
	h.t.Helper()
	out, err := h.command(cwd, env, args...)
	if err != nil {
		h.t.Fatal(err)
	}
	return out
}
func (h harness) write(name string, value any) string {
	h.t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		h.t.Fatal(err)
	}
	p := filepath.Join(h.directory, name)
	if err = os.WriteFile(p, data, 0600); err != nil {
		h.t.Fatal(err)
	}
	return p
}
func read(t *testing.T, p string) []byte {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func decode(t *testing.T, data []byte) any {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
func number(t *testing.T, s string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("invalid exact number %q", s)
	}
	return r
}
func exactEqual(t *testing.T, a, b any) bool {
	t.Helper()
	switch a := a.(type) {
	case json.Number:
		b, ok := b.(json.Number)
		return ok && number(t, string(a)).Cmp(number(t, string(b))) == 0
	case []any:
		b, ok := b.([]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for i := range a {
			if !exactEqual(t, a[i], b[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		b, ok := b.(map[string]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for key, value := range a {
			other, ok := b[key]
			if !ok || !exactEqual(t, value, other) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}
func cleanEnvironment() []string {
	env := []string{}
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "SYSTEMROOT"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}
func (h harness) upstreamEnvironment(event, output, history, profile, tool, revision string) []string {
	values := map[string]string{
		"GITHUB_EVENT_PATH": event, "GITHUB_EVENT_NAME": "push", "GITHUB_REPOSITORY": "fixture/disposable", "GITHUB_WORKSPACE": h.directory,
		"GITHUB_REF": "refs/heads/main", "GITHUB_SHA": revision, "GITHUB_ACTOR": "fixture", "GITHUB_SERVER_URL": "https://example.invalid",
		"INPUT_TOOL": tool, "INPUT_NAME": profile, "INPUT_OUTPUT-FILE-PATH": output, "INPUT_GH-PAGES-BRANCH": "gh-pages",
		"INPUT_BENCHMARK-DATA-DIR-PATH": filepath.Join(h.directory, "bench"), "INPUT_EXTERNAL-DATA-JSON-PATH": history,
		"INPUT_AUTO-PUSH": "false", "INPUT_SAVE-DATA-FILE": "true", "INPUT_SKIP-FETCH-GH-PAGES": "true",
		"INPUT_COMMENT-ALWAYS": "false", "INPUT_COMMENT-ON-ALERT": "false", "INPUT_SUMMARY-ALWAYS": "false",
		"INPUT_FAIL-ON-ALERT": "false", "INPUT_ALERT-THRESHOLD": "200%",
	}
	env := cleanEnvironment()
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}
func (h harness) normalize(label, revision, toolchain, parser, log, suite string) string {
	m := model.Manifest{SchemaVersion: 1, Revision: revision, Environment: model.Environment{Toolchain: toolchain, OS: "linux", Arch: "amd64", Runner: "synthetic-runner"}, ExpectedSuites: []string{suite}, Suites: []model.Suite{{ID: suite, Parser: model.Parser{Name: parser, Version: "1"}, Command: "synthetic fixture; no workload executed", Files: []string{log}}}}
	manifest := h.write(label+"-manifest.json", m)
	out := filepath.Join(h.directory, label+"-normalized.json")
	h.run(h.repo, nil, h.binary, "normalize", "--parser", parser, "--manifest", manifest, "--out", out)
	return out
}
func (h harness) export(normalized, out string) (map[string]string, error) {
	cfg := h.write("config.json", map[string]any{"schema_version": 1, "history": map[string]any{"enabled": true, "metrics": []string{"time", "bytes", "allocations", "throughput"}}})
	stdout, err := h.command(h.repo, nil, h.binary, "export", "--input", normalized, "--config", cfg, "--output-dir", out)
	if err != nil {
		return nil, err
	}
	var result struct {
		Files map[string]string `json:"files"`
	}
	if err = json.Unmarshal([]byte(stdout), &result); err != nil {
		return nil, err
	}
	return result.Files, nil
}
func TestUpstreamHistory(t *testing.T) {
	upstream := os.Getenv("BENCHREPORT_HISTORY_UPSTREAM")
	if upstream == "" {
		t.Skip("BENCHREPORT_HISTORY_UPSTREAM unset; pinned upstream Node checkout required")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	upstream, err = filepath.Abs(upstream)
	if err != nil {
		t.Fatal(err)
	}
	h := harness{t: t, repo: repo, directory: t.TempDir(), upstream: upstream}
	h.binary = filepath.Join(h.directory, "benchreport")
	if strings.TrimSpace(h.run(upstream, nil, "git", "rev-parse", "HEAD")) != pin {
		t.Fatal("upstream checkout has wrong pin")
	}
	if strings.TrimSpace(h.run(upstream, nil, "git", "status", "--porcelain")) != "" {
		t.Fatal("upstream checkout is modified")
	}
	if _, err = os.Stat(filepath.Join(upstream, "dist/src/index.js")); err != nil {
		t.Fatal(err)
	}
	h.run(h.directory, nil, "git", "init", "--quiet", "--initial-branch=main")
	assertNoRemotes := func() {
		if strings.TrimSpace(h.run(h.directory, nil, "git", "remote")) != "" {
			t.Fatal("disposable repository gained a remote")
		}
	}
	assertNoRemotes()
	h.run(repo, nil, "go", "build", "-o", h.binary, "./cmd/benchreport")
	history := filepath.Join(h.directory, "history.json")
	counts := map[string]int{}
	expectedBenches := map[string][]any{}
	cases := []struct{ label, revision, toolchain, parser, log, profile string }{
		{"first", strings.Repeat("a", 40), "go1.25.0", "go", "go-first.txt", "go-linux-amd64-go1.25-median"},
		{"second", strings.Repeat("b", 40), "go1.25.0", "go", "go-second.txt", "go-linux-amd64-go1.25-median"},
		{"new-profile", strings.Repeat("c", 40), "go1.26.0", "go", "go-second.txt", "go-linux-amd64-go1.26-median"},
		{"criterion", strings.Repeat("d", 40), "rust1.94.0", "criterion", "criterion.txt", "criterion-linux-amd64-rust1.94-point"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			h := h
			h.t = t
			normalized := h.normalize(tc.label, tc.revision, tc.toolchain, tc.parser, filepath.Join(repo, "testdata/history", tc.log), "fixture")
			files, err := h.export(normalized, filepath.Join(h.directory, "exports"))
			if err != nil {
				t.Fatal(err)
			}
			var run model.Run
			if err = json.Unmarshal(read(t, normalized), &run); err != nil {
				t.Fatal(err)
			}
			measurements := map[string]model.Measurement{}
			for _, m := range run.Measurements {
				measurements[m.Key] = m
			}
			emitted := map[string]bool{}
			for direction, output := range files {
				benches := decode(t, read(t, output)).([]any)
				for _, value := range benches {
					row := value.(map[string]any)
					name := row["name"].(string)
					source, ok := measurements[name]
					if !ok {
						t.Fatalf("unknown exported identity %s", name)
					}
					if number(t, string(row["value"].(json.Number))).Cmp(number(t, source.Estimate)) != 0 || row["unit"] != source.Definition.Unit {
						t.Fatal("export changed normalized absolute value or unit")
					}
					expected := "smaller"
					if source.Definition.Direction == "higher" {
						expected = "bigger"
					}
					if direction != expected {
						t.Fatal("wrong direction group")
					}
					emitted[name] = true
				}
			}
			if len(emitted) != len(measurements) {
				t.Fatal("history export omitted normalized measurements")
			}
			if _, ok := files["bigger"]; !ok {
				if _, err = os.Stat(filepath.Join(h.directory, "exports/benchmark-bigger.json")); !os.IsNotExist(err) {
					t.Fatal("obsolete bigger-direction output retained")
				}
			}
			author := map[string]string{"name": "Fixture", "username": "fixture", "email": "fixture@example.invalid"}
			event := h.write("event.json", map[string]any{"ref": "refs/heads/main", "repository": map[string]string{"name": "disposable", "html_url": "https://example.invalid/fixture/disposable"}, "head_commit": map[string]any{"id": tc.revision, "message": "Synthetic history integration fixture", "timestamp": "2026-10-06T00:00:00Z", "url": "https://example.invalid/commit/" + tc.revision, "author": author, "committer": author}})
			for direction, output := range files {
				series := tc.profile + "-" + direction
				tool := "customSmallerIsBetter"
				if direction == "bigger" {
					tool = "customBiggerIsBetter"
				}
				stdout := h.run(h.directory, h.upstreamEnvironment(event, output, history, series, tool, tc.revision), "node", filepath.Join(upstream, "dist/src/index.js"))
				if !strings.Contains(stdout, "was run successfully") {
					t.Fatal("upstream action did not report success")
				}
				counts[series]++
				expectedBenches[series] = append(expectedBenches[series], decode(t, read(t, output)))
				entries := decode(t, read(t, history)).(map[string]any)["entries"].(map[string]any)[series].([]any)
				if len(entries) != counts[series] {
					t.Fatal("upstream append count mismatch")
				}
				last := entries[len(entries)-1].(map[string]any)
				if last["commit"].(map[string]any)["id"] != tc.revision || !exactEqual(t, last["benches"], decode(t, read(t, output))) {
					t.Fatal("upstream stored different commit or measurements")
				}
				for _, row := range last["benches"].([]any) {
					if _, ok := row.(map[string]any)["extra"]; !ok {
						t.Fatal("upstream lost estimator/environment extra")
					}
				}
				if tc.parser == "criterion" {
					if _, ok := last["benches"].([]any)[0].(map[string]any)["range"]; !ok {
						t.Fatal("upstream lost Criterion range")
					}
				}
			}
		})
	}
	expectedCounts := map[string]int{"go-linux-amd64-go1.25-median-smaller": 2, "go-linux-amd64-go1.25-median-bigger": 2, "go-linux-amd64-go1.26-median-smaller": 1, "go-linux-amd64-go1.26-median-bigger": 1, "criterion-linux-amd64-rust1.94-point-smaller": 1}
	if !reflect.DeepEqual(counts, expectedCounts) {
		t.Fatalf("profile isolation: %v", counts)
	}
	stored := decode(t, read(t, history)).(map[string]any)["entries"].(map[string]any)
	if len(stored) != len(expectedCounts) {
		t.Fatal("final stored history has unexpected or missing profiles")
	}
	for series, count := range expectedCounts {
		entries, ok := stored[series].([]any)
		if !ok || len(entries) != count {
			t.Fatalf("final stored profile %s: want %d appends", series, count)
		}
		for i, entry := range entries {
			if !exactEqual(t, entry.(map[string]any)["benches"], expectedBenches[series][i]) {
				t.Fatalf("later append changed stored measurements in %s entry %d", series, i)
			}
		}
	}
	h.t = t
	assertNoRemotes()
	numericBoundaries(h)
	assertNoRemotes()
}
func numericBoundaries(h harness) {
	parent := h.t
	tests := []struct {
		text     string
		accepted bool
	}{{"0.1", true}, {"61.145", true}, {"9007199254740992", true}, {"9007199254740993", false}, {"0.10000000000000000001", false}, {"5e-324", true}, {"2e-324", false}, {"1e309", false}, {"1.7976931348623157e308", true}}
	script := `const fs=require('fs');const {BenchmarkResults}=require(process.argv[1]);const rows=BenchmarkResults.parse(JSON.parse(fs.readFileSync(process.argv[2],'utf8')));process.stdout.write(JSON.stringify(rows[0].value));`
	for i, tc := range tests {
		parent.Run("number-"+tc.text, func(t *testing.T) {
			h := h
			h.t = t
			raw := filepath.Join(h.directory, fmt.Sprintf("number-%d.txt", i))
			if err := os.WriteFile(raw, []byte("pkg: numeric\nBenchmarkNumber-4 10 "+tc.text+" ns/op\n"), 0600); err != nil {
				t.Fatal(err)
			}
			normalized := h.normalize(fmt.Sprintf("number-%d", i), strings.Repeat("a", 40), "go1.25.0", "go", raw, "numeric")
			var run model.Run
			if err := json.Unmarshal(read(t, normalized), &run); err != nil {
				t.Fatal(err)
			}
			if len(run.Measurements) != 1 || number(t, run.Measurements[0].Estimate).Cmp(number(t, tc.text)) != 0 {
				t.Fatal("normalization changed the independent numeric fixture")
			}
			out := filepath.Join(h.directory, fmt.Sprintf("number-%d-export", i))
			files, err := h.export(normalized, out)
			if (err == nil) != tc.accepted {
				t.Fatalf("export acceptance=%t; expected %t: %v", err == nil, tc.accepted, err)
			}
			if tc.accepted {
				parsed := h.run(h.directory, cleanEnvironment(), "node", "-e", script, filepath.Join(h.upstream, "dist/src/extract.js"), files["smaller"])
				if number(t, parsed).Cmp(number(t, tc.text)) != 0 {
					t.Fatalf("upstream numeric roundtrip changed %s to %s", tc.text, parsed)
				}
			} else {
				entries, err := filepath.Glob(filepath.Join(out, "*.json"))
				if err != nil || len(entries) != 0 {
					t.Fatal("failed export wrote JSON output")
				}
			}
		})
	}
}
