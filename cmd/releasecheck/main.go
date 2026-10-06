// Command releasecheck verifies licensed release archives before publication.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sshaplygin/benchmark-report/internal/contracts"
)

const benchstatPin = contracts.BenchstatVersion
const maxArchiveBytes = 256 << 20
const maxMemberBytes = 128 << 20
const maxExpandedBytes = 512 << 20

type member struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type manifest struct {
	SchemaVersion int      `json:"schema_version"`
	Version       string   `json:"version"`
	OS            string   `json:"os"`
	Arch          string   `json:"arch"`
	Benchstat     string   `json:"benchstat"`
	Candidate     *bool    `json:"candidate"`
	Files         []member `json:"files"`
}

func main() {
	assets := flag.String("assets", "", "directory containing archives and checksum sidecars")
	flag.Parse()
	if flag.NArg() != 0 || *assets == "" {
		fmt.Fprintln(os.Stderr, "releasecheck requires --assets DIR and no positional arguments")
		os.Exit(1)
	}
	if err := run(*assets, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(assets string, stdout io.Writer) error {
	data, err := os.ReadFile("report/VERSION")
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(data))
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.-]+)?$`).MatchString(version) {
		return fmt.Errorf("invalid report/VERSION")
	}
	license, err := os.ReadFile("LICENSE")
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(license)) == 0 {
		return fmt.Errorf("empty project LICENSE")
	}
	archives, err := filepath.Glob(filepath.Join(assets, "*.tar.gz"))
	if err != nil {
		return err
	}
	if len(archives) == 0 {
		return fmt.Errorf("no release archives")
	}
	for _, archive := range archives {
		if err = verify(archive, version, license); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(archive), err)
		}
		if _, err = fmt.Fprintf(stdout, "PASS licensed release inventory and checksums: %s\n", filepath.Base(archive)); err != nil {
			return err
		}
	}
	return nil
}
func checksum(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func safeName(name string) bool {
	if name == "" || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") {
		return false
	}
	for _, r := range name {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
}
func verify(filename, version string, license []byte) error {
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > maxArchiveBytes {
		return fmt.Errorf("invalid or oversized archive")
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	sidecar, err := os.ReadFile(filename + ".sha256")
	if err != nil {
		return err
	}
	fields := strings.Fields(string(sidecar))
	if len(fields) != 2 || fields[1] != filepath.Base(filename) || fields[0] != checksum(data) {
		return fmt.Errorf("invalid archive checksum sidecar or checksum mismatch")
	}
	contents, err := readArchive(data)
	if err != nil {
		return err
	}
	m, err := decodeManifest(contents["manifest.json"])
	if err != nil {
		return err
	}
	if m.SchemaVersion != 1 || m.Candidate == nil || *m.Candidate || m.Version != version {
		return fmt.Errorf("candidate, unsupported schema, or mismatched release version")
	}
	if (m.OS != "linux" && m.OS != "darwin") || (m.Arch != "amd64" && m.Arch != "arm64") {
		return fmt.Errorf("unsupported release platform")
	}
	if m.Benchstat != benchstatPin {
		return fmt.Errorf("release benchstat pin mismatch")
	}
	if filepath.Base(filename) != fmt.Sprintf("benchreport_%s_%s_%s.tar.gz", version, m.OS, m.Arch) {
		return fmt.Errorf("release platform or version does not match asset name")
	}
	if !bytes.Equal(contents["LICENSE"], license) {
		return fmt.Errorf("release LICENSE differs from repository LICENSE")
	}
	if strings.TrimSpace(string(contents["VERSION"])) != version {
		return fmt.Errorf("archive VERSION mismatch")
	}
	if _, ok := contents["PROJECT-LICENSE-STATUS.txt"]; ok {
		return fmt.Errorf("candidate license notice in release")
	}
	listed := map[string]bool{}
	for _, file := range m.Files {
		if !safeName(file.Path) || file.Path == "manifest.json" || listed[file.Path] {
			return fmt.Errorf("invalid or duplicate inventory member %q", file.Path)
		}
		listed[file.Path] = true
		value, ok := contents[file.Path]
		if !ok || checksum(value) != file.SHA256 {
			return fmt.Errorf("member checksum mismatch: %s", file.Path)
		}
	}
	if len(listed) != len(contents)-1 {
		return fmt.Errorf("incomplete release inventory")
	}
	for _, name := range []string{"benchreport", "benchstat", "LICENSE", "VERSION", "LICENSES/NOTICE.txt", "LICENSES/Go_LICENSE"} {
		if len(contents[name]) == 0 {
			return fmt.Errorf("missing or empty required release member: %s", name)
		}
	}
	notice := fmt.Sprintf("Benchmark Report %s source code (MPL-2.0):\nhttps://github.com/sshaplygin/benchmark-report/tree/v%s\n", version, version)
	if !bytes.HasPrefix(contents["LICENSES/NOTICE.txt"], []byte(notice)) {
		return fmt.Errorf("missing versioned project source URL or MPL-2.0 notice")
	}
	return nil
}
func readArchive(data []byte) (map[string][]byte, error) {
	source := bytes.NewReader(data)
	gz, err := gzip.NewReader(source)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	gz.Multistream(false)
	tr := tar.NewReader(gz)
	files := map[string][]byte{}
	var expanded int64
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !safeName(header.Name) || header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("unsafe or nonregular archive member %q", header.Name)
		}
		if _, ok := files[header.Name]; ok {
			return nil, fmt.Errorf("duplicate archive member %q", header.Name)
		}
		expanded += header.Size
		if header.Size < 0 || header.Size > maxMemberBytes || expanded > maxExpandedBytes {
			return nil, fmt.Errorf("oversized expanded archive")
		}
		value, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[header.Name] = value
	}
	remainder, err := io.ReadAll(io.LimitReader(gz, maxMemberBytes+1))
	if err != nil {
		return nil, err
	}
	if len(remainder) > maxMemberBytes || len(bytes.Trim(remainder, "\x00")) != 0 || source.Len() != 0 {
		return nil, fmt.Errorf("unexpected trailing archive data")
	}
	return files, nil
}
func decodeManifest(data []byte) (manifest, error) {
	var m manifest
	if _, err := contracts.Decode(data); err != nil {
		return m, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, err
	}
	return m, nil
}
