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
| Global | `ctrl+r` | refresh |
| Global | `q` | quit |
| PRs | `enter` · `d` · `y` | open · diff · copy URL |
| Sessions | `enter` · `s` | resume · cycle sort |
| Linear | `enter` · `y` · `b` | open · copy URL · copy branch |

## Views

- **PRs** — fetched via `gh api graphql`. Shows state/CI/review glyphs, `+/−`
  diff size, comments, and labels; preview renders the description with Glamour.
- **Sessions** — scans `~/.claude`, `~/.codex`, and `~/.gemini/antigravity-cli`,
  caching parsed metadata by file signature. `enter` resumes the selected
  session in its original directory; `s` cycles sort (recent / oldest / cwd /
  tool / msgs). Originally a Python tool, ported to Go.
- **Linear** — issues assigned to you (active states), via the Linear GraphQL
  API. Preview shows status, priority, labels, branch name, and the description.

## License

MIT. See gh-dash's [MIT license](https://github.com/dlvhdr/gh-dash/blob/main/LICENSE.txt)
for the project whose ideas this builds on.
