package suitereview

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nafiskhan/mdbench/internal/suite"
)

type Styles struct {
	Text     lipgloss.Style
	Muted    lipgloss.Style
	Selected lipgloss.Style
	Accent   lipgloss.Style
	Warning  lipgloss.Style
}

type ContinueMsg struct{ Draft suite.Draft }
type CanceledMsg struct{}

type mode int

const (
	modeCases mode = iota
	modeCaseDetail
	modeDimensions
	modeEditCase
	modeEditRubric
)

type Model struct {
	draft           suite.Draft
	styles          Styles
	mode            mode
	caseCursor      int
	dimensionCursor int
	offset          int
	width           int
	height          int
	editor          textarea.Model
	originalText    string
	validationErr   string
}

func New(draft suite.Draft, styles Styles) Model {
	editor := textarea.New()
	editor.Prompt = "│ "
	editor.ShowLineNumbers = false
	editor.CharLimit = 8 << 10
	editor.SetWidth(68)
	editor.SetHeight(10)
	return Model{draft: draft, styles: styles, width: 72, height: 14, editor: editor}
}

func (m *Model) SetSize(width, height int) {
	m.width, m.height = max(24, width), max(6, height)
	if m.draft.SchemaVersion == 0 {
		return
	}
	m.editor.SetWidth(max(20, m.width-2))
	m.editor.SetHeight(max(4, m.height-3))
}

func (m *Model) SetStyles(styles Styles) { m.styles = styles }

func (m Model) Draft() suite.Draft { return m.draft }

func (m Model) Update(message tea.Msg) (Model, tea.Cmd) {
	key, isKey := message.(tea.KeyPressMsg)
	if m.mode == modeEditCase || m.mode == modeEditRubric {
		if isKey {
			switch key.String() {
			case "esc":
				m.editor.Blur()
				m.editor.SetValue(m.originalText)
				if m.mode == modeEditCase {
					m.mode = modeCaseDetail
				} else {
					m.mode = modeDimensions
				}
				return m, nil
			case "super+enter":
				value := strings.TrimSpace(m.editor.Value())
				if value == "" {
					return m, nil
				}
				m.editor.Blur()
				if m.mode == modeEditCase {
					m.draft.Cases[m.caseCursor].Prompt = value
					m.mode = modeCaseDetail
				} else {
					m.draft.Rubric = value
					m.mode = modeDimensions
				}
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(message)
		return m, cmd
	}
	if !isKey {
		return m, nil
	}

	switch m.mode {
	case modeCases:
		return m.updateCases(key)
	case modeCaseDetail:
		return m.updateCaseDetail(key)
	case modeDimensions:
		return m.updateDimensions(key)
	}
	return m, nil
}

func (m Model) updateCases(key tea.KeyPressMsg) (Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		return m, func() tea.Msg { return CanceledMsg{} }
	case "tab":
		m.mode, m.offset = modeDimensions, 0
	case "up":
		m.caseCursor = max(0, m.caseCursor-1)
	case "down":
		m.caseCursor = min(len(m.draft.Cases)-1, m.caseCursor+1)
	case "enter":
		m.mode, m.offset = modeCaseDetail, 0
	case " ", "space":
		m.draft.Cases[m.caseCursor].Enabled = !m.draft.Cases[m.caseCursor].Enabled
	case "+", "=":
		m.draft.Cases[m.caseCursor].Weight += 0.5
	case "-":
		m.draft.Cases[m.caseCursor].Weight = max(0.5, m.draft.Cases[m.caseCursor].Weight-0.5)
	case "[":
		if m.caseCursor > 0 {
			current := m.caseCursor
			m.draft.Cases[current-1], m.draft.Cases[current] = m.draft.Cases[current], m.draft.Cases[current-1]
			m.caseCursor--
		}
	case "]":
		if m.caseCursor+1 < len(m.draft.Cases) {
			current := m.caseCursor
			m.draft.Cases[current], m.draft.Cases[current+1] = m.draft.Cases[current+1], m.draft.Cases[current]
			m.caseCursor++
		}
	case "r":
		return m.startEdit(modeEditRubric, m.draft.Rubric)
	case "f":
		if err := suite.Validate(m.draft); err != nil {
			m.validationErr = err.Error()
			return m, nil
		}
		return m, func() tea.Msg { return ContinueMsg{Draft: m.draft} }
	}
	return m, nil
}

func (m Model) updateCaseDetail(key tea.KeyPressMsg) (Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.mode, m.offset = modeCases, 0
	case "up":
		m.offset = max(0, m.offset-1)
	case "down":
		m.offset++
	case "e":
		return m.startEdit(modeEditCase, m.draft.Cases[m.caseCursor].Prompt)
	case " ", "space":
		m.draft.Cases[m.caseCursor].Enabled = !m.draft.Cases[m.caseCursor].Enabled
	}
	return m, nil
}

