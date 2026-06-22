// Package prs is agenda's GitHub pull-requests view. It lists the PRs matching
// a configurable search query and previews the selected one.
//
// Data comes from the GitHub GraphQL API via `gh api graphql`, which reuses
// the user's existing gh auth and — unlike `gh search prs --json` — exposes
// the rich fields that make the view useful: CI check rollup, review decision,
// diff size, comment count, mergeability, and colored labels. This mirrors the
// approach gh-dash takes.
package prs

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/obliadp/agenda/internal/cache"
	"github.com/obliadp/agenda/internal/store"
	"github.com/obliadp/agenda/internal/ui"
)

// --- styles -----------------------------------------------------------------

func fg(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }

var (
	green  = fg("2")
	red    = fg("1")
	yellow = fg("3")
	grey   = fg("8")
	cyan   = fg("6")
	purple = fg("5")
	bold   = lipgloss.NewStyle().Bold(true)
	faint  = lipgloss.NewStyle().Faint(true)
)

// --- data -------------------------------------------------------------------

type label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// pr is one row, decoded from the GraphQL search result.
type pr struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	State          string    `json:"state"`
	IsDraft        bool      `json:"isDraft"`
	UpdatedAt      time.Time `json:"updatedAt"`
	HeadRefName    string    `json:"headRefName"`
	Additions      int       `json:"additions"`
	Deletions      int       `json:"deletions"`
	Mergeable      string    `json:"mergeable"`
	ReviewDecision string    `json:"reviewDecision"`
	Body           string    `json:"body"`
	Author         struct {
		Login string `json:"login"`
	} `json:"author"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Comments struct {
		TotalCount int `json:"totalCount"`
	} `json:"comments"`
	Labels struct {
		Nodes []label `json:"nodes"`
	} `json:"labels"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

func (p pr) repo() string { return p.Repository.NameWithOwner }

func (p pr) ciState() string {
	if len(p.Commits.Nodes) == 0 {
		return ""
	}
	return p.Commits.Nodes[0].Commit.StatusCheckRollup.State
}

func (p pr) Filter() string {
	return fmt.Sprintf("%s #%d %s", p.repo(), p.Number, p.Title)
}

// linearRefRe matches a Linear issue identifier (team key + number), e.g.
// "SRE-4228" in a title or "sre-3686" in a branch name like
// "orjan/sre-3686-add-foo". The team key is letters only, so version-ish
// tokens like "v2-foo" don't match.
var linearRefRe = regexp.MustCompile(`(?i)\b([a-z]{2,}-\d+)\b`)

