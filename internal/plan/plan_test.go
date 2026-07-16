package plan

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nafiskhan/mdbench/internal/harness"
	"github.com/nafiskhan/mdbench/internal/model"
	"github.com/nafiskhan/mdbench/internal/suite"
)

func TestBuildCountsTrialsAndIndependentModelCalls(t *testing.T) {
	artifact := model.Artifact{ID: "artifact-1", EffectiveSHA: strings.Repeat("a", 64)}
	draft, err := harness.NewFakeGenerator().GenerateSuite(context.Background(), harness.GenerateRequest{Artifact: artifact, CaseCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := suite.Freeze(draft, 1, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	config.TrialsPerCase = 2
	value, err := Build(artifact, frozen, config)
	if err != nil {
		t.Fatal(err)
	}
	if value.PlannedTrials != 6 || value.EstimatedModelCalls != 12 {
		t.Fatalf("plan has %d trials and %d calls", value.PlannedTrials, value.EstimatedModelCalls)
	}
}
