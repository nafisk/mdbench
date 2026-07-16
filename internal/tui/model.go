package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/nafiskhan/mdbench/internal/app"
	"github.com/nafiskhan/mdbench/internal/fixture"
	"github.com/nafiskhan/mdbench/internal/model"
	"github.com/nafiskhan/mdbench/internal/suite"
	"github.com/nafiskhan/mdbench/internal/tui/component/filebrowser"
	"github.com/nafiskhan/mdbench/internal/tui/component/suitereview"
)

type screen int

const (
	screenHome screen = iota
	screenSource
	screenFile
	screenPaste
	screenLoading
	screenInspect
	screenSaved
	screenSuiteChoice
	screenSuiteConfig
	screenSuiteReview
	screenSuiteReuse
	screenSuiteReuseConfirm
	screenSuiteReady
	screenError
)

type styles struct {
	text     lipgloss.Style
	muted    lipgloss.Style
	accent   lipgloss.Style
	warning  lipgloss.Style
	error    lipgloss.Style
	success  lipgloss.Style
	box      lipgloss.Style
	header   lipgloss.Style
	footer   lipgloss.Style
	selected lipgloss.Style
}

type artifactMsg struct {
	artifact model.Artifact
	err      error
}

type savedMsg struct {
	path string
	err  error
}

type suiteMsg struct {
	draft suite.Draft
	err   error
}

type frozenSuiteMsg struct {
	value suite.Frozen
	path  string
	err   error
}

type suiteListMsg struct {
	values []suite.Frozen
	err    error
}

type Model struct {
	service *app.Service
	config  app.Config

	screen       screen
	returnTo     screen
	width        int
	height       int
	dark         bool
	styles       styles
	homeCursor   int
	sourceCursor int
	suiteCursor  int
	configCursor int
	reuseCursor  int
	status       string
	err          error

	fileBrowser  filebrowser.Model
	paste        textarea.Model
	labelInput   textinput.Model
	editingLabel bool
	discardArmed bool

	artifact      model.Artifact
	showBundle    bool
	inspectOffset int
	savedPath     string
	fixtures      []fixture.Snapshot
	fixtureOn     map[string]bool
	caseCount     int
	draft         suite.Draft
	frozen        suite.Frozen
	suiteList     []suite.Frozen
	suitePath     string
	suiteReview   suitereview.Model
}

