package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// View is one switchable screen in agenda (PRs, sessions, Linear issues).
//
// Each view owns its own data, list state, and selection. The root model is
// responsible only for the chrome around views: the tab bar, view switching,
// the list/preview split layout, and the footer. This mirrors gh-dash's
// Section abstraction, trimmed to what agenda needs.
//
// Views are pointer types that mutate in place, so Update returns only a
// command. This keeps each view package free of any dependency on tui — a
// view satisfies this interface structurally just by having the methods.
type View interface {
	// Title is the label shown in the tab bar.
	Title() string

	// Init kicks off the view's first data fetch. Called once at startup.
	Init() tea.Cmd

	// Update handles a message, mutating the view in place, and returns any
	// follow-up command.
	Update(msg tea.Msg) tea.Cmd

	// SetSize tells the view how much room it has. listWidth + previewWidth
	// span the content area; height excludes the tab bar and footer.
	SetSize(listWidth, previewWidth, height int)

	// ListView renders the left/main list pane.
	ListView() string

	// PreviewView renders the right pane for the current selection.
	PreviewView() string

	// Bindings are the view-specific key bindings shown in the footer and
	// dispatched while this view is focused.
	Bindings() []key.Binding

	// Status is a short right-aligned summary for the footer (e.g. "12 PRs"),
	// or "" for none.
	Status() string

	// InputActive reports whether the view is currently capturing text input
	// (e.g. an open filter box). While true the root model routes all keys to
	// the view, so global single-letter bindings don't hijack typing.
	InputActive() bool

	// PreviewKey identifies the currently-selected item (e.g. its URL or path),
	// or "" if none. The root model uses it to reset the preview scroll offset
	// when the selection changes.
	PreviewKey() string

	// Loading reports whether the view is currently fetching data, so the
	// chrome can show a spinner in its tab.
	Loading() bool
}
