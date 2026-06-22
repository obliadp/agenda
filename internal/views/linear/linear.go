// Package linear is agenda's Linear-issues view. It lists the issues assigned
// to the authenticated user and previews the selected one.
//
// Data comes from the Linear GraphQL API over HTTP, authenticated with a
// personal API key (lin_api_...) from agenda's config. When no token is set
// the view renders a short setup hint instead of fetching.
package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/obliadp/agenda/internal/config"
	"github.com/obliadp/agenda/internal/store"
	"github.com/obliadp/agenda/internal/ui"
)

const endpoint = "https://api.linear.app/graphql"

// --- styles -----------------------------------------------------------------

func fg(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }

var (
	red    = fg("1")
	yellow = fg("3")
	blue   = fg("4")
	grey   = fg("8")
	cyan   = fg("6")
	green  = fg("2")
	purple = fg("5")
	bold   = lipgloss.NewStyle().Bold(true)
	faint  = lipgloss.NewStyle().Faint(true)
)

// hexColor returns a lipgloss style whose foreground is the given Linear hex
// color (which may or may not include a leading '#'), falling back to grey.
func hexStyle(hex string) lipgloss.Style {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return grey
	}
	return fg("#" + hex)
}

// --- data -------------------------------------------------------------------

type label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type issue struct {
	Identifier    string    `json:"identifier"`
	Title         string    `json:"title"`
	URL           string    `json:"url"`
	Priority      int       `json:"priority"`
	PriorityLabel string    `json:"priorityLabel"`
	BranchName    string    `json:"branchName"`
	UpdatedAt     time.Time `json:"updatedAt"`
	Description   string    `json:"description"`
	State         struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Color string `json:"color"`
	} `json:"state"`
	Team struct {
		Key string `json:"key"`
	} `json:"team"`
	Project struct {
		Name string `json:"name"`
	} `json:"project"`
	Assignee struct {
		DisplayName string `json:"displayName"`
	} `json:"assignee"`
	Labels struct {
		Nodes []label `json:"nodes"`
	} `json:"labels"`
	Attachments struct {
		Nodes []attachment `json:"nodes"`
	} `json:"attachments"`
}

// attachment is a Linear attachment; for GitHub PRs the metadata carries the
// PR's status (used as a fallback when the PRs view hasn't loaded the PR).
type attachment struct {
	URL        string `json:"url"`
	SourceType string `json:"sourceType"`
	Title      string `json:"title"`
	Metadata   struct {
		Draft        bool   `json:"draft"`
		Status       string `json:"status"` // open | inReview | merged | closed
		HasConflicts bool   `json:"hasConflicts"`
		Reviews      []struct {
			State string `json:"state"` // approved | changes_requested | ...
		} `json:"reviews"`
	} `json:"metadata"`
}

// toPR builds the store metadata a GitHub PR attachment implies. CI status
// isn't in Linear's data, so it stays unknown (no CI glyph).
func (a attachment) toPR() store.PR {
	p := store.PR{HasConflicts: a.Metadata.HasConflicts}
	switch {
	case a.Metadata.Status == "merged":
		p.State = store.PRMerged
	case a.Metadata.Status == "closed":
		p.State = store.PRClosed
	case a.Metadata.Draft:
		p.State = store.PRDraft
	default:
		p.State = store.PROpen
	}
	changes, approved := false, false
	for _, r := range a.Metadata.Reviews {
		switch r.State {
		case "changes_requested":
			changes = true
		case "approved":
			approved = true
		}
	}
	switch {
	case changes:
		p.Review = store.ReviewChanges
	case approved:
		p.Review = store.ReviewApproved
	case a.Metadata.Status == "inReview":
		p.Review = store.ReviewPending
	}
	return p
}

func (i issue) Filter() string {
	return fmt.Sprintf("%s %s %s %s", i.Identifier, i.State.Name, i.Project.Name, i.Title)
}

