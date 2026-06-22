package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/obliadp/agenda/internal/config"
	"github.com/obliadp/agenda/internal/ui"
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

	// cross-reference picker (nil unless the modal is open).
	picker     *ui.Picker
	pickerRefs []ui.Ref
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
		// While the cross-reference picker is open it captures all keys.
		if m.picker != nil {
			switch m.picker.Update(msg) {
			case ui.PickerCancel:
				m.picker, m.pickerRefs = nil, nil
			case ui.PickerConfirm:
				ref := m.pickerRefs[m.picker.Index()]
				m.picker, m.pickerRefs = nil, nil
				return m, m.followRef(ref)
			case ui.PickerOpenURL:
				// Open the selected ref in the browser, where it has a URL.
				if ref := m.pickerRefs[m.picker.Index()]; ref.URL != "" {
					m.picker, m.pickerRefs = nil, nil
					return m, ui.OpenURL(ref.URL)
				}
			}
			return m, nil
		}
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
		case key.Matches(msg, m.keys.Follow):
			// Follow a cross-reference: always confirm via the picker (even for
			// a single target) so navigation never happens without a prompt.
			if refs := m.currentRefs(); len(refs) > 0 {
				items, aligned := m.pickerItems(refs)
				p := ui.NewPicker("Follow reference", items)
				m.picker, m.pickerRefs = &p, aligned
				return m, nil
			}
			// No references: fall through to the view.
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

// currentRefs is the cross-references the focused view exposes for its
// selection, filtered to those we can act on — either a loaded view resolves
// them, or they carry a browser-fallback URL. This drops regex false-positives
// (no resolver, no URL) while keeping links to items that aren't loaded. nil if
// the view isn't a Referencer.
func (m Model) currentRefs() []ui.Ref {
	r, ok := m.views[m.current].(ui.Referencer)
	if !ok {
		return nil
	}
	var out []ui.Ref
	for _, ref := range r.Refs() {
		if m.resolves(ref) || ref.URL != "" {
			out = append(out, ref)
		}
	}
	return out
}

// resolves reports whether a loaded view can select the ref's target.
func (m Model) resolves(ref ui.Ref) bool {
	for _, v := range m.views {
		if t, ok := v.(ui.RefTarget); ok && t.RefKind() == ref.Kind && t.HasRef(ref.ID) {
			return true
		}
	}
	return false
}

// followRef jumps to the ref's target if a view can resolve it, otherwise opens
// its URL in the browser. Returns the command to run (nil for an in-app jump).
func (m *Model) followRef(ref ui.Ref) tea.Cmd {
	for i, v := range m.views {
		if t, ok := v.(ui.RefTarget); ok && t.RefKind() == ref.Kind && t.HasRef(ref.ID) {
			t.SelectRef(ref.ID)
			m.current = i
			m.syncPreviewKey(true)
			return nil
		}
	}
	return ui.OpenURL(ref.URL) // unresolved → browser (no-op if URL is "")
}

// pickerItems builds the picker entries from refs and a parallel ref slice
// aligned to them (a zero Ref sits at any separator row, which is never
// selectable). Browser-bound refs get a ↗; a "sessions" separator divides the
// issue/PR refs from the agent-session refs. Each ref's context snippet becomes
// the dimmed detail line.
func (m Model) pickerItems(refs []ui.Ref) ([]ui.PickerItem, []ui.Ref) {
	var items []ui.PickerItem
	var aligned []ui.Ref
	hasPrimary, sepDone := false, false
	for _, r := range refs {
		if r.Kind == "session" && hasPrimary && !sepDone {
			items = append(items, ui.PickerItem{Separator: true, Label: "sessions"})
			aligned = append(aligned, ui.Ref{})
			sepDone = true
		}
		if r.Kind != "session" {
			hasPrimary = true
		}
		label := r.Label
		if !m.resolves(r) {
			label += "  ↗"
		}
		items = append(items, ui.PickerItem{Label: label, Detail: r.Detail})
		aligned = append(aligned, r)
	}
	return items, aligned
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

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderTabs(),
		body,
		m.renderFooter(),
	)

	// Composite the picker modal centered over the content, if open.
	if m.picker != nil {
		box := m.picker.View()
		x := max(0, (m.width-lipgloss.Width(box))/2)
		y := max(0, (m.height-lipgloss.Height(box))/2)
		content = lipgloss.NewCompositor(
			lipgloss.NewLayer(content),
			lipgloss.NewLayer(box).X(x).Y(y).Z(1),
		).Render()
	}

	v.Content = content
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

	// View-specific bindings first, then a contextual "related" hint (only when
	// the selection actually links somewhere), then the global ones.
	bindings := m.views[m.current].Bindings()
	if len(m.currentRefs()) > 0 {
		bindings = append(bindings, m.keys.Follow)
	}
	bindings = append(bindings,
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
