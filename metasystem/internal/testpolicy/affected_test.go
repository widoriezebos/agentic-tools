package testpolicy

import (
	"reflect"
	"strings"
	"testing"
)

func TestAffectedGroupsDeclareAChangedPath(t *testing.T) {
	t.Parallel()
	contract := Contract{Groups: []Group{
		{ID: "plans", Inputs: []string{"metasystem/plans/**"}},
		{ID: "internal", Inputs: []string{"metasystem/internal/**", "metasystem/testing.json"}},
		{ID: "go-affected", PackageSelection: "affected", Inputs: []string{"metasystem/docs/**"}},
	}}
	for _, tc := range []struct {
		name  string
		paths []string
		want  AffectedResult
	}{
		{"plans", []string{"metasystem/plans/x.md"}, AffectedResult{Groups: []AffectedGroup{{Group: contract.Groups[0], Paths: []string{"metasystem/plans/x.md"}}}}},
		{"template", []string{"metasystem/docs/x.md"}, AffectedResult{TemplateCovered: []string{"metasystem/docs/x.md"}}},
		{"uncovered", []string{"README.md"}, AffectedResult{Uncovered: []string{"README.md"}}},
		{"exact file", []string{"metasystem/testing.json"}, AffectedResult{Groups: []AffectedGroup{{Group: contract.Groups[1], Paths: []string{"metasystem/testing.json"}}}}},
		{"contract order", []string{"metasystem/internal/x.go", "metasystem/plans/b.md", "metasystem/plans/a.md"}, AffectedResult{Groups: []AffectedGroup{
			{Group: contract.Groups[0], Paths: []string{"metasystem/plans/b.md", "metasystem/plans/a.md"}},
			{Group: contract.Groups[1], Paths: []string{"metasystem/internal/x.go"}},
		}}},
		{"empty", nil, AffectedResult{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Affected(contract, tc.paths)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Affected(%v) = %+v, %v; want %+v", tc.paths, got, err, tc.want)
			}
		})
	}
	t.Run("exact directory and overlapping inputs", func(t *testing.T) {
		copy := Contract{Groups: append([]Group(nil), contract.Groups...)}
		copy.Groups[0].Inputs = []string{"metasystem/plans", "metasystem/plans/x.md"}
		copy.Groups[2].Inputs = []string{"metasystem/plans/**", "metasystem/plans"}
		path := "metasystem/plans/x.md"
		paths := []string{path, "metasystem/plans/nested/y.md"}
		want := AffectedResult{
			Groups:          []AffectedGroup{{Group: copy.Groups[0], Paths: paths}},
			TemplateCovered: paths,
		}
		got, err := Affected(copy, paths)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("directory coverage = %+v, %v; want %+v", got, err, want)
		}
	})
	t.Run("invalid input", func(t *testing.T) {
		copy := Contract{Groups: append([]Group(nil), contract.Groups...)}
		copy.Groups[2].Inputs = []string{"../outside"}
		got, err := Affected(copy, []string{"metasystem/plans/x.md"})
		if err == nil || !strings.Contains(err.Error(), "go-affected") || !strings.Contains(err.Error(), "../outside") || !reflect.DeepEqual(got, AffectedResult{}) {
			t.Fatalf("invalid unmatched template input returned %+v, %v", got, err)
		}
		got, err = Affected(copy, nil)
		if err != nil || !reflect.DeepEqual(got, AffectedResult{}) {
			t.Fatalf("empty paths returned %+v, %v", got, err)
		}
	})
}
