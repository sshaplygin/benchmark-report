// Command package builds one native platform archive for offline report generation.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sshaplygin/benchmark-report/internal/contracts"
)

type member struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type archiveManifest struct {
	SchemaVersion int      `json:"schema_version"`
	Version       string   `json:"version"`
	OS            string   `json:"os"`
	Arch          string   `json:"arch"`
	Benchstat     string   `json:"benchstat"`
	Candidate     bool     `json:"candidate"`
	Files         []member `json:"files"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	out := flag.String("output-dir", "dist", "archive destination")
	release := flag.Bool("release", false, "require a project license and mark archives as release artifacts")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if (runtime.GOOS != "linux" && runtime.GOOS != "darwin") || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	versionBytes, err := os.ReadFile("report/VERSION")
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(versionBytes))
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.-]+)?$`).MatchString(version) {
		return fmt.Errorf("invalid report/VERSION")
	}
	license, licenseErr := os.ReadFile("LICENSE")
	if *release && (licenseErr != nil || len(bytes.TrimSpace(license)) == 0) {
		return fmt.Errorf("release packaging requires a nonempty project LICENSE; no license has been selected")
	}
	if licenseErr != nil && !os.IsNotExist(licenseErr) {
		return licenseErr
	}
	work, err := os.MkdirTemp("", "benchreport-package-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }() // Temporary build files are best-effort cleanup.
	stage := filepath.Join(work, "stage")
	if err = os.Mkdir(stage, 0755); err != nil {
		return err
	}
	env := []string{"CGO_ENABLED=0", "GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH}
	if _, err = command(root, env, "go", "build", "-trimpath", "-ldflags=-X main.version="+version, "-o", filepath.Join(stage, "benchreport"), "./cmd/benchreport"); err != nil {
		return err
	}
	if _, err = command(root, append(env, "GOBIN="+stage), "go", "install", "-trimpath", "golang.org/x/perf/cmd/benchstat@"+contracts.BenchstatVersion); err != nil {
		return err
	}
	files := map[string][]byte{"VERSION": []byte(version + "\n")}
	if len(bytes.TrimSpace(license)) > 0 {
		files["LICENSE"] = license
	} else {
		files["PROJECT-LICENSE-STATUS.txt"] = []byte("Unreleased test archive. No project license has been selected.\nThird-party license texts are included in LICENSES/.\nRelease packaging requires a project LICENSE.\n")
	}
	modules := map[string]string{}
	for _, name := range []string{"benchreport", "benchstat"} {
		binary := filepath.Join(stage, name)
		info, e := buildinfo.ReadFile(binary)
		if e != nil {
			return e
		}
		if name == "benchstat" && (info.Main.Path != "golang.org/x/perf" || info.Main.Version != contracts.BenchstatVersion || info.Main.Replace != nil) {
			return fmt.Errorf("benchstat build metadata does not match pin")
		}
		deps := info.Deps
		if name == "benchstat" {
			deps = append(deps, &info.Main)
		}
		for _, dep := range deps {
			if dep.Replace != nil {
				return fmt.Errorf("replacement module %s is not allowed in archives", dep.Path)
			}
			if old, ok := modules[dep.Path]; ok && old != dep.Version {
				return fmt.Errorf("conflicting module versions for %s", dep.Path)
			}
			modules[dep.Path] = dep.Version
		}
		data, e := os.ReadFile(binary)
		if e != nil {
			return e
		}
		files[name] = data
	}
	moduleNames := make([]string, 0, len(modules))
	for name := range modules {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)
	var notices strings.Builder
	notices.WriteString("Dependency notices for the bundled executables\n\n")
	for _, name := range moduleNames {
		data, e := command(root, nil, "go", "mod", "download", "-json", name+"@"+modules[name])
		if e != nil {
			return e
		}
		var mod struct {
			Dir   string
			Error string
		}
		if e = json.Unmarshal(data, &mod); e != nil {
			return e
		}
		if mod.Error != "" || mod.Dir == "" {
			return fmt.Errorf("resolve license for %s: %s", name, mod.Error)
		}
		prefix := strings.NewReplacer("/", "_", "@", "_").Replace(name + "@" + modules[name])
		found := false
		for _, base := range []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING", "NOTICE", "PATENTS", "ATTRIB"} {
			body, e := os.ReadFile(filepath.Join(mod.Dir, base))
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			files["LICENSES/"+prefix+"_"+base] = body
			if strings.HasPrefix(base, "LICENSE") || base == "COPYING" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("no license text for %s@%s", name, modules[name])
		}
		fmt.Fprintf(&notices, "%s %s\n", name, modules[name])
	}
	goroot, e := command(root, nil, "go", "env", "GOROOT")
	if e != nil {
		return e
	}
	for _, name := range []string{"LICENSE", "PATENTS"} {
		goRoot := strings.TrimSpace(string(goroot))
		body, e := os.ReadFile(filepath.Join(goRoot, name))
		// Homebrew installs LICENSE beside libexec rather than inside GOROOT.
		if os.IsNotExist(e) && name == "LICENSE" && filepath.Base(goRoot) == "libexec" {
			body, e = os.ReadFile(filepath.Join(filepath.Dir(goRoot), name))
		}
		if os.IsNotExist(e) && name == "PATENTS" {
			continue
		}
		if e != nil {
			return e
		}
		files["LICENSES/Go_"+name] = body
	}
	files["LICENSES/NOTICE.txt"] = []byte(notices.String())
	manifest := archiveManifest{SchemaVersion: 1, Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH, Benchstat: contracts.BenchstatVersion, Candidate: !*release, Files: []member{}}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		sum := sha256.Sum256(files[name])
		manifest.Files = append(manifest.Files, member{name, hex.EncodeToString(sum[:])})
	}
	data, e := json.MarshalIndent(manifest, "", "  ")
	if e != nil {
		return e
	}
	files["manifest.json"] = append(data, '\n')
	names = append(names, "manifest.json")
	sort.Strings(names)
	destination, e := filepath.Abs(*out)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(destination, 0755); e != nil {
		return e
	}
	name := fmt.Sprintf("benchreport_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	temporary, e := os.CreateTemp(destination, ".archive-")
	if e != nil {
		return e
	}
	temporaryName := temporary.Name()
	defer func() { _ = temporary.Close() }() // The successful write path checks Close below.
	defer func() { _ = os.Remove(temporaryName) }()
	gz := gzip.NewWriter(temporary)
	tw := tar.NewWriter(gz)
	for _, name := range names {
		mode := int64(0644)
		if name == "benchreport" || name == "benchstat" {
			mode = 0755
		}
		if e = tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(files[name])), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); e != nil {
			return e
		}
		if _, e = tw.Write(files[name]); e != nil {
			return e
		}
	}
	if e = tw.Close(); e != nil {
		return e
	}
	if e = gz.Close(); e != nil {
		return e
	}
	if e = temporary.Close(); e != nil {
		return e
	}
	archive := filepath.Join(destination, name)
	if e = os.Rename(temporaryName, archive); e != nil {
		return e
	}
	handle, e := os.Open(archive)
	if e != nil {
		return e
	}
	hash := sha256.New()
	_, e = io.Copy(hash, handle)
	_ = handle.Close() // Read-only handle; preserve any copy error.
	if e != nil {
		return e
	}
	checksum := fmt.Sprintf("%x  %s\n", hash.Sum(nil), name)
	if e = os.WriteFile(filepath.Join(destination, name+".sha256"), []byte(checksum), 0644); e != nil {
		return e
	}
	fmt.Println(archive)
	return nil
}
func command(dir string, overrides []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for _, entry := range overrides {
		key := strings.SplitN(entry, "=", 2)[0] + "="
		filtered := cmd.Env[:0]
		for _, old := range cmd.Env {
			if !strings.HasPrefix(old, key) {
				filtered = append(filtered, old)
			}
		}
		cmd.Env = append(filtered, entry)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	result, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w\n%s", name, args, err, stderr.String())
	}
	return result, nil
}
