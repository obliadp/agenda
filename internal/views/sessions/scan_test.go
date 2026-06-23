package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanText(t *testing.T) {
	cases := map[string]string{
		"a  b\n c":   "a b c",
		"  trim  ":   "trim",
		"one\t\ttwo": "one two",
		"":           "",
	}
	for in, want := range cases {
		if got := cleanText(in); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsRealUserText(t *testing.T) {
	real := []string{"fix the bug", "what does this do?"}
	noise := []string{
		"",
		"   ",
		"<system-reminder>do x</system-reminder>",
		`{"tool_use_id": "abc"}`,
		"Caveat: the messages below were generated…",
		"[Request interrupted by user]",
	}
	for _, s := range real {
		if !isRealUserText(s) {
			t.Errorf("isRealUserText(%q) = false, want true", s)
		}
	}
	for _, s := range noise {
		if isRealUserText(s) {
			t.Errorf("isRealUserText(%q) = true, want false", s)
		}
	}
}

func TestTextFromContent(t *testing.T) {
	// String form.
	if got := textFromContent(json.RawMessage(`"hello"`)); got != "hello" {
		t.Errorf("string content = %q, want hello", got)
	}
	// Array of typed blocks; only text/input_text contribute.
	arr := json.RawMessage(`[{"type":"text","text":"a"},{"type":"image","text":"skip"},{"type":"input_text","text":"b"}]`)
	if got := textFromContent(arr); got != "a b" {
		t.Errorf("array content = %q, want %q", got, "a b")
	}
	if got := textFromContent(nil); got != "" {
		t.Errorf("nil content = %q, want empty", got)
	}
}

func TestShortenPath(t *testing.T) {
	h := home()
	cases := map[string]string{
		"":         "?",
		h:          "~",
		h + "/x/y": "~/x/y",
		"/other":   "/other",
	}
	for in, want := range cases {
		if got := shortenPath(in); got != want {
			t.Errorf("shortenPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseClaude(t *testing.T) {
	dir := t.TempDir()
	// ai-title present -> wins over user prompts; tool/reminder lines ignored.
	content := `{"type":"user","cwd":"/home/u/proj","message":{"content":"first prompt"}}
{"type":"assistant","message":{"content":"hi"}}
{"type":"user","message":{"content":"<system-reminder>ignore me</system-reminder>"}}
{"type":"user","message":{"content":"second prompt"}}
{"type":"ai-title","aiTitle":"Generated Title"}
`
	p := writeFile(t, dir, "abc123.jsonl", content)
	m := parseClaude(p)
	if m.Cwd != "/home/u/proj" {
		t.Errorf("Cwd = %q, want /home/u/proj", m.Cwd)
	}
	if m.Msgs != 2 {
		t.Errorf("Msgs = %d, want 2 (reminder line excluded)", m.Msgs)
	}
	if m.Title != "Generated Title" {
		t.Errorf("Title = %q, want ai-title to win", m.Title)
	}
	if m.SessionID != "abc123" {
		t.Errorf("SessionID = %q, want filename stem abc123", m.SessionID)
	}
}

func TestParseClaudeCustomTitleWins(t *testing.T) {
	dir := t.TempDir()
	// A renamed session: /rename writes a custom-title record. It must override
	// the ai-title (which claude -r ignores once you've renamed).
	content := `{"type":"user","cwd":"/home/u/proj","message":{"content":"first prompt"}}
{"type":"ai-title","aiTitle":"Generated Title"}
{"type":"custom-title","customTitle":"my renamed session","sessionId":"abc123"}
`
	m := parseClaude(writeFile(t, dir, "abc123.jsonl", content))
	if m.Title != "my renamed session" {
		t.Errorf("Title = %q, want custom-title to override ai-title", m.Title)
	}
}

func TestParseClaudeLastCustomTitleWins(t *testing.T) {
	dir := t.TempDir()
	// custom-title is re-appended on each load and can change on a later rename;
	// the most recent one wins.
	content := `{"type":"custom-title","customTitle":"old name"}
{"type":"ai-title","aiTitle":"Generated Title"}
{"type":"custom-title","customTitle":"new name"}
`
	m := parseClaude(writeFile(t, dir, "s.jsonl", content))
	if m.Title != "new name" {
		t.Errorf("Title = %q, want last custom-title", m.Title)
	}
}

func TestParseClaudeFallsBackToAiTitleWhenNoCustom(t *testing.T) {
	dir := t.TempDir()
	// No custom-title -> ai-title still wins (unchanged behavior).
	content := `{"type":"user","message":{"content":"a prompt"}}
{"type":"ai-title","aiTitle":"Generated Title"}
`
	m := parseClaude(writeFile(t, dir, "s.jsonl", content))
	if m.Title != "Generated Title" {
		t.Errorf("Title = %q, want ai-title when no custom-title", m.Title)
	}
}

func TestParseClaudeFallsBackToLastPrompt(t *testing.T) {
	dir := t.TempDir()
	content := `{"type":"user","message":{"content":"only prompt"}}
{"type":"user","message":{"content":"latest prompt"}}
`
	m := parseClaude(writeFile(t, dir, "s.jsonl", content))
	if m.Title != "latest prompt" {
		t.Errorf("Title = %q, want last user prompt when no ai-title", m.Title)
	}
}

func TestParseClaudeFallsBackToFirstPrompt(t *testing.T) {
	dir := t.TempDir()
	// A single user prompt and no titles: first == last, so the first-prompt
	// fallback is what surfaces.
	content := `{"type":"user","message":{"content":"only prompt"}}
`
	m := parseClaude(writeFile(t, dir, "s.jsonl", content))
	if m.Title != "only prompt" {
		t.Errorf("Title = %q, want the sole user prompt", m.Title)
	}
}

func TestParseCodex(t *testing.T) {
	dir := t.TempDir()
	content := `{"type":"session_meta","payload":{"cwd":"/x/y","id":"sess-42"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello there"}]}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"text","text":"hi"}]}}
`
	m := parseCodex(writeFile(t, dir, "rollout-x.jsonl", content))
	if m.Cwd != "/x/y" {
		t.Errorf("Cwd = %q, want /x/y", m.Cwd)
	}
	if m.SessionID != "sess-42" {
		t.Errorf("SessionID = %q, want sess-42 from meta", m.SessionID)
	}
	if m.Msgs != 1 {
		t.Errorf("Msgs = %d, want 1 (only user messages counted)", m.Msgs)
	}
	if m.Title != "hello there" {
		t.Errorf("Title = %q, want first user prompt", m.Title)
	}
}

func TestConversationTurnsClaude(t *testing.T) {
	dir := t.TempDir()
	content := `{"type":"user","message":{"content":"q1"}}
{"type":"assistant","message":{"content":"a1"}}
{"type":"user","message":{"content":"<system-reminder>skip</system-reminder>"}}
{"type":"user","message":{"content":"q2"}}
`
	turns := conversationTurns(writeFile(t, dir, "s.jsonl", content), toolClaude)
	want := []turn{{"user", "q1"}, {"assistant", "a1"}, {"user", "q2"}}
	if len(turns) != len(want) {
		t.Fatalf("got %d turns, want %d: %+v", len(turns), len(want), turns)
	}
	for i, w := range want {
		if turns[i] != w {
			t.Errorf("turn %d = %+v, want %+v", i, turns[i], w)
		}
	}
}

func TestScanMentions(t *testing.T) {
	dir := t.TempDir()
	content := `{"type":"user","message":{"content":"working on SRE-4419 today"}}
{"type":"assistant","message":{"content":"see https://github.com/sanity-io/argocd-apps/pull/7314 for the change"}}
{"type":"user","message":{"content":"SRE-4419 again, should be deduped"}}
`
	ms := scanMentions(writeFile(t, dir, "s.jsonl", content), toolClaude)

	var linear, pr *mention
	for i := range ms {
		switch ms[i].Kind {
		case "linear":
			linear = &ms[i]
		case "pr":
			pr = &ms[i]
		}
	}
	if linear == nil || linear.ID != "SRE-4419" {
		t.Errorf("linear mention = %+v, want SRE-4419", linear)
	}
	if linear != nil && linear.Snippet == "" {
		t.Error("linear mention has no context snippet")
	}
	if pr == nil || pr.ID != "https://github.com/sanity-io/argocd-apps/pull/7314" {
		t.Errorf("pr mention = %+v, want the PR URL", pr)
	}
	// SRE-4419 appears twice but should be recorded once.
	count := 0
	for _, m := range ms {
		if m.ID == "SRE-4419" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("SRE-4419 recorded %d times, want 1 (deduped)", count)
	}
}

func TestSortSessions(t *testing.T) {
	base := time.Now()
	in := []session{
		{meta: meta{Cwd: "/b", Msgs: 1}, Tool: toolCodex, MTime: base.Add(-2 * time.Hour)},
		{meta: meta{Cwd: "/a", Msgs: 9}, Tool: toolClaude, MTime: base.Add(-1 * time.Hour)},
		{meta: meta{Cwd: "/c", Msgs: 5}, Tool: toolClaude, MTime: base.Add(-3 * time.Hour)},
	}

	recent := sortSessions(in, sortRecent)
	if !recent[0].MTime.After(recent[1].MTime) || !recent[1].MTime.After(recent[2].MTime) {
		t.Error("sortRecent not newest-first")
	}
	oldest := sortSessions(in, sortOldest)
	if !oldest[0].MTime.Before(oldest[2].MTime) {
		t.Error("sortOldest not oldest-first")
	}
	msgs := sortSessions(in, sortMsgs)
	if msgs[0].Msgs != 9 {
		t.Errorf("sortMsgs[0].Msgs = %d, want 9 (highest first)", msgs[0].Msgs)
	}
	byCwd := sortSessions(in, sortCwd)
	if byCwd[0].Cwd != "/a" || byCwd[2].Cwd != "/c" {
		t.Errorf("sortCwd order = %q..%q, want /a../c", byCwd[0].Cwd, byCwd[2].Cwd)
	}
	// Original slice must be untouched (sortSessions copies).
	if in[0].Cwd != "/b" {
		t.Error("sortSessions mutated its input")
	}
}