// linearRefs returns the Linear identifiers this PR references (uppercased,
// de-duplicated, in order of appearance). It scans the title, branch, then
// body — the places a Linear issue is conventionally named.
func (p pr) linearRefs() []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range []string{p.Title, p.HeadRefName, p.Body} {
		for _, m := range linearRefRe.FindAllStringSubmatch(s, -1) {
			id := strings.ToUpper(m[1])
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// --- icon rendering ---------------------------------------------------------

func (p pr) stateIcon() string {
	switch {
	case p.IsDraft:
		return grey.Render(ui.IconDraft)
	case p.State == "MERGED":
		return purple.Render(ui.IconMerged)
	case p.State == "CLOSED":
		return red.Render(ui.IconClosed)
	default:
		return green.Render(ui.IconOpen)
	}
}

func (p pr) ciIcon() string {
	switch p.ciState() {
	case "SUCCESS":
		return green.Render(ui.IconCIOK)
	case "FAILURE", "ERROR":
		return red.Render(ui.IconCIFail)
	case "PENDING", "EXPECTED":
		return yellow.Render(ui.IconCIPending)
	default:
		return grey.Render(ui.IconDot)
	}
}

func (p pr) reviewIcon() string {
	switch p.ReviewDecision {
	case "APPROVED":
		return green.Render(ui.IconApproved)
	case "CHANGES_REQUESTED":
		return red.Render(ui.IconChanges)
	case "REVIEW_REQUIRED":
		return yellow.Render(ui.IconReviewReq)
	default:
		return grey.Render(ui.IconDot)
	}
}

func (p pr) diffCell() string {
	if p.Additions == 0 && p.Deletions == 0 {
		return ""
	}
	return green.Render("+"+strconv.Itoa(p.Additions)) + " " +
		red.Render("-"+strconv.Itoa(p.Deletions))
}

func (p pr) commentsCell() string {
	if p.Comments.TotalCount == 0 {
		return ""
	}
	return grey.Render(fmt.Sprintf("%s%d", ui.IconComment, p.Comments.TotalCount))
}

// Render draws one PR as a two-line block, à la gh-dash's non-compact layout:
//
//	▌  ● ● ●  repo #123 · @author · branch          +12 -3  3  2d
//	          The pull request title, in bold
//
// Line one is dimmed metadata with the status glyphs; line two is the title,
// indented to align under the metadata. The selected row gets an accent bar on
// both lines (rather than a full-row background, which lipgloss's per-segment
// resets would clobber).
func (p pr) Render(width int, selected bool) string {
	glyphs := p.stateIcon() + " " + p.ciIcon() + " " + p.reviewIcon()

	// Right cluster: diff · comments · age.
	right := strings.TrimSpace(p.diffCell() + "  " + p.commentsCell() + "  " + grey.Render(ui.Age(p.UpdatedAt)))

	// Metadata: repo #num · @author · branch (plain for measurement/truncation,
	// styled for display).
	plain := fmt.Sprintf("%s #%d", p.repo(), p.Number)
	styled := cyan.Render(p.repo()) + yellow.Render(fmt.Sprintf(" #%d", p.Number))
	if p.Author.Login != "" {
		plain += " · @" + p.Author.Login
		styled += grey.Render(" · @" + p.Author.Login)
	}
	if p.HeadRefName != "" {
		plain += " · " + p.HeadRefName
		styled += grey.Render(" · " + p.HeadRefName)
	}

	return ui.TwoLineRow(width, selected, glyphs, plain, styled, right, p.Title)
}

// --- messages ---------------------------------------------------------------

type loadedMsg []pr
type errMsg struct{ err error }

// --- view -------------------------------------------------------------------

type View struct {
	filter string
	list   ui.List[pr]
	store  *store.Store

	loading bool
	err     error

	listW, prevW, height int

	// memoized glamour render of the selected PR's body, keyed by number+width
	// so it isn't re-rendered every frame.
	bodyKey string
	body    string

	keys viewKeys
}

type viewKeys struct {
	Open key.Binding
	Copy key.Binding
	Diff key.Binding
}

func New(filter string, st *store.Store) *View {
	v := &View{
		filter:  filter,
		store:   st,
		list:    ui.NewList[pr](),
		loading: true,
		keys: viewKeys{
			Open: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
			Copy: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy url")),
			Diff: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "diff")),
		},
	}
	v.list.SetRowHeight(2) // two-line rows: metadata + title

	// Paint last run's PRs immediately; the live fetch refreshes them.
	if cached, ok := cache.Load[[]pr](cacheName); ok && len(cached) > 0 {
		v.list.SetItems(cached)
		v.publish(cached)
		v.loading = false
	}
	return v
}

const cacheName = "prs"

func (v *View) Title() string { return "PRs" }

func (v *View) Init() tea.Cmd { return v.fetch() }

const graphqlQuery = `query($q: String!) {
  search(query: $q, type: ISSUE, first: 100) {
    nodes {
      ... on PullRequest {
        number title url state isDraft updatedAt headRefName
        additions deletions mergeable reviewDecision body
        author { login }
        repository { nameWithOwner }
        comments { totalCount }
        labels(first: 6) { nodes { name color } }
        commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
      }
    }
  }
}`

func (v *View) fetch() tea.Cmd {
	q := v.filter
	if !strings.Contains(q, "is:pr") {
		q = strings.TrimSpace(q + " is:pr")
	}
	return func() tea.Msg {
		out, err := exec.Command("gh", "api", "graphql",
			"-f", "query="+graphqlQuery,
			"-f", "q="+q,
			"--jq", ".data.search.nodes",
		).Output()
		if err != nil {
			return errMsg{fmt.Errorf("gh api graphql: %w", cmdErr(err))}
		}
		var prs []pr
		if err := json.Unmarshal(out, &prs); err != nil {
			return errMsg{fmt.Errorf("parsing gh output: %w", err)}
		}
		// type:ISSUE can include non-PR nodes as empty objects; drop them.
		kept := prs[:0]
		for _, p := range prs {
			if p.Number != 0 {
				kept = append(kept, p)
			}
		}
		return loadedMsg(kept)
	}
}

func (v *View) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case loadedMsg:
		v.loading = false
		v.err = nil
		v.list.SetItems([]pr(msg))
		v.publish([]pr(msg))
		_ = cache.Save(cacheName, []pr(msg))
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
			return v.openSelected()
		case key.Matches(msg, v.keys.Copy):
			return v.copySelected()
		case key.Matches(msg, v.keys.Diff):
			return v.diffSelected()
		}
	}
	return nil
}

