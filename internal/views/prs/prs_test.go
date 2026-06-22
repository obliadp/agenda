package prs

import (
	"slices"
	"testing"
)

func TestLinearRefs(t *testing.T) {
	cases := []struct {
		name string
		pr   pr
		want []string
	}{
		{
			name: "uppercase in title parens",
			pr:   pr{Title: "docs: plan for shard migration (SRE-4419)"},
			want: []string{"SRE-4419"},
		},
		{
			name: "lowercase in title prefix",
			pr:   pr{Title: "ci(sre-4228): migrate to orb"},
			want: []string{"SRE-4228"},
		},
		{
			name: "from branch when title has none",
			pr:   pr{Title: "feat: add gateway routes", HeadRefName: "orjan/sre-3717-add-gateway"},
			want: []string{"SRE-3717"},
		},
		{
			name: "multiple refs in order, de-duplicated",
			pr:   pr{Title: "ENG-1 and ENG-2", Body: "also ENG-1 again and OPS-9"},
			want: []string{"ENG-1", "ENG-2", "OPS-9"},
		},
		{
			name: "title before branch before body",
			pr:   pr{Title: "do ENG-1", HeadRefName: "u/sre-2-x", Body: "ref OPS-3"},
			want: []string{"ENG-1", "SRE-2", "OPS-3"},
		},
		{
			name: "no reference",
			pr:   pr{Title: "fix the bug", HeadRefName: "fix/the-bug"},
			want: nil,
		},
		{
			name: "version token is not an issue ref",
			pr:   pr{Title: "bump to v2", HeadRefName: "chore/v2-bump"},
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.pr.linearRefs(); !slices.Equal(got, c.want) {
				t.Errorf("linearRefs() = %v, want %v", got, c.want)
			}
		})
	}
}
