package goal

import (
	"reflect"
	"strings"
	"testing"
)

func restampFixture(t *testing.T, id string) (string, VerbRequest) {
	t.Helper()
	_, root := oneClone(t)
	seedLedger(t, root)
	request := verbReq(root, "01J5X00000000000000000CE10", "mac-a")
	if result, err := openClaimForTest(t, request, id, "Keep stop authority current.", OriginMain, "Prove it.", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim fixture goal: %+v %v", result, err)
	}
	request.CallerClass = "MAIN"
	return root, request
}

func TestRestampMovesTheStopCapabilityToTheLeaseEpoch(t *testing.T) {
	root, request := restampFixture(t, "epoch-move")
	before, _ := acceptedTree(t, root, request.Now)
	beforeFile := before.Live["epoch-move"]
	claim := *beforeFile.Claimed
	if beforeFile.StopCapability == nil || beforeFile.StopCapability.ClaimEpoch != 1 {
		t.Fatalf("today's stored record did not load before restamp: %+v", beforeFile.StopCapability)
	}

	request.Ulid = "01J5X00000000000000000CE20"
	request.ClaimEpoch = 5
	result, err := Restamp(request, "epoch-move")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("restamp: %+v %v", result, err)
	}
	after, _ := acceptedTree(t, root, request.Now)
	afterFile := after.Live["epoch-move"]
	if afterFile.StopCapability == nil || afterFile.StopCapability.ClaimEpoch != 5 {
		t.Fatalf("capability epoch = %+v, want 5", afterFile.StopCapability)
	}
	if !reflect.DeepEqual(*afterFile.Claimed, claim) {
		t.Fatalf("restamp changed the claim record:\nbefore=%+v\nafter=%+v", claim, afterFile.Claimed)
	}
	if err := ValidateCommit(root, result.Tip); err != nil {
		t.Fatalf("restamped record does not validate: %v", err)
	}
	loaded, err := loadTree(root, result.Tip)
	if err != nil || loaded.Live["epoch-move"].StopCapability.ClaimEpoch != 5 {
		t.Fatalf("today's stored record did not load after restamp: %+v %v", loaded, err)
	}
}

func TestRestampRefusesAForeignPairALowerEpochAndAnUnclaimedGoal(t *testing.T) {
	t.Run("foreign pair", func(t *testing.T) {
		_, request := restampFixture(t, "foreign-pair")
		request.Ulid = "01J5X00000000000000000CF10"
		request.Actor.Lineage = "foreign-lineage"
		request.ClaimEpoch = 5
		result, err := Restamp(request, "foreign-pair")
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "caller pair") || !strings.Contains(result.Detail, "does not match") {
			t.Fatalf("foreign pair refusal: %+v %v", result, err)
		}
	})

	t.Run("main non-holder with claim epoch", func(t *testing.T) {
		_, request := restampFixture(t, "main-non-holder")
		request.Ulid = "01J5X00000000000000000CF20"
		request.Actor = Actor{Machine: "mac-b", Lineage: "other-main"}
		request.CallerClass = "MAIN"
		request.ClaimEpoch = 5
		result, err := Restamp(request, "main-non-holder")
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "caller pair mac-b+other-main does not match") {
			t.Fatalf("foreign MAIN pair refusal: %+v %v", result, err)
		}
	})

	t.Run("lower epoch", func(t *testing.T) {
		_, request := restampFixture(t, "lower-epoch")
		request.Ulid = "01J5X00000000000000000CK10"
		request.ClaimEpoch = 5
		if result, err := Restamp(request, "lower-epoch"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("raise fixture epoch: %+v %v", result, err)
		}
		request.Ulid = "01J5X00000000000000000CK20"
		request.ClaimEpoch = 4
		result, err := Restamp(request, "lower-epoch")
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "cannot move down") {
			t.Fatalf("lower epoch refusal: %+v %v", result, err)
		}
	})

	t.Run("unclaimed goal", func(t *testing.T) {
		_, root := oneClone(t)
		seedLedger(t, root)
		request := verbReq(root, "01J5X00000000000000000CV10", "mac-a")
		if result, err := Open(request, "unclaimed", "Wait before claiming.", OriginMain, "Wait."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open fixture goal: %+v %v", result, err)
		}
		request.Ulid = "01J5X00000000000000000CV20"
		request.CallerClass = "MAIN"
		request.ClaimEpoch = 5
		result, err := Restamp(request, "unclaimed")
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "is not claimed") {
			t.Fatalf("unclaimed refusal: %+v %v", result, err)
		}
	})

	t.Run("caller is not the holder", func(t *testing.T) {
		_, request := restampFixture(t, "not-holder")
		request.Ulid = "01J5X00000000000000000CH10"
		request.CallerClass = "DELEGATE"
		request.ClaimEpoch = 5
		result, err := Restamp(request, "not-holder")
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "live lease holder of class MAIN") {
			t.Fatalf("non-holder refusal: %+v %v", result, err)
		}
	})
}

func TestRestampIsANoOpWhenTheEpochsAgree(t *testing.T) {
	root, request := restampFixture(t, "already-current")
	before := acceptedTip(t, root)
	request.Ulid = "01J5X00000000000000000CN10"
	opid := request.opid()
	result, err := Restamp(request, "already-current")
	if err != nil || result.Outcome != OutcomeAbandoned || !strings.Contains(result.Detail, "already carries lease epoch 1") {
		t.Fatalf("equal epoch result: %+v %v", result, err)
	}
	after := acceptedTip(t, root)
	if after != before || result.Tip != before || result.Commit != "" {
		t.Fatalf("equal epoch changed the ledger: before=%s after=%s result=%+v", before, after, result)
	}
	entry, err := ReadEntry(root, opid)
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeAbandoned {
		t.Fatalf("equal epoch journal entry: %+v %v", entry, err)
	}
	request.Ulid = "01J5X00000000000000000CN20"
	replayed, err := Restamp(request, "already-current")
	if err != nil || replayed.Outcome != result.Outcome || replayed.Detail != result.Detail || replayed.Tip != before || replayed.Commit != "" {
		t.Fatalf("equal epoch replay: %+v %v", replayed, err)
	}
}