func (m Model) updateDimensions(key tea.KeyPressMsg) (Model, tea.Cmd) {
	switch key.String() {
	case "esc", "tab":
		m.mode, m.offset = modeCases, 0
	case "up":
		m.dimensionCursor = max(0, m.dimensionCursor-1)
	case "down":
		m.dimensionCursor = min(len(m.draft.Dimensions)-1, m.dimensionCursor+1)
	case " ", "space":
		dimension := &m.draft.Dimensions[m.dimensionCursor]
		dimension.Applicable = !dimension.Applicable
	case "+", "=":
		m.draft.Dimensions[m.dimensionCursor].Weight += 0.5
	case "-":
		dimension := &m.draft.Dimensions[m.dimensionCursor]
		dimension.Weight = max(0.5, dimension.Weight-0.5)
	case "r":
		return m.startEdit(modeEditRubric, m.draft.Rubric)
	case "f":
		if err := suite.Validate(m.draft); err != nil {
			m.validationErr = err.Error()
			return m, nil
		}
		return m, func() tea.Msg { return ContinueMsg{Draft: m.draft} }
	}
	return m, nil
}

func (m Model) startEdit(next mode, value string) (Model, tea.Cmd) {
	m.mode, m.originalText = next, value
	m.editor.SetValue(value)
	m.editor.CursorEnd()
	return m, m.editor.Focus()
}

func (m Model) View() string {
	switch m.mode {
	case modeCases:
		return m.caseList()
	case modeCaseDetail:
		return m.caseDetail()
	case modeDimensions:
		return m.dimensionList()
	case modeEditCase:
		return strings.Join([]string{m.styles.Accent.Render("Edit task prompt"), m.styles.Muted.Render("Enter adds a line. Command+Enter saves."), "", m.editor.View()}, "\n")
	case modeEditRubric:
		return strings.Join([]string{m.styles.Accent.Render("Refine judge rubric"), m.styles.Muted.Render("Enter adds a line. Command+Enter saves."), "", m.editor.View()}, "\n")
	default:
		return ""
	}
}

func (m Model) Footer() string {
	switch m.mode {
	case modeCases:
		if m.width < 60 {
			return "↑↓ enter  space on/off  tab scores  f freeze"
		}
		return "↑↓ enter  space on/off  [ ] order  +/- weight  tab scores  r rubric  f freeze"
	case modeCaseDetail:
		return "↑↓ scroll  e edit  space on/off  esc cases"
	case modeDimensions:
		if m.width < 60 {
			return "↑↓  space  +/-  r rubric  tab cases  f freeze"
		}
		return "↑↓ move  space applies  +/- weight  r rubric  tab cases  f freeze"
	case modeEditCase, modeEditRubric:
		return "enter newline  cmd+enter save  esc cancel"
	default:
		return ""
	}
}

