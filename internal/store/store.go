package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nafiskhan/mdbench/internal/model"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Store struct {
	root string
}

func New(dataDir string) (*Store, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("data directory is empty")
	}
	root := filepath.Join(dataDir, "artifacts")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create artifact store: %w", err)
	}
	result := &Store{root: root}
	if err := result.Reconcile(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) Reconcile() error {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return fmt.Errorf("read artifact store: %w", err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".artifact-") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.root, entry.Name())); err != nil {
			return fmt.Errorf("remove interrupted artifact write: %w", err)
		}
	}
	return nil
}

func (s *Store) SaveArtifact(artifact model.Artifact) (string, error) {
	if artifact.SchemaVersion != model.SchemaVersion {
		return "", fmt.Errorf("unsupported artifact schema %d", artifact.SchemaVersion)
	}
	if !safeIDPattern.MatchString(artifact.ID) || artifact.ID == "." || artifact.ID == ".." {
		return "", errors.New("artifact ID is invalid")
	}
	if artifact.HasBlockingFindings() {
		return "", errors.New("artifact has blocking findings")
	}
	if len(artifact.Markdown) == 0 || len(artifact.EffectiveMarkdown) == 0 {
		return "", errors.New("artifact content is empty")
	}

	temporary, err := os.MkdirTemp(s.root, ".artifact-")
	if err != nil {
		return "", fmt.Errorf("create artifact transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(temporary)
		}
	}()
	if err := os.Chmod(temporary, 0o700); err != nil {
		return "", fmt.Errorf("protect artifact transaction: %w", err)
	}

	for _, file := range artifact.Files {
		relative, err := safeRelative(file.Path)
		if err != nil {
			return "", err
		}
		if err := writeFile(filepath.Join(temporary, "source", relative), file.Content, 0o600); err != nil {
			return "", err
		}
	}
	if err := writeFile(filepath.Join(temporary, "effective", "SKILL.md"), artifact.EffectiveMarkdown, 0o600); err != nil {
		return "", err
	}
	manifest, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode artifact manifest: %w", err)
	}
	manifest = append(manifest, '\n')
	if err := writeFile(filepath.Join(temporary, "manifest.json"), manifest, 0o600); err != nil {
		return "", err
	}
	if err := syncDirectory(temporary); err != nil {
		return "", err
	}

	destination := filepath.Join(s.root, artifact.ID)
	if _, err := os.Lstat(destination); err == nil {
		return "", errors.New("artifact ID already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check artifact destination: %w", err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		return "", fmt.Errorf("commit artifact: %w", err)
	}
	committed = true
	if err := syncDirectory(s.root); err != nil {
		return "", err
	}
	return destination, nil
}

func writeFile(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create artifact file: %w", err)
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return fmt.Errorf("write artifact file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("flush artifact file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close artifact file: %w", err)
	}
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}

func safeRelative(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return "", errors.New("artifact file path is invalid")
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("artifact file path escapes its root")
	}
	return clean, nil
}

func Redact(text string) (string, bool) {
	redacted := text
	changed := false
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z0-9 ]*PRIVATE KEY-----`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`),
		regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`),
		regexp.MustCompile(`\bAKIA[A-Z0-9]{16}\b`),
	}
	for _, pattern := range patterns {
		next := pattern.ReplaceAllString(redacted, "[REDACTED]")
		if next != redacted {
			changed = true
			redacted = next
		}
	}
	return redacted, changed
}
