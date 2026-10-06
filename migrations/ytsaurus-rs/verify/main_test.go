package main

import (
	"math/big"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLegacySeparateSyntax(t *testing.T) {
	result, err := legacy("Benchmarking noise\n time: [1 ns 2 ns 3 ns]\nseparate/name\n time: [1.1 µs 1.2 µs 1.3 µs]\ninline time: [1 ns 2 ns 3 ns]\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result["separate/name"].Cmp(big.NewRat(1200, 1)) != 0 {
		t.Fatal(result)
	}
}
func TestArchivedIndependentOracle(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "benchreport")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/benchreport")
	cmd.Dir = root
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, data)
	}
	if err := verify(root, binary); err != nil {
		t.Fatal(err)
	}
}