// priorityCell renders a one-glyph, color-coded priority indicator. Linear:
// 0 none, 1 urgent, 2 high, 3 medium, 4 low.
func (i issue) priorityCell() string {
	switch i.Priority {
	case 1:
		return red.Bold(true).Render("!")
	case 2:
		return yellow.Render("↑")
	case 3:
		return blue.Render("•")
	case 4:
		return grey.Render("↓")
	default:
		return grey.Render("·")
	}
}

func (i issue) Render(width int, selected bool) string {
	glyphs := i.priorityCell()

	// Metadata: state · identifier (· project).
	plain := i.State.Name + "  " + i.Identifier
	styled := hexStyle(i.State.Color).Render(i.State.Name) + "  " + cyan.Render(i.Identifier)
	if i.Project.Name != "" {
		plain += " · " + i.Project.Name
		styled += grey.Render(" · " + i.Project.Name)
	}

	right := grey.Render(ui.Age(i.UpdatedAt))

	return ui.TwoLineRow(width, selected, glyphs, plain, styled, right, i.Title)
}

// --- messages ---------------------------------------------------------------

type loadedMsg []issue
type errMsg struct{ err error }

// --- view -------------------------------------------------------------------

type View struct {
	token string
	list  ui.List[issue]
	store *store.Store

	loading bool
	err     error

	listW, prevW, height int

	bodyKey string
	body    string

	keys viewKeys
}

type viewKeys struct {
	Open   key.Binding
	Copy   key.Binding
	Branch key.Binding
}

func New(token string, st *store.Store) *View {
	v := &View{
		token:   token,
		store:   st,
		list:    ui.NewList[issue](),
		loading: token != "",
		keys: viewKeys{
			Open:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
			Copy:   key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy url")),
			Branch: key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "copy branch")),
		},
	}
	v.list.SetRowHeight(2) // two-line rows: state/identifier + title
	return v
}

func (v *View) Title() string { return "Linear" }

func (v *View) Init() tea.Cmd {
	if v.token == "" {
		return nil
	}
	return v.fetch()
}

const graphqlQuery = `query {
  viewer {
    assignedIssues(
      first: 100
      filter: { completedAt: { null: true }, canceledAt: { null: true } }
    ) {
      nodes {
        identifier title url priority priorityLabel branchName updatedAt description
        state { name type color }
        team { key }
        project { name }
        assignee { displayName }
        labels(first: 10) { nodes { name color } }
        attachments(first: 20) { nodes { url sourceType title metadata } }
      }
    }
  }
}`

