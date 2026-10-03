package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	for _, test := range []struct {
		args []string
		code int
		out  string
	}{{[]string{"--help"}, 0, "normalize"}, {[]string{"--version"}, 0, "0.1.0-dev"}, {[]string{"normalize", "--help"}, 0, "--manifest"}, {[]string{"normalize", "--version"}, 0, "0.1.0-dev"}, {[]string{"compare"}, 1, ""}, {[]string{"normalize", "--parser", "tinybench", "--manifest", "m", "--out", "o"}, 1, ""}, {[]string{"normalize", "--unknown"}, 1, ""}, {[]string{"normalize"}, 1, ""}, {[]string{"normalize", "--parser", "go", "--manifest", "m", "--out", "o", "extra"}, 1, ""}} {
		var out, err bytes.Buffer
		code := run(test.args, &out, &err)
		if code != test.code || !strings.Contains(out.String(), test.out) {
			t.Fatalf("%v code%d out%s err%s", test.args, code, out.String(), err.String())
		}
	}
}
