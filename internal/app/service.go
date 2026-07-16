package app

import (
	"context"
	"fmt"
	"os"

	"github.com/nafiskhan/mdbench/internal/analyze"
	"github.com/nafiskhan/mdbench/internal/harness"
	"github.com/nafiskhan/mdbench/internal/model"
	"github.com/nafiskhan/mdbench/internal/plan"
	"github.com/nafiskhan/mdbench/internal/store"
	"github.com/nafiskhan/mdbench/internal/suite"
)

type Service struct {
	analyzer  analyze.Analyzer
	store     *store.Store
	generator harness.Generator
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
		store: artifactStore, generator: harness.NewFakeGenerator(),
	}, nil
}

func (s *Service) GenerateSuite(ctx context.Context, artifact model.Artifact, caseCount int, fixtureIDs []string) (suite.Draft, error) {
	draft, err := s.generator.GenerateSuite(ctx, harness.GenerateRequest{Artifact: artifact, CaseCount: caseCount, FixtureIDs: fixtureIDs})
	if err != nil {
		return suite.Draft{}, fmt.Errorf("generate suite: %w", err)
	}
	return draft, nil
}

func (s *Service) FreezeSuite(draft suite.Draft) (suite.Frozen, string, error) {
	frozen, path, err := s.store.SaveSuiteDraft(draft)
	if err != nil {
		return suite.Frozen{}, "", fmt.Errorf("freeze suite: %w", err)
	}
	return frozen, path, nil
}

func (s *Service) ListSuites() ([]suite.Frozen, error) {
	values, err := s.store.ListSuites()
	if err != nil {
		return nil, fmt.Errorf("list suites: %w", err)
	}
	return values, nil
}

func (s *Service) BuildExecutionPlan(artifact model.Artifact, frozen suite.Frozen, config plan.Config) (plan.ExecutionPlan, error) {
	value, err := plan.Build(artifact, frozen, config)
	if err != nil {
		return plan.ExecutionPlan{}, fmt.Errorf("build execution plan: %w", err)
	}
	return value, nil
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
