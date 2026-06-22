package ui

import (
	"strings"
	"testing"

	"github.com/obliadp/agenda/internal/store"
)

func TestPRRefWithTitle(t *testing.T) {
	r := PRRef(store.PR{State: store.PROpen}, "o/r", 12, "Fix the thing", "u")
	if r.Kind != "pr" || r.ID != "u" || r.URL != "u" {
		t.Fatalf("ref identity = %+v", r)
	}
	if !strings.Contains(r.Label, "Fix the thing") {
		t.Errorf("Label = %q, want it to contain the title", r.Label)
	}
	if r.Detail != "o/r#12" {
		t.Errorf("Detail = %q, want o/r#12", r.Detail)
	}
}

func TestPRRefWithoutTitle(t *testing.T) {
	// Unknown PR (zero record): no icons, no title -> repo#num on the main line,
	// no redundant detail.
	r := PRRef(store.PR{}, "o/r", 7, "", "u")
	if r.Label != "o/r#7" {
		t.Errorf("Label = %q, want o/r#7", r.Label)
	}
	if r.Detail != "" {
		t.Errorf("Detail = %q, want empty (no second line)", r.Detail)
	}
}

func TestPRIconsUnknownIsEmpty(t *testing.T) {
	if got := PRIcons(store.PR{}); got != "" {
		t.Errorf("PRIcons(zero) = %q, want empty", got)
	}
}

func TestIssueRef(t *testing.T) {
	r := IssueRef("SRE-1", "Do a thing")
	if r.Kind != "linear" || r.ID != "SRE-1" {
		t.Fatalf("ref identity = %+v", r)
	}
	if r.Label != "Linear  SRE-1" {
		t.Errorf("Label = %q", r.Label)
	}
	if r.Detail != "Do a thing" {
		t.Errorf("Detail = %q, want the title", r.Detail)
	}
}
