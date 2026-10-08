package launch

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestRevisionJoinsCorrectionAndBoundDecisions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, correction, supplied string
		refused                    bool
	}{
		{"fixed after accepted", "| critic:1 | fixed | file.go:12 |\n", "| critic:1 | accepted | add the missing case |\n| critic:2 | accepted | add the other case |\n", false},
		{"duplicate correction", "| critic:1 | fixed | file.go:12 |\n| critic:1 | fixed | file.go:20 |\n", "| critic:2 | accepted | add the other case |\n", true},
		{"duplicate bound decision", "| critic:1 | fixed | file.go:12 |\n", "| critic:1 | accepted | add the missing case |\n| critic:1 | accepted | add it again |\n| critic:2 | accepted | add the other case |\n", true},
		{"missing finding", "| critic:1 | fixed | file.go:12 |\n", "| critic:1 | accepted | add the missing case |\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			round := UnitRound{Number: 1, Stop: &loopstop.Stop{Decision: "continue"}, Reads: []readsubject.Read{{ID: "critic", Findings: []readsubject.Finding{{ID: "critic:1", Material: true}, {ID: "critic:2", Material: true}}}}}
			err := (&UnitRunner{}).reviseDecided(UnitRunRecord{ID: "unit"}, round, []byte("## Decisions on round 1\n"+tc.correction), []byte(tc.supplied))
			if tc.refused {
				if !IsCode(err, "UNIT_REVISE_UNDECIDED") {
					t.Fatalf("ambiguous or incomplete decisions admitted: %v", err)
				}
			} else if err != nil {
				t.Fatalf("correction and bound decisions could not be joined: %v", err)
			}
		})
	}
}
