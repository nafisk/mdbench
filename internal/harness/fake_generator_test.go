package harness

import (
	"context"
	"strings"
	"testing"

	"github.com/nafiskhan/mdbench/internal/model"
	"github.com/nafiskhan/mdbench/internal/suite"
)

func TestFakeGeneratorReturnsRequestedValidatedCases(t *testing.T) {
	artifact := model.Artifact{EffectiveSHA: strings.Repeat("a", 64)}
	draft, err := NewFakeGenerator().GenerateSuite(context.Background(), GenerateRequest{
		Artifact: artifact, CaseCount: 6,
		FixtureIDs: []string{"basic-go", "basic-node", "basic-python", "empty"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Cases) != 6 || len(draft.Dimensions) != 8 || len(draft.HardFailureRules) != 6 {
		t.Fatalf("generated %d cases, %d dimensions, %d rules", len(draft.Cases), len(draft.Dimensions), len(draft.HardFailureRules))
	}
	if err := suite.Validate(draft); err != nil {
		t.Fatalf("generated suite is invalid: %v", err)
	}
	for _, testCase := range draft.Cases {
		if strings.Contains(strings.ToLower(testCase.Prompt), "ponytail") {
			t.Fatalf("case prompt contains an origin-specific name: %q", testCase.Prompt)
		}
	}
}

func TestFakeGeneratorHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewFakeGenerator().GenerateSuite(ctx, GenerateRequest{CaseCount: 1})
	if err == nil {
		t.Fatal("canceled generation succeeded")
	}
}
