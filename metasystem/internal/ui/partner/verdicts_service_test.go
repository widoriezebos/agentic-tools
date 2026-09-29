package partner_test

// The closing deposit carries the tip (g1-s69 D1), and the Behaves walk
// carries the candidate's address and both commits (D3).

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

const (
	closingTip = "9c1f0a2b3c4d5e6f708192a3b4c5d6e7f8091a2b"
	movedTip   = "4d2e7b1c3d4e5f60718293a4b5c6d7e8f9a0b1c2"
)

func TestTheClosingDepositCarriesTheReviewedTipBesideTheVerdict(t *testing.T) {
	t.Parallel()
	held := reviewService(t, fakeacp.Script{
		Reads: []fakeacp.Read{{Name: "mcp__metasystem__deposit", Title: "deposit(outcome)",
			Result: "prepared\nDeposit: outcome\n--- the deposit follows, whole and to the end ---\nVerdict: clear to land\n"}},
		Chunks: []string{"Here is what it came to."},
	})
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()
	_, err := held.service.Sit(ctx, "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	drain(t, events)

	_, err = held.service.ClosingAt(ctx, "Wido", reviewA, partner.VerdictClear, closingTip, inTheRoom(reviewA))
	testutil.Require(t, "End", err, nil)
	drain(t, events)
	read, err := held.service.SnapshotIn("Wido", reviewA, 100)
	testutil.Require(t, "read back", err, nil)
	var outcome *partner.Deposit
	for _, message := range read.Messages {
		for index := range message.Deposits {
			if message.Deposits[index].Kind == partner.DepositOutcome {
				outcome = &message.Deposits[index]
			}
		}
	}
	testutil.Require(t, "the outcome was offered", outcome != nil, true)
	testutil.Expect(t, "its verdict", outcome.Verdict, partner.VerdictClear)
	testutil.Expect(t, "the tip it was drafted for", outcome.Tip, closingTip)
}

func TestTheBehavesWalkCarriesTheCandidateAndBothCommits(t *testing.T) {
	t.Parallel()
	held := reviewService(t, fakeacp.Script{Chunks: []string{"Behaves: ..."}})
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()
	sitting, err := held.service.Sit(ctx, "Wido", reviewOf(reviewA), partner.PurposeReview, inTheRoom(reviewA))
	testutil.Require(t, "the review opened", err, nil)
	drain(t, events)
	walks := 0
	asked := func(part string, candidate *partner.Candidate) string {
		t.Helper()
		walks++
		_, err := held.service.WalkWith(ctx, "Wido", reviewA, part, inTheRoom(reviewA), candidate)
		testutil.Require(t, fmt.Sprintf("walk %d, %s", walks, part), err, nil)
		drain(t, events)
		read, err := held.service.SnapshotIn("Wido", reviewA, 100)
		testutil.Require(t, fmt.Sprintf("read back walk %d", walks), err, nil)
		return read.Messages[len(read.Messages)-2].Text
	}

	moved := asked("behaves", &partner.Candidate{Address: "http://127.0.0.1:7981/", Running: movedTip, Reviewed: closingTip})
	testutil.Expect(t, "the fixed words first", strings.HasPrefix(moved, partner.WalkRequest("behaves", sitting)), true)
	for _, said := range []string{"http://127.0.0.1:7981/", movedTip, closingTip, "What runs is not the version under review",
		"do not present what it does as evidence of the reviewed version"} {
		if !strings.Contains(moved, said) {
			t.Errorf("the moved case does not say %q:\n%s", said, moved)
		}
	}
	same := asked("behaves", &partner.Candidate{Address: "http://127.0.0.1:7981/", Running: closingTip, Reviewed: closingTip})
	if strings.Contains(same, "not the version under review") || !strings.Contains(same, "at commit "+closingTip+"; the review is of "+closingTip) {
		t.Fatalf("the running reviewed tip is said as a mismatch or not said:\n%s", same)
	}
	none := asked("behaves", &partner.Candidate{Reviewed: closingTip})
	testutil.Expect(t, "no candidate running", strings.Contains(none, "No candidate of this goal is running now"), true)
	testutil.Expect(t, "every other walk is told nothing",
		asked("built", &partner.Candidate{Address: "http://127.0.0.1:7981/", Running: movedTip, Reviewed: closingTip}),
		partner.WalkRequest("built", sitting))
}
