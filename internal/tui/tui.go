// Package tui hosts the ruust bubbletea dashboard, "the nest": a nocturnal
// ember interactive view over the caller's Eggs.
//
// The programme opens on a loading state while it fetches the caller's Eggs,
// then shows a styled list. Pressing enter on an Egg opens a detail pane with
// its domains, env var keys (masked), latest deployment, and a scrollable log
// viewport. Everything is rendered through the shared ui ember vocabulary and
// responds to terminal resize.
//
// Customer-facing vocabulary is used throughout: the unit is an Egg (never a
// Blob), and its lifecycle states are incubating, hatching, hatched, cold, and
// cracked.
package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RuustRun/cli/internal/api"
	"github.com/RuustRun/cli/internal/ui"
)

// screen is the pane the dashboard is currently showing.
type screen int

const (
	screenLoading screen = iota // fetching the Egg list
	screenList                  // the styled list of Eggs
	screenDetail                // a single Egg's detail pane
	screenLogs                  // the scrollable log viewport for an Egg
	screenError                 // a fatal error (for example not signed in)
)

// keyMap is the set of bindings the dashboard responds to. It doubles as the
// source of truth for the footer help bar.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Open    key.Binding
	Back    key.Binding
	Logs    key.Binding
	Refresh key.Binding
	Quit    key.Binding
}

// defaultKeys builds the standard binding set.
func defaultKeys() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Open: key.NewBinding(
			key.WithKeys("enter", "l", "right"),
			key.WithHelp("enter", "open"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "h", "left"),
			key.WithHelp("esc", "back"),
		),
		Logs: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "logs"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// model is the root bubbletea model for the nest.
type model struct {
	client *api.Client
	keys   keyMap

	width  int
	height int

	screen  screen
	spinner spinner.Model

	// list state
	eggs   []api.Egg
	cursor int

	// detail state
	detail    *api.EggDetail
	detailErr string

	// logs state
	logs      []api.LogLine
	logsErr   string
	viewport  viewport.Model
	vpReady   bool
	loadingID string // id being fetched for detail or logs

	// fatal error state (for example not signed in)
	fatalErr  string
	loginHint bool
}

// eggsLoadedMsg carries the result of the initial Egg list fetch.
type eggsLoadedMsg struct {
	eggs []api.Egg
	err  error
}

// detailLoadedMsg carries the result of a GetEgg fetch.
type detailLoadedMsg struct {
	detail api.EggDetail
	err    error
}

// logsLoadedMsg carries the result of a Logs fetch.
type logsLoadedMsg struct {
	lines []api.LogLine
	err   error
}

// Run launches the interactive dashboard against the given API client.
func Run(client *api.Client) error {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ui.ColourEmber)

	m := model{
		client:  client,
		keys:    defaultKeys(),
		screen:  screenLoading,
		spinner: sp,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init kicks off the spinner and the initial Egg list fetch.
func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadEggs())
}

// loadEggs returns a Cmd that fetches the caller's Eggs.
func (m model) loadEggs() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		eggs, err := client.ListEggs()
		return eggsLoadedMsg{eggs: eggs, err: err}
	}
}

// loadDetail returns a Cmd that fetches a single Egg's detail.
func (m model) loadDetail(id string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		d, err := client.GetEgg(id)
		return detailLoadedMsg{detail: d, err: err}
	}
}

// loadLogs returns a Cmd that fetches an Egg's logs.
func (m model) loadLogs(id string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Logs(id)
		return logsLoadedMsg{lines: res.Lines, err: err}
	}
}

