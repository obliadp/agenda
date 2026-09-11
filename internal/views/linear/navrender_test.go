package linear

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sanity-labs/agenda/internal/config"
	"github.com/sanity-labs/agenda/internal/ui"
)

func TestNavPaneActuallyRenders(t *testing.T) {
	// Glyph widths vary across environments (private-use runes measure 1 or
	// 2 cells); this test checks pane structure, so render without them.
	ui.SetGlyphs(false)
	defer ui.SetGlyphs(true)
	v := New(config.LinearConfig{Token: "x"}, nil, nil, nil)
	v.SetSize(60, 60, 20)
	v.raw = mkIDs("A-1")
	v.applySort()

	v.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	out := v.ListView()

	// The combined block must keep the full list width: tree + border + list.
	if got := lipgloss.Width(out); got != 60 {
		t.Fatalf("ListView width = %d after ctrl+p, want 60 (tree missing?)", got)
	}
	// The tree entries render on their own lines (not just the status line).
	var treeLines int
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "My Issues") || strings.Contains(l, "All Issues") {
			treeLines++
		}
	}
	if treeLines < 2 {
		t.Fatalf("tree rows missing, found %d lines with source labels", treeLines)
	}
	// Toggling off restores the flat layout.
	v.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	if got := lipgloss.Width(v.ListView()); got != 60 {
		t.Fatalf("ListView width = %d after toggle off, want 60", got)
	}
}
