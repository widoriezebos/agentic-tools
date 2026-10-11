package goal

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestTrunkRedBatchClearPublishesOnceAndReplays(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"publish", "recover", "changed last entry", "unknown last entry", "different proof"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			endpoint := localTrunkRedEndpoint(t)
			entries := []TrunkRedEntry{
				testTrunkRedEntry("red:unit:TestOne", "red:unit:TestOne", "2026-09-17T01:00:00Z"),
				testTrunkRedEntry("red:unit:TestTwo", "red:unit:TestTwo", "2026-09-17T01:00:00Z"),
			}
			before := publishTrunkRedFor(t, endpoint, entries).Tip
			r := trunkRedVerbReqFor(endpoint, "01J5X0000000000000000000B1", "mac-a")
			args := make([]TrunkRedClearArgs, len(entries))
			for i, entry := range entries {
				args[i] = TrunkRedClearArgs{Entry: entry.ID, ExpectedEntry: entry, Attempt: "green-proof", BaseCommit: "main", BaseTree: "main-tree", Executed: true}
			}
			switch mode {
			case "changed last entry":
				args[1].ExpectedEntry.Owner.Machine = "other"
			case "unknown last entry":
				args[1].Entry = "missing"
			case "different proof":
				args[1].Attempt = "other-proof"
			}
			if mode == "recover" {
				request := trunkRedClearBatchRequest(r, args)
				if request.Message != "trunk-red clear green-proof: 2 incidents" {
					t.Fatalf("clear message: %s", request.Message)
				}
				createDeadTrunkRedEntry(t, endpoint.Root, request)
				reports, err := Recover(endpoint)
				if err != nil || len(reports) == 0 {
					t.Fatalf("recover batch: %+v %v", reports, err)
				}
			} else {
				result, err := ClearTrunkReds(r, args)
				if mode != "publish" {
					if err == nil && result.Outcome != OutcomeRejected || acceptedTipForEndpoint(t, endpoint) != before {
						t.Fatalf("invalid batch published a partial clear: %+v %v", result, err)
					}
					return
				}
				if err != nil || result.Outcome != OutcomeConfirmed {
					t.Fatalf("clear batch: %+v %v", result, err)
				}
			}
			tip := acceptedTipForEndpoint(t, endpoint)
			store := endpoint.Repository.(*fakeGoalRepository).store
			if tip == before || store.commits[tip].parent != before {
				t.Fatalf("batch must publish exactly one commit: before=%s tip=%s parent=%s", before, tip, store.commits[tip].parent)
			}
			projection, err := trunkRedProjectAt(endpoint, tip, r.Now)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range projection.Tree.TrunkRed {
				if entry.Closed == nil || entry.Closed.Opid != r.opid() || entry.Closed.Attempt != "green-proof" || len(entry.Holds) != 0 {
					t.Fatalf("batch lost closure content: %+v", entry)
				}
			}
			result, err := ClearTrunkReds(r, args)
			if err != nil || result.Outcome != OutcomeConfirmed || result.Tip != tip {
				t.Fatalf("replay moved the ledger: %+v %v", result, err)
			}
		})
	}
}

func TestTrunkRedRecordMessageNamesAttemptAndIncidentCount(t *testing.T) {
	t.Parallel()
	for _, count := range []int{1, 3} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			t.Parallel()
			args := TrunkRedRecordArgs{Batch: "batch", Attempt: "red-proof", Groups: make([]TrunkRedRecordGroup, count)}
			want := "trunk-red record red-proof: 1 incident"
			if count == 3 {
				want = "trunk-red record red-proof: 3 incidents"
			}
			if got := trunkRedRecordRequest(VerbRequest{}, args).Message; got != want {
				t.Fatalf("record message: %s, want %s", got, want)
			}
		})
	}
}

func TestTrunkRedClearMessagesNameEntryAndIncidentCount(t *testing.T) {
	t.Parallel()
	for _, attempt := range []string{"", "green-proof"} {
		t.Run("single/"+attempt, func(t *testing.T) {
			t.Parallel()
			want := "trunk-red clear red:unit:TestOne: 1 incident"
			if attempt != "" {
				want = "trunk-red clear red:unit:TestOne (green-proof): 1 incident"
			}
			args := TrunkRedClearArgs{Entry: "red:unit:TestOne", Attempt: attempt, legacyUnbound: true}
			if got := trunkRedClearRequest(VerbRequest{}, args).Message; got != want {
				t.Fatalf("single clear message: %s, want %s", got, want)
			}
		})
	}
	for _, count := range []int{1, 3} {
		t.Run(fmt.Sprintf("batch/%d", count), func(t *testing.T) {
			t.Parallel()
			args := make([]TrunkRedClearArgs, count)
			for i := range args {
				args[i].Attempt = "green-proof"
			}
			want := "trunk-red clear green-proof: 1 incident"
			if count == 3 {
				want = "trunk-red clear green-proof: 3 incidents"
			}
			if got := trunkRedClearBatchRequest(VerbRequest{}, args).Message; got != want {
				t.Fatalf("batch clear message: %s, want %s", got, want)
			}
		})
	}
}

