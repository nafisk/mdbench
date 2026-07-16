package fixture

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed all:assets
var assets embed.FS

type File struct {
	Path       string `json:"path"`
	Mode       uint32 `json:"mode"`
	Size       int64  `json:"size"`
	ContentSHA string `json:"content_sha"`
	Content    []byte `json:"-"`
}

type Snapshot struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ContentSHA  string `json:"content_sha"`
	Files       []File `json:"files"`
}

var definitions = []struct {
	id          string
	name        string
	description string
}{
	{"empty", "Empty workspace", "No starter files."},
	{"basic-go", "Basic Go module", "A minimal Go module with one package."},
	{"basic-node", "Basic Node package", "A minimal dependency-free ES module package."},
	{"basic-python", "Basic Python package", "A minimal dependency-free Python package."},
}

func Catalog() ([]Snapshot, error) {
	result := make([]Snapshot, 0, len(definitions))
	for _, definition := range definitions {
		snapshot, err := load(definition.id, definition.name, definition.description)
		if err != nil {
			return nil, err
		}
		result = append(result, snapshot)
	}
	return result, nil
}

func Find(id string) (Snapshot, error) {
	fixtures, err := Catalog()
	if err != nil {
		return Snapshot{}, err
	}
	for _, candidate := range fixtures {
		if candidate.ID == id {
			return candidate, nil
		}
	}
	return Snapshot{}, fmt.Errorf("fixture %q does not exist", id)
}

func load(id, name, description string) (Snapshot, error) {
	root := path.Join("assets", id)
	files := []File{}
	if id != "empty" {
		err := fs.WalkDir(assets, root, func(filePath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			content, err := assets.ReadFile(filePath)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(content)
			relative := strings.TrimPrefix(filePath, root+"/")
			relative = strings.TrimSuffix(relative, ".fixture")
			files = append(files, File{
				Path: relative, Mode: 0o644,
				Size: int64(len(content)), ContentSHA: hex.EncodeToString(digest[:]), Content: content,
			})
			return nil
		})
		if err != nil {
			return Snapshot{}, fmt.Errorf("load fixture %q: %w", id, err)
		}
	}
	sort.Slice(files, func(left, right int) bool { return files[left].Path < files[right].Path })
	hash := sha256.New()
	for _, file := range files {
		fmt.Fprintf(hash, "%s\x00%d\x00%d\x00%s\n", file.Path, file.Mode, file.Size, file.ContentSHA)
	}
	return Snapshot{
		ID: id, Name: name, Description: description,
		ContentSHA: hex.EncodeToString(hash.Sum(nil)), Files: files,
	}, nil
}
