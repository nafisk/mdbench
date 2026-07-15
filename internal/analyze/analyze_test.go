package analyze

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactHashesIncludeReviewedBundle(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	markdown := []byte("---\nname: sample-skill\nscripts:\n  - scripts/check.sh\n---\n# Sample\n\nFollow the rules.\n")
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), markdown, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "check.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	analyzer := Analyzer{MaxArtifactBytes: 1 << 20, MaxBundleBytes: 8 << 20, MaxBundleFiles: 128}

	first, err := analyzer.InspectFile(filepath.Join(root, "SKILL.md"), "v1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := analyzer.InspectFile(filepath.Join(root, "SKILL.md"), "v1")
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA != second.ContentSHA || first.BundleSHA != second.BundleSHA || first.EffectiveSHA != second.EffectiveSHA {
		t.Fatal("content-derived hashes changed between identical inspections")
	}
	if len(first.Files) != 2 {
		t.Fatalf("bundle has %d files, want 2", len(first.Files))
	}
	if first.HasBlockingFindings() {
		t.Fatalf("valid artifact has blocking findings: %#v", first.Findings)
	}
}

func TestSecretFindingBlocksWithoutEchoingValue(t *testing.T) {
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	analyzer := Analyzer{MaxArtifactBytes: 1 << 20, MaxBundleBytes: 8 << 20, MaxBundleFiles: 128}
	artifact, err := analyzer.InspectPaste([]byte("# Skill\n\nToken: "+secret+"\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !artifact.HasBlockingFindings() {
		t.Fatal("secret-like input did not block the artifact")
	}
	for _, finding := range artifact.Findings {
		if strings.Contains(finding.Message, secret) || strings.Contains(finding.Hint, secret) {
			t.Fatal("secret value leaked into a finding")
		}
	}
}

func TestPasteAcceptsFullSkillAndInstructionsOnly(t *testing.T) {
	analyzer := Analyzer{MaxArtifactBytes: 1 << 20, MaxBundleBytes: 8 << 20, MaxBundleFiles: 128}
	fullSkill := []byte("---\nname: ponytail\ndescription: Enforces lazy mode: prefer the smallest correct change.\n---\n# Ponytail\n\nFollow the rules.\n")

	full, err := analyzer.InspectPaste(fullSkill, "")
	if err != nil {
		t.Fatal(err)
	}
	if full.HasBlockingFindings() {
		t.Fatalf("full skill has blocking findings: %#v", full.Findings)
	}
	if full.Frontmatter["name"] != "ponytail" || full.TransformVersion != "" {
		t.Fatalf("full skill was not preserved: %#v", full)
	}

	instructions, err := analyzer.InspectPaste([]byte("# Instructions\n\nReturn concise, correct code.\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if instructions.HasBlockingFindings() {
		t.Fatalf("instructions have blocking findings: %#v", instructions.Findings)
	}
	if instructions.TransformVersion != transformVersion || !bytes.Contains(instructions.EffectiveMarkdown, []byte("name: mdbench-candidate")) {
		t.Fatal("instructions-only paste did not receive the compatibility wrapper")
	}
}

func TestInspectFileAcceptsSkillFolder(t *testing.T) {
	root := t.TempDir()
	markdown := []byte("# Instructions\n\nKeep changes small.\n")
	if err := os.WriteFile(filepath.Join(root, "skill.md"), markdown, 0o600); err != nil {
		t.Fatal(err)
	}
	analyzer := Analyzer{MaxArtifactBytes: 1 << 20, MaxBundleBytes: 8 << 20, MaxBundleFiles: 128}

	artifact, err := analyzer.InspectFile(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if artifact.EntryPath != "skill.md" || !bytes.Equal(artifact.Markdown, markdown) {
		t.Fatalf("folder did not resolve its skill file: %#v", artifact)
	}
}
