package plan

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nafiskhan/mdbench/internal/model"
	"github.com/nafiskhan/mdbench/internal/suite"
)

type Config struct {
	Harness        string `json:"harness"`
	ExecutorModel  string `json:"executor_model"`
	JudgeModel     string `json:"judge_model"`
	TrialsPerCase  int    `json:"trials_per_case"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Network        bool   `json:"network"`
	Concurrency    int    `json:"concurrency"`
}

type ExecutionPlan struct {
	ArtifactID          string   `json:"artifact_id"`
	ArtifactLabel       string   `json:"artifact_label,omitempty"`
	ArtifactSHA         string   `json:"artifact_sha"`
	SuiteID             string   `json:"suite_id"`
	SuiteRevision       int      `json:"suite_revision"`
	SuiteSHA            string   `json:"suite_sha"`
	FixtureIDs          []string `json:"fixture_ids"`
	EnabledCases        int      `json:"enabled_cases"`
	PlannedTrials       int      `json:"planned_trials"`
	EstimatedModelCalls int      `json:"estimated_model_calls"`
	Config              Config   `json:"config"`
}

func DefaultConfig() Config {
	return Config{Harness: "codex", ExecutorModel: "default", JudgeModel: "default", TrialsPerCase: 1, TimeoutSeconds: 300, Network: false, Concurrency: 1}
}

func Build(artifact model.Artifact, frozen suite.Frozen, config Config) (ExecutionPlan, error) {
	artifactSHA := artifact.EffectiveSHA
	if artifactSHA == "" {
		artifactSHA = artifact.ContentSHA
	}
	if strings.TrimSpace(artifactSHA) == "" || strings.TrimSpace(frozen.ContentSHA) == "" {
		return ExecutionPlan{}, errors.New("artifact and frozen suite hashes are required")
	}
	if config.Harness != "codex" {
		return ExecutionPlan{}, fmt.Errorf("unsupported harness %q", config.Harness)
	}
	if strings.TrimSpace(config.ExecutorModel) == "" || strings.TrimSpace(config.JudgeModel) == "" {
		return ExecutionPlan{}, errors.New("executor and judge models are required")
	}
	if config.TrialsPerCase < 1 || config.TrialsPerCase > 3 {
		return ExecutionPlan{}, errors.New("trials per case must be between 1 and 3")
	}
	if config.TimeoutSeconds < 30 || config.TimeoutSeconds > 3600 {
		return ExecutionPlan{}, errors.New("timeout must be between 30 and 3600 seconds")
	}
	if config.Concurrency < 1 || config.Concurrency > 4 {
		return ExecutionPlan{}, errors.New("concurrency must be between 1 and 4")
	}
	enabled := 0
	fixtureSet := map[string]bool{}
	for _, testCase := range frozen.Cases {
		if testCase.Enabled {
			enabled++
			fixtureSet[testCase.Fixture.ID] = true
		}
	}
	if enabled == 0 {
		return ExecutionPlan{}, errors.New("suite has no enabled cases")
	}
	fixtures := make([]string, 0, len(fixtureSet))
	for id := range fixtureSet {
		fixtures = append(fixtures, id)
	}
	sort.Strings(fixtures)
	plannedTrials := enabled * config.TrialsPerCase
	return ExecutionPlan{
		ArtifactID: artifact.ID, ArtifactLabel: artifact.Label, ArtifactSHA: artifactSHA,
		SuiteID: frozen.ID, SuiteRevision: frozen.Revision, SuiteSHA: frozen.ContentSHA,
		FixtureIDs: fixtures, EnabledCases: enabled, PlannedTrials: plannedTrials,
		EstimatedModelCalls: plannedTrials * 2, Config: config,
	}, nil
}
