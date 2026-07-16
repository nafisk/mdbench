package filebrowser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nafiskhan/mdbench/internal/tui/component/controls"
)

type Styles struct {
	Text     lipgloss.Style
	Muted    lipgloss.Style
	Selected lipgloss.Style
}

type SelectedMsg struct {
	Path string
}

type CanceledMsg struct{}

type entry struct {
	name  string
	path  string
	mode  os.FileMode
	size  int64
	isDir bool
}

type Model struct {
	startDir   string
	currentDir string
	entries    []entry
	visible    []int
	cursor     int
	offset     int
	width      int
	height     int
	search     textinput.Model
	styles     Styles
	err        error
}

func New(startDir string, styles Styles) Model {
	search := textinput.New()
	search.Prompt = "search: "
	search.Placeholder = "filename"
	search.CharLimit = 128
	search.Focus()
	model := Model{
		startDir: startDir, currentDir: startDir,
		width: 72, height: 12, search: search, styles: styles,
	}
	model.SetStyles(styles)
	model.load(startDir)
	return model
}

func (m *Model) Reset() {
	m.clearSearch()
	m.load(m.startDir)
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = max(20, width), max(4, height)
	m.search.SetWidth(max(10, m.width-22))
	m.keepCursorVisible()
}

func (m *Model) SetStyles(styles Styles) {
	m.styles = styles
	inputStyles := m.search.Styles()
	inputStyles.Focused.Prompt = styles.Muted
	inputStyles.Focused.Text = styles.Text
	inputStyles.Focused.Placeholder = styles.Muted
	inputStyles.Blurred = inputStyles.Focused
	m.search.SetStyles(inputStyles)
}

func (m Model) CurrentDirectory() string { return m.currentDir }

func (m Model) View() string {
	if m.err != nil {
		return m.styles.Muted.Render(m.err.Error())
	}
	lines := []string{m.statusLine()}
	rowCount := max(1, m.height-1)
	end := min(len(m.visible), m.offset+rowCount)
	for visibleIndex := m.offset; visibleIndex < end; visibleIndex++ {
		current := m.entries[m.visible[visibleIndex]]
		name := current.name
		if current.isDir {
			name += "/"
		}
		line := fmt.Sprintf("  %s %6s %s", current.mode.String(), formatSize(current.size), name)
		style := m.styles.Text
		if visibleIndex == m.cursor {
			line = ">" + line[1:]
			style = m.styles.Selected
		} else if !current.isDir && !isMarkdown(current.path) {
			style = m.styles.Muted
		}
		line = lipgloss.NewStyle().MaxWidth(m.width).Render(line)
		lines = append(lines, style.Render(line))
	}
	if len(m.visible) == 0 {
		lines = append(lines, m.styles.Muted.Render("  No matching entries"))
	}
	return strings.Join(lines, "\n")
}

func (m Model) Footer() string {
	return controls.Help(m.styles.Selected, m.styles.Muted,
		controls.Binding{Key: "type", Action: "filter"},
		controls.Binding{Key: "↑/↓", Action: "move"},
		controls.Binding{Key: "→", Action: "open"},
		controls.Binding{Key: "←", Action: "back"},
		controls.Binding{Key: "enter", Action: "choose"},
	)
}

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	key, isKey := message.(tea.KeyPressMsg)
	if isKey {
		switch key.String() {
		case "esc":
			if m.search.Value() != "" {
				m.clearSearch()
				return m, nil
			}
			return m, func() tea.Msg { return CanceledMsg{} }
		case "up":
			m.moveWrap(-1)
			return m, nil
		case "down":
			m.moveWrap(1)
			return m, nil
		case "pgup":
			m.move(-max(1, m.height-1))
			return m, nil
		case "pgdown":
			m.move(max(1, m.height-1))
			return m, nil
		case "left":
			m.load(filepath.Dir(m.currentDir))
			return m, nil
		case "right":
			if selected, ok := m.selected(); ok && selected.isDir {
				m.load(selected.path)
			}
			return m, nil
		case "enter":
			selected, ok := m.selected()
			if !ok {
				return m, nil
			}
			if selected.isDir {
				m.load(selected.path)
				return m, nil
			}
			if isMarkdown(selected.path) {
				return m, func() tea.Msg { return SelectedMsg{Path: selected.path} }
			}
			return m, nil
		}
	}

	before := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(message)
	if m.search.Value() != before {
		m.applyFilter()
	}
	return m, cmd
}

func (m Model) statusLine() string {
	count := fmt.Sprintf("%d items", len(m.visible))
	if strings.TrimSpace(m.search.Value()) != "" {
		count = fmt.Sprintf("%d matches", len(m.visible))
	}
	return m.search.View() + "  " + m.styles.Muted.Render(count)
}

func (m *Model) move(delta int) {
	if len(m.visible) == 0 {
		return
	}
	m.cursor = min(len(m.visible)-1, max(0, m.cursor+delta))
	m.keepCursorVisible()
}

func (m *Model) moveWrap(delta int) {
	if len(m.visible) == 0 {
		return
	}
	m.cursor = controls.Wrap(m.cursor, delta, len(m.visible))
	m.keepCursorVisible()
}

func (m *Model) keepCursorVisible() {
	rows := max(1, m.height-1)
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
}

func (m Model) selected() (entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return entry{}, false
	}
	return m.entries[m.visible[m.cursor]], true
}

func (m *Model) load(directory string) {
	// ponytail: local directory reads are synchronous for the MVP. Move this to
	// a tea.Cmd if remote or very large filesystems make navigation noticeable.
	items, err := os.ReadDir(directory)
	if err != nil {
		m.err = fmt.Errorf("read %s: %w", directory, err)
		return
	}
	loaded := make([]entry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			continue
		}
		loaded = append(loaded, entry{
			name: item.Name(), path: filepath.Join(directory, item.Name()),
			mode: info.Mode(), size: info.Size(), isDir: item.IsDir(),
		})
	}
	sort.Slice(loaded, func(left, right int) bool {
		if loaded[left].isDir != loaded[right].isDir {
			return loaded[left].isDir
		}
		return strings.ToLower(loaded[left].name) < strings.ToLower(loaded[right].name)
	})
	m.currentDir, m.entries, m.err = directory, loaded, nil
	m.clearSearch()
}

func (m *Model) applyFilter() {
	term := strings.ToLower(strings.TrimSpace(m.search.Value()))
	m.visible = m.visible[:0]
	for index, item := range m.entries {
		if term == "" || strings.Contains(strings.ToLower(item.name), term) {
			m.visible = append(m.visible, index)
		}
	}
	m.cursor, m.offset = 0, 0
}

func (m *Model) clearSearch() {
	m.search.Reset()
	m.search.Focus()
	m.applyFilter()
}

func isMarkdown(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".md" || extension == ".markdown"
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	value, unit := float64(size)/1024, "KB"
	if value >= 1024 {
		value, unit = value/1024, "MB"
	}
	if value >= 10 {
		return fmt.Sprintf("%.0f%s", value, unit)
	}
	return fmt.Sprintf("%.1f%s", value, unit)
}
