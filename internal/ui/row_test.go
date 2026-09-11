package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestTwoLineRowTruncatedMetaKeepsColorsAndWidth(t *testing.T) {
	styled := Cyan.Render("owner/repo") + Yellow.Render(" #42") + Dim.Render(" · a-very-long-branch-name-that-overflows")
	plain := "owner/repo #42 · a-very-long-branch-name-that-overflows"
	out := TwoLineRow(40, false, "●", plain, styled, "1d", "title", Highlighter{})
	line1 := strings.Split(out, "\n")[0]
	if w := lipgloss.Width(line1); w > 40 {
		t.Errorf("line1 width = %d, want <= 40", w)
	}
	if !strings.Contains(line1, "…") {
		t.Error("overflowing meta not truncated with ellipsis")
	}
	if !strings.Contains(line1, "\x1b[") {
		t.Error("truncated meta lost its styling entirely")
	}
}
