package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Check archived bytes against independently recorded capture metadata, before
// future parser tests use them as evidence of consumer compatibility.
func TestCapturedFixtureIntegrity(t *testing.T) {
	artifacts, err := filepath.Glob("../../testdata/captured/*/artifact-*.json")
	if err != nil || len(artifacts) != 5 {
		t.Fatalf("expected five captured artifacts: %v, %v", artifacts, err)
	}
	for _, path := range artifacts {
		t.Run(filepath.Base(path), func(t *testing.T) {
			var artifact struct {
				Members []struct {
					Path           string `json:"path"`
					OriginalSHA256 string `json:"original_sha256"`
					StoredSHA256   string `json:"stored_sha256"`
					Size           int    `json:"size_bytes"`
					StoredSize     int    `json:"stored_size_bytes"`
					Trimmed        bool   `json:"trimmed"`
				} `json:"members"`
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &artifact); err != nil {
				t.Fatal(err)
			}
			if len(artifact.Members) == 0 {
				t.Fatal("empty artifact inventory")
			}
			for _, member := range artifact.Members {
				data, err := os.ReadFile(filepath.Join(filepath.Dir(path), member.Path))
				if err != nil {
					t.Fatal(err)
				}
				wantSHA, wantSize := member.OriginalSHA256, member.Size
				if member.Trimmed {
					wantSHA, wantSize = member.StoredSHA256, member.StoredSize
				}
				sum := sha256.Sum256(data)
				if hex.EncodeToString(sum[:]) != wantSHA || len(data) != wantSize {
					t.Errorf("%s: captured checksum/size changed", member.Path)
				}
			}
		})
	}
}

func TestCapturedManifests(t *testing.T) {
	paths, err := filepath.Glob("../../testdata/captured/*/*-manifest.json")
	if err != nil || len(paths) != 4 {
		t.Fatalf("expected four manifests: %v, %v", paths, err)
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := Validate("../../schemas/input-manifest.schema.json", data); err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Suites []struct {
					Files []string `json:"files"`
				} `json:"suites"`
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			for _, suite := range manifest.Suites {
				for _, input := range suite.Files {
					info, err := os.Stat(filepath.Join(filepath.Dir(path), input))
					if err != nil {
						t.Fatal(err)
					}
					if !info.Mode().IsRegular() || info.Size() == 0 {
						t.Errorf("%s: expected nonempty regular input", input)
					}
				}
			}
		})
	}
}
