// contractcheck is a development validator; it is not the planned benchreport CLI.
package main

import (
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/contracts"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: contractcheck SCHEMA DOCUMENT...")
		os.Exit(1)
	}
	for _, path := range os.Args[2:] {
		data, err := os.ReadFile(path)
		if err == nil {
			err = contracts.Validate(os.Args[1], data)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(1)
		}
	}
}
