package steward

// What the box a machine publishes may say, and what it may never say.
//
// The projection builds most of its reasons from an err.Error(), so a file
// this machine could not read puts a machine-local absolute path in one. That
// string would be written onto the presence record and pushed to a remote
// every other seat reads, and the seat package's rule is that a record
// carries no free text at all. So the class travels and the words do not.

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

// seatBoxClock is the one time these tests read. Nothing here touches the wall.
var seatBoxClock = time.Date(2026, 9, 25, 14, 37, 0, 0, time.UTC)

// leakedPath is what a read failure puts in a projection's reason: this
// machine's own layout, which has no business on another machine's screen.
const leakedPath = "/Users/wido/LocalStorage/GitHub/agentic-tools-private/metasystem"

func unknownProjection(record, reason string) dispatch.ConsumptionProjection {
	return dispatch.ConsumptionProjection{
		Status: dispatch.BudgetUnknown, GoalID: "g1-s15", GoalRevision: 3,
		Unknown: &dispatch.BudgetUnknownEvidence{
			Code: dispatch.BudgetUnknown, Record: record, Reason: reason,
		},
	}
}

func TestAnUnknownBoxNamesTheClassOfEvidenceAndNeverTheProjectionsWords(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		record string
		want   string
	}{
		{"plans/goals/g1-s15.md", boxUnknownGoalRecord},
		{"plans/goals", boxUnknownGoalRecord},
		{"artifacts/agents/jobs/20260925t0900.json", boxUnknownJobRecords},
		{"artifacts/agents/proof-runs/attempts", boxUnknownProofRecords},
		{"artifacts/agents/governed-obligations", boxUnknownGovernedRecords},
		{"artifacts/agents/validation-weight.json", boxUnknownWeightRecord},
		{"metasystem.conf", boxUnknownConfiguration},
		{"something this engine has never heard of", boxUnknownProjection},
		{"", boxUnknownProjection},
	} {
		got := unknownBoxReason(unknownProjection(one.record, "unreadable: open "+leakedPath+"/x: denied"))
		if got != one.want {
			t.Fatalf("record %q gave %q, want %q", one.record, got, one.want)
		}
	}
	if got := unknownBoxReason(dispatch.ConsumptionProjection{Status: dispatch.BudgetUnknown}); got != boxUnknownProjection {
		t.Fatalf("a projection with no evidence gave %q", got)
	}
}

func TestAnUnknownBoxCarriesNoNumbersRatherThanZeros(t *testing.T) {
	t.Parallel()
	// A goal that has spent nothing and a goal nobody could count look
	// identical in numbers and are not the same fact.
	box := boxOf(unknownProjection("plans/goals/g1-s15.md", "the goal record is missing"))
	if box == nil || box.Problem != boxUnknownGoalRecord {
		t.Fatalf("box = %+v", box)
	}
	if box.Attempts != nil || box.AttemptLimit != nil || box.ReservedMinutes != nil || box.ReservedMinutesLimit != nil {
		t.Fatalf("an unknown box carries numbers: %+v", *box)
	}
}

func TestAProjectionsRawReasonNeverReachesThePublishedRecord(t *testing.T) {
	t.Parallel()
	reason := "the authoritative job record is unreadable: open " +
		leakedPath + "/artifacts/agents/jobs/20260925t0900.json: permission denied"
	box := boxOf(unknownProjection("artifacts/agents/jobs/20260925t0900.json", reason))
	jobs := seat.JobSet{Records: []seat.JobRecord{{
		Job: "j-17", Role: "implementer", Goal: "g1-s15", Round: 2, Status: "running",
		CreatedAt: "2026-09-25T14:00:00Z", StartedAt: "2026-09-25T14:00:00Z",
	}}}
	record, _, err := seat.Compose("m1e", seat.RunnerContext{
		RepoIdentity: "repo-a", Generation: 4, Engine: "3f9c1e2abc",
		ArmedLineage: "lineage-7", TickSeconds: 600,
	}, jobs, func(string) *seat.Box { return box }, seatBoxClock)
	if err != nil {
		t.Fatal(err)
	}
	// The whole published file, because that is what leaves this machine.
	encoded, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}
	published := string(encoded)
	for _, forbidden := range []string{leakedPath, "/Users/", "permission denied", "unreadable", reason} {
		if strings.Contains(published, forbidden) {
			t.Fatalf("the published record carries %q:\n%s", forbidden, published)
		}
	}
	if !strings.Contains(published, boxUnknownJobRecords) {
		t.Fatalf("the published record does not say which evidence stopped the projection:\n%s", published)
	}
}

func TestALedgerThisMachineCouldNotReadNamesNoPathEither(t *testing.T) {
	t.Parallel()
	// SeatBox reads a checkout that is not one: the words it answers with are
	// the same fixed sentences, and the temporary directory it was handed is
	// not among them.
	root := t.TempDir()
	box := SeatBox(root, seatBoxClock)("g1-s15")
	if box == nil {
		t.Fatalf("a checkout with no ledger answered no box at all")
	}
	if strings.Contains(box.Problem, root) || strings.Contains(box.Problem, "/") {
		t.Fatalf("the box names a path: %q", box.Problem)
	}
	switch box.Problem {
	case boxUnknownTip, boxUnknownLedger, boxUnknownGoal:
	default:
		t.Fatalf("the box says %q, which is not one of this file's sentences", box.Problem)
	}
}
