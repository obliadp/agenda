package prs

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sanity-labs/agenda/internal/ui"
)

func viewWithThread(t *testing.T) *View {
	t.Helper()
	v := &View{pane: paneDiff}
	v.list.SetRowHeight(2)
	v.raw = []pr{mkPR("u1", "theirs", 0)}
	v.applySort()
	th := mkThread("t1", "foo.go", intp(3), false, false, "hm")
	st := &commentsState{done: true}
	st.data.ReviewThreads.Nodes = []prThread{th}
	v.comments = map[string]*commentsState{"u1": st}
	v.anchors = []ui.DiffAnchor{{Line: 5, ID: "t1"}}
	return v
}

func TestCurrentThread(t *testing.T) {
	v := viewWithThread(t)
	th, ok := v.currentThread()
	if !ok || th.ID != "t1" {
		t.Fatalf("currentThread = %+v ok=%v, want t1", th, ok)
	}
	v.pane = paneBody
	if _, ok := v.currentThread(); ok {
		t.Error("currentThread should require a data pane")
	}
}

func TestThreadInputFlow(t *testing.T) {
	v := viewWithThread(t)
	v.input = &threadFlow{kind: "reply", threadID: "t1", target: "foo.go:3"}
	if cmd := v.updateThreadInput(special(tea.KeyEnter)); cmd != nil {
		t.Error("empty body should not post")
	}
	for _, r := range "lgtm" {
		v.updateThreadInput(press(r))
	}
	if cmd := v.updateThreadInput(special(tea.KeyEnter)); cmd == nil {
		t.Error("non-empty body should post")
	}
	if !v.input.submitting {
		t.Error("input not marked submitting")
	}
	v.Update(threadDoneMsg{url: "u1", what: "replied"})
	if v.input != nil {
		t.Error("input should close on completion")
	}
	// The cache entry is replaced by an in-flight refetch.
	if st, ok := v.comments["u1"]; ok && st.done {
		t.Error("comments cache should be refetching, not still done")
	}
}
