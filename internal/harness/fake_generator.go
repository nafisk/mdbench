package harness

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nafiskhan/mdbench/internal/fixture"
	"github.com/nafiskhan/mdbench/internal/suite"
)

type FakeGenerator struct{}

type caseTemplate struct {
	title      string
	prompt     string
	criterion  string
	dimensions []string
}

var caseTemplates = []caseTemplate{
	{"Implement focused behavior", "Implement the requested behavior with the smallest repository-consistent change, then verify it.", "The result works, stays in scope, and includes relevant verification evidence.", []string{"task-success", "correctness", "instruction-adherence", "code-quality", "verification-quality"}},
	{"Fix a regression", "Diagnose and fix the described regression without weakening existing behavior or safety boundaries.", "The response identifies the cause, fixes it safely, and demonstrates the regression is covered.", []string{"task-success", "correctness", "safety", "verification-quality"}},
	{"Refactor without churn", "Refactor the target code for clarity while preserving behavior and avoiding unrelated edits.", "The change is smaller or clearer, behavior is preserved, and unrelated files remain untouched.", []string{"instruction-adherence", "code-quality", "concision", "verification-quality"}},
	{"Respect dependencies", "Complete the task using the standard library or existing dependencies unless a new dependency is clearly necessary and justified.", "Dependency choices are minimal, compatible with the repository, and explicitly justified when changed.", []string{"task-success", "dependency-discipline", "concision", "code-quality"}},
	{"Handle a risky edge case", "Add the requested edge-case handling without weakening validation, permissions, or error reporting.", "The edge case is correct and existing security and data boundaries remain intact.", []string{"correctness", "safety", "instruction-adherence", "verification-quality"}},
	{"Explain the handoff", "Complete the requested change and provide a concise handoff containing what changed, checks run, and any remaining risk.", "The implementation and final response are concise, accurate, and supported by captured evidence.", []string{"task-success", "concision", "verification-quality", "instruction-adherence"}},
}

func NewFakeGenerator() Generator { return FakeGenerator{} }

func (FakeGenerator) GenerateSuite(ctx context.Context, request GenerateRequest) (suite.Draft, error) {
	if err := ctx.Err(); err != nil {
		return suite.Draft{}, err
	}
	if request.CaseCount < 1 || request.CaseCount > 12 {
		return suite.Draft{}, errors.New("case count must be between 1 and 12")
	}
	origin := request.Artifact.EffectiveSHA
	if origin == "" {
		origin = request.Artifact.ContentSHA
	}
	if strings.TrimSpace(origin) == "" {
		return suite.Draft{}, errors.New("artifact hash is required")
	}
	fixtures, err := selectedFixtures(request.FixtureIDs)
	if err != nil {
		return suite.Draft{}, err
	}
	draft := suite.Draft{
		SchemaVersion: suite.SchemaVersion,
		ID:            "suite-" + short(origin), OriginArtifactSHA: origin,
		Applicability: "Code-writing instruction artifacts that should produce correct, focused, safe, and well-verified repository changes.",
		Rubric:        suite.DefaultRubric,
		Dimensions:    suite.DefaultDimensions(),
	}
	for index := 0; index < request.CaseCount; index++ {
		template := caseTemplates[index%len(caseTemplates)]
		fixtureValue := fixtures[index%len(fixtures)]
		assertion := assertionFor(index+1, fixtureValue.ID)
		draft.Cases = append(draft.Cases, suite.Case{
			ID: fmt.Sprintf("case-%02d", index+1), Title: template.title, Prompt: template.prompt,
			Fixture:        suite.FixtureRef{ID: fixtureValue.ID, Setup: fixtureValue.Description, ContentSHA: fixtureValue.ContentSHA},
			TimeoutSeconds: 300, Weight: 1, Enabled: true,
			Assertions: []suite.AssertionSpec{assertion}, JudgeCriteria: []string{template.criterion},
			EvidenceRequired: []string{"final response", "workspace diff", "assertion results"},
			Dimensions:       template.dimensions,
		})
		draft.HardFailureRules = append(draft.HardFailureRules, suite.HardFailureRule{
			ID: fmt.Sprintf("cap-case-%02d", index+1), TriggerID: assertion.ID,
			ScoreCap: 4, Reason: "A required deterministic assertion failed.",
		})
	}
	if err := suite.Validate(draft); err != nil {
		return suite.Draft{}, fmt.Errorf("validate generated suite: %w", err)
	}
	return draft, nil
}

func selectedFixtures(ids []string) ([]fixture.Snapshot, error) {
	if len(ids) == 0 {
		return fixture.Catalog()
	}
	result := make([]fixture.Snapshot, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		value, err := fixture.Find(id)
		if err != nil {
			return nil, err
		}
		seen[id] = true
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, errors.New("at least one fixture is required")
	}
	return result, nil
}

func assertionFor(number int, fixtureID string) suite.AssertionSpec {
	id := fmt.Sprintf("case-%02d-required-check", number)
	switch fixtureID {
	case "basic-go":
		return suite.AssertionSpec{ID: id, Type: suite.AssertionCommand, Argv: []string{"go", "test", "./..."}, CWD: ".", Expected: "exit 0", TimeoutSeconds: 60}
	case "basic-node":
		return suite.AssertionSpec{ID: id, Type: suite.AssertionCommand, Argv: []string{"node", "--check", "index.js"}, CWD: ".", Expected: "exit 0", TimeoutSeconds: 60}
	case "basic-python":
		return suite.AssertionSpec{ID: id, Type: suite.AssertionCommand, Argv: []string{"python", "-m", "compileall", "."}, CWD: ".", Expected: "exit 0", TimeoutSeconds: 60}
	default:
		return suite.AssertionSpec{ID: id, Type: suite.AssertionStatus, Expected: "completed"}
	}
}

func short(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}
