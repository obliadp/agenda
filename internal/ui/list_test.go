package ui

import (
	"strings"
	"testing"
)

// strItem is a trivial Item whose render and filter text are the string itself.
type strItem string

func (s strItem) Render(width int, selected bool) string { return string(s) }
func (s strItem) Filter() string                         { return string(s) }

func newTestList(items ...string) List[strItem] {
	l := NewList[strItem]()
	conv := make([]strItem, len(items))
	for i, s := range items {
		conv[i] = strItem(s)
	}
	l.SetItems(conv)
	return l
}

func TestListSelectionDefaults(t *testing.T) {
	l := newTestList("a", "b", "c")
	if l.Total() != 3 || l.Len() != 3 {
		t.Fatalf("Total=%d Len=%d, want 3/3", l.Total(), l.Len())
	}
	if got := l.Selected(); got != "a" {
		t.Errorf("Selected() = %q, want first item %q", got, "a")
	}

	empty := NewList[strItem]()
	if got := empty.Selected(); got != "" {
		t.Errorf("empty Selected() = %q, want zero value", got)
	}
}

func TestListSelectionPreservedAcrossSetItems(t *testing.T) {
	l := newTestList("a", "b", "c")
	l.cursor = 2 // select "c"
	if l.Selected() != "c" {
		t.Fatalf("precondition: Selected()=%q", l.Selected())
	}
	// Replace with a reordered set that still contains "c".
	l.SetItems([]strItem{"c", "a", "b"})
	if got := l.Selected(); got != "c" {
		t.Errorf("after SetItems Selected() = %q, want %q (preserved by identity)", got, "c")
	}
}

func TestListSelectionResetsWhenItemGone(t *testing.T) {
	l := newTestList("a", "b", "c")
	l.cursor = 2
	l.SetItems([]strItem{"x", "y"}) // "c" no longer present
	if got := l.Selected(); got != "x" {
		t.Errorf("Selected() = %q, want clamped to first %q", got, "x")
	}
}

func TestListFilterSubsequence(t *testing.T) {
	l := newTestList("apple", "apricot", "banana")
	l.query = "ap"
	l.applyFilter()
	if l.Len() != 2 {
		t.Errorf("Len() = %d, want 2 (apple, apricot)", l.Len())
	}
	l.query = "ban"
	l.applyFilter()
	if l.Len() != 1 || l.Selected() != "banana" {
		t.Errorf("Len=%d Selected=%q, want 1/banana", l.Len(), l.Selected())
	}
	if l.Total() != 3 {
		t.Errorf("Total() = %d, want 3 (unfiltered count)", l.Total())
	}
}

func TestMatchesSubsequence(t *testing.T) {
	cases := []struct {
		s, q string
		want bool
	}{
		{"banana", "ban", true},
		{"banana", "bnn", true}, // subsequence, non-contiguous
		{"banana", "xyz", false},
		{"banana", "", true},   // empty query matches anything
		{"abc", "abcd", false}, // query longer than string
	}
	for _, c := range cases {
		if got := matchesSubsequence(c.s, c.q); got != c.want {
			t.Errorf("matchesSubsequence(%q, %q) = %v, want %v", c.s, c.q, got, c.want)
		}
	}
}

func TestVisibleItemsWithRowHeight(t *testing.T) {
	l := newTestList("a", "b", "c", "d", "e", "f")
	l.SetSize(40, 10)
	if got := l.visibleItems(); got != 10 {
		t.Errorf("visibleItems() with rowHeight 1, height 10 = %d, want 10", got)
	}
	l.SetRowHeight(2)
	if got := l.visibleItems(); got != 5 {
		t.Errorf("visibleItems() with rowHeight 2, height 10 = %d, want 5", got)
	}
}

func TestScrollbar(t *testing.T) {
	hasThumb := func(s string) bool { return strings.Contains(s, "┃") }

	// Everything fits: no bar (all blank cells).
	for _, c := range Scrollbar(5, 3, 5, 0) {
		if strings.TrimSpace(c) != "" {
			t.Fatalf("no-overflow cell = %q, want blank", c)
		}
	}
	// At the top, the thumb is at the top and not the bottom.
	top := Scrollbar(10, 100, 10, 0)
	if !hasThumb(top[0]) || hasThumb(top[9]) {
		t.Errorf("offset 0: thumb should be at the top, not the bottom")
	}
	// At the bottom (max offset = total-visible), the thumb is at the bottom.
	bot := Scrollbar(10, 100, 10, 90)
	if !hasThumb(bot[9]) || hasThumb(bot[0]) {
		t.Errorf("max offset: thumb should be at the bottom, not the top")
	}
}

func TestClampCursorWindow(t *testing.T) {
	l := newTestList("0", "1", "2", "3", "4", "5", "6", "7", "8", "9")
	l.SetSize(40, 3) // window of 3 items
	l.cursor = 5
	l.clampCursor()
	if l.offset != 3 {
		t.Errorf("offset = %d, want 3 (cursor 5 - window 3 + 1)", l.offset)
	}
	// Cursor above the window pulls the offset up.
	l.cursor = 1
	l.clampCursor()
	if l.offset != 1 {
		t.Errorf("offset = %d, want 1 (cursor scrolled above window)", l.offset)
	}
}