func New(service *app.Service, config app.Config) Model {
	startDir, err := os.UserHomeDir()
	if err != nil {
		startDir = string(os.PathSeparator)
	}
	paste := textarea.New()
	paste.Prompt = "│ "
	paste.Placeholder = "Paste a complete skill or instructions..."
	paste.ShowLineNumbers = false
	paste.CharLimit = int(config.MaxArtifactBytes)
	paste.SetWidth(72)
	paste.SetHeight(14)

	label := textinput.New()
	label.Prompt = "version: "
	label.Placeholder = "optional, e.g. v2"
	label.CharLimit = 120
	label.SetWidth(48)

	appStyles := newStyles(true)
	fixtures, _ := fixture.Catalog()
	fixtureOn := make(map[string]bool, len(fixtures))
	for _, value := range fixtures {
		fixtureOn[value.ID] = true
	}
	return Model{
		service:     service,
		config:      config,
		screen:      screenHome,
		dark:        true,
		styles:      appStyles,
		fileBrowser: filebrowser.New(startDir, fileBrowserStyles(appStyles)),
		paste:       paste,
		labelInput:  label,
		fixtures:    fixtures,
		fixtureOn:   fixtureOn,
		caseCount:   6,
	}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeInputs()
		return m, nil
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		m.styles = newStyles(m.dark)
		m.fileBrowser.SetStyles(fileBrowserStyles(m.styles))
		m.suiteReview.SetStyles(suiteReviewStyles(m.styles))
		return m, nil
	case filebrowser.SelectedMsg:
		updated, cmd, _ := m.inspectFile(msg.Path)
		return updated, cmd
	case filebrowser.CanceledMsg:
		m.screen = screenSource
		return m, nil
	case artifactMsg:
		if msg.err != nil {
			m.err, m.screen = msg.err, screenError
			return m, nil
		}
		m.artifact = msg.artifact
		m.labelInput.SetValue(msg.artifact.Label)
		m.inspectOffset, m.showBundle = 0, false
		m.screen, m.status = screenInspect, ""
		return m, nil
	case savedMsg:
		if msg.err != nil {
			m.err, m.screen = msg.err, screenError
			return m, nil
		}
		m.savedPath, m.screen = msg.path, screenSaved
		return m, nil
	case suiteMsg:
		if msg.err != nil {
			m.err, m.screen = msg.err, screenError
			return m, nil
		}
		m.draft = msg.draft
		m.suiteReview = suitereview.New(msg.draft, suiteReviewStyles(m.styles))
		m.suiteReview.SetSize(m.inputWidth(), max(6, m.canvasHeight()-8))
		m.screen, m.status = screenSuiteReview, ""
		return m, nil
	case suitereview.ContinueMsg:
		m.draft = msg.Draft
		m.returnTo, m.screen, m.status = screenSuiteReview, screenLoading, "Freezing test suite..."
		return m, func() tea.Msg {
			value, path, err := m.service.FreezeSuite(msg.Draft)
			return frozenSuiteMsg{value: value, path: path, err: err}
		}
	case frozenSuiteMsg:
		if msg.err != nil {
			m.err, m.screen = msg.err, screenError
			return m, nil
		}
		m.frozen, m.draft, m.suitePath = msg.value, msg.value.EditableDraft(), msg.path
		m.screen, m.status = screenSuiteReady, ""
		if msg.value.OriginArtifactSHA != m.currentArtifactHash() {
			m.returnTo, m.screen = screenSuiteChoice, screenSuiteReuseConfirm
		}
		return m, nil
	case suiteListMsg:
		if msg.err != nil {
			m.err, m.screen = msg.err, screenError
			return m, nil
		}
		m.suiteList, m.reuseCursor = msg.values, 0
		m.screen, m.status = screenSuiteReuse, ""
		return m, nil
	case suitereview.CanceledMsg:
		m.screen = screenSuiteConfig
		return m, nil
	}

	key, isKey := message.(tea.KeyPressMsg)
	if isKey && key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if isKey && m.width >= 40 && m.height >= 16 {
		if updated, cmd, handled := m.handleKey(key); handled {
			return updated, cmd
		}
	}

	var cmd tea.Cmd
	switch m.screen {
	case screenFile:
		m.fileBrowser, cmd = m.fileBrowser.Update(message)
	case screenPaste:
		m.paste, cmd = m.paste.Update(message)
	case screenInspect:
		if m.editingLabel {
			m.labelInput, cmd = m.labelInput.Update(message)
		}
	case screenSuiteReview:
		m.suiteReview, cmd = m.suiteReview.Update(message)
	}
	return m, cmd
}

