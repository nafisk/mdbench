package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nafiskhan/mdbench/internal/model"
	"github.com/nafiskhan/mdbench/internal/suite"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Store struct {
	root      string
	suiteRoot string
	mu        sync.RWMutex
}

func New(dataDir string) (*Store, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("data directory is empty")
	}
	root := filepath.Join(dataDir, "artifacts")
	suiteRoot := filepath.Join(dataDir, "suites")
	for _, directory := range []string{root, suiteRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, fmt.Errorf("create data store: %w", err)
		}
	}
	result := &Store{root: root, suiteRoot: suiteRoot}
	if err := result.Reconcile(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) SaveSuiteDraft(draft suite.Draft) (suite.Frozen, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	revision, err := s.nextSuiteRevision(draft.ID)
	if err != nil {
		return suite.Frozen{}, "", err
	}
	frozen, err := suite.Freeze(draft, revision, time.Now().UTC())
	if err != nil {
		return suite.Frozen{}, "", err
	}
	directory := filepath.Join(s.suiteRoot, frozen.ID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return suite.Frozen{}, "", fmt.Errorf("create suite directory: %w", err)
	}
	path := filepath.Join(directory, fmt.Sprintf("%06d.json", frozen.Revision))
	encoded, err := json.MarshalIndent(frozen, "", "  ")
	if err != nil {
		return suite.Frozen{}, "", fmt.Errorf("encode suite revision: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := writeAtomic(path, encoded); err != nil {
		return suite.Frozen{}, "", err
	}
	return frozen, path, nil
}

func (s *Store) ListSuites() ([]suite.Frozen, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.suiteRoot)
	if err != nil {
		return nil, fmt.Errorf("read suite store: %w", err)
	}
	result := []suite.Frozen{}
	for _, entry := range entries {
		if !entry.IsDir() || !safeIDPattern.MatchString(entry.Name()) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(s.suiteRoot, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read suite %q: %w", entry.Name(), err)
		}
		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}
			frozen, err := readSuite(filepath.Join(s.suiteRoot, entry.Name(), file.Name()))
			if err != nil {
				return nil, err
			}
			result = append(result, frozen)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].ID == result[right].ID {
			return result[left].Revision > result[right].Revision
		}
		return result[left].ID < result[right].ID
	})
	return result, nil
}

func (s *Store) nextSuiteRevision(id string) (int, error) {
	if !safeIDPattern.MatchString(id) || id == "." || id == ".." {
		return 0, errors.New("suite ID is invalid")
	}
	directory := filepath.Join(s.suiteRoot, id)
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read suite revisions: %w", err)
	}
	latest := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		revision, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
		if err == nil {
			latest = max(latest, revision)
		}
	}
	return latest + 1, nil
}

func readSuite(path string) (suite.Frozen, error) {
	file, err := os.Open(path)
	if err != nil {
		return suite.Frozen{}, fmt.Errorf("open suite revision: %w", err)
	}
	defer file.Close()
	var frozen suite.Frozen
	decoder := json.NewDecoder(io.LimitReader(file, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&frozen); err != nil {
		return suite.Frozen{}, fmt.Errorf("decode suite revision %q: %w", path, err)
	}
	expected, err := suite.RevisionHash(frozen.Draft, frozen.Revision)
	if err != nil {
		return suite.Frozen{}, fmt.Errorf("validate suite revision %q: %w", path, err)
	}
	if expected != frozen.ContentSHA {
		return suite.Frozen{}, fmt.Errorf("suite revision %q hash does not match its content", path)
	}
	return frozen, nil
}

func writeAtomic(path string, content []byte) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("suite revision already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check suite destination: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".suite-")
	if err != nil {
		return fmt.Errorf("create suite transaction: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect suite transaction: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return fmt.Errorf("write suite transaction: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("flush suite transaction: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close suite transaction: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("commit suite revision: %w", err)
	}
	return syncDirectory(filepath.Dir(path))
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
