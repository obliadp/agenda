package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/obliadp/agenda/internal/config"
)

const (
	tabBarHeight = 2 // tab labels + bottom border
	footerHeight = 1
	// Percent of width given to the preview pane. Two-line list rows give the
	// title its own line, so the list column can be narrower and the preview
	// gets the larger share.
	previewRatio = 50
)

// Model is agenda's root Bubble Tea model: chrome around a set of views.
type Model struct {
	cfg     config.Config
	keys    globalKeys
	theme   theme
	views   []View
	current int

	width, height int
	ready         bool

	// preview scrolling, owned centrally so it works the same in every view.
	previewScroll int
	previewKey    string
}

// New builds the root model from config. Views are constructed by the caller
// (main) and passed in, so the tui package doesn't import every view package.
func New(cfg config.Config, views []View) Model {
	return Model{
		cfg:   cfg,
		keys:  defaultKeys(),
		theme: defaultTheme(),
		views: views,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.views))
	for _, v := range m.views {
		cmds = append(cmds, v.Init())
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case tea.KeyMsg:
		// While the focused view is capturing text input, route everything to
		// it (except a hard ctrl+c quit) so global bindings don't steal keys.
		if len(m.views) > 0 && m.views[m.current].InputActive() {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m.updateCurrent(msg)
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.NextView):
			m.current = (m.current + 1) % len(m.views)
			m.syncPreviewKey(true)
			return m, nil
		case key.Matches(msg, m.keys.PrevView):
			m.current = (m.current - 1 + len(m.views)) % len(m.views)
			m.syncPreviewKey(true)
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			return m, m.views[m.current].Init()
		case key.Matches(msg, m.keys.PreviewUp):
			m.scrollPreview(-1)
			return m, nil
		case key.Matches(msg, m.keys.PreviewDown):
			m.scrollPreview(1)
			return m, nil
		case key.Matches(msg, m.keys.PreviewPgUp):
			m.scrollPreview(-(m.contentHeight() - 2))
			return m, nil
		case key.Matches(msg, m.keys.PreviewPgDn):
			m.scrollPreview(m.contentHeight() - 2)
			return m, nil
		}
		// Anything else goes to the focused view.
		return m.updateCurrent(msg)
	}

	// Non-key messages (data-fetch results, spinner ticks) are broadcast to
	// every view; each ignores messages that aren't its own.
	return m.broadcast(msg)
}

// updateCurrent threads a message through only the focused view.
func (m Model) updateCurrent(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.views) == 0 {
		return m, nil
	}
	cmd := m.views[m.current].Update(msg)
	m.syncPreviewKey(false) // a key may have moved the selection
	return m, cmd
}

// broadcast threads a message through every view, collecting their commands.
func (m Model) broadcast(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, len(m.views))
	for _, v := range m.views {
		if cmd := v.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	m.syncPreviewKey(false) // a data load may have changed the selection
	return m, tea.Batch(cmds...)
}

// syncPreviewKey resets the preview scroll to the top when the selected item
// changes (or always, when force is set, e.g. on a view switch).
func (m *Model) syncPreviewKey(force bool) {
	if len(m.views) == 0 {
		return
	}
	k := m.views[m.current].PreviewKey()
	if force || k != m.previewKey {
		m.previewKey = k
		m.previewScroll = 0
	}
}

// scrollPreview moves the preview offset by delta lines, clamped to content.
func (m *Model) scrollPreview(delta int) {
	lines := strings.Count(m.views[m.current].PreviewView(), "\n") + 1
	maxOff := max(0, lines-m.contentHeight())
	m.previewScroll = clamp(m.previewScroll+delta, 0, maxOff)
}

func (m Model) contentHeight() int {
	return max(1, m.height-tabBarHeight-footerHeight)
}

// layout recomputes per-view sizes after a resize.
func (m *Model) layout() {
	if !m.ready {
		return
	}
	contentH := max(1, m.height-tabBarHeight-footerHeight)
	previewPane := m.width * previewRatio / 100
	listW := m.width - previewPane
	// Preview style overhead: 1 border column + 2 left padding.
	previewContentW := max(1, previewPane-3)
	for _, v := range m.views {
		v.SetSize(listW, previewContentW, contentH)
	}
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	if !m.ready || len(m.views) == 0 {
		v.Content = "Loading agenda…"
		return v
	}

	contentH := max(1, m.height-tabBarHeight-footerHeight)
	cur := m.views[m.current]

	// Clip each pane to the content height so tall content can't overflow and
	// push the footer off-screen. The preview is clipped from the scroll
	// offset so it can be scrolled; the list manages its own window.
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		clipFrom(cur.ListView(), 0, contentH),
		m.theme.preview.Height(contentH).Render(clipFrom(cur.PreviewView(), m.previewScroll, contentH)),
	)

	v.Content = lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderTabs(),
		body,
		m.renderFooter(),
	)
	return v
}

// clipFrom returns at most n lines of s starting at line offset, so a pane
// can't overflow its row budget. offset enables scrolling.
func clipFrom(s string, offset, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	offset = clamp(offset, 0, len(lines))
	lines = lines[offset:]
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

func (m Model) renderTabs() string {
	labels := make([]string, len(m.views))
	for i, v := range m.views {
		style := m.theme.tabInactive
		if i == m.current {
			style = m.theme.tabActive
		}
		labels[i] = style.Render(v.Title())
	}
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, labels...)
	return m.theme.tabBar.Width(m.width).Render(row)
}

func (m Model) renderFooter() string {
	var b strings.Builder

	// View-specific bindings first, then the global ones.
	bindings := append(m.views[m.current].Bindings(),
		m.keys.NextView, m.keys.PreviewUp, m.keys.Refresh, m.keys.Quit)

	first := true
	for _, bnd := range bindings {
		h := bnd.Help()
		if h.Key == "" {
			continue
		}
		if !first {
			b.WriteString(m.theme.footerSep.String())
		}
		first = false
		b.WriteString(m.theme.footerKey.Render(h.Key))
		b.WriteString(" ")
		b.WriteString(m.theme.footerDesc.Render(h.Desc))
	}

	left := b.String()
	status := m.views[m.current].Status()
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(status))
	return m.theme.footer.Width(m.width).Render(
		left + strings.Repeat(" ", gap) + status,
	)
}