func (m Model) handleKey(key tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	value := key.String()
	switch m.screen {
	case screenHome:
		if value == "q" {
			return m, tea.Quit, true
		}
		if value == "up" && m.homeCursor > 0 {
			m.homeCursor--
			return m, nil, true
		}
		if value == "down" && m.homeCursor < 3 {
			m.homeCursor++
			return m, nil, true
		}
		if value == "n" || value == "enter" && m.homeCursor == 0 {
			m.screen, m.status = screenSource, ""
			return m, nil, true
		}
		if value == "enter" {
			m.status = "This flow arrives in a later MVP stage."
			return m, nil, true
		}
	case screenSource:
		if value == "esc" {
			m.screen = screenHome
			return m, nil, true
		}
		if value == "up" || value == "down" {
			m.sourceCursor = 1 - m.sourceCursor
			return m, nil, true
		}
		if value == "enter" {
			if m.sourceCursor == 0 {
				m.fileBrowser.Reset()
				m.screen, m.status = screenFile, ""
				return m, nil, true
			}
			m.screen, m.discardArmed = screenPaste, false
			return m, m.paste.Focus(), true
		}
	case screenPaste:
		if value == "super+enter" {
			return m.inspectPaste()
		}
		if value == "esc" {
			if strings.TrimSpace(m.paste.Value()) == "" || m.discardArmed {
				m.paste.Blur()
				m.paste.Reset()
				m.screen, m.discardArmed, m.status = screenSource, false, ""
				return m, nil, true
			}
			m.discardArmed = true
			m.status = "Press esc again to discard the pasted text."
			return m, nil, true
		}
		m.discardArmed = false
		return m, nil, false
	case screenInspect:
		if m.editingLabel {
			switch value {
			case "enter":
				m.artifact.Label = strings.TrimSpace(m.labelInput.Value())
				m.editingLabel = false
				m.labelInput.Blur()
				return m, nil, true
			case "esc":
				m.labelInput.SetValue(m.artifact.Label)
				m.editingLabel = false
				m.labelInput.Blur()
				return m, nil, true
			}
			return m, nil, false
		}
		switch value {
		case "esc":
			m.screen = screenSource
			return m, nil, true
		case "e":
			if m.artifact.Source == "stdin" {
				return m, nil, true
			}
			m.editingLabel = true
			return m, m.labelInput.Focus(), true
		case "b":
			if m.artifact.Source == "stdin" {
				return m, nil, true
			}
			m.showBundle = !m.showBundle
			m.inspectOffset = 0
			return m, nil, true
		case "up":
			if m.inspectOffset > 0 {
				m.inspectOffset--
			}
			return m, nil, true
		case "down":
			m.inspectOffset++
			return m, nil, true
		case "enter":
			if m.artifact.HasBlockingFindings() {
				m.status = "Fix blocking issues before saving."
				return m, nil, true
			}
			m.returnTo, m.screen = screenInspect, screenLoading
			return m, func() tea.Msg {
				path, err := m.service.SaveArtifact(m.artifact)
				return savedMsg{path: path, err: err}
			}, true
		}
	case screenSaved:
		if value == "enter" {
			m.screen, m.suiteCursor, m.status = screenSuiteChoice, 0, ""
			return m, nil, true
		}
		if value == "h" || value == "esc" {
			m.screen, m.artifact, m.savedPath, m.status = screenHome, model.Artifact{}, "", ""
			return m, nil, true
		}
		if value == "q" {
			return m, tea.Quit, true
		}
	case screenSuiteChoice:
		switch value {
		case "esc":
			m.screen, m.status = screenSaved, ""
			return m, nil, true
		case "up":
			m.suiteCursor = max(0, m.suiteCursor-1)
			return m, nil, true
		case "down":
			m.suiteCursor = min(1, m.suiteCursor+1)
			return m, nil, true
		case "enter":
			if m.suiteCursor == 0 {
				m.screen, m.configCursor, m.status = screenSuiteConfig, 0, ""
			} else {
				m.returnTo, m.screen, m.status = screenSuiteChoice, screenLoading, "Loading frozen suites..."
				return m, func() tea.Msg {
					values, err := m.service.ListSuites()
					return suiteListMsg{values: values, err: err}
				}, true
			}
			return m, nil, true
		}
	case screenSuiteConfig:
		last := len(m.fixtures) + 1
		switch value {
		case "esc":
			m.screen, m.status = screenSuiteChoice, ""
			return m, nil, true
		case "up":
			m.configCursor = max(0, m.configCursor-1)
			return m, nil, true
		case "down":
			m.configCursor = min(last, m.configCursor+1)
			return m, nil, true
		case "left":
			if m.configCursor == 0 {
				m.caseCount = max(1, m.caseCount-1)
			}
			return m, nil, true
		case "right":
			if m.configCursor == 0 {
				m.caseCount = min(12, m.caseCount+1)
			}
			return m, nil, true
		case " ", "space":
			if m.configCursor > 0 && m.configCursor <= len(m.fixtures) {
				id := m.fixtures[m.configCursor-1].ID
				m.fixtureOn[id] = !m.fixtureOn[id]
			}
			return m, nil, true
		case "enter":
			if m.configCursor > 0 && m.configCursor <= len(m.fixtures) {
				id := m.fixtures[m.configCursor-1].ID
				m.fixtureOn[id] = !m.fixtureOn[id]
				return m, nil, true
			}
			if m.configCursor == last {
				fixtureIDs := m.selectedFixtureIDs()
				if len(fixtureIDs) == 0 {
					m.status = "Select at least one fixture."
					return m, nil, true
				}
				m.returnTo, m.screen, m.status = screenSuiteConfig, screenLoading, "Generating test suite..."
				return m, func() tea.Msg {
					draft, err := m.service.GenerateSuite(context.Background(), m.artifact, m.caseCount, fixtureIDs)
					return suiteMsg{draft: draft, err: err}
				}, true
			}
		}
	case screenSuiteReuse:
		if value == "esc" {
			m.screen = screenSuiteChoice
			return m, nil, true
		}
		if len(m.suiteList) == 0 {
			return m, nil, true
		}
		switch value {
		case "up":
			m.reuseCursor = max(0, m.reuseCursor-1)
			return m, nil, true
		case "down":
			m.reuseCursor = min(len(m.suiteList)-1, m.reuseCursor+1)
			return m, nil, true
		case "e":
			m.frozen = m.suiteList[m.reuseCursor]
			m.suitePath = ""
			m.draft = m.frozen.EditableDraft()
			m.suiteReview = suitereview.New(m.draft, suiteReviewStyles(m.styles))
			m.suiteReview.SetSize(m.inputWidth(), max(6, m.canvasHeight()-8))
			m.screen = screenSuiteReview
			return m, nil, true
		case "enter":
			m.frozen = m.suiteList[m.reuseCursor]
			m.suitePath = ""
			m.draft = m.frozen.EditableDraft()
			m.returnTo, m.screen = screenSuiteReuse, screenSuiteReuseConfirm
			return m, nil, true
		}
	case screenSuiteReuseConfirm:
		if value == "esc" {
			m.screen = m.returnTo
			return m, nil, true
		}
		if value == "enter" {
			m.screen, m.status = screenSuiteReady, ""
			return m, nil, true
		}
	case screenSuiteReady:
		if value == "esc" {
			m.screen = screenSuiteChoice
			return m, nil, true
		}
		if value == "enter" {
			m.status = "Execution planning is the next feature in this stage."
			return m, nil, true
		}
	case screenError:
		if value == "esc" || value == "enter" {
			m.screen, m.err = m.returnTo, nil
			return m, nil, true
		}
	}
	return m, nil, false
}

