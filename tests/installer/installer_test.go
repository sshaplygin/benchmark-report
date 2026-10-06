package installer_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type member struct {
	header tar.Header
	data   []byte
}

func TestInstaller(t *testing.T) {
	assets := os.Getenv("BENCHREPORT_TEST_ASSETS")
	if assets == "" {
		t.Skip("BENCHREPORT_TEST_ASSETS unset; native candidate archive required")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	write := func(path string, data []byte, mode os.FileMode) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, mode); err != nil {
			t.Fatal(err)
		}
	}
	version := strings.TrimSpace(string(read(filepath.Join(repo, "report/VERSION"))))
	asset := fmt.Sprintf("benchreport_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	original := read(filepath.Join(assets, asset))
	gz, err := gzip.NewReader(bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var members []member
	var manifest map[string]any
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		members = append(members, member{*header, data})
		if header.Name == "manifest.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if manifest == nil {
		t.Fatal("candidate manifest missing")
	}
	mutate := func(replace string, data []byte, omit string, extra *member) []byte {
		t.Helper()
		var buffer bytes.Buffer
		gz := gzip.NewWriter(&buffer)
		tw := tar.NewWriter(gz)
		add := func(item member) {
			header := item.header
			header.Size = int64(len(item.data))
			if err := tw.WriteHeader(&header); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write(item.data); err != nil {
				t.Fatal(err)
			}
		}
		for _, item := range members {
			if item.header.Name == omit {
				continue
			}
			if item.header.Name == replace {
				item.data = data
			}
			add(item)
		}
		if extra != nil {
			add(*extra)
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	root := t.TempDir()
	script := filepath.Join(root, "scripts/install.sh")
	write(script, read(filepath.Join(repo, "scripts/install.sh")), 0644)
	write(filepath.Join(root, "report/VERSION"), []byte(version+"\n"), 0644)
	fake := filepath.Join(root, "fake")
	executable := func(name, body string) { write(filepath.Join(fake, name), []byte("#!/bin/bash\nset -eu\n"+body), 0755) }
	executable("curl", `printf '%s\n' "$*" >> "$TEST_CURL_LOG"
url= output=
while [[ $# -gt 0 ]]; do
 case "$1" in --output) output=$2; shift 2;; https://*) url=$1; shift;; *) shift;; esac
done
[[ "$url" == "$TEST_BASE_URL/$TEST_ASSET" || "$url" == "$TEST_BASE_URL/$TEST_ASSET.sha256" ]]
case "$url" in *.sha256) cp "$TEST_CHECKSUM" "$output";; *) cp "$TEST_ARCHIVE" "$output";; esac
`)
	executable("uname", "if [[ \"$1\" == -s ]]; then printf '%s\\n' \"$TEST_OS\"; else printf '%s\\n' \"$TEST_ARCH\"; fi\n")
	for _, tool := range []string{"go", "python", "python3"} {
		executable(tool, "printf forbidden >> \"$TEST_FORBIDDEN\"; exit 99\n")
	}
	sentinel := filepath.Join(root, "outside")
	write(sentinel, []byte("preserved"), 0644)
	marker := filepath.Join(root, "binary-executed")
	checksum := func(data []byte) string { return fmt.Sprintf("%x  %s\n", sha256.Sum256(data), asset) }
	nativeOS := map[string]string{"darwin": "Darwin", "linux": "Linux"}[runtime.GOOS]
	nativeArch := map[string]string{"amd64": "x86_64", "arm64": "arm64"}[runtime.GOARCH]
	type testCase struct {
		name               string
		archive            []byte
		checksum           *string
		expected, os, arch string
	}
	strptr := func(s string) *string { return &s }
	cases := []testCase{
		{name: "native executable without runtime toolchains", archive: original},
		{name: "corrupt archive", archive: append(append([]byte{}, original...), []byte("corrupt")...), checksum: strptr(checksum(original)), expected: "Archive checksum mismatch"},
		{name: "checksum rejection before execution", archive: mutate("benchreport", []byte(fmt.Sprintf("#!/bin/sh\ntouch '%s'\nprintf 'benchreport %s\\n'\n", marker, version)), "", nil), checksum: strptr(checksum(original)), expected: "Archive checksum mismatch"},
		{name: "missing checksum", archive: original, checksum: strptr(""), expected: "Missing, duplicate, or invalid archive checksum"},
		{name: "duplicate checksum", archive: original, checksum: strptr(strings.Repeat(checksum(original), 2)), expected: "Missing, duplicate, or invalid archive checksum"},
		{name: "wrong asset checksum", archive: original, checksum: strptr(fmt.Sprintf("%x  other.tar.gz\n", sha256.Sum256(original))), expected: "Missing, duplicate, or invalid archive checksum"},
	}
	for _, field := range []string{"version", "os", "arch", "benchstat"} {
		altered := make(map[string]any, len(manifest))
		for key, value := range manifest {
			altered[key] = value
		}
		altered[field] = "wrong"
		data, err := json.Marshal(altered)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, testCase{name: "manifest " + field, archive: mutate("manifest.json", data, "", nil), expected: "Archive platform or version mismatch"})
	}
	cases = append(cases, testCase{name: "archive VERSION mismatch", archive: mutate("VERSION", []byte("9.9.9\n"), "", nil), expected: "Archive version mapping mismatch"}, testCase{name: "binary version mismatch", archive: mutate("benchreport", []byte("#!/bin/sh\nprintf 'benchreport 9.9.9\\n'\n"), "", nil), expected: "Installed binary version mismatch"})
	for _, name := range []string{"../outside", "/outside", "LICENSES/../../outside"} {
		extra := member{header: tar.Header{Name: name, Mode: 0644, Typeflag: tar.TypeReg}, data: []byte("changed")}
		cases = append(cases, testCase{name: "unsafe path " + name, archive: mutate("", nil, "", &extra), expected: "archive"})
	}
	for _, kind := range []byte{tar.TypeSymlink, tar.TypeLink} {
		extra := member{header: tar.Header{Name: "benchstat", Typeflag: kind, Linkname: sentinel}}
		name := "symlink"
		if kind == tar.TypeLink {
			name = "hardlink"
		}
		cases = append(cases, testCase{name: name, archive: mutate("", nil, "benchstat", &extra), expected: "Archive contains links or nonregular entries"})
	}
	extra := member{header: tar.Header{Name: "VERSION", Mode: 0644, Typeflag: tar.TypeReg}, data: []byte(version + "\n")}
	cases = append(cases, testCase{name: "duplicate archive member", archive: mutate("", nil, "", &extra), expected: "Duplicate archive members"}, testCase{name: "unsupported OS", archive: original, os: "Plan9", expected: "Unsupported operating system"}, testCase{name: "unsupported architecture", archive: original, arch: "sparc", expected: "Unsupported architecture"})
	if len(cases) != 20 {
		t.Fatalf("expected 20 cases, got %d", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "archive")
			sum := filepath.Join(dir, "checksum")
			output := filepath.Join(dir, "output")
			forbidden := filepath.Join(dir, "forbidden")
			log := filepath.Join(dir, "curl-log")
			write(archive, tc.archive, 0644)
			expectedChecksum := checksum(tc.archive)
			if tc.checksum != nil {
				expectedChecksum = *tc.checksum
			}
			write(sum, []byte(expectedChecksum), 0644)
			write(output, nil, 0644)
			osName, arch := nativeOS, nativeArch
			if tc.os != "" {
				osName = tc.os
			}
			if tc.arch != "" {
				arch = tc.arch
			}
			overrides := map[string]string{"PATH": fake + string(os.PathListSeparator) + os.Getenv("PATH"), "RUNNER_TEMP": dir, "GITHUB_OUTPUT": output, "TEST_ARCHIVE": archive, "TEST_CHECKSUM": sum, "TEST_CURL_LOG": log, "TEST_FORBIDDEN": forbidden, "TEST_OS": osName, "TEST_ARCH": arch, "TEST_ASSET": asset, "TEST_BASE_URL": "https://github.com/sshaplygin/benchmark-report/releases/download/v" + version}
			var env []string
			for _, value := range os.Environ() {
				key, _, _ := strings.Cut(value, "=")
				if _, exists := overrides[key]; !exists {
					env = append(env, value)
				}
			}
			for key, value := range overrides {
				env = append(env, key+"="+value)
			}
			command := exec.Command("/bin/bash", script)
			command.Env = env
			result, err := command.CombinedOutput()
			if _, statErr := os.Stat(forbidden); !os.IsNotExist(statErr) {
				t.Fatal("runtime toolchain invoked")
			}
			if string(read(sentinel)) != "preserved" {
				t.Fatal("outside file modified")
			}
			if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
				t.Fatal("unverified binary executed")
			}
			emitted := string(read(output))
			if tc.expected == "" {
				if err != nil {
					t.Fatalf("installer: %v: %s", err, result)
				}
				lines := strings.Split(strings.TrimSuffix(emitted, "\n"), "\n")
				if len(lines) != 3 || lines[0] != "bin-dir<<"+lines[2] {
					t.Fatalf("invalid outputs: %q", emitted)
				}
				tool := filepath.Join(lines[1], "benchreport")
				actual, err := exec.Command(tool, "--version").Output()
				if err != nil || strings.TrimSpace(string(actual)) != "benchreport "+version {
					t.Fatalf("installed version: %v %q", err, actual)
				}
				if _, err := os.Stat(filepath.Join(lines[1], "benchstat")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil || !strings.Contains(string(result), tc.expected) {
					t.Fatalf("expected %q failure, got %v: %s", tc.expected, err, result)
				}
				if emitted != "" {
					t.Fatalf("failure emitted output: %q", emitted)
				}
				dirs, err := filepath.Glob(filepath.Join(dir, "benchreport-install.*"))
				if err != nil || len(dirs) != 0 {
					t.Fatalf("installation not cleaned: %v %v", dirs, err)
				}
			}
			logData, err := os.ReadFile(log)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(logData)), "\n") {
				if line != "" && !strings.Contains(line, "--proto =https --proto-redir =https") {
					t.Fatalf("unsafe curl flags: %s", line)
				}
			}
		})
	}
}
