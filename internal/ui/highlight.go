package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Highlighter carries the active filter query for in-row highlighting of the
// runes that participate in a subsequence match. The zero value (empty Query)
// highlights nothing.
type Highlighter struct {
	Query         string
	CaseSensitive bool
}

var highlightStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))

// matchIndices returns the rune indices of plain (indexing into []rune(plain))
// that match Query as a subsequence, in order. nil if Query is empty or there
// is no match.
func (hl Highlighter) matchIndices(plain string) []int {
	if hl.Query == "" {
		return nil
	}
	s := []rune(plain)
	q := []rune(hl.Query)
	if !hl.CaseSensitive {
		s = []rune(strings.ToLower(plain))
		q = []rune(strings.ToLower(hl.Query))
	}
	var idx []int
	qi := 0
	for i, r := range s {
		if qi < len(q) && r == q[qi] {
			idx = append(idx, i)
			qi++
		}
	}
	if qi != len(q) {
		return nil // not a full subsequence match
	}
	return idx
}

// Highlight returns plain with its matched runes wrapped in the highlight
// style. It styles each matched rune individually (the match may be
// non-contiguous, since matching is a fuzzy subsequence). Returns plain
// unchanged when Query is empty or there is no match.
func (hl Highlighter) Highlight(plain string) string {
	idx := hl.matchIndices(plain)
	if len(idx) == 0 {
		return plain
	}
	hit := make(map[int]bool, len(idx))
	for _, i := range idx {
		hit[i] = true
	}
	var b strings.Builder
	for i, r := range []rune(plain) {
		if hit[i] {
			b.WriteString(highlightStyle.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