func (v *View) fetch() tea.Cmd {
	token := v.token
	return func() tea.Msg {
		body, _ := json.Marshal(map[string]string{"query": graphqlQuery})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return errMsg{err}
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", token) // personal API keys: no "Bearer"

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return errMsg{err}
		}
		defer resp.Body.Close()

		var out struct {
			Data struct {
				Viewer struct {
					AssignedIssues struct {
						Nodes []issue `json:"nodes"`
					} `json:"assignedIssues"`
				} `json:"viewer"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return errMsg{fmt.Errorf("decoding Linear response: %w", err)}
		}
		if len(out.Errors) > 0 {
			return errMsg{fmt.Errorf("linear: %s", out.Errors[0].Message)}
		}

		issues := out.Data.Viewer.AssignedIssues.Nodes
		sort.SliceStable(issues, func(a, b int) bool {
			return issues[a].UpdatedAt.After(issues[b].UpdatedAt)
		})
		return loadedMsg(issues)
	}
}

func (v *View) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case loadedMsg:
		v.loading = false
		v.err = nil
		v.list.SetItems([]issue(msg))
		return nil
	case errMsg:
		v.loading = false
		v.err = msg.err
		return nil
	case tea.KeyMsg:
		if consumed, cmd := v.list.Update(msg); consumed {
			return cmd
		}
		if v.list.Filtering() {
			return nil
		}
		switch {
		case key.Matches(msg, v.keys.Open):
			return ui.OpenURL(v.list.Selected().URL)
		case key.Matches(msg, v.keys.Copy):
			return copyCmd(v.list.Selected().URL)
		case key.Matches(msg, v.keys.Branch):
			return copyCmd(v.list.Selected().BranchName)
		}
	}
	return nil
}

func (v *View) SetSize(listW, prevW, h int) {
	v.listW, v.prevW, v.height = listW, prevW, h
	v.list.SetSize(listW, max(1, h-1))
	v.bodyKey = ""
}

func (v *View) ListView() string {
	if v.token == "" {
		return faint.Render(v.setupHint())
	}
	header := v.list.FilterLine()
	if header == "" {
		header = faint.Render(v.statusText())
	}
	return header + "\n" + v.list.View()
}

func (v *View) setupHint() string {
	path, _ := config.Path()
	return "Linear isn't configured.\n\n" +
		"Add a personal API key to\n" + path + " :\n\n" +
		"linear:\n  token: lin_api_xxx\n\n" +
		"Create one at linear.app → Settings → Security & access → API keys."
}

func (v *View) statusText() string {
	switch {
	case v.loading:
		return "Loading Linear…"
	case v.err != nil:
		return "Error (ctrl+r to retry)"
	default:
		return fmt.Sprintf("%d issues", v.list.Total())
	}
}

func (v *View) PreviewView() string {
	if v.token == "" {
		return faint.Width(v.prevW).Render(v.setupHint())
	}
	if v.err != nil {
		return red.Width(v.prevW).Render(v.err.Error())
	}
	i := v.list.Selected()
	if i.Identifier == "" {
		return faint.Render("No issue selected.")
	}

	var b strings.Builder
	b.WriteString(bold.Width(v.prevW).Render(i.Title))
	b.WriteByte('\n')
	b.WriteString(grey.Render(fmt.Sprintf("%s · %s ago", i.Identifier, ui.Age(i.UpdatedAt))))
	b.WriteByte('\n')

	statusLine := hexStyle(i.State.Color).Render(i.State.Name)
	if i.PriorityLabel != "" && i.Priority != 0 {
		statusLine += "   " + i.priorityCell() + " " + i.PriorityLabel
	}
	if i.Project.Name != "" {
		statusLine += "   " + grey.Render("◇ "+i.Project.Name)
	}
	b.WriteString(statusLine)
	b.WriteByte('\n')

	if i.BranchName != "" {
		b.WriteString(grey.Render(" " + i.BranchName))
		b.WriteByte('\n')
	}
	if pills := labelPills(i.Labels.Nodes); pills != "" {
		b.WriteString(pills)
		b.WriteByte('\n')
	}

	b.WriteString(grey.Render(strings.Repeat("─", min(v.prevW, 60))))
	b.WriteByte('\n')
	b.WriteString(v.renderedBody(i))
	return b.String()
}

func (v *View) renderedBody(i issue) string {
	desc := strings.TrimSpace(i.Description)
	if desc == "" {
		return faint.Render("(no description)")
	}
	key := i.Identifier + ":" + fmt.Sprint(v.prevW)
	if v.bodyKey == key {
		return v.body
	}
	out := desc
	if r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(max(20, v.prevW)),
	); err == nil {
		if rendered, rerr := r.Render(desc); rerr == nil {
			out = strings.TrimRight(rendered, "\n")
		}
	}
	v.bodyKey, v.body = key, out
	return out
}

func (v *View) Bindings() []key.Binding {
	if v.token == "" {
		return nil
	}
	return []key.Binding{v.keys.Open, v.keys.Copy, v.keys.Branch}
}

func (v *View) Status() string { return grey.Render(v.statusText()) }

func (v *View) InputActive() bool { return v.list.Filtering() }

func (v *View) PreviewKey() string { return v.list.Selected().Identifier }

// RefKind / HasRef / SelectRef implement ui.RefTarget so other views can jump
// to an issue here (e.g. a PR that references it).
func (v *View) RefKind() string { return "linear" }

func matchID(id string) func(issue) bool {
	return func(i issue) bool { return strings.EqualFold(i.Identifier, id) }
}

func (v *View) HasRef(id string) bool    { return v.list.Any(matchID(id)) }
func (v *View) SelectRef(id string) bool { return v.list.Select(matchID(id)) }

// prIcons renders state/CI/review/conflict glyphs for a stored PR, matching
// the PRs view's vocabulary.
func prIcons(p store.PR) string {
	var parts []string
	switch p.State {
	case store.PRMerged:
		parts = append(parts, purple.Render(ui.IconMerged))
	case store.PRClosed:
		parts = append(parts, red.Render(ui.IconClosed))
	case store.PRDraft:
		parts = append(parts, grey.Render(ui.IconDraft))
	default:
		parts = append(parts, green.Render(ui.IconOpen))
	}
	switch p.CI {
	case store.CIPassing:
		parts = append(parts, green.Render(ui.IconCIOK))
	case store.CIFailing:
		parts = append(parts, red.Render(ui.IconCIFail))
	case store.CIPending:
		parts = append(parts, yellow.Render(ui.IconCIPending))
	}
	switch p.Review {
	case store.ReviewApproved:
		parts = append(parts, green.Render(ui.IconApproved))
	case store.ReviewChanges:
		parts = append(parts, red.Render(ui.IconChanges))
	case store.ReviewPending:
		parts = append(parts, yellow.Render(ui.IconReviewReq))
	}
	if p.HasConflicts {
		parts = append(parts, red.Render("⚠"))
	}
	return strings.Join(parts, " ")
}

// prRefLabel builds a PR cross-reference for the picker: status icons and the
// PR title on the main line, with repo#number as the dimmed detail — mirroring
// how the PRs view itself presents a PR.
func prRefLabel(icons, repo string, num int, title, url string) ui.Ref {
	label := icons
	if title != "" {
		label = strings.TrimSpace(icons + "  " + ui.Truncate(title, 60))
	} else {
		label = strings.TrimSpace(fmt.Sprintf("%s  %s#%d", icons, repo, num))
	}
	return ui.Ref{
		Kind:   "pr",
		ID:     url,
		Label:  label,
		Detail: fmt.Sprintf("%s#%d", repo, num),
		URL:    url,
	}
}

// Refs implements ui.Referencer: the GitHub PRs attached to the selected issue
// (with CI/review status icons sourced from the shared store), plus the agent
// sessions that mention the issue.
func (v *View) Refs() []ui.Ref {
	sel := v.list.Selected()
	var refs []ui.Ref
	seen := map[string]bool{}
	for _, a := range sel.Attachments.Nodes {
		repo, num, ok := ui.ParsePRURL(a.URL)
		if a.SourceType != "github" || !ok || seen[a.URL] {
			continue
		}
		seen[a.URL] = true

		// Prefer the PRs view's live status (it has CI) and clean title; fall
		// back to what Linear records in the attachment metadata.
		pr := a.toPR()
		if v.store != nil {
			if sp, ok := v.store.PR(a.URL); ok {
				pr = sp
			}
		}
		title := pr.Title
		if title == "" {
			title = a.Title
		}

		refs = append(refs, prRefLabel(prIcons(pr), repo, num, title, a.URL))
	}
	if v.store != nil {
		for _, s := range v.store.SessionsMentioning(store.Key("linear", sel.Identifier)) {
			refs = append(refs, ui.SessionRef(s.Path, s.Tool, s.Cwd, s.Title, s.Snippet))
		}
	}
	return refs
}

// --- helpers ----------------------------------------------------------------

func labelPills(labels []label) string {
	if len(labels) == 0 {
		return ""
	}
	pills := make([]string, 0, len(labels))
	for _, l := range labels {
		pills = append(pills, hexStyle(l.Color).Render("● ")+l.Name)
	}
	return strings.Join(pills, "  ")
}

func copyCmd(s string) tea.Cmd {
	if s == "" {
		return nil
	}
	return func() tea.Msg {
		c := exec.Command("pbcopy")
		c.Stdin = strings.NewReader(s)
		_ = c.Run()
		return nil
	}
}
