package filebrowser

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestLiveFilterAndDirectoryNavigation(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, ".codex")
	second := filepath.Join(root, ".codexbar")
	for _, directory := range []string{first, second} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"alpha.md", "beta.md", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	model := New(root, Styles{})
	for _, character := range ".codex" {
		model, _ = model.Update(tea.KeyPressMsg{Code: character, Text: string(character)})
	}
	if len(model.visible) != 2 {
		t.Fatalf("search returned %#v", model.visible)
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if model.cursor != 1 {
		t.Fatalf("down moved cursor to %d, want 1", model.cursor)
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if model.CurrentDirectory() != second {
		t.Fatalf("enter opened %q, want %q", model.CurrentDirectory(), second)
	}
	if model.search.Value() != "" || len(model.visible) != len(model.entries) {
		t.Fatal("entering a directory did not reset the filter")
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if model.CurrentDirectory() != root {
		t.Fatalf("left opened %q, want %q", model.CurrentDirectory(), root)
	}
}

func TestEscapeClearsFilterBeforeCanceling(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "skill.md"), []byte("# Skill\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := New(root, Styles{})
	model, _ = model.Update(tea.KeyPressMsg{Code: 's', Text: "s"})

	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil || model.search.Value() != "" || len(model.visible) != len(model.entries) {
		t.Fatal("first escape did not clear the active filter")
	}
	_, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("second escape did not cancel the browser")
	}
	message := cmd()
	if _, ok := message.(CanceledMsg); !ok {
		t.Fatalf("second escape returned %T", message)
	}
}

func TestEnterSelectsMarkdownOnly(t *testing.T) {
	root := t.TempDir()
	markdown := filepath.Join(root, "skill.md")
	if err := os.WriteFile(markdown, []byte("# Skill\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := New(root, Styles{})
	model.cursor = 0

	_, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter did not select Markdown")
	}
	message, ok := cmd().(SelectedMsg)
	if !ok || message.Path != markdown {
		t.Fatalf("selection was %#v", message)
	}
}

func TestSearchIsSubstringNotFuzzy(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a-b-c.md", "abc-skill.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	model := New(root, Styles{})
	model.search.SetValue("abc")
	model.applyFilter()
	if len(model.visible) != 1 || model.entries[model.visible[0]].name != "abc-skill.md" {
		t.Fatalf("substring results were %#v", model.visible)
	}
}

func TestArrowNavigationWraps(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	model := New(root, Styles{})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if model.cursor != 1 {
		t.Fatalf("up from first row moved to %d", model.cursor)
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if model.cursor != 0 {
		t.Fatalf("down from last row moved to %d", model.cursor)
	}
}
