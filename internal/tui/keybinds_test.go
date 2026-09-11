package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sanity-labs/agenda/internal/config"
)

func TestRegistryDefaultsMatchGlobalKeys(t *testing.T) {
	// The registry claims defaults; newKeys resolves the real ones. They
	// must agree, or the editor lies about what a reset restores.
	g := newKeys(nil)
	byAction := map[string][]string{}
	for _, e := range keyRegistry() {
		if e.scope == "global" {
			byAction[e.action] = e.def
		}
	}
	checks := map[string][]string{
		"quit":      g.Quit.Keys(),
		"next_view": g.NextView.Keys(),
		"zoom":      g.Zoom.Keys(),
		"config":    g.Config.Keys(),
		"help":      g.Help.Keys(),
	}
	for action, want := range checks {
		got := byAction[action]
		if len(got) != len(want) {
			t.Fatalf("registry default for %s = %v, real = %v", action, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("registry default for %s = %v, real = %v", action, got, want)
			}
		}
	}
}

func TestFindCollision(t *testing.T) {
	km := config.Keymap{}
	target := keyEntry{scope: "prs", action: "open", label: "open", def: []string{"enter"}}

	// 'q' is global quit: collides from any view scope.
	if other, clash := findCollision(km, target, "q"); !clash || other.action != "quit" {
		t.Errorf("expected q to collide with global quit, got %+v %v", other, clash)
	}
	// 'j' is list down: collides too.
	if _, clash := findCollision(km, target, "j"); !clash {
		t.Error("expected j to collide with list down")
	}
	// 'b' is linear copy_branch: different view scope, no collision with prs.
	if other, clash := findCollision(km, target, "b"); clash {
		t.Errorf("b should not collide across view scopes, hit %+v", other)
	}
	// A free key doesn't collide.
	if _, clash := findCollision(km, target, "F5"); clash {
		t.Error("F5 should be free")
	}
	// Overrides are honored: rebind linear copy_branch to global scope key.
	km = config.Keymap{"prs": {"diff": config.Chord{"o"}}}
	if other, clash := findCollision(km, target, "o"); !clash || other.action != "diff" {
		t.Errorf("expected o to collide with the overridden prs diff, got %+v %v", other, clash)
	}
	// The old default of an overridden action is free again.
	if _, clash := findCollision(km, target, "d"); clash {
		t.Error("d should be free once prs diff moved to o")
	}
}

func TestKeybindEditorFlow(t *testing.T) {
	o := newKeybindEditor()
	km := config.Keymap{}

	// Rebind the first row (global next_view) to a free key.
	change, closed := o.Update(tea.KeyPressMsg{Code: tea.KeyEnter}, km)
	if change != nil || closed || !o.capturing {
		t.Fatal("enter should start capture")
	}
	change, _ = o.Update(tea.KeyPressMsg{Code: 'n', Text: "n"}, km)
	if change == nil || len(change.keys) != 1 || change.keys[0] != "n" {
		t.Fatalf("capture produced %+v, want [n]", change)
	}

	// Capturing a colliding key refuses and reports where it's bound.
	o.Update(tea.KeyPressMsg{Code: tea.KeyEnter}, km)
	change, _ = o.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}, km)
	if change != nil || o.errMsg == "" {
		t.Fatalf("colliding key accepted: %+v (err %q)", change, o.errMsg)
	}

	// x disables, r resets to default.
	change, _ = o.Update(tea.KeyPressMsg{Code: 'x', Text: "x"}, km)
	if change == nil || len(change.keys) != 0 {
		t.Fatalf("x should disable, got %+v", change)
	}
	change, _ = o.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}, km)
	if change == nil || len(change.keys) != 2 || change.keys[0] != "tab" {
		t.Fatalf("r should reset next_view to [tab L], got %+v", change)
	}

	// esc closes.
	if _, closed := o.Update(tea.KeyPressMsg{Code: tea.KeyEscape}, km); !closed {
		t.Error("esc should close the editor")
	}
}

func TestDigitsCollideWithViewJump(t *testing.T) {
	target := keyEntry{scope: "prs", action: "diff", label: "diff", def: []string{"d"}}
	if other, clash := findCollision(config.Keymap{}, target, "2"); !clash || other.action != "view_jump" {
		t.Errorf("digit capture should collide with the view jump, got %+v %v", other, clash)
	}
}
