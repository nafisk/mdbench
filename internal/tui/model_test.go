package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/nafiskhan/mdbench/internal/app"
	"github.com/nafiskhan/mdbench/internal/model"
)

func TestFileBrowserStartsAtUserHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	model := New(nil, app.Config{MaxArtifactBytes: 1 << 20})
	if model.fileBrowser.CurrentDirectory() != home {
		t.Fatalf("file browser starts at %q, want %q", model.fileBrowser.CurrentDirectory(), home)
	}
}

func TestInitialResizeBeforeSuiteReview(t *testing.T) {
	model := New(nil, app.Config{MaxArtifactBytes: 1 << 20})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if updated.(Model).width != 80 {
		t.Fatal("initial resize was not applied")
	}
}

func TestGenerateSuiteFlowReachesReview(t *testing.T) {
	root := t.TempDir()
	config := app.Config{
		ConfigDir: filepath.Join(root, "config"), DataDir: filepath.Join(root, "data"), CacheDir: filepath.Join(root, "cache"),
		MaxArtifactBytes: 1 << 20, MaxBundleBytes: 8 << 20, MaxBundleFiles: 128,
	}
	service, err := app.NewService(config)
	if err != nil {
		t.Fatal(err)
	}
	current := New(service, config)
	current.screen = screenSaved
	current.artifact = model.Artifact{EffectiveSHA: strings.Repeat("a", 64)}

	current, _, _ = current.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	current, _, _ = current.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	current.configCursor = len(current.fixtures) + 1
	current, cmd, handled := current.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !handled || cmd == nil || current.screen != screenLoading {
		t.Fatal("generate did not enter the loading screen")
	}
	updated, _ := current.Update(cmd())
	result := updated.(Model)
	if result.screen != screenSuiteReview || len(result.draft.Cases) != 6 {
		t.Fatalf("generation reached screen %d with %d cases", result.screen, len(result.draft.Cases))
	}
}

func TestPasteUsesCommandEnterForReview(t *testing.T) {
	input := textarea.New()
	input.SetValue("# Instructions\n\nKeep changes small.\n")
	model := Model{screen: screenPaste, paste: input}

	if _, _, handled := model.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter}); handled {
		t.Fatal("plain enter should remain available for a new line")
	}
	updated, cmd, handled := model.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModSuper})
	if !handled || cmd == nil || updated.screen != screenLoading {
		t.Fatal("command+enter did not start review")
	}
}
