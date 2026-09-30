package main

// goal land-without-sitting and a sitting's hold (g1-s70 D2, D4): the same
// decision at the same tip, and the same hold, recorded again, are success
// with no record; and the public refusals name only public forms.

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// gateOwners are the terminal's owners with the goal branch at the bed's tip.
func (b *intentBed) gateOwners() intentOwners {
	owners := b.terminalOwners()
	owners.delivery = &intentDeliveryOwners{branchTip: func(string, string) (string, error) { return reviewBedTip, nil }}
	return owners
}

// expectRepeatUnchanged runs args again under owners and requires exit 0,
// outcome unchanged, the phrase, and no publication and no ledger change.
func (b *intentBed) expectRepeatUnchanged(owners intentOwners, phrase string, args ...string) {
	b.t.Helper()
	publications, files := b.publications(), goalLedgerBytes(b)
	code, second := b.runJSON(owners, args...)
	if code != 0 || second.Outcome != intentUnchanged || !strings.Contains(second.Summary, phrase) {
		b.t.Fatalf("%v repeat = exit %d %+v; want exit 0, unchanged, %q", args, code, second, phrase)
	}
	if b.publications() != publications || goalLedgerBytes(b) != files {
		b.t.Fatalf("%v repeat changed the ledger", args)
	}
}

func init() {
	registerIdempotency("goal land-without-sitting", idemStateful, "the same decision at the same tip is success with no record",
		func(t *testing.T) {
			bed := newIntentBed(t, false, waitingToLandBed)
			args := []string{"goal", "land-without-sitting", bedGoal, "--reason", "one-line doc fix, read the diff on the card"}
			code, first := bed.runJSON(bed.gateOwners(), args...)
			if code != 0 || first.Outcome != intentConfirmed {
				t.Fatalf("the first decision = %d %+v", code, first)
			}
			bed.expectRepeatUnchanged(bed.gateOwners(), "already carries Wido's decision to land without a sitting", args...)
		})
}

func TestIntentGoalLandWithoutSittingRecordsTheDecisionAtTheTip(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, waitingToLandBed)
	code, result := bed.runJSON(bed.gateOwners(), "goal", "land-without-sitting", bedGoal)
	if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "carries your reason") {
		t.Fatalf("a decision without a reason = %d %+v", code, result)
	}
	code, result = bed.runJSON(bed.gateOwners(), "goal", "land-without-sitting", bedGoal, "--reason", "read the diff")
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("the decision = %d %+v", code, result)
	}
	file := acceptedFile(bed)
	last := file.History[len(file.History)-1]
	if last.Verb != goal.LandWithoutSittingVerb || last.Reason != "landed-without-sitting tip="+reviewBedTip+" by=Wido because=read the diff" {
		t.Fatalf("the decision line = %+v", last)
	}
	noTip := bed.terminalOwners()
	noTip.delivery = &intentDeliveryOwners{branchTip: func(string, string) (string, error) { return "", nil }}
	if code, result = bed.runJSON(noTip, "goal", "land-without-sitting", bedGoal, "--reason", "x"); code == 0 || !strings.Contains(result.Summary, "has no commits at origin") {
		t.Fatalf("a decision with no branch = %d %+v", code, result)
	}
}

func TestIntentGoalReviewHoldsAndReleasesTheSitting(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, waitingToLandBed)
	hold := []string{"goal", "review", bedGoal, "--record", reviewBedRecord, "--hold"}
	code, result := bed.runJSON(bed.terminalOwners(), hold...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("the hold = %d %+v", code, result)
	}
	if holds := goal.HoldsOf(acceptedFile(bed)); len(holds) != 1 || holds[0].By != "Wido" || holds[0].Record != reviewBedRecord {
		t.Fatalf("the hold does not stand on the ledger: %+v", holds)
	}
	bed.expectRepeatUnchanged(bed.terminalOwners(), "already held by Wido's sitting", hold...)
	release := []string{"goal", "review", bedGoal, "--record", reviewBedRecord, "--release"}
	if code, result = bed.runJSON(bed.terminalOwners(), release...); code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("the release = %d %+v", code, result)
	}
	if holds := goal.HoldsOf(acceptedFile(bed)); len(holds) != 0 {
		t.Fatalf("the release left a hold: %+v", holds)
	}
	bed.expectRepeatUnchanged(bed.terminalOwners(), "no standing sitting", release...)
	for _, row := range []struct {
		args []string
		want string
	}{
		{[]string{"goal", "review", bedGoal, "--record", reviewBedRecord, "--hold", "--release"}, "give one"},
		{[]string{"goal", "review", bedGoal, "--record", reviewBedRecord, "--hold", "--verdict", "clear-to-land"}, "goes with a verdict"},
		{[]string{"goal", "review", bedGoal, "--hold"}, "needs its review record file"},
	} {
		if code, result := bed.runJSON(bed.terminalOwners(), row.args...); code == 0 || !strings.Contains(result.Summary, row.want) {
			t.Errorf("%v = %d %+v, want %q", row.args, code, result, row.want)
		}
	}
}

// acceptedFile is the bed's goal as the ledger accepted it; the fixture reads
// the canonical tip after each act.
func acceptedFile(bed *intentBed) *goal.GoalFile {
	bed.t.Helper()
	file, _ := bed.acceptedGoal()
	return file
}
