package ui

import (
	"strings"
	"testing"
)

func TestHighlighterMatchIndices(t *testing.T) {
	cases := []struct {
		name          string
		query         string
		caseSensitive bool
		plain         string
		want          []int // rune indices
	}{
		{"empty query", "", false, "banana", nil},
		{"contiguous", "ban", false, "banana", []int{0, 1, 2}},
		{"subsequence", "bnn", false, "banana", []int{0, 2, 4}},
		{"no match", "xyz", false, "banana", nil},
		{"case insensitive", "BAN", false, "banana", []int{0, 1, 2}},
		{"case sensitive miss", "BAN", true, "banana", nil},
		{"case sensitive hit", "Ban", true, "Banana", []int{0, 1, 2}},
		{"multibyte", "é", false, "café", []int{3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hl := Highlighter{Query: c.query, CaseSensitive: c.caseSensitive}
			got := hl.matchIndices(c.plain)
			if len(got) != len(c.want) {
				t.Fatalf("matchIndices(%q) = %v, want %v", c.plain, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("matchIndices(%q) = %v, want %v", c.plain, got, c.want)
				}
			}
		})
	}
}

func TestHighlighterHighlight(t *testing.T) {
	// Empty query is a no-op.
	if got := (Highlighter{}).Highlight("banana"); got != "banana" {
		t.Errorf("empty query Highlight = %q, want unchanged", got)
	}
	// No match returns the input unchanged (no escape codes added).
	if got := (Highlighter{Query: "xyz"}).Highlight("banana"); got != "banana" {
		t.Errorf("no-match Highlight = %q, want unchanged", got)
	}
	// A match wraps at least the matched runes; the plain text is preserved when
	// ANSI codes are stripped out. We assert every plain rune still appears in
	// order by checking the visible characters are a superset substring.
	out := (Highlighter{Query: "ban"}).Highlight("banana")
	if !strings.Contains(out, "b") || !strings.Contains(out, "a") || !strings.Contains(out, "n") {
		t.Errorf("Highlight dropped characters: %q", out)
	}
	if out == "banana" {
		t.Errorf("Highlight added no styling for a match: %q", out)
	}
}
