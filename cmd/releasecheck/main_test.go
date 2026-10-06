package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, mutate func(map[string][]byte, *manifest), extra *tar.Header) string {
	t.Helper()
	files := map[string][]byte{
		"LICENSE": []byte("project license\n"), "VERSION": []byte("0.1.0\n"),
		"benchreport": []byte("generator"), "benchstat": []byte("statistics"),
		"LICENSES/Go_LICENSE": []byte("Go license"),
		"LICENSES/NOTICE.txt": []byte("Benchmark Report 0.1.0 source code (MPL-2.0):\nhttps://github.com/sshaplygin/benchmark-report/tree/v0.1.0\n"),
	}
	candidate := false
	m := manifest{SchemaVersion: 1, Version: "0.1.0", OS: "darwin", Arch: "arm64", Benchstat: benchstatPin, Candidate: &candidate}
	if mutate != nil {
		mutate(files, &m)
	}
	if m.Files == nil {
		for name, value := range files {
			m.Files = append(m.Files, member{Path: name, SHA256: checksum(value)})
		}
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	files["manifest.json"] = data
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	for name, value := range files {
		if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(value)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err = tw.Write(value); err != nil {
			t.Fatal(err)
		}
	}
	if extra != nil {
		if err = tw.WriteHeader(extra); err != nil {
			t.Fatal(err)
		}
	}
	if err = tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = gz.Close(); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "benchreport_0.1.0_darwin_arm64.tar.gz")
	if err = os.WriteFile(name, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(name+".sha256", []byte(checksum(archive.Bytes())+"  "+filepath.Base(name)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}
func TestVerifyRelease(t *testing.T) {
	license := []byte("project license\n")
	if err := verify(fixture(t, nil, nil), "0.1.0", license); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, want string
		change     func(map[string][]byte, *manifest)
		extra      *tar.Header
	}{
		{name: "candidate", want: "candidate", change: func(_ map[string][]byte, m *manifest) { *m.Candidate = true }},
		{name: "missing candidate", want: "candidate", change: func(_ map[string][]byte, m *manifest) { m.Candidate = nil }},
		{name: "schema", want: "schema", change: func(_ map[string][]byte, m *manifest) { m.SchemaVersion = 2 }},
		{name: "wrong pin", want: "pin", change: func(_ map[string][]byte, m *manifest) { m.Benchstat = "unverified" }},
		{name: "unsupported OS", want: "platform", change: func(_ map[string][]byte, m *manifest) { m.OS = "windows" }},
		{name: "unsupported arch", want: "platform", change: func(_ map[string][]byte, m *manifest) { m.Arch = "386" }},
		{name: "license", want: "LICENSE", change: func(f map[string][]byte, _ *manifest) { f["LICENSE"] = []byte("different license") }},
		{name: "source notice", want: "source URL", change: func(f map[string][]byte, _ *manifest) { f["LICENSES/NOTICE.txt"] = []byte("missing project source") }},
		{name: "unlisted members", want: "inventory", change: func(_ map[string][]byte, m *manifest) { m.Files = []member{} }},
		{name: "duplicate inventory", want: "duplicate", change: func(_ map[string][]byte, m *manifest) {
			m.Files = []member{{Path: "LICENSE", SHA256: checksum(license)}, {Path: "LICENSE", SHA256: checksum(license)}}
		}},
		{name: "member checksum", want: "checksum", change: func(_ map[string][]byte, m *manifest) {
			m.Files = []member{{Path: "LICENSE", SHA256: strings.Repeat("0", 64)}}
		}},
		{name: "traversal", want: "unsafe", change: func(f map[string][]byte, _ *manifest) { f["../outside"] = nil }},
		{name: "absolute path", want: "unsafe", change: func(f map[string][]byte, _ *manifest) { f["/outside"] = nil }},
		{name: "backslash", want: "unsafe", change: func(f map[string][]byte, _ *manifest) { f[`LICENSES\outside`] = nil }},
		{name: "symlink", want: "nonregular", extra: &tar.Header{Name: "linked", Typeflag: tar.TypeSymlink, Linkname: "LICENSE"}},
		{name: "duplicate tar", want: "duplicate", extra: &tar.Header{Name: "LICENSE", Typeflag: tar.TypeReg}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verify(fixture(t, tt.change, tt.extra), "0.1.0", license)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v; want %s", err, tt.want)
			}
		})
	}
}
func TestManifestRejectsUnknownDuplicateAndTrailing(t *testing.T) {
	for _, data := range []string{`{"unknown":1}`, `{"version":"0.1.0","version":"0.2.0"}`, `{"files":[{"path":"LICENSE","path":"other"}]}`, `{} {}`} {
		if _, err := decodeManifest([]byte(data)); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
}
func TestChecksumBeforeArchiveRead(t *testing.T) {
	name := fixture(t, nil, nil)
	if err := os.WriteFile(name, []byte("corrupt archive"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verify(name, "0.1.0", []byte("project license\n")); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("got %v", err)
	}
}
