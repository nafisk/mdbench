package runconfig

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/nafiskhan/mdbench/internal/plan"
)

func TestConfigurationUsesBoundedControls(t *testing.T) {
	model := New(plan.DefaultConfig(), Styles{})
	model.cursor = 3
	for range 5 {
		model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	if model.config.TrialsPerCase != 3 {
		t.Fatalf("trials reached %d", model.config.TrialsPerCase)
	}
	model.cursor = 7
	_, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("review plan did not continue")
	}
	if message := cmd().(ContinueMsg); message.Config.TrialsPerCase != 3 {
		t.Fatalf("continued with %#v", message.Config)
	}
}

func TestConfigurationEditsModelNames(t *testing.T) {
	model := New(plan.DefaultConfig(), Styles{})
	model.cursor = 1
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !model.editing {
		t.Fatal("executor model did not enter edit mode")
	}
	model.input.SetValue("custom-executor")
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.config.ExecutorModel != "custom-executor" || model.editing {
		t.Fatalf("executor model edit produced %#v", model.config)
	}
}

func TestConfigurationNavigationWrapsToContinueAction(t *testing.T) {
	model := New(plan.DefaultConfig(), Styles{})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if model.cursor != 7 {
		t.Fatalf("up from first option moved to %d", model.cursor)
	}
}
