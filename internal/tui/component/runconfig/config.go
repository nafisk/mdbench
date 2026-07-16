package runconfig

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nafiskhan/mdbench/internal/plan"
	"github.com/nafiskhan/mdbench/internal/tui/component/controls"
)

type Styles struct {
	Text     lipgloss.Style
	Muted    lipgloss.Style
	Selected lipgloss.Style
	Accent   lipgloss.Style
	Warning  lipgloss.Style
}

type ContinueMsg struct{ Config plan.Config }
type CanceledMsg struct{}

type Model struct {
	config    plan.Config
	styles    Styles
	cursor    int
	editing   bool
	editField int
	original  string
	input     textinput.Model
	width     int
}

func New(config plan.Config, styles Styles) Model {
	input := textinput.New()
	input.Prompt = "model: "
	input.CharLimit = 160
	input.SetWidth(48)
	return Model{config: config, styles: styles, input: input, width: 72}
}

func (m *Model) SetSize(width int) {
	m.width = max(24, width)
	if m.config.Harness != "" {
		m.input.SetWidth(max(16, m.width-8))
	}
}

func (m *Model) SetStyles(styles Styles) { m.styles = styles }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	key, isKey := message.(tea.KeyPressMsg)
	if m.editing {
		if isKey {
			switch key.String() {
			case "esc":
				m.editing = false
				m.input.Blur()
				m.input.SetValue(m.original)
				return m, nil
			case "enter":
				value := strings.TrimSpace(m.input.Value())
				if value == "" {
					return m, nil
				}
				if m.editField == 1 {
					m.config.ExecutorModel = value
				} else {
					m.config.JudgeModel = value
				}
				m.editing = false
				m.input.Blur()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(message)
		return m, cmd
	}
	if !isKey {
		return m, nil
	}
	switch key.String() {
	case "esc":
		return m, func() tea.Msg { return CanceledMsg{} }
	case "up":
		m.cursor = controls.Wrap(m.cursor, -1, 8)
	case "down":
		m.cursor = controls.Wrap(m.cursor, 1, 8)
	case "left":
		m.adjust(-1)
	case "right":
		m.adjust(1)
	case "enter":
		if m.cursor == 1 || m.cursor == 2 {
			return m.startEdit()
		}
		if m.cursor == 7 {
			return m, func() tea.Msg { return ContinueMsg{Config: m.config} }
		}
	}
	return m, nil
}

func (m *Model) adjust(delta int) {
	switch m.cursor {
	case 3:
		m.config.TrialsPerCase = min(3, max(1, m.config.TrialsPerCase+delta))
	case 4:
		m.config.TimeoutSeconds = min(3600, max(30, m.config.TimeoutSeconds+delta*30))
	case 5:
		m.config.Network = !m.config.Network
	case 6:
		m.config.Concurrency = min(4, max(1, m.config.Concurrency+delta))
	}
}

func (m Model) startEdit() (Model, tea.Cmd) {
	m.editing, m.editField = true, m.cursor
	value := m.config.ExecutorModel
	if m.cursor == 2 {
		value = m.config.JudgeModel
	}
	m.original = value
	m.input.SetValue(value)
	m.input.CursorEnd()
	return m, m.input.Focus()
}

func (m Model) View() string {
	if m.editing {
		role := "executor"
		if m.editField == 2 {
			role = "judge"
		}
		return strings.Join([]string{m.styles.Accent.Render("Choose " + role + " model"), m.styles.Muted.Render("Use a model name supported by the configured Codex CLI."), "", m.input.View()}, "\n")
	}
	network := "off"
	if m.config.Network {
		network = "on"
	}
	rows := []string{
		"Harness             Codex",
		"Executor model      " + m.config.ExecutorModel,
		"Judge model         " + m.config.JudgeModel,
		fmt.Sprintf("Trials per case     %d", m.config.TrialsPerCase),
		fmt.Sprintf("Trial timeout       %ds", m.config.TimeoutSeconds),
		"Command network     " + network,
		fmt.Sprintf("Maximum concurrency %d", m.config.Concurrency),
	}
	lines := []string{m.styles.Accent.Render("Configure evaluation"), m.styles.Muted.Render("Executor and judge always run in independent sessions."), ""}
	for index, row := range rows {
		prefix, style := "  ", m.styles.Text
		if index == m.cursor {
			prefix, style = "> ", m.styles.Selected
		}
		lines = append(lines, style.Render(lipgloss.NewStyle().MaxWidth(m.width).Render(prefix+row)))
	}
	lines = append(lines, "", m.styles.Muted.Render("Ready to continue?"))
	actionStyle, prefix := m.styles.Text, "  "
	if m.cursor == 7 {
		actionStyle, prefix = m.styles.Selected, "> "
	}
	lines = append(lines, actionStyle.Render(prefix+"Review execution plan"))
	if m.config.ExecutorModel == m.config.JudgeModel {
		lines = append(lines, "", m.styles.Muted.Render("Same model, separate sessions."))
	}
	if m.config.Network {
		lines = append(lines, m.styles.Warning.Render("Trial commands may access the network."))
	}
	return strings.Join(lines, "\n")
}

func (m Model) Footer() string {
	if m.editing {
		return controls.Help(m.styles.Selected, m.styles.Muted,
			controls.Binding{Key: "enter", Action: "save"},
			controls.Binding{Key: "esc", Action: "cancel"},
		)
	}
	return controls.Help(m.styles.Selected, m.styles.Muted,
		controls.Binding{Key: "↑/↓", Action: "move"},
		controls.Binding{Key: "←/→", Action: "change"},
		controls.Binding{Key: "enter", Action: "select"},
		controls.Binding{Key: "esc", Action: "tests"},
	)
}
