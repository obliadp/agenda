package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Picker is a small modal list for choosing one option (e.g. which referenced
// issue to jump to when a PR links several). The host model owns it, routes key
// messages to Update while it's open, and composites View over its content.
type Picker struct {
	title  string
	items  []string
	cursor int
}

func NewPicker(title string, items []string) Picker {
	return Picker{title: title, items: items}
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
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
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
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(p.title))
	b.WriteString("\n\n")
	for i, it := range p.items {
		if i == p.cursor {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Render("▌ "))
			b.WriteString(lipgloss.NewStyle().Bold(true).Render(it))
		} else {
			b.WriteString("  ")
			b.WriteString(it)
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(lipgloss.NewStyle().Faint(true).Render("↑/↓ select · enter open · esc cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("13")).
		Padding(1, 2).
		Render(b.String())
}
