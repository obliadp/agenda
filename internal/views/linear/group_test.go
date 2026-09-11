package linear

import (
	"testing"
	"time"

	"github.com/sanity-labs/agenda/internal/ui"
)

func mkGrouped(id, stateName, stateType, project string, priority int, age time.Duration) issue {
	i := issue{Identifier: id, Priority: priority, UpdatedAt: time.Now().Add(-age)}
	i.State.Name, i.State.Type = stateName, stateType
	i.Project.Name = project
	return i
}

func lanes(items []issue) []string {
	var out []string
	for _, i := range items {
		if i.Separator != "" {
			out = append(out, "sep:"+i.Separator)
		} else {
			out = append(out, i.Identifier)
		}
	}
	return out
}

// grouped mirrors applySort's grouping path for a given sort mode.
func grouped(in []issue, mode sortMode, rev bool) []issue {
	items := sortIssues(in, mode, rev)
	if label := groupLabelFn(mode); label != nil {
		items = ui.InsertGroups(items, label, func(l string) issue { return issue{Separator: l} })
	}
	return items
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestGroupByStatusSort(t *testing.T) {
	in := []issue{
		mkGrouped("A-1", "Backlog", "backlog", "", 0, time.Hour),
		mkGrouped("A-2", "In Progress", "started", "", 0, 2*time.Hour),
		mkGrouped("A-3", "Todo", "unstarted", "", 0, time.Hour),
		mkGrouped("A-4", "In Progress", "started", "", 0, time.Hour),
	}
	eq(t, lanes(grouped(in, sortStatus, false)),
		[]string{"sep:In Progress", "A-4", "A-2", "sep:Todo", "A-3", "sep:Backlog", "A-1"})
	// Reversed sort reverses the lanes with the items, headers still leading.
	eq(t, lanes(grouped(in, sortStatus, true)),
		[]string{"sep:Backlog", "A-1", "sep:Todo", "A-3", "sep:In Progress", "A-2", "A-4"})
}

func TestGroupByProjectSort(t *testing.T) {
	in := []issue{
		mkGrouped("A-1", "Todo", "unstarted", "", 0, 0),
		mkGrouped("A-2", "Todo", "unstarted", "Zebra", 0, 0),
		mkGrouped("A-3", "Todo", "unstarted", "Alpha", 0, 0),
	}
	eq(t, lanes(grouped(in, sortProject, false)),
		[]string{"sep:Alpha", "A-3", "sep:Zebra", "A-2", "sep:no project", "A-1"})
}

func TestGroupByPrioritySort(t *testing.T) {
	in := []issue{
		mkGrouped("A-1", "Todo", "unstarted", "", 0, 0),
		mkGrouped("A-2", "Todo", "unstarted", "", 1, 0),
		mkGrouped("A-3", "Todo", "unstarted", "", 3, 0),
	}
	eq(t, lanes(grouped(in, sortPriority, false)),
		[]string{"sep:Urgent", "A-2", "sep:Medium", "A-3", "sep:No priority", "A-1"})
}

func TestGroupByDateSortUsesTimeBuckets(t *testing.T) {
	in := []issue{
		mkGrouped("A-1", "Todo", "unstarted", "", 0, 40*24*time.Hour),
		mkGrouped("A-2", "Todo", "unstarted", "", 0, 0),
		mkGrouped("A-3", "Todo", "unstarted", "", 0, 5*24*time.Hour),
	}
	// "Today" must not depend on the wall clock crossing midnight: pin A-2
	// to the start of the current calendar day.
	y, m, d := time.Now().Date()
	in[1].UpdatedAt = time.Date(y, m, d, 0, 0, 1, 0, time.Local)
	got := lanes(grouped(in, sortRecent, false))
	want := []string{"sep:Today", "A-2", "sep:Last 7 days", "A-3", "sep:Older", "A-1"}
	eq(t, got, want)
}

func TestGroupingOffStaysFlat(t *testing.T) {
	v := &View{}
	v.list.SetRowHeight(2)
	v.raw = []issue{mkGrouped("A-1", "Todo", "unstarted", "", 0, 0)}
	v.applySort()
	if v.list.Total() != 1 {
		t.Errorf("flat list expected with grouping off, got %d rows", v.list.Total())
	}
	v.Update(ui.GroupingMsg(true))
	if v.list.Total() != 2 {
		t.Errorf("grouping on: expected header + issue, got %d rows", v.list.Total())
	}
	v.Update(ui.GroupingMsg(false))
	if v.list.Total() != 1 {
		t.Errorf("grouping toggled back off: expected flat, got %d rows", v.list.Total())
	}
}
