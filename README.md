# agenda

[![CI](https://github.com/obliadp/agenda/actions/workflows/ci.yml/badge.svg)](https://github.com/obliadp/agenda/actions/workflows/ci.yml)

A terminal dashboard that unifies the things you keep checking into one TUI you
tab between:

- **PRs** — your open GitHub pull requests
- **Sessions** — your local agent sessions (Claude Code, Codex, Antigravity)
- **Linear** — your assigned Linear issues

Each is a distinct *view*; switch with `tab` / `shift+tab`. Every view shares
the same two-line row layout, fuzzy filter, and scrollable markdown preview.

Built with [Bubble Tea v2](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss), and
[Glamour](https://github.com/charmbracelet/glamour).

## Credit: gh-dash

agenda's PR view — and much of its overall design — is **heavily inspired by
[gh-dash](https://github.com/dlvhdr/gh-dash)** by [Dolev Hadar](https://github.com/dlvhdr)
(MIT licensed). agenda doesn't vendor or copy gh-dash's code; it was built fresh
by studying gh-dash's source and reimplementing the ideas. Specifically, the
following are modeled on gh-dash:

- **The tabbed-views architecture** — a root model that hosts a slice of
  interchangeable views, each owning its own list, data fetch, and preview
  (gh-dash calls these "sections").
- **The two-line ("non-compact") row layout** — a dimmed metadata line over a
  bold title line, with the selection indicator spanning both.
- **Fetching PRs via the GitHub GraphQL API** rather than the search/REST JSON,
  so rows can show CI check rollup, review decision, diff size, comments, and
  mergeability — none of which `gh search prs --json` exposes.
- **The status-glyph vocabulary** — the state / CI / review Nerd Font icons.
- **Rendering issue/PR bodies with Glamour** in the preview pane.

If you work primarily inside a single repo and want the full-featured original,
use gh-dash. agenda's niche is unifying PRs *plus* local agent sessions *plus*
Linear in one switcher.

## Install

```sh
go install github.com/obliadp/agenda@latest
```

Requirements:
- A **Nerd Font** in your terminal (for the status glyphs) — same as gh-dash.
- The **`gh` CLI**, authenticated (`gh auth login`) — powers the PRs view.

## Configuration

Config lives at `$XDG_CONFIG_HOME/agenda/config.yml` (defaults to
`~/.config/agenda/config.yml`). It's optional — agenda runs with sensible
defaults, and ships no personal details in the binary. See
[`config.example.yml`](./config.example.yml) for all options.

The only view that needs setup is **Linear**: add a personal API key
(linear.app → Settings → Security & access → API keys):

```yaml
linear:
  token: lin_api_xxx
```

## Keys

| Scope | Key | Action |
|-------|-----|--------|
| Global | `tab` / `shift+tab` | switch view |
| Global | `/` | fuzzy filter |
| Global | `j`/`k`, `g`/`G`, `ctrl+d`/`ctrl+u` | navigate list |
| Global | `shift+↑`/`shift+↓`, `PgUp`/`PgDn` | scroll preview |
| Global | `l` | follow a cross-reference to a related item (picker if several) |
| Global | `ctrl+r` | refresh |
| Global | `q` | quit |
| PRs | `enter` · `d` · `y` | open · diff · copy URL |
| Sessions | `enter` · `s` | resume · cycle sort |
| Linear | `enter` · `y` · `b` | open · copy URL · copy branch |

## Views

- **PRs** — fetched via `gh api graphql`. Shows state/CI/review glyphs, `+/−`
  diff size, comments, and labels; preview renders the description with Glamour.
- **Sessions** — scans `~/.claude`, `~/.codex`, and `~/.gemini/antigravity-cli`,
  caching parsed metadata by file signature. Each agent is shown as a Nerd Font
  icon (claude = robot, codex = code, antigravity = rocket) rather than its
  name. `enter` resumes the selected session in its original directory; `s`
  cycles sort (recent / oldest / cwd / tool / msgs). Originally a Python tool,
  ported to Go.
- **Linear** — issues assigned to you (active states), via the Linear GraphQL
  API. Preview shows status, priority, labels, branch name, and the description.

## Cross-references

Views link to each other and `l` follows the link, in every direction:

- **PR** → the Linear issue it references (from the title, branch, or body),
  shown with the issue's title on a second line.
- **Linear issue** → the GitHub PRs attached to it (each shown with its title
  and live state/CI/review icons) and the agent **sessions** that mention it.
- **Session** → the issues and PRs its conversation mentions (rendered like the
  other views — issue titles and PR status icons/titles from the store).
- **PR / issue** → the **sessions** that mention them, each with a dimmed line
  of context from the session.

A picker lists the targets (always, even for a single one, so navigation never
happens without a prompt), with issue/PR references grouped above a `sessions`
separator. References that resolve to a loaded item jump in-app;
ones that don't (e.g. a merged PR, or a PR by someone else) open in the browser,
marked with `↗`. References that resolve to nothing — like regex false-positives
with no URL — are dropped.

### How it fits together

A small shared **metadata store** (`internal/store`) decouples the views: each
publishes the facts it owns — the PRs view publishes pull-request status, the
sessions view publishes which issues/PRs each session mentions — and any view
reads the others' to enrich its display. That's how the Linear view shows CI
icons for a PR (data the PRs view has) and lists the sessions referencing an
issue (data the sessions view has), without depending on those packages.

The cross-reference wiring itself is generic: a view exposes links by
implementing `Referencer`, and becomes a jump destination by implementing
`RefTarget`. Adding a new link type is just implementing those interfaces and,
if needed, publishing to the store — no changes to the core.

## License

MIT. See gh-dash's [MIT license](https://github.com/dlvhdr/gh-dash/blob/main/LICENSE.txt)
for the project whose ideas this builds on.