// Update is the root reducer. It routes window resizes and load results, then
// delegates key handling to the per-screen handlers.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case eggsLoadedMsg:
		if msg.err != nil {
			m.screen = screenError
			m.fatalErr = msg.err.Error()
			m.loginHint = isAuthError(msg.err)
			return m, nil
		}
		m.eggs = msg.eggs
		if m.cursor >= len(m.eggs) {
			m.cursor = 0
		}
		m.screen = screenList
		return m, nil

	case detailLoadedMsg:
		if msg.err != nil {
			m.detail = nil
			m.detailErr = msg.err.Error()
			if isAuthError(msg.err) {
				m.screen = screenError
				m.fatalErr = msg.err.Error()
				m.loginHint = true
				return m, nil
			}
			m.screen = screenDetail
			return m, nil
		}
		d := msg.detail
		m.detail = &d
		m.detailErr = ""
		m.screen = screenDetail
		return m, nil

	case logsLoadedMsg:
		if msg.err != nil {
			m.logs = nil
			m.logsErr = msg.err.Error()
		} else {
			m.logs = msg.lines
			m.logsErr = ""
		}
		m.screen = screenLogs
		m.ensureViewport()
		m.viewport.SetContent(m.renderLogBody())
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward everything else to the viewport when it is on screen.
	if m.screen == screenLogs && m.vpReady {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// handleKey routes keystrokes to the active screen.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Quit is global.
	if key.Matches(msg, m.keys.Quit) {
		// On the list, q quits. Elsewhere ctrl+c still quits but esc goes back;
		// esc is handled per screen below, so only honour the Quit binding
		// keys here.
		return m, tea.Quit
	}

	switch m.screen {
	case screenError:
		// Any key besides quit refreshes an attempt to reload.
		if key.Matches(msg, m.keys.Refresh) {
			m.screen = screenLoading
			m.fatalErr = ""
			return m, tea.Batch(m.spinner.Tick, m.loadEggs())
		}
		return m, nil

	case screenList:
		return m.handleListKey(msg)

	case screenDetail:
		if key.Matches(msg, m.keys.Back) {
			m.screen = screenList
			m.detail = nil
			m.detailErr = ""
			return m, nil
		}
		if key.Matches(msg, m.keys.Logs) && m.detail != nil {
			m.loadingID = m.detail.ID
			m.screen = screenLoading
			return m, tea.Batch(m.spinner.Tick, m.loadLogs(m.detail.ID))
		}
		return m, nil

	case screenLogs:
		if key.Matches(msg, m.keys.Back) {
			m.screen = screenDetail
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleListKey drives navigation on the Egg list.
func (m model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.eggs)-1 {
			m.cursor++
		}
	case key.Matches(msg, m.keys.Refresh):
		m.screen = screenLoading
		return m, tea.Batch(m.spinner.Tick, m.loadEggs())
	case key.Matches(msg, m.keys.Open):
		if len(m.eggs) == 0 {
			return m, nil
		}
		id := m.eggs[m.cursor].ID
		m.loadingID = id
		m.detail = nil
		m.detailErr = ""
		m.screen = screenLoading
		return m, tea.Batch(m.spinner.Tick, m.loadDetail(id))
	}
	return m, nil
}

// resizeViewport recomputes the log viewport dimensions on resize.
func (m *model) resizeViewport() {
	w, h := m.viewportSize()
	if !m.vpReady {
		m.viewport = viewport.New(w, h)
		m.vpReady = true
	} else {
		m.viewport.Width = w
		m.viewport.Height = h
	}
}

// ensureViewport guarantees the viewport exists before content is set, even if
// a WindowSizeMsg has not arrived yet.
func (m *model) ensureViewport() {
	if !m.vpReady {
		w, h := m.viewportSize()
		m.viewport = viewport.New(w, h)
		m.vpReady = true
	}
}

// viewportSize returns the inner width and height available to the log body,
// leaving room for the header, log frame and footer.
func (m model) viewportSize() (int, int) {
	w := m.width - 6
	if w < 20 {
		w = 20
	}
	h := m.height - 9
	if h < 3 {
		h = 3
	}
	return w, h
}

// View renders the active screen.
func (m model) View() string {
	if m.width == 0 {
		// Before the first WindowSizeMsg, render a compact loading note.
		return ui.Logo() + " " + m.spinner.View() + " " +
			ui.Subtle.Render("warming the nest...")
	}

	switch m.screen {
	case screenLoading:
		return m.loadingView()
	case screenError:
		return m.errorView()
	case screenList:
		return m.listView()
	case screenDetail:
		return m.detailView()
	case screenLogs:
		return m.logsView()
	}
	return ""
}

// loadingView shows the ember logo and a spinner while data is in flight.
func (m model) loadingView() string {
	verb := "gathering your Eggs"
	if m.loadingID != "" {
		verb = "peering into the nest"
	}
	body := lipgloss.JoinVertical(lipgloss.Center,
		ui.Logo(),
		"",
		m.spinner.View()+" "+ui.Subtle.Render(verb+"..."),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

// errorView shows a fatal error, with a 'ruust login' nudge when unauthorised.
func (m model) errorView() string {
	crack := ui.StateStyle("cracked").Render("a crack in the shell")
	lines := []string{
		ui.Logo(),
		"",
		crack,
		"",
		ui.Bone.Render(m.fatalErr),
	}
	if m.loginHint {
		lines = append(lines,
			"",
			ui.Subtle.Render("Run ")+ui.Key.Render("ruust login")+
				ui.Subtle.Render(" to sign in, then open the nest again."),
		)
	}
	lines = append(lines, "", ui.Subtle.Render("press ")+ui.Key.Render("r")+
		ui.Subtle.Render(" to try again, ")+ui.Key.Render("q")+ui.Subtle.Render(" to quit"))

	body := lipgloss.JoinVertical(lipgloss.Center, lines...)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

// isAuthError reports whether an error looks like a 401 / not-signed-in error.
// The api package appends "(run 'ruust login')" to auth errors, which we key on
// as well as common unauthorised phrasing.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "ruust login") ||
		strings.Contains(s, "not signed in") ||
		strings.Contains(s, "unauthorised") ||
		strings.Contains(s, "unauthorized")
}

// footer renders the help bar for a set of bindings in the ember vocabulary.
func (m model) footer(bindings []key.Binding) string {
	sep := ui.Subtle.Render("  •  ")
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		h := b.Help()
		parts = append(parts, ui.Key.Render(h.Key)+" "+ui.Subtle.Render(h.Desc))
	}
	bar := strings.Join(parts, sep)
	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 2).
		Foreground(ui.ColourMuted).
		Render(bar)
}