func (m Model) inspectFile(path string) (Model, tea.Cmd, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		m.status = "Enter a skill file or folder."
		return m, nil, true
	}
	m.returnTo, m.screen, m.status = screenFile, screenLoading, "Reviewing input..."
	return m, func() tea.Msg {
		artifact, err := m.service.InspectFile(path, "")
		return artifactMsg{artifact: artifact, err: err}
	}, true
}

func (m Model) inspectPaste() (Model, tea.Cmd, bool) {
	content := []byte(m.paste.Value())
	m.returnTo, m.screen, m.status = screenPaste, screenLoading, "Reviewing pasted text..."
	return m, func() tea.Msg {
		artifact, err := m.service.InspectPaste(content, "")
		return artifactMsg{artifact: artifact, err: err}
	}, true
}

func (m *Model) resizeInputs() {
	inputWidth := m.inputWidth()
	m.fileBrowser.SetSize(inputWidth, max(5, m.canvasHeight()-11))
	m.labelInput.SetWidth(min(48, inputWidth))
	m.paste.SetWidth(inputWidth)
	m.paste.SetHeight(max(5, min(14, m.canvasHeight()-9)))
	m.suiteReview.SetSize(inputWidth, max(6, m.canvasHeight()-8))
}

func (m Model) View() tea.View {
	content := m.render()
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "mdbench"
	return view
}

