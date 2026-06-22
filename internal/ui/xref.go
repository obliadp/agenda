package ui

// Cross-references let one view's selection point at items in another view
// (e.g. a PR references a Linear issue). The mechanism is general: a source
// view implements Referencer, a destination view implements RefTarget, and the
// root model wires them together — discovering both by capability, so adding a
// new link type is just implementing the interfaces.

// Ref is a single cross-reference from the current selection to a target item.
type Ref struct {
	Kind  string // which target view handles it, e.g. "linear", "pr"
	ID    string // identifier to select in that view
	Label string // human-facing label shown in the picker
}

// Referencer is implemented by a view whose current selection links to items
// elsewhere. Refs returns the links for the selection (empty if none).
type Referencer interface {
	Refs() []Ref
}

// RefTarget is implemented by a view that can be navigated to. RefKind reports
// the Ref.Kind it handles; HasRef reports whether the identified item is
// present (used to filter out references that can't actually be resolved, e.g.
// regex false-positives); SelectRef selects it and reports success.
type RefTarget interface {
	RefKind() string
	HasRef(id string) bool
	SelectRef(id string) bool
}