// Refs implements ui.Referencer: the Linear issues this PR points at, plus the
// agent sessions that mention this PR (sourced from the shared store).
func (v *View) Refs() []ui.Ref {
	sel := v.list.Selected()
	var refs []ui.Ref
	for _, id := range sel.linearRefs() {
		var title, url string
		if v.store != nil {
			if iss, ok := v.store.Issue(id); ok {
				title, url = iss.Title, iss.URL
			}
		}
		refs = append(refs, ui.IssueRef(id, title, url))
	}
	if v.store != nil && sel.URL != "" {
		for _, s := range v.store.SessionsMentioning(store.Key("pr", sel.URL)) {
			refs = append(refs, ui.SessionRef(s.Path, s.Tool, s.Cwd, s.Title, s.Snippet))
		}
	}
	return refs
}

// publish pushes the loaded PRs' status into the shared store so other views
// (Linear) can render CI/review/merge icons for PRs they reference.
func (v *View) publish(prs []pr) {
	if v.store == nil {
		return
	}
	recs := make([]store.PR, 0, len(prs))
	for _, p := range prs {
		recs = append(recs, store.PR{
			URL:          p.URL,
			Repo:         p.repo(),
			Number:       p.Number,
			Title:        p.Title,
			State:        prState(p),
			CI:           ciState(p),
			Review:       reviewState(p),
			HasConflicts: p.Mergeable == "CONFLICTING",
			UpdatedAt:    p.UpdatedAt,
		})
	}
	v.store.PutPRs(recs)
}

func prState(p pr) store.PRState {
	switch {
	case p.IsDraft:
		return store.PRDraft
	case p.State == "MERGED":
		return store.PRMerged
	case p.State == "CLOSED":
		return store.PRClosed
	default:
		return store.PROpen
	}
}

func ciState(p pr) store.CIState {
	switch p.ciState() {
	case "SUCCESS":
		return store.CIPassing
	case "FAILURE", "ERROR":
		return store.CIFailing
	case "PENDING", "EXPECTED":
		return store.CIPending
	default:
		return store.CIUnknown
	}
}

func reviewState(p pr) store.ReviewState {
	switch p.ReviewDecision {
	case "APPROVED":
		return store.ReviewApproved
	case "CHANGES_REQUESTED":
		return store.ReviewChanges
	case "REVIEW_REQUIRED":
		return store.ReviewPending
	default:
		return store.ReviewNone
	}
}

// RefKind / HasRef / SelectRef implement ui.RefTarget so other views (e.g.
// Linear) can jump to a PR here. PRs are keyed by URL.
func (v *View) RefKind() string { return "pr" }

func matchURL(url string) func(pr) bool {
	return func(p pr) bool { return p.URL == url }
}

func (v *View) HasRef(id string) bool    { return v.list.Any(matchURL(id)) }
func (v *View) SelectRef(id string) bool { return v.list.Select(matchURL(id)) }

func (v *View) openSelected() tea.Cmd {
	p := v.list.Selected()
	if p.URL == "" {
		return nil
	}
	return func() tea.Msg {
		_ = exec.Command("gh", "pr", "view", "--web",
			strconv.Itoa(p.Number), "-R", p.repo()).Start()
		return nil
	}
}

func (v *View) copySelected() tea.Cmd {
	p := v.list.Selected()
	if p.URL == "" {
		return nil
	}
	return func() tea.Msg {
		c := exec.Command("pbcopy")
		c.Stdin = strings.NewReader(p.URL)
		_ = c.Run()
		return nil
	}
}

func (v *View) diffSelected() tea.Cmd {
	p := v.list.Selected()
	if p.URL == "" {
		return nil
	}
	c := exec.Command("sh", "-c",
		fmt.Sprintf("gh pr diff %d -R %s | less -R", p.Number, p.repo()))
	return tea.ExecProcess(c, func(error) tea.Msg { return nil })
}

func (v *View) SetSize(listW, prevW, h int) {
	v.listW, v.prevW, v.height = listW, prevW, h
	v.list.SetSize(listW, max(1, h-1)) // reserve a row for the header line
	v.bodyKey = ""                     // width changed: invalidate the body cache
}

func (v *View) ListView() string {
	header := v.list.FilterLine()
	if header == "" {
		header = faint.Render(v.statusText())
	}
	return header + "\n" + v.list.View()
}

func (v *View) statusText() string {
	switch {
	case v.loading:
		return "Loading PRs…"
	case v.err != nil:
		return "Error (ctrl+r to retry)"
	default:
		return fmt.Sprintf("%d PRs", v.list.Total())
	}
}

