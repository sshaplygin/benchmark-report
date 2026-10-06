package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/sshaplygin/benchmark-report/internal/model"
	"os"
	"path/filepath"
	"testing"
)

func TestFullReplayAndTampering(t *testing.T) {
	for _, fixture := range []struct{ parser, name string }{{"go", "go-pr15"}, {"criterion", "criterion-four-suites"}} {
		t.Run(fixture.parser, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", "captured", fixture.name)
			out := filepath.Join(t.TempDir(), "original")
			_, err := Run(Options{Parser: fixture.parser, BaseManifest: filepath.Join(root, "base-manifest.json"), HeadManifest: filepath.Join(root, "head-manifest.json"), Version: "test", CommentHeader: "benchmark-report"}, out)
			if err != nil {
				t.Fatal(err)
			}
			moved := filepath.Join(t.TempDir(), "moved")
			if err = os.Rename(out, moved); err != nil {
				t.Fatal(err)
			}
			replay := filepath.Join(t.TempDir(), "replay")
			if _, err = Replay(filepath.Join(moved, "reproduction.json"), replay, "", "test"); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"comparison.json", "report.md", "report.json", "comment.md", "reproduction.json"} {
				a, _ := os.ReadFile(filepath.Join(moved, name))
				b, _ := os.ReadFile(filepath.Join(replay, name))
				if string(a) != string(b) {
					t.Fatalf("changed %s", name)
				}
			}
			if _, err = Replay(filepath.Join(moved, "reproduction.json"), replay, "", "test"); err == nil {
				t.Fatal("accepted nonempty destination")
			}
			// A checksum-repaired edit of raw evidence cannot pass fresh calculations.
			bytes, _ := os.ReadFile(filepath.Join(moved, "reproduction.json"))
			var m model.Reproduction
			if err = json.Unmarshal(bytes, &m); err != nil {
				t.Fatal(err)
			}
			for i, file := range m.Files {
				if file.Role == "raw" {
					raw, _ := os.ReadFile(filepath.Join(moved, file.Path))
					raw = append(raw, []byte("\nBenchmarkBroken 0 1 ns/op\n")...)
					if err = os.WriteFile(filepath.Join(moved, file.Path), raw, 0600); err != nil {
						t.Fatal(err)
					}
					m.Files[i].SHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
					break
				}
			}
			bytes, _ = json.Marshal(m)
			if err = os.WriteFile(filepath.Join(moved, "reproduction.json"), bytes, 0600); err != nil {
				t.Fatal(err)
			}
			bad := filepath.Join(t.TempDir(), "bad")
			if _, err = Replay(filepath.Join(moved, "reproduction.json"), bad, "", "test"); err == nil {
				t.Fatal("accepted modified raw evidence")
			}
			if _, err = os.Stat(bad); !os.IsNotExist(err) {
				t.Fatal("failure created destination")
			}
		})
	}
}
