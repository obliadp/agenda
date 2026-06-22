// Package sessions is agenda's agent-sessions view. It lists Claude Code,
// Codex, and Antigravity sessions across the filesystem, previews their
// conversation, and resumes the selected one in its original directory. This
// is a Go port of the user's Python `sessions` tool.
package sessions

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/obliadp/agenda/internal/store"
	"github.com/obliadp/agenda/internal/ui"
)

// --- styles -----------------------------------------------------------------

func fg(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }

var (
	magenta = fg("5")
	green   = fg("2")
	blue    = fg("4")
	cyan    = fg("6")
	yellow  = fg("3")
	grey    = fg("8")
	faint   = lipgloss.NewStyle().Faint(true)
)

func (s session) toolStyle() lipgloss.Style {
	switch s.Tool {
	case toolCodex:
		return green
	case toolAgy:
		return blue
	default:
		return magenta
	}
}

func (s session) titleOr() string {
	if s.Title == "" {
		return "(no prompt)"
	}
	return s.Title
}

func (s session) Filter() string {
	return fmt.Sprintf("%s %s %s", s.Tool, shortenPath(s.Cwd), s.Title)
}

func (s session) Render(width int, selected bool) string {
	// Glyph column: the agent's Nerd Font icon (claude/codex/antigravity)
	// instead of its spelled-out name.
	glyphs := ui.AgentIcon(string(s.Tool))

	cwd := shortenPath(s.Cwd)
	right := yellow.Render(strconv.Itoa(s.Msgs)) + "  " + grey.Render(ui.Age(s.MTime))

	return ui.TwoLineRow(width, selected, glyphs, cwd, cyan.Render(cwd), right, s.titleOr())
}

// --- sorting ----------------------------------------------------------------

type sortMode int

const (
	sortRecent sortMode = iota
	sortOldest
	sortCwd
	sortTool
	sortMsgs
)

var sortOrder = []sortMode{sortRecent, sortOldest, sortCwd, sortTool, sortMsgs}
var sortName = map[sortMode]string{
	sortRecent: "recent", sortOldest: "oldest", sortCwd: "cwd",
	sortTool: "tool", sortMsgs: "msgs",
}

func sortSessions(in []session, mode sortMode) []session {
	out := make([]session, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch mode {
		case sortOldest:
			return a.MTime.Before(b.MTime)
		case sortCwd:
			if a.Cwd != b.Cwd {
				return strings.ToLower(a.Cwd) < strings.ToLower(b.Cwd)
			}
			return a.MTime.After(b.MTime)
		case sortTool:
			if a.Tool != b.Tool {
				return a.Tool < b.Tool
			}
			return a.MTime.After(b.MTime)
		case sortMsgs:
			return a.Msgs > b.Msgs
		default: // recent
			return a.MTime.After(b.MTime)
		}
	})
	return out
}

// --- messages ---------------------------------------------------------------

type loadedMsg []session
type resumedMsg struct{}

// --- view -------------------------------------------------------------------

type View struct {
	list  ui.List[session]
	raw   []session
	sort  sortMode
	store *store.Store

	loading bool

	listW, prevW, height int

	keys viewKeys
}

type viewKeys struct {
	Resume key.Binding
	Sort   key.Binding
}

func New(st *store.Store) *View {
	v := &View{
		store:   st,
		list:    ui.NewList[session](),
		loading: true,
		keys: viewKeys{
			Resume: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "resume")),
			Sort:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		},
	}
	v.list.SetRowHeight(2) // two-line rows: cwd + title
	return v
}

func (v *View) Title() string { return "Sessions" }

func (v *View) Init() tea.Cmd { return v.fetch() }

func (v *View) fetch() tea.Cmd {
	return func() tea.Msg { return loadedMsg(collect()) }
}

func (v *View) applySort() {
	v.list.SetItems(sortSessions(v.raw, v.sort))
}

func (v *View) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case loadedMsg:
		v.loading = false
		v.raw = []session(msg)
		v.applySort()
		v.publishMentions()
		return nil
	case resumedMsg:
		// Resuming likely changed mtimes; rescan so order/age stay accurate.
		return v.fetch()
	case tea.KeyMsg:
		if consumed, cmd := v.list.Update(msg); consumed {
			return cmd
		}
		if v.list.Filtering() {
			return nil
		}
		switch {
		case key.Matches(msg, v.keys.Resume):
			return v.resume()
		case key.Matches(msg, v.keys.Sort):
			v.sort = sortOrder[(int(v.sort)+1)%len(sortOrder)]
			v.applySort()
			return nil
		}
	}
	return nil
}

// resume launches the selected agent CLI in the session's directory, suspending
// agenda until it exits.
func (v *View) resume() tea.Cmd {
	s := v.list.Selected()
	if s.SessionID == "" {
		return nil
	}
	var c *exec.Cmd
	switch s.Tool {
	case toolCodex:
		c = exec.Command("codex", "resume", s.SessionID)
	case toolAgy:
		c = exec.Command("agy", "--conversation", s.SessionID)
	default:
		c = exec.Command("claude", "--resume", s.SessionID)
	}
	if s.Cwd != "" {
		if fi, err := os.Stat(s.Cwd); err == nil && fi.IsDir() {
			c.Dir = s.Cwd
		}
	}
	return tea.ExecProcess(c, func(error) tea.Msg { return resumedMsg{} })
}

