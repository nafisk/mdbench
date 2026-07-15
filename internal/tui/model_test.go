package tui

import (
	"os"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/nafiskhan/mdbench/internal/app"
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
