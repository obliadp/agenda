package linear

import (
	"testing"

	"github.com/sanity-labs/agenda/internal/config"
)

func TestBuildFilterDefault(t *testing.T) {
	f := buildFilter(config.LinearFilter{})
	if _, ok := f["completedAt"]; !ok {
		t.Error("default filter must exclude completed issues")
	}
	if _, ok := f["canceledAt"]; !ok {
		t.Error("default filter must exclude canceled issues")
	}
	if len(f) != 2 {
		t.Errorf("default filter = %v, want only the two exclusions", f)
	}
}

func TestBuildFilterOptions(t *testing.T) {
	f := buildFilter(config.LinearFilter{
		IncludeCompleted: true,
		Teams:            []string{"SRE"},
		States:           []string{"In Progress", "Todo"},
	})
	if _, ok := f["completedAt"]; ok {
		t.Error("include_completed should drop the completedAt exclusion")
	}
	if _, ok := f["canceledAt"]; !ok {
		t.Error("canceled issues stay excluded unless opted in")
	}
	team := f["team"].(map[string]any)["key"].(map[string]any)["in"].([]string)
	if len(team) != 1 || team[0] != "SRE" {
		t.Errorf("team filter = %v, want [SRE]", team)
	}
	states := f["state"].(map[string]any)["name"].(map[string]any)["in"].([]string)
	if len(states) != 2 {
		t.Errorf("state filter = %v, want two states", states)
	}
}
