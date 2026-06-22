// Package ui holds reusable widgets shared by every view: a generic,
// fuzzy-filterable selectable list, plus small rendering helpers. Keeping
// these here (rather than in tui) lets view packages depend on them without
// depending on the root model.
package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Item is anything a List can hold. Render returns the row text for the given
// width and selection state; Filter returns the text matched against the
// filter query.
type Item interface {
	Render(width int, selected bool) string
	Filter() string
}

// List is a vertically-scrolling, filterable, single-selection list. It is
// generic over the concrete item type so views get type-safe Selected().
type List[T Item] struct {
	items    []T
	filtered []int // indices into items that match the current filter
	cursor   int   // index into filtered
	offset   int   // first visible row (index into filtered)

	width, height int
	rowHeight     int // lines per item (default 1); set higher for multi-line rows

	filtering bool
	query     string

	keys listKeys
}

type listKeys struct {
	Up, Down, Top, Bottom, HalfUp, HalfDown, Filter, Clear key.Binding
}

func defaultListKeys() listKeys {
	return listKeys{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		HalfUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "½ page up")),
		HalfDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "½ page down")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Clear:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")),
	}
}

func NewList[T Item]() List[T] {
	return List[T]{keys: defaultListKeys(), rowHeight: 1}
}

// SetRowHeight declares how many lines each item's Render produces, so the
// list's scrolling and windowing account for multi-line rows. Items must
// render exactly this many lines.
func (l *List[T]) SetRowHeight(h int) {
	l.rowHeight = max(h, 1)
	l.clampCursor()
}

// visibleItems is how many items fit in the current height.
func (l *List[T]) visibleItems() int {
	rh := max(l.rowHeight, 1)
	return max(1, l.height/rh)
}

// SetItems replaces the contents, preserving the selected item by identity of
// its Filter() value where possible.
func (l *List[T]) SetItems(items []T) {
	prev := l.Selected()
	var prevKey string
	var hadPrev bool
	if any(prev) != nil {
		prevKey = prev.Filter()
		hadPrev = true
	}

	l.items = items
	l.applyFilter()

	if hadPrev {
		found := false
		for i, idx := range l.filtered {
			if l.items[idx].Filter() == prevKey {
				l.cursor = i
				found = true
				break
			}
		}
		// If the previously-selected item is gone, jump to the top rather than
		// leaving the cursor on whatever now sits at the old index.
		if !found {
			l.cursor = 0
		}
	}
	l.clampCursor()
}

func (l *List[T]) SetSize(w, h int) { l.width, l.height = w, h; l.clampCursor() }

// Filtering reports whether the list is currently capturing filter input.
func (l *List[T]) Filtering() bool { return l.filtering }

// Query is the active filter string.
func (l *List[T]) Query() string { return l.query }

// Len is the number of items after filtering.
func (l *List[T]) Len() int { return len(l.filtered) }

// Total is the number of items before filtering.
func (l *List[T]) Total() int { return len(l.items) }

// Any reports whether any visible item matches pred, without moving the cursor.
func (l *List[T]) Any(pred func(T) bool) bool {
	for _, idx := range l.filtered {
		if pred(l.items[idx]) {
			return true
		}
	}
	return false
}

// Select moves the cursor to the first visible item matching pred, returning
// whether one was found.
func (l *List[T]) Select(pred func(T) bool) bool {
	for i, idx := range l.filtered {
		if pred(l.items[idx]) {
			l.cursor = i
			l.clampCursor()
			return true
		}
	}
	return false
}

// Selected returns the currently-highlighted item, or the zero value if empty.
func (l *List[T]) Selected() T {
	var zero T
	if l.cursor < 0 || l.cursor >= len(l.filtered) {
		return zero
	}
	return l.items[l.filtered[l.cursor]]
}

// Update handles navigation and filter-editing keys. It reports whether the
// key was consumed, so the host view can ignore keys the list handled.
func (l *List[T]) Update(msg tea.Msg) (consumed bool, cmd tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return false, nil
	}

	if l.filtering {
		switch km.String() {
		case "esc":
			l.filtering = false
			l.query = ""
			l.applyFilter()
			return true, nil
		case "enter":
			l.filtering = false // keep the query, just stop editing
			return true, nil
		case "backspace":
			if l.query != "" {
				l.query = l.query[:len(l.query)-1]
				l.applyFilter()
			}
			return true, nil
		// Keys that can't be confused with typed text navigate while filtering,
		// so you can refine the query and move the selection at the same time.
		case "up":
			l.move(-1)
		case "down":
			l.move(1)
		case "ctrl+u":
			l.move(-l.visibleItems() / 2)
		case "ctrl+d":
			l.move(l.visibleItems() / 2)
		case "home":
			l.cursor = 0
		case "end":
			l.cursor = len(l.filtered) - 1
		default:
			// Append the key's text — a letter, digit, space, etc. Non-text keys
			// (arrows, …) have empty Text; control chars like Tab are ignored.
			if kp, ok := msg.(tea.KeyPressMsg); ok && kp.Text != "" {
				if r := []rune(kp.Text)[0]; r >= 0x20 && r != 0x7f {
					l.query += kp.Text
					l.applyFilter()
				}
			}
			return true, nil
		}
		l.clampCursor()
		return true, nil
	}

	switch {
	case key.Matches(km, l.keys.Up):
		l.move(-1)
	case key.Matches(km, l.keys.Down):
		l.move(1)
	case key.Matches(km, l.keys.HalfUp):
		l.move(-l.visibleItems() / 2)
	case key.Matches(km, l.keys.HalfDown):
		l.move(l.visibleItems() / 2)
	case key.Matches(km, l.keys.Top):
		l.cursor = 0
	case key.Matches(km, l.keys.Bottom):
		l.cursor = len(l.filtered) - 1
	case key.Matches(km, l.keys.Filter):
		l.filtering = true
	case key.Matches(km, l.keys.Clear):
		if l.query != "" {
			l.query = ""
			l.applyFilter()
		}
	default:
		return false, nil
	}
	l.clampCursor()
	return true, nil
}

