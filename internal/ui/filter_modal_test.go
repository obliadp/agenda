package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func typeRunes(m *FilterModal, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestFilterModalTypingAndApply(t *testing.T) {
	// Cursor starts on the query row, so typing edits the query.
	m := NewFilterModal("Filter PRs", "", []string{"repo", "branch", "title"}, nil, false)
	typeRunes(&m, "oauth")
	if m.Query() != "oauth" {
		t.Fatalf("Query() = %q, want oauth", m.Query())
	}
	if done, cancelled := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); !done || cancelled {
		t.Errorf("enter: done=%v cancelled=%v, want true/false", done, cancelled)
	}
}

func TestFilterModalToggleFields(t *testing.T) {
	// One continuous list: row 0 is the query, rows 1.. are field toggles.
	// Down twice moves query -> repo -> branch, then space toggles branch OFF.
	m := NewFilterModal("x", "", []string{"repo", "branch", "title"}, nil, false)
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // query -> repo
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // repo -> branch
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	got := m.EnabledFields()
	if strings.Join(got, ",") != "repo,title" {
		t.Errorf("EnabledFields() = %v, want [repo title]", got)
	}
}

func TestFilterModalCaseSensitiveRow(t *testing.T) {
	// Down past the single field row lands on the case-sensitive row.
	m := NewFilterModal("x", "", []string{"repo"}, nil, false)
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // query -> repo
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // repo -> case sensitive
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if !m.CaseSensitive() {
		t.Errorf("CaseSensitive() = false after toggling its row, want true")
	}
}

func TestFilterModalCursorClamps(t *testing.T) {
	// Up from the top row stays on the query; down past the last row clamps.
	m := NewFilterModal("x", "", []string{"repo"}, nil, false)
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp}) // already at top (query) -> stays
	typeRunes(&m, "z")
	if m.Query() != "z" {
		t.Errorf("Query()=%q, want z (cursor stayed on query row at top)", m.Query())
	}
	// Walk to the bottom (query -> repo -> case) and one extra down (clamps).
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // clamp at case row
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if !m.CaseSensitive() {
		t.Errorf("expected cursor clamped on case row; CaseSensitive()=false")
	}
}

func TestFilterModalCancel(t *testing.T) {
	m := NewFilterModal("x", "", []string{"repo"}, nil, false)
	if done, cancelled := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); done || !cancelled {
		t.Errorf("esc: done=%v cancelled=%v, want false/true", done, cancelled)
	}
}

func TestFilterModalBackspace(t *testing.T) {
	m := NewFilterModal("x", "ab", []string{"repo"}, nil, false)
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.Query() != "a" {
		t.Errorf("Query() = %q after backspace, want a", m.Query())
	}
}

func TestFilterModalNoLeakOffQueryRow(t *testing.T) {
	// When the cursor is on a toggle row, printable keys must NOT type into
	// the query.
	m := NewFilterModal("x", "", []string{"repo"}, nil, false)
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // query -> repo
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if m.Query() != "" {
		t.Errorf("char leaked into query while cursor off the query row: Query()=%q, want empty", m.Query())
	}
}

func TestFilterModalVimNavOffQueryRow(t *testing.T) {
	// j/k navigate when the cursor is off the query row (matching the list/picker).
	m := NewFilterModal("x", "", []string{"repo", "branch"}, nil, false)
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})    // query -> repo
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"}) // repo -> branch
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "}) // toggle branch off
	if strings.Join(m.EnabledFields(), ",") != "repo" {
		t.Errorf("EnabledFields() = %v, want [repo] (j moved to branch, space toggled it)", m.EnabledFields())
	}
	m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"}) // branch -> repo
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "}) // toggle repo off
	if len(m.EnabledFields()) != 0 {
		t.Errorf("EnabledFields() = %v, want [] (k moved back to repo, space toggled it)", m.EnabledFields())
	}
}

func TestFilterModalJKTypeOnQueryRow(t *testing.T) {
	// On the query row (cursor 0), j/k are literal characters, not navigation.
	m := NewFilterModal("x", "", []string{"repo"}, nil, false)
	typeRunes(&m, "jk")
	if m.Query() != "jk" {
		t.Errorf("Query() = %q, want jk (j/k type literally on the query row)", m.Query())
	}
}

func TestFilterModalSpaceTypesOnQueryRow(t *testing.T) {
	// Space on the query row is a literal space (it only toggles on toggle rows).
	m := NewFilterModal("x", "", []string{"repo"}, nil, false)
	typeRunes(&m, "a")
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	typeRunes(&m, "b")
	if m.Query() != "a b" {
		t.Errorf("Query() = %q, want %q (space types on the query row)", m.Query(), "a b")
	}
}

func TestFilterModalBackspaceMultibyte(t *testing.T) {
	m := NewFilterModal("x", "café", []string{"repo"}, nil, false)
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.Query() != "caf" {
		t.Errorf("Query()=%q after backspace, want caf", m.Query())
	}
}
