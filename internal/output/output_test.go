package output

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func save(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestCommitAndRollback(t *testing.T) {
	for fail := 1; fail <= 4; fail++ {
		t.Run(string(rune('0'+fail)), func(t *testing.T) {
			root := t.TempDir()
			save(t, filepath.Join(root, "a"), "old-a")
			save(t, filepath.Join(root, "b"), "old-b")
			calls := 0
			rename := func(a, b string) error {
				calls++
				if calls == fail {
					return errors.New("injected rename failure")
				}
				return os.Rename(a, b)
			}
			if err := commit(root, []Entry{{"a", []byte("new-a")}, {"b", []byte("new-b")}}, nil, rename); err == nil {
				t.Fatal("missing failure")
			}
			for _, name := range []string{"a", "b"} {
				got, err := os.ReadFile(filepath.Join(root, name))
				if err != nil || string(got) != "old-"+name {
					t.Fatal("rollback lost original", name, string(got), err)
				}
			}
			stages, _ := filepath.Glob(filepath.Join(root, ".benchreport-stage-*"))
			if len(stages) != 0 {
				t.Fatal("temporary stage left", stages)
			}
		})
	}
	root := t.TempDir()
	if err := Commit(root, []Entry{{"nested/a", []byte("first")}, {"b", []byte("second")}}, nil); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "nested/a")); err != nil || string(data) != "first" {
		t.Fatal(err)
	}
}
func TestRollbackRemovesNewOutputs(t *testing.T) {
	root := t.TempDir()
	calls := 0
	rename := func(a, b string) error {
		calls++
		if calls == 2 {
			return errors.New("fail second install")
		}
		return os.Rename(a, b)
	}
	if err := commit(root, []Entry{{"a/new", []byte("a")}, {"z/new", []byte("z")}}, nil, rename); err == nil {
		t.Fatal("success")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("partial outputs left", entries)
	}
}
func TestPortableCollisionsAndTraversal(t *testing.T) {
	for _, names := range [][]string{{"x", "x"}, {"x", "x/y"}, {"Report.txt", "report.txt"}, {"café.txt", "cafe\u0301.txt"}, {"z", "Z/report.json"}, {"../outside"}, {"/absolute"}, {"a/./b"}, {"a\\b"}, {"C:/windows"}} {
		root := t.TempDir()
		entries := []Entry{}
		for _, name := range names {
			entries = append(entries, Entry{name, []byte("bad")})
		}
		if err := Commit(root, entries, nil); err == nil {
			t.Fatal("accepted", names)
		}
		files, _ := os.ReadDir(root)
		if len(files) != 0 {
			t.Fatal("files left", names)
		}
	}
}
func TestSymlinkAndInputProtection(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	save(t, filepath.Join(outside, "sentinel"), "safe")
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := Commit(root, []Entry{{"link/sentinel", []byte("bad")}}, nil); err == nil {
		t.Fatal("symlink escape")
	}
	source := filepath.Join(root, "source")
	save(t, source, "input")
	if err := os.Link(source, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"source", "alias"} {
		if err := Commit(root, []Entry{{name, []byte("bad")}}, []string{source}); err == nil {
			t.Fatal("overwrote input", name)
		}
	}
	data, _ := os.ReadFile(source)
	if string(data) != "input" {
		t.Fatal("changed source")
	}
	data, _ = os.ReadFile(filepath.Join(outside, "sentinel"))
	if string(data) != "safe" {
		t.Fatal("escaped output root")
	}
}
func TestRootPreventsChangedSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	save(t, filepath.Join(root, "nested/old"), "safe")
	cap, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer cap.Close()
	calls := 0
	rename := func(a, b string) error {
		calls++
		if calls == 1 {
			if err := os.Rename(filepath.Join(root, "nested"), filepath.Join(root, "moved")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, "nested")); err != nil {
				t.Fatal(err)
			}
		}
		from, _ := filepath.Rel(root, a)
		to, _ := filepath.Rel(root, b)
		return cap.Rename(from, to)
	}
	if err := commit(root, []Entry{{"nested/new", []byte("never outside")}}, nil, rename); err == nil {
		t.Fatal("late symlink escaped")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("wrote outside", entries)
	}
}

func TestTransactionalDeletionAndRollback(t *testing.T) {
	for fail := 1; fail <= 3; fail++ {
		root := t.TempDir()
		save(t, filepath.Join(root, "a-stale"), "old-a")
		save(t, filepath.Join(root, "z-new"), "old-z")
		calls := 0
		rename := func(a, b string) error {
			calls++
			if calls == fail {
				return errors.New("injected failure")
			}
			return os.Rename(a, b)
		}
		if err := transaction(root, []Entry{{Path: "z-new", Data: []byte("new-z")}}, []string{"a-stale"}, nil, rename); err == nil {
			t.Fatal("accepted injected failure")
		}
		for name, want := range map[string]string{"a-stale": "old-a", "z-new": "old-z"} {
			got, err := os.ReadFile(filepath.Join(root, name))
			if err != nil || string(got) != want {
				t.Fatal("lost original after failed deletion", name, err)
			}
		}
	}
	root := t.TempDir()
	save(t, filepath.Join(root, "stale"), "obsolete")
	save(t, filepath.Join(root, "unmanaged"), "keep")
	if err := Transaction(root, []Entry{{Path: "new", Data: []byte("new")}}, []string{"stale", "missing/unused"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "stale")); !os.IsNotExist(err) {
		t.Fatal("stale file left")
	}
	if _, err := os.Stat(filepath.Join(root, "missing")); !os.IsNotExist(err) {
		t.Fatal("created nonexistent stale directory")
	}
	got, _ := os.ReadFile(filepath.Join(root, "unmanaged"))
	if string(got) != "keep" {
		t.Fatal("deleted arbitrary file")
	}
}
func TestDeletionInputAndSymlinkProtection(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	save(t, source, "input")
	if err := Transaction(root, []Entry{{Path: "new", Data: []byte("new")}}, []string{"source"}, []string{source}); err == nil {
		t.Fatal("deleted input")
	}
	outside := filepath.Join(t.TempDir(), "outside")
	save(t, outside, "safe")
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := Transaction(root, []Entry{{Path: "new", Data: []byte("new")}}, []string{"link"}, nil); err == nil {
		t.Fatal("deleted symlink")
	}
}