func (m Model) render() string {
	width, height := m.width, m.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
	if width < 40 || height < 16 {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, "mdbench needs at least 40x16.\nResize the terminal to continue.")
	}

	canvasWidth, canvasHeight := m.canvasWidth(), m.canvasHeight()
	header := m.styles.header.Width(canvasWidth - 4).Render("mdbench  " + m.flowName())
	bodyHeight := max(5, canvasHeight-lipgloss.Height(header)-4)
	body := m.styles.box.Width(canvasWidth - 4).Height(bodyHeight).Render(m.body(bodyHeight - 2))
	footer := m.styles.footer.Width(canvasWidth - 2).Render(m.footer())
	frame := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, frame)
}

func (m Model) body(available int) string {
	switch m.screen {
	case screenHome:
		options := []string{"New evaluation", "Compare runs", "Saved runs", "Settings"}
		lines := []string{m.styles.accent.Render("Evaluate Markdown instructions with repeatable evidence."), ""}
		for index, option := range options {
			prefix := "  "
			style := m.styles.text
			if index == m.homeCursor {
				prefix, style = "> ", m.styles.selected
			}
			if index > 0 {
				option += "  " + m.styles.muted.Render("coming later")
			}
			lines = append(lines, prefix+style.Render(option))
		}
		if m.status != "" {
			lines = append(lines, "", m.styles.muted.Render(m.status))
		}
		return strings.Join(lines, "\n")
	case screenSource:
		return m.selectionBody("Choose input method", []string{"File path", "Paste text"}, m.sourceCursor)
	case screenFile:
		return strings.Join([]string{m.styles.accent.Render("Browse files"), m.styles.muted.Render(m.fileBrowser.CurrentDirectory()), "", m.fileBrowser.View()}, "\n")
	case screenPaste:
		return strings.Join([]string{m.styles.accent.Render("Paste skill or instructions"), m.styles.muted.Render("Paste a complete skill file or instructions only."), "", m.paste.View(), m.styles.warning.Render(m.status)}, "\n")
	case screenLoading:
		return lipgloss.Place(m.canvasWidth()-6, available, lipgloss.Center, lipgloss.Center, m.styles.accent.Render(m.status))
	case screenInspect:
		return m.inspectBody(available)
	case screenSaved:
		return strings.Join([]string{
			m.styles.success.Render("Input saved"), "", m.savedPath, "", m.styles.muted.Render("Continue to choose or generate a test suite."),
		}, "\n")
	case screenSuiteChoice:
		return m.suiteChoiceBody()
	case screenSuiteConfig:
		return m.suiteConfigBody()
	case screenSuiteReview:
		return m.suiteReview.View()
	case screenSuiteReuse:
		return m.suiteReuseBody()
	case screenSuiteReuseConfirm:
		return m.suiteReuseConfirmBody()
	case screenSuiteReady:
		return m.suiteReadyBody()
	case screenError:
		message := "Unknown error"
		if m.err != nil {
			message = m.err.Error()
		}
		return strings.Join([]string{m.styles.error.Render("Unable to continue"), "", message, "", m.styles.muted.Render("Enter or Esc returns to the previous screen.")}, "\n")
	default:
		return ""
	}
}

