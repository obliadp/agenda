package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// PickerItem is one row in a Picker. A normal item has a Label and an optional
// dimmed Detail line beneath it. A Separator item is a non-selectable group
// divider (its Label, if any, is shown in the rule).
type PickerItem struct {
	Label     string
	Detail    string
	Separator bool
}

// Picker is a small modal list for choosing one option (e.g. which referenced
// item to jump to). The host model owns it, routes key messages to Update while
// it's open, and composites View over its content.
type Picker struct {
	title  string
	items  []PickerItem
	cursor int
}

func NewPicker(title string, items []PickerItem) Picker {
	p := Picker{title: title, items: items}
	if len(items) > 0 && items[0].Separator {
		p.cursor = p.nextSelectable(0, 1) // never rest on a separator
	}
	return p
}

// nextSelectable returns the next non-separator index from start in direction
// dir (+1/-1), or the original cursor if there is none.
func (p *Picker) nextSelectable(start, dir int) int {
	for i := start; i >= 0 && i < len(p.items); i += dir {
		if !p.items[i].Separator {
			return i
		}
	}
	return p.cursor
}

// Update handles navigation keys. done is true when the user confirmed a
// choice (read it with Index); cancelled is true when they dismissed the modal.
func (p *Picker) Update(msg tea.Msg) (done, cancelled bool) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return false, false
	}
	switch km.String() {
	case "up", "k":
		p.cursor = p.nextSelectable(p.cursor-1, -1)
	case "down", "j":
		p.cursor = p.nextSelectable(p.cursor+1, 1)
	case "enter":
		return true, false
	case "esc", "ctrl+c", "q":
		return false, true
	}
	return false, false
}

// Index is the selected option's index.
func (p *Picker) Index() int { return p.cursor }

// View renders the modal box. The caller composites it over its own content.
func (p *Picker) View() string {
	dim := lipgloss.NewStyle().Faint(true)
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	bold := lipgloss.NewStyle().Bold(true)

	var b strings.Builder
	b.WriteString(bold.Render(p.title))
	b.WriteString("\n\n")
	for i, it := range p.items {
		if it.Separator {
			rule := "────────"
			if it.Label != "" {
				rule = "── " + it.Label + " ──────"
			}
			b.WriteString("  ")
			b.WriteString(dim.Render(rule))
			b.WriteByte('\n')
			continue
		}
		if i == p.cursor {
			b.WriteString(accent.Render("▌ "))
			b.WriteString(bold.Render(it.Label))
		} else {
			b.WriteString("  ")
			b.WriteString(it.Label)
		}
		b.WriteByte('\n')
		if it.Detail != "" {
			b.WriteString("    ")
			b.WriteString(dim.Render(Truncate(it.Detail, 70)))
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(dim.Render("↑/↓ select · enter open · esc cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("13")).
		Padding(1, 2).
		Render(b.String())
}
