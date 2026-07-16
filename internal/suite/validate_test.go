package suite

import (
	"strings"
	"testing"

	"github.com/nafiskhan/mdbench/internal/fixture"
)

func TestValidateAndCanonicalHash(t *testing.T) {
	draft := validDraft(t)
	first, err := CanonicalHash(draft)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalHash(draft)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("canonical hashes are %q and %q", first, second)
	}
	draft.Cases[0].Prompt += " updated"
	changed, err := CanonicalHash(draft)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("content change did not change the suite hash")
	}
}

func TestValidateRejectsUnsafeOrIncompleteSuite(t *testing.T) {
	draft := validDraft(t)
	draft.Cases[0].Assertions[0].CWD = "../host"
	if err := Validate(draft); err == nil || !strings.Contains(err.Error(), "inside the fixture") {
		t.Fatalf("unsafe working directory returned %v", err)
	}

	draft = validDraft(t)
	draft.Dimensions[0].Anchors.Five = ""
	if err := Validate(draft); err == nil || !strings.Contains(err.Error(), "anchors") {
		t.Fatalf("missing score anchor returned %v", err)
	}
}

func validDraft(t *testing.T) Draft {
	t.Helper()
	fixtureValue, err := fixture.Find("basic-go")
	if err != nil {
		t.Fatal(err)
	}
	return Draft{
		SchemaVersion: SchemaVersion, ID: "suite-example", OriginArtifactSHA: strings.Repeat("a", 64),
		Applicability: "Code-writing instruction artifacts.", Rubric: DefaultRubric,
		Dimensions: DefaultDimensions(),
		Cases: []Case{{
			ID: "case-1", Title: "Make a focused change", Prompt: "Add the requested behavior without unrelated edits.",
			Fixture:        FixtureRef{ID: fixtureValue.ID, Setup: fixtureValue.Description, ContentSHA: fixtureValue.ContentSHA},
			TimeoutSeconds: 300, Weight: 1, Enabled: true,
			Assertions:       []AssertionSpec{{ID: "case-1-tests", Type: AssertionCommand, Argv: []string{"go", "test", "./..."}, CWD: ".", TimeoutSeconds: 60}},
			JudgeCriteria:    []string{"The implementation is correct and focused."},
			EvidenceRequired: []string{"final response", "workspace diff", "test output"},
			Dimensions:       []string{"task-success", "correctness", "code-quality"},
		}},
	}
}
