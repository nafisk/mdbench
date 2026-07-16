package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nafiskhan/mdbench/internal/analyze"
	"github.com/nafiskhan/mdbench/internal/harness"
	"github.com/nafiskhan/mdbench/internal/model"
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

func TestSuiteRevisionsAreImmutableAndListed(t *testing.T) {
	value, err := harness.NewFakeGenerator().GenerateSuite(context.Background(), harness.GenerateRequest{
		Artifact: model.Artifact{EffectiveSHA: strings.Repeat("a", 64)}, CaseCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, firstPath, err := store.SaveSuiteDraft(value)
	if err != nil {
		t.Fatal(err)
	}
	value.Cases[0].Prompt = "A revised prompt."
	second, _, err := store.SaveSuiteDraft(value)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || second.Revision != 2 || first.ContentSHA == second.ContentSHA {
		t.Fatalf("revisions are %#v and %#v", first, second)
	}
	saved, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "A revised prompt.") {
		t.Fatal("first revision was mutated")
	}
	listed, err := store.ListSuites()
	if err != nil || len(listed) != 2 {
		t.Fatalf("listed %d revisions: %v", len(listed), err)
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
