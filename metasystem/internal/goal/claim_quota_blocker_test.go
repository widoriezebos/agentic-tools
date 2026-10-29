package goal

import (
	"strings"
	"testing"
)

// claimQuotaBed opens and approves the named goals on mac-a and claims the
// first, so the machine's one claim is held.
func claimQuotaBed(t *testing.T, ids ...string) Endpoint {
	t.Helper()
	endpoint, _ := fakeGoalEndpoint(t)
	for index, id := range ids {
		request := verbReqFor(endpoint, "01J5X00000000000000000Q"+string(rune('A'+index))+"00", "mac-a")
		if result, err := Open(request, id, "Exercise the claim quota.", OriginHuman, "Claim it."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
		approveGoalForTest(t, request, id, testBudget())
	}
	if result, err := Claim(verbReqFor(endpoint, "01J5X00000000000000000QZ00", "mac-a"), ids[0]); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim %s: %+v %v", ids[0], result, err)
	}
	return endpoint
}

func laneStates(states map[string]string) func(string, string) (string, error) {
	return func(id, _ string) (string, error) { return states[id], nil }
}

func TestTheClaimAndTheRowNameOneBlocker(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, state, target, outcome, held, command string
		waiting                                     string
	}{
		{name: "held", target: "ready-r", outcome: "held", held: "held-h", command: "metasystem goal release held-h --reason TEXT"},
		{name: "returned", state: "returned", target: "ready-r", outcome: "returned", held: "held-h", command: "metasystem work land held-h"},
		{name: "landed", state: "landed", target: "ready-r", outcome: "landed", held: "held-h", command: "metasystem goal done held-h --reason TEXT"},
		{name: "slot", state: "waiting", target: "third-t", outcome: "slot", held: "held-h", command: "metasystem work land held-h", waiting: "ready-r"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			endpoint := claimQuotaBed(t, "held-h", "ready-r", "third-t")
			states := map[string]string{"held-h": row.state}
			if row.waiting != "" {
				// The first hand-in waits; the claim beside it marks it, then
				// the second claim's hand-in waits too: two slots.
				claim := verbReqFor(endpoint, "01J5X00000000000000000QY00", "mac-a")
				claim.ClaimLaneState = laneStates(states)
				if result, err := Claim(claim, row.waiting); err != nil || result.Outcome != OutcomeConfirmed {
					t.Fatalf("claim beside the waiting hand-in: %+v %v", result, err)
				}
				states[row.waiting] = "waiting"
			}
			tip := acceptedTipForEndpoint(t, endpoint)
			tree, err := loadTreeFor(endpoint, tip)
			if err != nil {
				t.Fatal(err)
			}
			block := ClaimQuotaBlocker(tree, row.target, "mac-a", tip, laneStates(states))
			if block.Held != row.held || block.Outcome != row.outcome || block.Command != row.command ||
				!strings.Contains(block.Text, "\nrun: "+row.command) {
				t.Fatalf("the query's block = %+v, want held %s outcome %s command %q", block, row.held, row.outcome, row.command)
			}
			claim := verbReqFor(endpoint, "01J5X00000000000000000QX00", "mac-a")
			claim.ClaimLaneState = laneStates(states)
			result, err := Claim(claim, row.target)
			if err != nil || result.Outcome != OutcomeRejected || result.Code != ClaimQuotaCode || result.Detail != block.Text {
				t.Fatalf("the claim's refusal is not the query's text:\n got %+v %v\nwant %q", result, err, block.Text)
			}
			if after := acceptedTipForEndpoint(t, endpoint); after != tip {
				t.Fatalf("the refused claim moved the ledger: %s -> %s", tip, after)
			}
		})
	}

	t.Run("held text", func(t *testing.T) {
		t.Parallel()
		endpoint := claimQuotaBed(t, "held-h", "ready-r")
		tip := acceptedTipForEndpoint(t, endpoint)
		tree, err := loadTreeFor(endpoint, tip)
		if err != nil {
			t.Fatal(err)
		}
		want := "machine mac-a already claims held-h, and a machine holds one claim at a time\n" +
			"run: metasystem goal release held-h --reason TEXT, then metasystem goal claim ready-r"
		if block := ClaimQuotaBlocker(tree, "ready-r", "mac-a", tip, nil); block.Text != want {
			t.Fatalf("held text = %q, want %q", block.Text, want)
		}
	})

	t.Run("waiting hand-in", func(t *testing.T) {
		t.Parallel()
		endpoint := claimQuotaBed(t, "held-h", "ready-r")
		states := map[string]string{"held-h": "waiting"}
		tip := acceptedTipForEndpoint(t, endpoint)
		tree, err := loadTreeFor(endpoint, tip)
		if err != nil {
			t.Fatal(err)
		}
		block := ClaimQuotaBlocker(tree, "ready-r", "mac-a", tip, laneStates(states))
		if block.Held != "" || block.Command != "" || len(block.Waiting) != 1 || block.Waiting[0].Id != "held-h" {
			t.Fatalf("a waiting hand-in is no block but a marker to record: %+v", block)
		}
		if after := acceptedTipForEndpoint(t, endpoint); after != tip {
			t.Fatalf("the query recorded something: %s -> %s", tip, after)
		}
		if reread, err := loadTreeFor(endpoint, tip); err != nil || reread.Live["held-h"].Landing != nil {
			t.Fatalf("the query marked the hand-in: %+v %v", reread.Live["held-h"], err)
		}
		claim := verbReqFor(endpoint, "01J5X00000000000000000QW00", "mac-a")
		claim.ClaimLaneState = laneStates(states)
		result, err := Claim(claim, "ready-r")
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("the claim beside a waiting hand-in: %+v %v", result, err)
		}
		after, err := loadTreeFor(endpoint, result.Tip)
		if err != nil || after.Live["held-h"].Landing == nil || after.Live["ready-r"].Claimed == nil {
			t.Fatalf("the claim did not record the marker beside its claim: %+v %v", after, err)
		}
		if err := validateCommitFor(endpoint, result.Tip); err != nil {
			t.Fatalf("the claim's tree does not validate: %v", err)
		}
	})
}