func (v *View) PreviewView() string {
	if v.err != nil {
		return red.Width(v.prevW).Render(v.err.Error())
	}
	p := v.list.Selected()
	if p.URL == "" {
		return faint.Render("No PR selected.")
	}

	var b strings.Builder
	b.WriteString(bold.Width(v.prevW).Render(p.Title))
	b.WriteString("\n")
	b.WriteString(grey.Render(fmt.Sprintf("%s #%d  ·  @%s  ·  %s ago",
		p.repo(), p.Number, p.Author.Login, ui.Age(p.UpdatedAt))))
	b.WriteString("\n\n")

	// Status line: state · CI · review · diff · comments.
	fmt.Fprintf(&b, "%s %s   %s %s   %s %s\n",
		p.stateIcon(), stateWord(p), p.ciIcon(), ciWord(p), p.reviewIcon(), reviewWord(p))
	if d := p.diffCell(); d != "" {
		b.WriteString(d)
		b.WriteString("   ")
	}
	if c := p.commentsCell(); c != "" {
		b.WriteString(c)
	}
	if p.Mergeable == "CONFLICTING" {
		b.WriteString("   ")
		b.WriteString(red.Render("⚠ conflicts"))
	}
	b.WriteString("\n")

	if pills := labelPills(p.Labels.Nodes); pills != "" {
		b.WriteString(pills)
		b.WriteByte('\n')
	}

	b.WriteString(grey.Render(strings.Repeat("─", min(v.prevW, 60))))
	b.WriteString("\n")
	b.WriteString(v.renderedBody(p))
	return b.String()
}

// renderedBody returns the glamour-rendered PR body, memoized per (PR, width).
func (v *View) renderedBody(p pr) string {
	body := strings.TrimSpace(p.Body)
	if body == "" {
		return faint.Render("(no description)")
	}
	key := fmt.Sprintf("%d:%d", p.Number, v.prevW)
	if v.bodyKey == key {
		return v.body
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(max(20, v.prevW)),
	)
	out := body
	if err == nil {
		if rendered, rerr := r.Render(body); rerr == nil {
			out = strings.TrimRight(rendered, "\n")
		}
	}
	v.bodyKey, v.body = key, out
	return out
}

func (v *View) Bindings() []key.Binding {
	return []key.Binding{v.keys.Open, v.keys.Diff, v.keys.Copy}
}

func (v *View) Status() string {
	return grey.Render(v.statusText())
}

func (v *View) InputActive() bool { return v.list.Filtering() }

func (v *View) PreviewKey() string { return v.list.Selected().URL }

// --- preview text helpers ---------------------------------------------------

func stateWord(p pr) string {
	switch {
	case p.IsDraft:
		return grey.Render("draft")
	case p.State == "MERGED":
		return purple.Render("merged")
	case p.State == "CLOSED":
		return red.Render("closed")
	default:
		return green.Render("open")
	}
}

func ciWord(p pr) string {
	switch p.ciState() {
	case "SUCCESS":
		return green.Render("checks passing")
	case "FAILURE", "ERROR":
		return red.Render("checks failing")
	case "PENDING", "EXPECTED":
		return yellow.Render("checks running")
	default:
		return grey.Render("no checks")
	}
}

func reviewWord(p pr) string {
	switch p.ReviewDecision {
	case "APPROVED":
		return green.Render("approved")
	case "CHANGES_REQUESTED":
		return red.Render("changes requested")
	case "REVIEW_REQUIRED":
		return yellow.Render("review required")
	default:
		return grey.Render("no review")
	}
}

func labelPills(labels []label) string {
	if len(labels) == 0 {
		return ""
	}
	pills := make([]string, 0, len(labels))
	for _, l := range labels {
		style := lipgloss.NewStyle().Padding(0, 1)
		if c := "#" + l.Color; len(l.Color) == 6 {
			style = style.Background(lipgloss.Color(c)).Foreground(contrastFg(l.Color))
		}
		pills = append(pills, style.Render(l.Name))
	}
	return strings.Join(pills, " ")
}

// contrastFg picks black or white text for a hex background by luminance.
func contrastFg(hex string) color.Color {
	if len(hex) != 6 {
		return lipgloss.Color("15")
	}
	r, _ := strconv.ParseInt(hex[0:2], 16, 0)
	g, _ := strconv.ParseInt(hex[2:4], 16, 0)
	bl, _ := strconv.ParseInt(hex[4:6], 16, 0)
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)
	if lum > 140 {
		return lipgloss.Color("0")
	}
	return lipgloss.Color("15")
}

// cmdErr unwraps *exec.ExitError to surface stderr in the message.
func cmdErr(err error) error {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}
