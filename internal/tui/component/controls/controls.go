package controls

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type Binding struct {
	Key    string
	Action string
}

func Help(keyStyle, actionStyle lipgloss.Style, bindings ...Binding) string {
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		parts = append(parts, keyStyle.Render("["+binding.Key+"]")+" "+actionStyle.Render(binding.Action))
	}
	return strings.Join(parts, actionStyle.Render("  •  "))
}

func Wrap(index, delta, count int) int {
	if count <= 0 {
		return 0
	}
	return ((index+delta)%count + count) % count
}

func AcceptText(key string) bool {
	switch key {
	case "super+enter", "ctrl+enter", "ctrl+s":
		return true
	default:
		return false
	}
}
