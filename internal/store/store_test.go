package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nafiskhan/mdbench/internal/analyze"
)

func TestSaveArtifactCommitsCompleteDirectory(t *testing.T) {
	dataDir := t.TempDir()
	analyzer := analyze.Analyzer{MaxArtifactBytes: 1 << 20, MaxBundleBytes: 8 << 20, MaxBundleFiles: 128}
	artifact, err := analyzer.InspectPaste([]byte("# Skill\n\nWrite the smallest correct change.\n"), "v1")
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := New(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	path, err := artifactStore.SaveArtifact(artifact)
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"manifest.json", "source/SKILL.md", "effective/SKILL.md"} {
		if info, err := os.Stat(filepath.Join(path, relative)); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("committed file %s is missing: %v", relative, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if len(entry.Name()) >= len(".artifact-") && entry.Name()[:len(".artifact-")] == ".artifact-" {
			t.Fatalf("temporary transaction remains after commit: %s", entry.Name())
		}
	}
}

func TestNewReconcilesInterruptedWrite(t *testing.T) {
	dataDir := t.TempDir()
	crashDir := filepath.Join(dataDir, "artifacts", ".artifact-crash")
	if err := os.MkdirAll(crashDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dataDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(crashDir); !os.IsNotExist(err) {
		t.Fatalf("interrupted write was not removed: %v", err)
	}
}