func (v *View) SetSize(listW, prevW, h int) {
	v.listW, v.prevW, v.height = listW, prevW, h
	v.list.SetSize(listW, max(1, h-1))
}

func (v *View) ListView() string {
	header := v.list.FilterLine()
	if header == "" {
		header = faint.Render(v.statusText())
	}
	return header + "\n" + v.list.View()
}

func (v *View) statusText() string {
	if v.loading {
		return "Scanning sessions…"
	}
	return fmt.Sprintf("%d sessions · sort: %s", v.list.Total(), sortName[v.sort])
}

func (v *View) PreviewView() string {
	s := v.list.Selected()
	if s.Path == "" {
		return faint.Render("No session selected.")
	}

	var b strings.Builder
	b.WriteString(ui.AgentIcon(string(s.Tool)))
	b.WriteString("  ")
	b.WriteString(s.toolStyle().Bold(true).Render(strings.ToUpper(string(s.Tool))))
	b.WriteString("  ")
	b.WriteString(grey.Render(fmt.Sprintf("%s · %d msgs",
		s.MTime.Format("2006-01-02 15:04"), s.Msgs)))
	b.WriteString("\n")
	b.WriteString(cyan.Render(shortenPath(s.Cwd)))
	b.WriteString("\n")
	b.WriteString(faint.Render(s.SessionID))
	b.WriteString("\n")
	b.WriteString(grey.Render(strings.Repeat("─", min(v.prevW, 60))))
	b.WriteString("\n")

	turns := conversationTurns(s.Path, s.Tool)
	if len(turns) == 0 {
		b.WriteString(faint.Render("(no conversation content)"))
		return b.String()
	}

	// Show the most recent turns (chronological). The pane is clipped to its
	// height by the root model; recent context is what matters before resuming.
	const maxTurns = 14
	if len(turns) > maxTurns {
		turns = turns[len(turns)-maxTurns:]
	}
	wrap := lipgloss.NewStyle().Width(max(20, v.prevW))
	for _, t := range turns {
		label, style := "● ai ", blue
		if t.role == "user" {
			label, style = "▶ you", green
		}
		b.WriteString(style.Render(label))
		b.WriteByte(' ')
		b.WriteString(wrap.Render(ui.Truncate(t.text, 600)))
		b.WriteString("\n\n")
	}
	return b.String()
}

func (v *View) Bindings() []key.Binding {
	return []key.Binding{v.keys.Resume, v.keys.Sort}
}

func (v *View) Status() string { return grey.Render(v.statusText()) }

func (v *View) InputActive() bool { return v.list.Filtering() }

func (v *View) PreviewKey() string { return v.list.Selected().Path }

// publishMentions rebuilds the shared reverse index (entity -> sessions that
// mention it) so the PRs and Linear views can list the sessions referencing a
// given PR or issue.
func (v *View) publishMentions() {
	if v.store == nil {
		return
	}
	index := map[string][]store.SessionRef{}
	for _, s := range v.raw {
		for _, mn := range s.Mentions {
			key := store.Key(mn.Kind, mn.ID)
			index[key] = append(index[key], store.SessionRef{
				Path:    s.Path,
				Tool:    string(s.Tool),
				Title:   s.titleOr(),
				Cwd:     shortenPath(s.Cwd),
				Snippet: mn.Snippet,
			})
		}
	}
	v.store.SetSessionMentions(index)
}

// Refs implements ui.Referencer: the Linear issues and PRs this session
// mentions, rendered like the other views' references — issue titles and PR
// status icons/titles are sourced from the shared store.
func (v *View) Refs() []ui.Ref {
	var refs []ui.Ref
	for _, mn := range v.list.Selected().Mentions {
		switch mn.Kind {
		case "linear":
			var title, url string
			if v.store != nil {
				if iss, ok := v.store.Issue(mn.ID); ok {
					title, url = iss.Title, iss.URL
				}
			}
			refs = append(refs, ui.IssueRef(mn.ID, title, url))
		case "pr":
			repo, num, _ := ui.ParsePRURL(mn.ID)
			var pr store.PR
			if v.store != nil {
				pr, _ = v.store.PR(mn.ID)
			}
			refs = append(refs, ui.PRRef(pr, repo, num, pr.Title, mn.ID))
		}
	}
	return refs
}

// RefKind / HasRef / SelectRef implement ui.RefTarget so other views can jump
// to a session here. Sessions are keyed by file path.
func (v *View) RefKind() string { return "session" }

func matchPath(path string) func(session) bool {
	return func(s session) bool { return s.Path == path }
}

func (v *View) HasRef(id string) bool    { return v.list.Any(matchPath(id)) }
func (v *View) SelectRef(id string) bool { return v.list.Select(matchPath(id)) }
