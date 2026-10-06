package goal

import (
	"strings"
	"testing"
)

func TestClaimAreasArcClaimant(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, arc, machine, lineage string
		allow                       bool
	}{
		{"same claimant and arc", "shared-arc", "mac-a", "session-a", true},
		{"other machine", "shared-arc", "mac-b", "session-a", false},
		{"other session", "shared-arc", "mac-a", "session-b", false},
		{"other arc", "other-arc", "mac-a", "session-a", false},
		{"ungrouped", "", "mac-a", "session-a", false},
		{"no claimant", "shared-arc", "", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, queued := range []bool{false, true} {
				snapshot := AreaSnapshot{Known: true, Areas: []string{"shared/**"}, Source: "shared-design@" + strings.Repeat("a", 64)}
				tree := &TreeGoals{Live: map[string]*GoalFile{
					"candidate": {Id: "candidate", Arc: test.arc, State: StateApproved},
					"held": {Id: "held", Arc: "shared-arc", State: StateClaimed, Claimed: &ClaimRecord{
						Machine: "mac-a", Lineage: "session-a", AreaSnapshot: snapshot,
					}},
				}}
				if test.name == "ungrouped" {
					tree.Live["held"].Arc = ""
				}
				readers := ClaimAreaReaders{}
				if queued {
					readers.Queue = func() ([]ClaimAreaEntry, []string) {
						return []ClaimAreaEntry{{Goal: "held", State: "waiting", Snapshot: snapshot}}, nil
					}
				}
				warnings, err := ClaimAreas(tree, Endpoint{}, "tip", "candidate", snapshot, readers, Actor{Machine: test.machine, Lineage: test.lineage})
				if len(warnings) != 0 || (err == nil) != test.allow {
					t.Fatalf("waiting=%v allow=%v warnings=%v err=%v", queued, test.allow, warnings, err)
				}
				if err != nil && (RefusalCode(err) != ClaimAreasCode || !strings.Contains(err.Error(), "goal held")) {
					t.Fatalf("waiting=%v wrong refusal: %v", queued, err)
				}
			}
		})
	}
}

func TestAreasOverlap(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		a, b         string
		overlap, bad bool
	}{
		{"future/new.go", "future/new.go", true, false},
		{"src/*.go", "src/*.md", true, false},
		{"src/**/a.go", "src/[ab]?.md", true, false},
		{"src/", "src/nested/file.go", true, false},
		{"**/a.go", "elsewhere/b.go", true, false},
		{"src/a*", "other/a*", false, false},
		{".", "src/a", true, false},
		{"src/[", "src/a", false, true},
	} {
		t.Run(test.a+test.b, func(t *testing.T) {
			t.Parallel()
			overlap, err := AreasOverlap([]string{test.a}, []string{test.b})
			if (err != nil) != test.bad || (overlap != nil) != test.overlap {
				t.Fatalf("overlap=%+v err=%v", overlap, err)
			}
		})
	}
}