func (m Model) inspectBody(available int) string {
	artifact := m.artifact
	lines := []string{
		m.styles.accent.Render("Review input"),
		fmt.Sprintf("input        %s", artifact.Source),
		fmt.Sprintf("text hash    %s", shortHash(artifact.ContentSHA)),
		fmt.Sprintf("files hash   %s  %d file(s)", shortHash(artifact.BundleSHA), len(artifact.Files)),
		fmt.Sprintf("size         %d bytes, %d words, %d headings", artifact.Metrics.Bytes, artifact.Metrics.Words, artifact.Metrics.Headings),
	}
	if artifact.Source != "stdin" {
		if m.editingLabel {
			lines = append(lines, m.labelInput.View())
		} else {
			label := artifact.Label
			if label == "" {
				label = m.styles.muted.Render("none")
			}
			lines = append(lines, "version      "+label)
		}
	}
	lines = append(lines, "")
	if m.showBundle {
		lines = append(lines, m.styles.accent.Render("Bundled files"))
		for _, file := range artifact.Files {
			lines = append(lines, fmt.Sprintf("  %s  %d bytes", file.Path, file.Size))
		}
	} else {
		lines = append(lines, m.styles.accent.Render("Checks"))
		if len(artifact.Findings) == 0 {
			lines = append(lines, m.styles.success.Render("  No issues found"))
		}
		for _, finding := range artifact.Findings {
			style := m.styles.muted
			if finding.Severity == model.SeverityError {
				style = m.styles.error
			} else if finding.Severity == model.SeverityWarning {
				style = m.styles.warning
			}
			location := finding.Path
			if finding.Line > 0 {
				location += fmt.Sprintf(":%d", finding.Line)
			}
			lines = append(lines, style.Render(fmt.Sprintf("  %-7s %s", finding.Severity, finding.Message)), m.styles.muted.Render("          "+location+"  "+finding.Hint))
		}
	}
	if m.status != "" {
		lines = append(lines, "", m.styles.warning.Render(m.status))
	}
	visible := max(1, available)
	maxOffset := max(0, len(lines)-visible)
	offset := min(m.inspectOffset, maxOffset)
	return strings.Join(lines[offset:min(len(lines), offset+visible)], "\n")
}

