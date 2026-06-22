package tui

import "charm.land/bubbles/v2/key"

// globalKeys are handled by the root model regardless of the active view.
type globalKeys struct {
	NextView    key.Binding
	PrevView    key.Binding
	Refresh     key.Binding
	Quit        key.Binding
	Help        key.Binding
	Follow      key.Binding
	PreviewUp   key.Binding
	PreviewDown key.Binding
	PreviewPgUp key.Binding
	PreviewPgDn key.Binding
}

func defaultKeys() globalKeys {
	return globalKeys{
		NextView: key.NewBinding(
			key.WithKeys("tab", "L"),
			key.WithHelp("tab", "next view"),
		),
		PrevView: key.NewBinding(
			key.WithKeys("shift+tab", "H"),
			key.WithHelp("⇧tab", "prev view"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "refresh"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Follow: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "related"),
		),
		PreviewUp: key.NewBinding(
			key.WithKeys("shift+up"),
			key.WithHelp("⇧↑↓", "scroll preview"),
		),
		PreviewDown: key.NewBinding(
			key.WithKeys("shift+down"),
		),
		PreviewPgUp: key.NewBinding(
			key.WithKeys("pgup"),
		),
		PreviewPgDn: key.NewBinding(
			key.WithKeys("pgdown"),
		),
	}
}
