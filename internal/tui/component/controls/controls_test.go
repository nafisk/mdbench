package controls

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestHelpSeparatesBindings(t *testing.T) {
	result := Help(lipgloss.NewStyle(), lipgloss.NewStyle(),
		Binding{Key: "enter", Action: "select"},
		Binding{Key: "esc", Action: "back"},
	)
	if result != "[enter] select  •  [esc] back" {
		t.Fatalf("help rendered %q", result)
	}
	if strings.Count(result, "[") != 2 {
		t.Fatal("help did not render distinct key groups")
	}
}

func TestWrapMovesAcrossListEnds(t *testing.T) {
	if got := Wrap(0, -1, 4); got != 3 {
		t.Fatalf("up from the first row reached %d", got)
	}
	if got := Wrap(3, 1, 4); got != 0 {
		t.Fatalf("down from the last row reached %d", got)
	}
}

func TestAcceptTextHasTerminalFallback(t *testing.T) {
	for _, key := range []string{"super+enter", "ctrl+enter", "ctrl+s"} {
		if !AcceptText(key) {
			t.Fatalf("%s was not accepted", key)
		}
	}
	if AcceptText("enter") {
		t.Fatal("plain enter should remain available for new lines")
	}
}
