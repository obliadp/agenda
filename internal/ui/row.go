package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// TwoLineRow renders a list item as a two-line block, à la gh-dash's
// non-compact layout:
//
//	▌  <glyphs>  <meta….……….……….……….….>  <right>
//	            <title, bold>
//
// Line one is the glyphs followed by dimmed metadata, with `right` pinned to
// the far edge; line two is the title, indented to align under the metadata.
// The selection accent bar spans both lines (rather than a full-row background,
// which lipgloss's per-segment resets would clobber).
//
// metaStyled is the colored metadata; metaPlain is the same text uncolored,
// used for width measurement and safe truncation (so we never cut through an
// ANSI escape). glyphs and right may contain styling; their display width is
// measured with lipgloss.Width.
func TwoLineRow(width int, selected bool, glyphs, metaPlain, metaStyled, right, title string, hl Highlighter) string {
	bar := "  "
	if selected {
		bar = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Render("▌") + " "
	}

	prefix := bar + glyphs + "  "
	indent := lipgloss.Width(prefix)

	avail := max(1, width-indent-lipgloss.Width(right)-1)
	meta := metaStyled
	if lipgloss.Width(metaPlain) > avail {
		meta = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(Truncate(metaPlain, avail))
	}
	gap := max(1, width-indent-lipgloss.Width(meta)-lipgloss.Width(right))
	line1 := prefix + meta + strings.Repeat(" ", gap) + right

	plainTitle := Truncate(title, max(1, width-indent))
	t := hl.Highlight(plainTitle)
	if selected {
		t = lipgloss.NewStyle().Bold(true).Render(t)
	} else {
		t = lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Render(t)
	}
	line2 := bar + strings.Repeat(" ", indent-lipgloss.Width(bar)) + t

	return line1 + "\n" + line2
}
