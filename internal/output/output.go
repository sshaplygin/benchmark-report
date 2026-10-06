// Package output writes a set of contained files with rollback on any failure.
package output

import (
	"errors"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Entry struct {
	Path string
	Data []byte
}

// Commit validates every destination before staging; existing files survive failed commits.
func Commit(directory string, entries []Entry, protected []string) error {
	return commit(directory, entries, protected, nil)
}
func safeName(name string) error {
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") || strings.Contains(name, ":") {
		return fmt.Errorf("invalid relative output path %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid relative output path %q", name)
		}
	}
	return nil
}
func commit(directory string, entries []Entry, protected []string, rename func(string, string) error) (err error) {
	names := map[string]bool{}
	for _, entry := range entries {
		if err := safeName(entry.Path); err != nil {
			return err
		}
		if names[portable(entry.Path)] {
			return fmt.Errorf("output filename collision %q", entry.Path)
		}
		names[portable(entry.Path)] = true
	}
	ordered := append([]Entry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	for i := 1; i < len(ordered); i++ {
		for j := 0; j < i; j++ {
			if strings.HasPrefix(portable(ordered[i].Path), portable(ordered[j].Path)+"/") || strings.HasPrefix(portable(ordered[j].Path), portable(ordered[i].Path)+"/") {
				return fmt.Errorf("output file/directory collision %q", ordered[j].Path)
			}
		}
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	rootCreated := false
	if _, err := os.Stat(absolute); os.IsNotExist(err) {
		if err = os.MkdirAll(absolute, 0755); err != nil {
			return err
		}
		rootCreated = true
	}
	root, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil && rootCreated {
			os.Remove(root)
		}
	}()
	cap, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer cap.Close()
	relative := func(path string) string { rel, _ := filepath.Rel(root, path); return rel }
	if rename == nil {
		rename = func(a, b string) error { return cap.Rename(relative(a), relative(b)) }
	}
	protectedInfo := []os.FileInfo{}
	protectedPaths := map[string]bool{}
	for _, path := range protected {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		physical, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			if parent, e := filepath.EvalSymlinks(filepath.Dir(absolute)); e == nil {
				physical = filepath.Join(parent, filepath.Base(absolute))
			} else {
				physical = absolute
			}
		}
		protectedPaths[portable(physical)] = true
		if info, err := os.Stat(physical); err == nil {
			protectedInfo = append(protectedInfo, info)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if info, e := os.Stat(executable); e == nil {
		protectedInfo = append(protectedInfo, info)
	}
	for _, entry := range ordered {
		dest := filepath.Join(root, filepath.FromSlash(entry.Path))
		if protectedPaths[portable(dest)] {
			return fmt.Errorf("output %s would overwrite an input", entry.Path)
		}
		current := root
		parts := strings.Split(entry.Path, "/")
		for i, part := range parts {
			current = filepath.Join(current, part)
			info, e := cap.Lstat(relative(current))
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("output %s has symlink component", entry.Path)
			}
			if i < len(parts)-1 && !info.IsDir() {
				return fmt.Errorf("output %s parent is not a directory", entry.Path)
			}
			if i == len(parts)-1 {
				if !info.Mode().IsRegular() {
					return fmt.Errorf("output %s is not a regular file", entry.Path)
				}
				for _, source := range protectedInfo {
					if os.SameFile(info, source) {
						return fmt.Errorf("output %s would overwrite an input or executable", entry.Path)
					}
				}
			}
		}
	}
	stage, err := os.MkdirTemp(root, ".benchreport-stage-")
	if err != nil {
		return err
	}
	keepRecovery := false
	defer func() {
		if !keepRecovery {
			cap.RemoveAll(relative(stage))
		}
	}()
	// Prepare all bytes before moving any existing destination.
	for i, entry := range ordered {
		f, e := cap.OpenFile(relative(filepath.Join(stage, fmt.Sprintf("new-%d", i))), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if e != nil {
			return e
		}
		_, e = f.Write(entry.Data)
		if e == nil {
			e = f.Sync()
		}
		closeErr := f.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			return e
		}
	}
	created := []string{}
	backup := map[int]string{}
	installed := []int{}
	rollback := func(cause error) error {
		failures := []error{cause}
		for i := len(installed) - 1; i >= 0; i-- {
			if e := cap.Remove(ordered[installed[i]].Path); e != nil && !os.IsNotExist(e) {
				failures = append(failures, e)
			}
		}
		for i := len(ordered) - 1; i >= 0; i-- {
			if old, ok := backup[i]; ok {
				if e := rename(old, filepath.Join(root, filepath.FromSlash(ordered[i].Path))); e != nil {
					failures = append(failures, fmt.Errorf("rollback %s: %w", ordered[i].Path, e))
				}
			}
		}
		for i := len(created) - 1; i >= 0; i-- {
			cap.Remove(relative(created[i]))
		}
		if len(failures) > 1 {
			keepRecovery = true
			failures = append(failures, fmt.Errorf("recovery files retained at %s", stage))
		}
		return errors.Join(failures...)
	}
	for i, entry := range ordered {
		dest := filepath.Join(root, filepath.FromSlash(entry.Path))
		parent := filepath.Dir(dest)
		missing := []string{}
		for p := parent; p != root; p = filepath.Dir(p) {
			if _, e := cap.Stat(relative(p)); os.IsNotExist(e) {
				missing = append(missing, p)
			} else if e != nil {
				return rollback(e)
			} else {
				break
			}
		}
		for j := len(missing) - 1; j >= 0; j-- {
			if e := cap.Mkdir(relative(missing[j]), 0755); e != nil {
				return rollback(e)
			}
			created = append(created, missing[j])
		}
		if _, e := cap.Lstat(relative(dest)); e == nil {
			old := filepath.Join(stage, fmt.Sprintf("old-%d", i))
			if e = rename(dest, old); e != nil {
				return rollback(e)
			}
			backup[i] = old
		} else if !os.IsNotExist(e) {
			return rollback(e)
		}
		if e := rename(filepath.Join(stage, fmt.Sprintf("new-%d", i)), dest); e != nil {
			return rollback(e)
		}
		installed = append(installed, i)
	}
	return nil
}

func portable(path string) string { return norm.NFC.String(cases.Fold().String(path)) }
