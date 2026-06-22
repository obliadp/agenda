package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func press(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

func TestPickerNavigation(t *testing.T) {
	p := NewPicker("Follow", []PickerItem{{Label: "a"}, {Label: "b"}, {Label: "c"}})
	if p.Index() != 0 {
		t.Fatalf("initial Index = %d, want 0", p.Index())
	}

	p.Update(press(tea.KeyDown))
	p.Update(press(tea.KeyDown))
	if p.Index() != 2 {
		t.Errorf("after two downs Index = %d, want 2", p.Index())
	}
	p.Update(press(tea.KeyDown)) // clamps at the bottom
	if p.Index() != 2 {
		t.Errorf("Index = %d, want clamped at 2", p.Index())
	}
	p.Update(press(tea.KeyUp))
	if p.Index() != 1 {
		t.Errorf("after up Index = %d, want 1", p.Index())
	}
}

func TestPickerVimKeys(t *testing.T) {
	p := NewPicker("x", []PickerItem{{Label: "a"}, {Label: "b"}})
	p.Update(press('j'))
	if p.Index() != 1 {
		t.Errorf("after j Index = %d, want 1", p.Index())
	}
	p.Update(press('k'))
	if p.Index() != 0 {
		t.Errorf("after k Index = %d, want 0", p.Index())
	}
}

func TestPickerSkipsSeparators(t *testing.T) {
	p := NewPicker("x", []PickerItem{
		{Label: "a"},
		{Separator: true, Label: "sessions"},
		{Label: "b"},
	})
	if p.Index() != 0 {
		t.Fatalf("initial Index = %d, want 0", p.Index())
	}
	p.Update(press(tea.KeyDown))
	if p.Index() != 2 {
		t.Errorf("down Index = %d, want 2 (separator skipped)", p.Index())
	}
	p.Update(press(tea.KeyUp))
	if p.Index() != 0 {
		t.Errorf("up Index = %d, want 0 (separator skipped)", p.Index())
	}
}

func TestPickerInitialCursorSkipsLeadingSeparator(t *testing.T) {
	p := NewPicker("x", []PickerItem{{Separator: true, Label: "s"}, {Label: "a"}})
	if p.Index() != 1 {
		t.Errorf("initial Index = %d, want 1 (leading separator skipped)", p.Index())
	}
}

func TestPickerConfirmAndCancel(t *testing.T) {
	p := NewPicker("x", []PickerItem{{Label: "a"}, {Label: "b"}})
	if done, cancelled := p.Update(press(tea.KeyEnter)); !done || cancelled {
		t.Errorf("enter: done=%v cancelled=%v, want true/false", done, cancelled)
	}
	if done, cancelled := p.Update(press(tea.KeyEscape)); done || !cancelled {
		t.Errorf("esc: done=%v cancelled=%v, want false/true", done, cancelled)
	}
}
