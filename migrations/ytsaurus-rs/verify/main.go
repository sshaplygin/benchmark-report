// Command verify checks archived Criterion estimates against independent legacy syntax.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Legacy syntax accepts only a separate name followed by an anchored interval.
var timing = regexp.MustCompile(`^\s*time:\s+\[\s*([0-9.]+)\s*(ns|µs|us|ms|s)\s+([0-9.]+)\s*(ns|µs|us|ms|s)\s+([0-9.]+)\s*(ns|µs|us|ms|s)\s*\]$`)
var scales = map[string]int64{"ns": 1, "µs": 1000, "us": 1000, "ms": 1000000, "s": 1000000000}

func legacy(text string) (map[string]*big.Rat, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	result := map[string]*big.Rat{}
	for i := 0; i+1 < len(lines); i++ {
		name := strings.TrimSpace(lines[i])
		m := timing.FindStringSubmatch(lines[i+1])
		if name == "" || strings.HasPrefix(name, "Benchmarking ") || m == nil {
			continue
		}
		if m[2] != m[4] || m[4] != m[6] {
			return nil, fmt.Errorf("mixed legacy units")
		}
		value, ok := new(big.Rat).SetString(m[3])
		if !ok {
			return nil, fmt.Errorf("invalid decimal")
		}
		if _, ok = result[name]; ok {
			return nil, fmt.Errorf("duplicate legacy name")
		}
		result[name] = value.Mul(value, big.NewRat(scales[m[4]], 1))
	}
	return result, nil
}

type measurement struct {
	Identity struct {
		Suite     string `json:"suite"`
		Benchmark string `json:"benchmark"`
	} `json:"identity"`
	Estimate string `json:"estimate"`
}

func main() {
	binary := flag.String("binary", "", "benchreport executable")
	root := flag.String("root", ".", "benchmark-report checkout")
	flag.Parse()
	if *binary == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "requires --binary FILE [--root DIR]")
		os.Exit(1)
	}
	if err := verify(*root, *binary); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func verify(root, binary string) (result error) {
	binary, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	fixture := filepath.Join(root, "testdata/captured/criterion-four-suites")
	temp, err := os.MkdirTemp("", "rust-migration-oracle-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(temp)) }()
	inline := map[string]map[string]string{"base": {"read/read_table/1000": "244470", "read/read_table/10000": "1592800", "read/read_table/100000": "16179000"}, "head": {"read/read_table/1000": "246230", "read/read_table/10000": "1595300", "read/read_table/100000": "16490000"}}
	for _, side := range []string{"base", "head"} {
		output := filepath.Join(temp, side+".json")
		cmd := exec.Command(binary, "normalize", "--parser", "criterion", "--manifest", filepath.Join(fixture, side+"-manifest.json"), "--out", output)
		if data, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("normalize: %w: %s", err, data)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			return err
		}
		var document struct {
			Measurements []measurement `json:"measurements"`
		}
		if err = json.Unmarshal(data, &document); err != nil {
			return err
		}
		matched := 0
		folder := side
		if side == "head" {
			folder = "pr"
		}
		for _, s := range []struct {
			id    string
			count int
		}{{"client", 12}, {"job", 9}, {"skiff", 3}, {"yson", 4}} {
			data, err := os.ReadFile(filepath.Join(fixture, "criterion-main-vs-pr-"+s.id, folder, "benchmarks.txt"))
			if err != nil {
				return err
			}
			before, err := legacy(string(data))
			if err != nil {
				return err
			}
			after := map[string]string{}
			for _, m := range document.Measurements {
				if m.Identity.Suite == s.id {
					if _, exists := after[m.Identity.Benchmark]; exists {
						return fmt.Errorf("duplicate normalized identity")
					}
					after[m.Identity.Benchmark] = m.Estimate
				}
			}
			if len(after) != s.count {
				return fmt.Errorf("%s %s count %d", side, s.id, len(after))
			}
			for name, value := range before {
				estimate, ok := new(big.Rat).SetString(after[name])
				if !ok || estimate.Cmp(value) != 0 {
					return fmt.Errorf("%s %s estimate differs: %s", side, s.id, name)
				}
				delete(after, name)
				matched++
			}
			if s.id == "client" {
				if len(after) != 3 {
					return fmt.Errorf("unexpected inline inventory")
				}
				for name, value := range inline[side] {
					if after[name] != value {
						return fmt.Errorf("inline estimate differs %s", name)
					}
				}
			} else if len(after) != 0 {
				return fmt.Errorf("unexpected added identity %s", s.id)
			}
			fmt.Printf("%s %s legacy=%d normalized=%d\n", side, s.id, len(before), s.count)
		}
		if matched != 25 || len(document.Measurements) != 28 {
			return fmt.Errorf("incomplete four-suite inventory")
		}
		fmt.Printf("%s: 25 independent legacy decimals match; 3 inline additions; 28 estimates\n", side)
	}
	return nil
}
