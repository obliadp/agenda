package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/obliadp/agenda/internal/store"
)

// These ref builders live here so every view renders cross-references the same
// way: a PR shows status glyphs + title, an issue shows its title on a second
// line, a session shows its agent icon + a context snippet (see SessionRef).

func refFg(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)) }

// PRIcons renders state/CI/review/conflict glyphs for a stored PR. It returns
// "" when the PR's status is unknown (a zero record), so callers that only have
// a URL don't show a misleading "open" glyph.
func PRIcons(p store.PR) string {
	if p.State == "" {
		return ""
	}
	var parts []string
	switch p.State {
	case store.PRMerged:
		parts = append(parts, refFg("5").Render(IconMerged))
	case store.PRClosed:
		parts = append(parts, refFg("1").Render(IconClosed))
	case store.PRDraft:
		parts = append(parts, refFg("8").Render(IconDraft))
	default:
		parts = append(parts, refFg("2").Render(IconOpen))
	}
	switch p.CI {
	case store.CIPassing:
		parts = append(parts, refFg("2").Render(IconCIOK))
	case store.CIFailing:
		parts = append(parts, refFg("1").Render(IconCIFail))
	case store.CIPending:
		parts = append(parts, refFg("3").Render(IconCIPending))
	}
	switch p.Review {
	case store.ReviewApproved:
		parts = append(parts, refFg("2").Render(IconApproved))
	case store.ReviewChanges:
		parts = append(parts, refFg("1").Render(IconChanges))
	case store.ReviewPending:
		parts = append(parts, refFg("3").Render(IconReviewReq))
	}
	if p.HasConflicts {
		parts = append(parts, refFg("1").Render("⚠"))
	}
	return strings.Join(parts, " ")
}

// PRRef builds a PR cross-reference: status glyphs and the title on the main
// line, with repo#number as the dimmed detail. When the title is unknown the
// repo#number moves up to the main line (no redundant second line).
func PRRef(p store.PR, repo string, num int, title, url string) Ref {
	icons := PRIcons(p)
	repoNum := fmt.Sprintf("%s#%d", repo, num)
	ref := Ref{Kind: "pr", ID: url, URL: url}
	if title != "" {
		ref.Label = strings.TrimSpace(icons + "  " + Truncate(title, 60))
		ref.Detail = repoNum
	} else {
		ref.Label = strings.TrimSpace(icons + "  " + repoNum)
	}
	return ref
}

// IssueRef builds a Linear-issue cross-reference: the identifier on the main
// line, the issue title on the dimmed second line.
func IssueRef(id, title string) Ref {
	return Ref{Kind: "linear", ID: id, Label: "Linear  " + id, Detail: title}
}