func (m Model) selectionBody(title string, options []string, cursor int) string {
	lines := []string{m.styles.accent.Render(title), ""}
	for index, option := range options {
		if index == cursor {
			lines = append(lines, "> "+m.styles.selected.Render(option))
		} else {
			lines = append(lines, "  "+option)
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) footer() string {
	switch m.screen {
	case screenHome:
		return "↑/↓ move   enter select   q quit"
	case screenSource:
		return "↑/↓ move   enter select   esc back"
	case screenFile:
		return m.fileBrowser.Footer()
	case screenPaste:
		return "enter newline  cmd+enter review  esc discard"
	case screenInspect:
		if m.editingLabel {
			return "enter save version   esc cancel"
		}
		if m.artifact.Source == "stdin" {
			return "enter continue   esc back"
		}
		return "enter continue   b files   e version   esc back"
	case screenSaved:
		return "enter continue   h home   q quit"
	case screenSuiteChoice:
		return "↑↓ move  enter select  esc input"
	case screenSuiteConfig:
		return "↑↓ move  ←→ cases  space toggle  enter select  esc back"
	case screenSuiteReview:
		return m.suiteReview.Footer()
	case screenSuiteReuse:
		return "↑↓ move  enter reuse  e revise  esc back"
	case screenSuiteReuseConfirm:
		return "enter confirm relevance  esc cancel"
	case screenSuiteReady:
		return "enter continue  esc suites"
	case screenError:
		return "enter/esc back"
	default:
		return "ctrl+c quit"
	}
}

func (m Model) flowName() string {
	switch m.screen {
	case screenHome:
		return "home"
	case screenSource, screenFile, screenPaste:
		return "new evaluation / input"
	case screenInspect:
		return "new evaluation / review"
	case screenSaved:
		return "new evaluation / input saved"
	case screenSuiteChoice, screenSuiteConfig, screenSuiteReuse, screenSuiteReuseConfirm:
		return "new evaluation / tests"
	case screenSuiteReview, screenSuiteReady:
		return "new evaluation / review tests"
	case screenError:
		return "error"
	default:
		return "working"
	}
}

func (m Model) canvasWidth() int {
	width := m.width
	if width <= 0 {
		return 80
	}
	if width >= 80 {
		return 80
	}
	if width >= 50 {
		return 50
	}
	return width
}

func (m Model) canvasHeight() int {
	height := m.height
	if height <= 0 {
		return 24
	}
	return min(30, height)
}

func (m Model) inputWidth() int { return max(20, m.canvasWidth()-8) }

func (m Model) selectedFixtureIDs() []string {
	result := []string{}
	for _, value := range m.fixtures {
		if m.fixtureOn[value.ID] {
			result = append(result, value.ID)
		}
	}
	return result
}

func (m Model) currentArtifactHash() string {
	if m.artifact.EffectiveSHA != "" {
		return m.artifact.EffectiveSHA
	}
	return m.artifact.ContentSHA
}

func (m Model) suiteChoiceBody() string {
	options := []string{"Generate new suite", "Reuse frozen suite"}
	lines := []string{m.styles.accent.Render("Choose test suite"), m.styles.muted.Render("Generate a reviewable draft or reuse the exact tests from an earlier run."), ""}
	for index, option := range options {
		prefix, style := "  ", m.styles.text
		if index == m.suiteCursor {
			prefix, style = "> ", m.styles.selected
		}
		lines = append(lines, prefix+style.Render(option))
	}
	if m.status != "" {
		lines = append(lines, "", m.styles.warning.Render(m.status))
	}
	return strings.Join(lines, "\n")
}

func (m Model) suiteConfigBody() string {
	lines := []string{m.styles.accent.Render("Generate test suite"), m.styles.muted.Render("Choose the draft size and allowed starter workspaces."), ""}
	caseLine := fmt.Sprintf("  Cases                  %d", m.caseCount)
	if m.configCursor == 0 {
		caseLine = m.styles.selected.Render(">" + caseLine[1:])
	}
	lines = append(lines, caseLine, "  Fixtures")
	for index, value := range m.fixtures {
		mark := "[ ]"
		if m.fixtureOn[value.ID] {
			mark = "[x]"
		}
		line := fmt.Sprintf("    %s %-20s %s", mark, value.Name, shortHash(value.ContentSHA))
		if m.configCursor == index+1 {
			line = m.styles.selected.Render("  >" + line[3:])
		}
		lines = append(lines, line)
	}
	continueLine := "  Generate draft"
	if m.configCursor == len(m.fixtures)+1 {
		continueLine = m.styles.selected.Render("> Generate draft")
	}
	lines = append(lines, "", continueLine)
	if m.status != "" {
		lines = append(lines, "", m.styles.warning.Render(m.status))
	}
	return strings.Join(lines, "\n")
}

func (m Model) suiteReuseBody() string {
	lines := []string{m.styles.accent.Render("Reuse frozen suite"), m.styles.muted.Render("Enter reuses the exact revision. E creates a new editable revision."), ""}
	if len(m.suiteList) == 0 {
		return strings.Join(append(lines, m.styles.muted.Render("No frozen suites yet.")), "\n")
	}
	rows := max(1, m.canvasHeight()-14)
	start := max(0, m.reuseCursor-rows+1)
	end := min(len(m.suiteList), start+rows)
	for index := start; index < end; index++ {
		value := m.suiteList[index]
		line := fmt.Sprintf("  %s  r%d  %d cases  %s", value.ID, value.Revision, enabledCases(value.Draft), shortHash(value.ContentSHA))
		if index == m.reuseCursor {
			line = m.styles.selected.Render(">" + line[1:])
		}
		lines = append(lines, line)
	}
	selected := m.suiteList[m.reuseCursor]
	lines = append(lines, "", m.styles.accent.Render("Applicability"), lipgloss.NewStyle().Width(m.inputWidth()).Render(selected.Applicability), m.styles.muted.Render("origin  "+shortHash(selected.OriginArtifactSHA)))
	return strings.Join(lines, "\n")
}

func (m Model) suiteReuseConfirmBody() string {
	match := m.frozen.OriginArtifactSHA == m.currentArtifactHash()
	compatibility := m.styles.success.Render("Origin matches this input.")
	if !match {
		compatibility = m.styles.warning.Render("Origin differs from this input. Confirm every case is still relevant.")
	}
	return strings.Join([]string{
		m.styles.accent.Render("Confirm suite relevance"), "",
		fmt.Sprintf("suite    %s  r%d", m.frozen.ID, m.frozen.Revision),
		"origin   " + shortHash(m.frozen.OriginArtifactSHA),
		"input    " + shortHash(m.currentArtifactHash()), "", compatibility, "",
		lipgloss.NewStyle().Width(m.inputWidth()).Render(m.frozen.Applicability),
	}, "\n")
}

func (m Model) suiteReadyBody() string {
	title := "Suite ready"
	if m.suitePath != "" {
		title = "Suite frozen"
	}
	lines := []string{
		m.styles.success.Render(title), "",
		fmt.Sprintf("suite        %s", m.frozen.ID),
		fmt.Sprintf("revision     %d", m.frozen.Revision),
		fmt.Sprintf("suite hash   %s", shortHash(m.frozen.ContentSHA)),
		fmt.Sprintf("cases        %d enabled", enabledCases(m.frozen.Draft)),
	}
	if m.suitePath != "" {
		lines = append(lines, "", m.styles.muted.Render(m.suitePath))
	}
	if m.status != "" {
		lines = append(lines, "", m.styles.warning.Render(m.status))
	}
	return strings.Join(lines, "\n")
}

func enabledCases(draft suite.Draft) int {
	count := 0
	for _, testCase := range draft.Cases {
		if testCase.Enabled {
			count++
		}
	}
	return count
}

func newStyles(dark bool) styles {
	text, muted, border, accent, success, warning, failure := "#1B1F23", "#667078", "#D7DBDF", "#D94F00", "#1A7F37", "#9A6700", "#CF222E"
	if dark {
		text, muted, border, accent, success, warning, failure = "#F2F4F5", "#889096", "#3A3F42", "#FF6A00", "#2EA043", "#D29922", "#F85149"
	}
	return styles{
		text:     lipgloss.NewStyle().Foreground(lipgloss.Color(text)),
		muted:    lipgloss.NewStyle().Foreground(lipgloss.Color(muted)),
		accent:   lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true),
		warning:  lipgloss.NewStyle().Foreground(lipgloss.Color(warning)),
		error:    lipgloss.NewStyle().Foreground(lipgloss.Color(failure)),
		success:  lipgloss.NewStyle().Foreground(lipgloss.Color(success)),
		box:      lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(border)).Padding(0, 1),
		header:   lipgloss.NewStyle().Foreground(lipgloss.Color(text)).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(border)).Padding(0, 1).Bold(true),
		footer:   lipgloss.NewStyle().Foreground(lipgloss.Color(muted)).Padding(0, 1),
		selected: lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true),
	}
}

func fileBrowserStyles(value styles) filebrowser.Styles {
	return filebrowser.Styles{Text: value.text, Muted: value.muted, Selected: value.selected}
}

func suiteReviewStyles(value styles) suitereview.Styles {
	return suitereview.Styles{Text: value.text, Muted: value.muted, Selected: value.selected, Accent: value.accent, Warning: value.warning}
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}