func (m Model) caseList() string {
	lines := []string{m.styles.Accent.Render("Review generated tests"), m.styles.Muted.Render(fmt.Sprintf("case %d/%d   score dimensions %d", m.caseCursor+1, len(m.draft.Cases), len(m.draft.Dimensions))), ""}
	rows := max(1, m.height-3)
	start := max(0, m.caseCursor-rows+1)
	end := min(len(m.draft.Cases), start+rows)
	for index := start; index < end; index++ {
		testCase := m.draft.Cases[index]
		mark := "[ ]"
		if testCase.Enabled {
			mark = "[x]"
		}
		line := fmt.Sprintf("  %s %-28s %.1fx  %s", mark, testCase.Title, testCase.Weight, testCase.Fixture.ID)
		style := m.styles.Text
		if index == m.caseCursor {
			line, style = ">"+line[1:], m.styles.Selected
		}
		lines = append(lines, style.Render(m.clip(line)))
	}
	if m.validationErr != "" {
		lines = append(lines, "", m.styles.Warning.Render(m.wrap(m.validationErr)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) caseDetail() string {
	testCase := m.draft.Cases[m.caseCursor]
	lines := []string{
		m.styles.Accent.Render(testCase.Title),
		fmt.Sprintf("case         %s   enabled %t", testCase.ID, testCase.Enabled),
		fmt.Sprintf("fixture      %s   timeout %ds   weight %.1f", testCase.Fixture.ID, testCase.TimeoutSeconds, testCase.Weight),
		"", m.styles.Accent.Render("Task prompt"), m.wrap(testCase.Prompt),
		"", m.styles.Accent.Render("Assertions"),
	}
	for _, assertion := range testCase.Assertions {
		detail := string(assertion.Type)
		if len(assertion.Argv) > 0 {
			detail += "  " + strings.Join(assertion.Argv, " ")
		} else if assertion.Path != "" {
			detail += "  " + assertion.Path
		}
		lines = append(lines, m.wrap("  "+assertion.ID+"  "+detail+"  "+assertion.Expected))
	}
	lines = append(lines, "", m.styles.Accent.Render("Judge criteria"))
	for _, criterion := range testCase.JudgeCriteria {
		lines = append(lines, m.wrap("  "+criterion))
	}
	lines = append(lines, "", m.styles.Accent.Render("Required evidence"), m.wrap("  "+strings.Join(testCase.EvidenceRequired, ", ")))
	lines = append(lines, "", m.styles.Accent.Render("Scored dimensions"), m.wrap("  "+strings.Join(testCase.Dimensions, ", ")))
	lines = append(lines, "", m.styles.Accent.Render("Hard-failure rules"))
	for _, rule := range m.draft.HardFailureRules {
		for _, assertion := range testCase.Assertions {
			if rule.TriggerID == assertion.ID {
				lines = append(lines, m.wrap(fmt.Sprintf("  %s  cap %.1f  %s", rule.TriggerID, rule.ScoreCap, rule.Reason)))
			}
		}
	}
	return m.window(lines)
}

func (m Model) dimensionList() string {
	dimension := m.draft.Dimensions[m.dimensionCursor]
	mark := "[ ]"
	if dimension.Applicable {
		mark = "[x]"
	}
	lines := []string{
		m.styles.Accent.Render("Review scoring contract"),
		m.styles.Muted.Render(fmt.Sprintf("dimension %d/%d   Tab returns to cases", m.dimensionCursor+1, len(m.draft.Dimensions))),
		"", m.styles.Selected.Render(fmt.Sprintf("> %s %s   %.1fx", mark, dimension.Name, dimension.Weight)),
		"", m.wrap(dimension.Guidance),
		"", m.styles.Muted.Render("0  " + dimension.Anchors.Zero),
		m.styles.Muted.Render("5  " + dimension.Anchors.Five),
		m.styles.Muted.Render("10 " + dimension.Anchors.Ten),
		"", m.styles.Accent.Render("Judge rubric"), m.wrap(m.draft.Rubric),
	}
	if m.validationErr != "" {
		lines = append(lines, "", m.styles.Warning.Render(m.wrap(m.validationErr)))
	}
	return m.window(lines)
}

func (m Model) window(lines []string) string {
	limit := max(1, m.height)
	maxOffset := max(0, len(lines)-limit)
	offset := min(m.offset, maxOffset)
	return strings.Join(lines[offset:min(len(lines), offset+limit)], "\n")
}

func (m Model) wrap(value string) string {
	return lipgloss.NewStyle().Width(m.width).Render(value)
}

func (m Model) clip(value string) string {
	return lipgloss.NewStyle().MaxWidth(m.width).Render(value)
}