func (l *List[T]) move(delta int) { l.cursor += delta }

func (l *List[T]) clampCursor() {
	if len(l.filtered) == 0 {
		l.cursor, l.offset = 0, 0
		return
	}
	l.cursor = clamp(l.cursor, 0, len(l.filtered)-1)
	// Keep the cursor within the visible window (measured in items).
	win := l.visibleItems()
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+win {
		l.offset = l.cursor - win + 1
	}
	l.offset = clamp(l.offset, 0, max(0, len(l.filtered)-1))
}

func (l *List[T]) applyFilter() {
	l.filtered = l.filtered[:0]
	q := strings.ToLower(strings.TrimSpace(l.query))
	for i := range l.items {
		if q == "" || matchesSubsequence(strings.ToLower(l.items[i].Filter()), q) {
			l.filtered = append(l.filtered, i)
		}
	}
	l.clampCursor()
}

func (l *List[T]) View() string {
	if len(l.filtered) == 0 {
		empty := "No matches."
		if len(l.items) == 0 {
			empty = "Nothing here."
		}
		return lipgloss.NewStyle().Faint(true).Render(empty)
	}

	win := l.visibleItems()
	end := min(l.offset+win, len(l.filtered))

	// Reserve a right-hand gutter for the scrollbar (2 cols: bar + a gap) when
	// there's room, so item width stays stable whether or not it overflows.
	contentW := l.width
	gutter := l.width >= 3 && l.height > 0
	if gutter {
		contentW = l.width - 2
	}

	var lines []string
	for i := l.offset; i < end; i++ {
		block := l.items[l.filtered[i]].Render(contentW, i == l.cursor)
		lines = append(lines, strings.Split(block, "\n")...)
	}
	if !gutter {
		return strings.Join(lines, "\n")
	}

	// Pad/clip to exactly the pane height, then attach the scrollbar column.
	for len(lines) < l.height {
		lines = append(lines, "")
	}
	lines = lines[:l.height]
	bar := Scrollbar(l.height, len(l.filtered), win, l.offset)
	for i := range lines {
		pad := max(0, contentW-lipgloss.Width(lines[i]))
		lines[i] += strings.Repeat(" ", pad) + " " + bar[i]
	}
	return strings.Join(lines, "\n")
}

// Scrollbar returns height cells for a slim vertical scrollbar: a thumb sized
// to the visible fraction and positioned by offset (a heavy line), over a faint
// light-line track. When everything fits (no overflow) it returns blanks, so a
// reserved gutter stays empty. Shared by the list rows and the preview pane.
func Scrollbar(height, total, visible, offset int) []string {
	track := lipgloss.NewStyle().Faint(true).Render("│")
	thumb := lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Render("┃")

	out := make([]string, height)
	if total <= visible { // everything visible: no bar
		for i := range out {
			out[i] = " "
		}
		return out
	}
	size := min(max(1, height*visible/total), height)
	pos := (height - size) * offset / (total - visible)
	pos = clamp(pos, 0, height-size)
	for i := range out {
		if i >= pos && i < pos+size {
			out[i] = thumb
		} else {
			out[i] = track
		}
	}
	return out
}

// FilterLine renders the active filter prompt (for a view to show in a header).
func (l *List[T]) FilterLine() string {
	if !l.filtering && l.query == "" {
		return ""
	}
	cursor := ""
	if l.filtering {
		cursor = "█"
	}
	return lipgloss.NewStyle().Faint(true).Render("/"+l.query) + cursor
}

// --- helpers ---------------------------------------------------------------

func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

// matchesSubsequence reports whether all runes of q appear in s in order.
func matchesSubsequence(s, q string) bool {
	if q == "" {
		return true
	}
	qi := 0
	qr := []rune(q)
	for _, sr := range s {
		if sr == qr[qi] {
			qi++
			if qi == len(qr) {
				return true
			}
		}
	}
	return false
}
