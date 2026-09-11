package tui

import (
	"strings"
	"testing"

	"github.com/sanity-labs/agenda/internal/config"
	"github.com/sanity-labs/agenda/internal/ui"
)

func TestToastLifecycle(t *testing.T) {
	m := New(config.Default(), []View{&stubView{"PRs"}})

	next, cmd := m.Update(ui.ToastMsg{Title: "PR needs your review", Body: "acme/repo#7: fix (@bob)"})
	m = next.(Model)
	if m.toast == nil || cmd == nil {
		t.Fatal("toast should be set with a dismiss timer")
	}

	// A stale dismiss (older generation) is ignored; the matching one clears.
	next, _ = m.Update(toastGoneMsg{gen: m.toastGen - 1})
	m = next.(Model)
	if m.toast == nil {
		t.Fatal("stale dismiss cleared a newer toast")
	}
	next, _ = m.Update(toastGoneMsg{gen: m.toastGen})
	m = next.(Model)
	if m.toast != nil {
		t.Fatal("matching dismiss should clear the toast")
	}
}

func TestRenderToastContent(t *testing.T) {
	m := New(config.Default(), []View{&stubView{"PRs"}})
	m.toast = &ui.ToastMsg{Title: "agenda test", Body: "hello"}
	out := m.renderToast()
	if !strings.Contains(out, "agenda test") || !strings.Contains(out, "hello") {
		t.Errorf("toast missing content:\n%s", out)
	}
}