func TestTrunkRedCadenceClaimMessageNamesClaim(t *testing.T) {
	t.Parallel()
	args := TrunkRedRecordArgs{CadenceClaim: &CadenceClaim{Key: cadenceKeyFixture()}}
	want := "trunk-red cadence claim " + args.CadenceClaim.Key.TrunkTree
	if got := trunkRedRecordRequest(VerbRequest{}, args).Message; got != want {
		t.Fatalf("cadence claim message: %s, want %s", got, want)
	}
}

func TestTrunkRedRealRegisterRoundTrips(t *testing.T) {
	t.Parallel()
	// The frozen register includes eight green closures from one trunk proof
	// and the standing cadence state.
	data, err := os.ReadFile("testdata/trunk-red-7481259dd.json")
	if err != nil {
		t.Fatal(err)
	}
	entries, problems := ParseTrunkRed(data)
	if len(problems) != 0 {
		t.Fatalf("real register rejected: %v", problems)
	}
	var original trunkRedFile
	if err := json.Unmarshal(data, &original); err != nil {
		t.Fatal(err)
	}
	rendered := renderTrunkRedState(entries, original.Cadence, original.CadenceClaim)
	var roundTrip trunkRedFile
	if err := json.Unmarshal(rendered, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, roundTrip) {
		t.Fatal("round trip changed the real incidents or cadence state")
	}
	if _, problems := ParseTrunkRed(rendered); len(problems) != 0 {
		t.Fatalf("rendered register rejected: %v", problems)
	}
	closed := 0
	for _, entry := range entries {
		if entry.Closed != nil && entry.Closed.Attempt == "20261011T014956.528332000Z" {
			closed++
		}
	}
	if closed != 8 || original.Cadence == nil {
		t.Fatalf("historical shape lost: closures=%d cadence=%+v", closed, original.Cadence)
	}
}

func TestTrunkRedBatchClearPartlySatisfiedKeepsClosuresUnderAnyAttempt(t *testing.T) {
	t.Parallel()
	for _, attempt := range []string{"green-proof", "other-proof"} {
		t.Run(attempt, func(t *testing.T) {
			t.Parallel()
			endpoint := localTrunkRedEndpoint(t)
			entries := []TrunkRedEntry{
				testTrunkRedEntry("red:unit:TestOne", "red:unit:TestOne", "2026-09-17T01:00:00Z"),
				testTrunkRedEntry("red:unit:TestTwo", "red:unit:TestTwo", "2026-09-17T01:00:00Z"),
			}
			r := trunkRedVerbReqFor(endpoint, "01J5X0000000000000000000B2", "mac-a")
			args := make([]TrunkRedClearArgs, len(entries))
			for i, entry := range entries {
				args[i] = TrunkRedClearArgs{Entry: entry.ID, ExpectedEntry: entry, Attempt: "green-proof", BaseCommit: "main", BaseTree: "main-tree", Executed: true}
			}
			// The last entry is already closed, under this attempt or another one:
			// the clear it asks for already holds, so the batch closes only the
			// remaining entry and leaves the existing closure untouched (R-129-ui).
			closed := &TrunkRedClosure{At: r.stamp(), Attempt: attempt, BaseCommit: "main", How: "green",
				Opid: trunkRedVerbReqFor(endpoint, "01J5X0000000000000000000B1", "mac-a").opid()}
			entries[1].Closed, entries[1].Holds = closed, []string{}
			before := publishTrunkRedFor(t, endpoint, entries).Tip
			result, err := ClearTrunkReds(r, args)
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("partly satisfied batch: %+v %v", result, err)
			}
			store := endpoint.Repository.(*fakeGoalRepository).store
			if result.Tip == before || store.commits[result.Tip].parent != before {
				t.Fatalf("remaining clear must publish once: %+v", result)
			}
			projection, err := trunkRedProjectAt(endpoint, result.Tip, r.Now)
			if err != nil {
				t.Fatal(err)
			}
			if got := projection.Tree.TrunkRed[0].Closed; got == nil || got.Opid != r.opid() {
				t.Fatalf("remaining entry was not closed: %+v", got)
			}
			if !reflect.DeepEqual(projection.Tree.TrunkRed[1].Closed, closed) {
				t.Fatalf("satisfied closure was rewritten: %+v", projection.Tree.TrunkRed[1].Closed)
			}
		})
	}
}
