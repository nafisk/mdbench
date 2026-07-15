package app

import (
	"fmt"
	"os"

	"github.com/nafiskhan/mdbench/internal/analyze"
	"github.com/nafiskhan/mdbench/internal/model"
	"github.com/nafiskhan/mdbench/internal/store"
)

type Service struct {
	analyzer analyze.Analyzer
	store    *store.Store
}

func NewService(config Config) (*Service, error) {
	for _, directory := range []string{config.ConfigDir, config.DataDir, config.CacheDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, fmt.Errorf("create application directory %q: %w", directory, err)
		}
	}
	artifactStore, err := store.New(config.DataDir)
	if err != nil {
		return nil, err
	}
	return &Service{
		analyzer: analyze.Analyzer{
			MaxArtifactBytes: config.MaxArtifactBytes,
			MaxBundleBytes:   config.MaxBundleBytes,
			MaxBundleFiles:   config.MaxBundleFiles,
		},
		store: artifactStore,
	}, nil
}

func (s *Service) InspectFile(path, label string) (model.Artifact, error) {
	artifact, err := s.analyzer.InspectFile(path, label)
	if err != nil {
		return model.Artifact{}, fmt.Errorf("inspect file: %w", err)
	}
	return artifact, nil
}

func (s *Service) InspectPaste(markdown []byte, label string) (model.Artifact, error) {
	artifact, err := s.analyzer.InspectPaste(markdown, label)
	if err != nil {
		return model.Artifact{}, fmt.Errorf("inspect paste: %w", err)
	}
	return artifact, nil
}

func (s *Service) SaveArtifact(artifact model.Artifact) (string, error) {
	path, err := s.store.SaveArtifact(artifact)
	if err != nil {
		return "", fmt.Errorf("save artifact: %w", err)
	}
	return path, nil
}
