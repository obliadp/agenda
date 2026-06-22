package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func press(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

func TestPickerNavigation(t *testing.T) {
	p := NewPicker("Follow", []string{"a", "b", "c"})
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
	p := NewPicker("x", []string{"a", "b"})
	p.Update(press('j'))
	if p.Index() != 1 {
		t.Errorf("after j Index = %d, want 1", p.Index())
	}
	p.Update(press('k'))
	if p.Index() != 0 {
		t.Errorf("after k Index = %d, want 0", p.Index())
	}
}

func TestPickerConfirmAndCancel(t *testing.T) {
	p := NewPicker("x", []string{"a", "b"})
	if done, cancelled := p.Update(press(tea.KeyEnter)); !done || cancelled {
		t.Errorf("enter: done=%v cancelled=%v, want true/false", done, cancelled)
	}
	if done, cancelled := p.Update(press(tea.KeyEscape)); done || !cancelled {
		t.Errorf("esc: done=%v cancelled=%v, want false/true", done, cancelled)
	}
}
