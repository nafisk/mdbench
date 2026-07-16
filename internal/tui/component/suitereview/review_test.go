package suitereview

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/nafiskhan/mdbench/internal/harness"
	"github.com/nafiskhan/mdbench/internal/model"
)

func TestReviewEditsCasesAndScoringContract(t *testing.T) {
	draft, err := harness.NewFakeGenerator().GenerateSuite(context.Background(), harness.GenerateRequest{
		Artifact: model.Artifact{EffectiveSHA: strings.Repeat("a", 64)}, CaseCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	review := New(draft, Styles{})

	review, _ = review.Update(tea.KeyPressMsg{Code: ' '})
	if review.draft.Cases[0].Enabled {
		t.Fatal("space did not disable the selected case")
	}
	review, _ = review.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	secondID := review.draft.Cases[1].ID
	review, _ = review.Update(tea.KeyPressMsg{Code: '['})
	if review.draft.Cases[0].ID != secondID {
		t.Fatal("reorder did not move the selected case")
	}
	review, _ = review.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	before := review.draft.Dimensions[0].Weight
	review, _ = review.Update(tea.KeyPressMsg{Code: '+'})
	if review.draft.Dimensions[0].Weight != before+0.5 {
		t.Fatal("dimension weight did not increase")
	}
}

func TestReviewContinueReturnsValidatedDraft(t *testing.T) {
	draft, err := harness.NewFakeGenerator().GenerateSuite(context.Background(), harness.GenerateRequest{
		Artifact: model.Artifact{EffectiveSHA: strings.Repeat("b", 64)}, CaseCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	review := New(draft, Styles{})
	_, cmd := review.Update(tea.KeyPressMsg{Code: 'f'})
	if cmd == nil {
		t.Fatal("freeze did not continue")
	}
	message, ok := cmd().(ContinueMsg)
	if !ok || len(message.Draft.Cases) != 1 {
		t.Fatalf("continue returned %#v", message)
	}
}
