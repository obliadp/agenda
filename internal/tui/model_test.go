package tui

import "testing"

func TestViewIndexForKey(t *testing.T) {
	cases := map[string]int{
		"1":  0, // "1" jumps to the first view
		"2":  1,
		"5":  4, // mid-range exercises the s[0]-'1' formula
		"9":  8,
		"0":  -1, // 0 is not a view hotkey
		"a":  -1,
		"":   -1,
		"12": -1, // multi-char (e.g. a key name) is not a digit jump
	}
	for in, want := range cases {
		if got := viewIndexForKey(in); got != want {
			t.Errorf("viewIndexForKey(%q) = %d, want %d", in, got, want)
		}
	}
}
