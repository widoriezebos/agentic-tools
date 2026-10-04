package goal

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func flakeFixture() FlakeRecordArgs {
	return FlakeRecordArgs{Unit: "metasystem/internal/proofrun", Batch: "batch-1", Commit: "commit-1", Tree: "tree-1",
		Attempt: "red-1", LogPath: "logs/red", LogDigest: "red-digest", Load: 2.5, Repeat: "whole",
		Tests: []string{"TestOne"}, Surfaces: []string{"checks"},
		Rerun: TrunkRedRerun{Attempt: "green-1", LogPath: "logs/green", LogDigest: "green-digest"}}
}

func flakeRequest(e Endpoint, n int) VerbRequest {
	r := verbReqFor(e, fmt.Sprintf("01J5X0000000000000000000F%d", n), "lane")
	r.Now = time.Date(2026, 10, 4, 12, n, 0, 0, time.UTC)
	return r
}

func seedFlake(t *testing.T, e Endpoint, goalID string, archived ...*GoalFile) {
	t.Helper()
	client := e.Repository.(*fakeGoalRepository)
	seed := client.store.commits[client.store.canonical]
	entry := testTrunkRedEntry("flaky:"+flakeFixture().Unit, "flaky:"+flakeFixture().Unit, "2026-10-04T10:00:00Z")
	entry.Class, entry.FixGoal, entry.Owner, entry.Holds = TrunkRedClassPendingFlake, goalID, TrunkRedOwner{}, []string{}
	seed.files[trunkRedPath] = RenderTrunkRed([]TrunkRedEntry{entry})
	for _, f := range archived {
		seed.files[recordsGoalsPrefix+f.Id+".md"] = RenderFile(f)
	}
	client.store.commits[client.store.canonical] = seed
}

func TestRecordFlakeOpensOnceAndAddsNewTestOnce(t *testing.T) {
	t.Parallel()
	e, client := fakeGoalEndpoint(t)
	r, args := flakeRequest(e, 1), flakeFixture()
	before := acceptedTipForEndpoint(t, e)
	first, err := RecordFlake(r, args)
	id := "fix-flaky-metasystem-internal-proofrun"
	if err != nil || first.FixGoal != id || first.Action != "opened" || first.Sightings != 1 {
		t.Fatalf("first sighting: %+v %v", first, err)
	}
	tree, _ := acceptedTreeForEndpoint(t, e)
	f, entry := tree.Live[id], tree.TrunkRed[0]
	want := "Flaky: metasystem/internal/proofrun (TestOne) failed in the lane's check of commit-1 on 2026-10-04, passed when repeated (whole); load 2.5; log logs/red; seen once."
	if f.NextStep != want {
		t.Fatalf("first next step: got %q, want %q", f.NextStep, want)
	}
	if len(tree.Live) != 1 || len(tree.TrunkRed) != 1 || f.State != StateQueued || f.Tier != 1 || f.Priority != 1 || f.Sequence != 1 || f.Approved != nil || f.Origin != OriginMain || entry.FixGoal != id || entry.Class != TrunkRedClassPendingFlake || len(entry.Holds) != 0 {
		t.Fatalf("new fix and register: goal=%+v entry=%+v", f, entry)
	}
	s := entry.Sightings[0]
	if s.Attempt != args.Attempt || s.Batch != args.Batch || s.BaseCommit != args.Commit || s.Tree != args.Tree || s.LogPath != args.LogPath || s.LogDigest != args.LogDigest || s.SeenAt != r.stamp() || s.Load != args.Load || s.Repeat != args.Repeat || !reflect.DeepEqual(s.Surfaces, args.Surfaces) || !reflect.DeepEqual(s.Rerun, &args.Rerun) {
		t.Fatalf("incomplete sighting: %+v", s)
	}
	if len(client.store.commits) != 2 || client.store.commits[first.Publish.Commit].parent != before {
		t.Fatal("the entry and its fix did not publish in one transaction")
	}
	replay, err := RecordFlake(r, args)
	if err != nil || replay.FixGoal != id || replay.Action != "opened" || replay.Sightings != 1 || acceptedTipForEndpoint(t, e) != first.Publish.Tip {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	args.Tests = []string{"TestOne", "TestTwo", "TestTwo"}
	for _, n := range []int{2, 3} {
		got, err := RecordFlake(flakeRequest(e, n), args)
		if err != nil || got.FixGoal != id || got.Action != "extended" || got.Sightings != n {
			t.Fatalf("recurrence: %+v %v", got, err)
		}
	}
	tree, _ = acceptedTreeForEndpoint(t, e)
	if len(tree.Live) != 1 || len(tree.TrunkRed) != 1 || len(tree.TrunkRed[0].Failures) != 2 || tree.TrunkRed[0].Failures[1].Name != "TestTwo" || strings.Count(tree.Live[id].NextStep, "Flaky:") != 3 || !strings.HasSuffix(tree.Live[id].NextStep, "seen 3 times.") {
		t.Fatalf("recurrence duplicated a fix or test: %+v %+v", tree.Live, tree.TrunkRed)
	}
}

func TestRecordFlakeReusesGeneratedFixAfterEntryClosed(t *testing.T) {
	t.Parallel()
	for _, state := range []string{StateClaimed, StateDone} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			id := "fix-flaky-metasystem-internal-proofrun"
			f := vGoal(id, state)
			f.NextStep = "Keep this step."
			var e Endpoint
			var client *fakeGoalRepository
			action := "reopened"
			if state == StateClaimed {
				f = approvedGoalFixture(f, testBudget())
				f.State = StateClaimed
				f.Claimed = newClaimRecord("another-seat", "another-session", f.History[len(f.History)-1].At, f.Revision)
				e, client = fakeGoalEndpoint(t, f)
				seedFlake(t, e, id)
				action = "extended"
			} else {
				e, client = fakeGoalEndpoint(t)
				seedFlake(t, e, id, f)
			}
			before, _ := acceptedTreeForEndpoint(t, e)
			closed := &before.TrunkRed[0]
			closed.Closed = &TrunkRedClosure{At: "2026-10-04T11:00:00Z", How: "hand", Opid: flakeRequest(e, 0).opid(), By: "wido", Why: "Resolved."}
			seed := client.store.commits[client.store.canonical]
			seed.files[trunkRedPath] = RenderTrunkRed(before.TrunkRed)
			client.store.commits[client.store.canonical] = seed
			r := flakeRequest(e, 1)
			result, err := RecordFlake(r, flakeFixture())
			if err != nil || result.Action != action || result.FixGoal != id || result.Sightings != 1 {
				t.Fatalf("record after closure: %+v %v", result, err)
			}
			after, _ := acceptedTreeForEndpoint(t, e)
			got := after.Live[id]
			if len(after.Live) != 1 || len(after.Done) != 0 || len(after.Abandoned) != 0 || got == nil {
				t.Fatalf("expected exactly one live fix: %+v", after)
			}
			entry := openTrunkRedByIdentity(after.TrunkRed, closed.Identity)
			if len(after.TrunkRed) != 2 || !reflect.DeepEqual(after.TrunkRed[0], *closed) || entry == nil || entry.ID != closed.ID+"-2" || entry.FixGoal != id {
				t.Fatalf("closed and new entries: %+v", after.TrunkRed)
			}
			if !strings.HasPrefix(got.NextStep, "Keep this step. Flaky:") || !strings.HasSuffix(got.NextStep, "seen once.") {
				t.Fatalf("next step: %q", got.NextStep)
			}
			if state == StateClaimed {
				if got.State != f.State || !reflect.DeepEqual(got.Claimed, f.Claimed) || !reflect.DeepEqual(got.Approved, f.Approved) || !reflect.DeepEqual(got.Budget, f.Budget) {
					t.Fatalf("live fix authority changed: before=%+v after=%+v", f, got)
				}
			} else if got.State != StateQueued || got.Approved != nil || got.Claimed != nil {
				t.Fatalf("reopened fix: %+v", got)
			}
			if replay, err := RecordFlake(r, flakeFixture()); err != nil || replay.Action != action || replay.FixGoal != id || replay.Sightings != 1 || acceptedTipForEndpoint(t, e) != result.Publish.Tip {
				t.Fatalf("replayed closure recurrence: %+v %v", replay, err)
			}
		})
	}
}

func TestRecordFlakeExtendsLiveGoalsWithoutChangingAuthority(t *testing.T) {
	t.Parallel()
	for _, state := range []string{StateQueued, StateApproved, StateClaimed, StateParked} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			f := approvedGoalFixture(vGoal("existing-fix", StateQueued), testBudget())
			f.State, f.NextStep = state, "Keep this step."
			if state == StateQueued {
				f.Approved, f.Budget = nil, nil
			}
			if state == StateClaimed {
				f.Claimed = newClaimRecord("another-seat", "another-session", f.History[len(f.History)-1].At, f.Revision)
			}
			if state == StateParked {
				f.Parked = &ParkRecord{By: "human:wido", At: "2026-10-04T10:00:00Z", Because: "waiting"}
			}
			e, _ := fakeGoalEndpoint(t, f)
			seedFlake(t, e, f.Id)
			before, _ := acceptedTreeForEndpoint(t, e)
			r, args := flakeRequest(e, 1), flakeFixture()
			args.Repeat = "alone"
			result, err := RecordFlake(r, args)
			if err != nil || result.Action != "extended" || result.FixGoal != f.Id || result.Sightings != 2 {
				t.Fatalf("extend: %+v %v", result, err)
			}
			after, _ := acceptedTreeForEndpoint(t, e)
			old, got := before.Live[f.Id], after.Live[f.Id]
			want := "Keep this step. Flaky: metasystem/internal/proofrun (TestOne) failed in the lane's check of commit-1 on 2026-10-04, passed when repeated (alone); load 2.5; log logs/red; seen 2 times."
			if got.NextStep != want || got.State != old.State || !reflect.DeepEqual(got.Claimed, old.Claimed) || !reflect.DeepEqual(got.Approved, old.Approved) || !reflect.DeepEqual(got.Parked, old.Parked) || !reflect.DeepEqual(got.Budget, old.Budget) {
				t.Fatalf("authority or next step changed: before=%+v after=%+v", old, got)
			}
			if state == StateClaimed || state == StateParked {
				next := "ordinary edit"
				result, err := Edit(flakeRequest(e, 2), f.Id, EditFields{NextStep: &next})
				if err != nil || result.Outcome != OutcomeRejected {
					t.Fatalf("ordinary edit no longer refuses: %+v %v", result, err)
				}
			}
		})
	}
}

func TestRecordFlakeReopensGeneratedFix(t *testing.T) {
	t.Parallel()
	e, _ := fakeGoalEndpoint(t)
	first, err := RecordFlake(flakeRequest(e, 1), flakeFixture())
	if err != nil {
		t.Fatal(err)
	}
	claim := flakeRequest(e, 2)
	if got, err := claimApprovedForTest(t, claim, first.FixGoal, testBudget()); err != nil || got.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", got, err)
	}
	if got, err := Done(flakeRequest(e, 3), first.FixGoal, "Fixed the intermittent failure."); err != nil || got.Outcome != OutcomeConfirmed {
		t.Fatalf("conclude: %+v %v", got, err)
	}
	result, err := RecordFlake(flakeRequest(e, 4), flakeFixture())
	if err != nil || result.FixGoal != first.FixGoal || result.Action != "reopened" || result.Sightings != 2 {
		t.Fatalf("reopen: %+v %v", result, err)
	}
	tree, _ := acceptedTreeForEndpoint(t, e)
	f := tree.Live[first.FixGoal]
	if len(tree.Live) != 1 || len(tree.Done) != 0 || len(tree.TrunkRed) != 1 || f.State != StateQueued || f.Approved != nil || f.Claimed != nil || f.Priority != 1 || !strings.Contains(f.NextStep, "seen 2 times.") {
		t.Fatalf("reopened tree: %+v", tree)
	}
}

func TestRecordFlakeRefusesAbandonedFixAtomically(t *testing.T) {
	t.Parallel()
	e, _ := fakeGoalEndpoint(t)
	f := vGoal("abandoned-fix", StateAbandoned)
	r := flakeRequest(e, 0)
	r.Actor.Human = "wido"
	touch(f, r, "abandon", []string{f.Id})
	f.History[len(f.History)-1].Reason = "obsolete"
	f.Abandoned = &AbandonRecord{By: r.Actor.historyActor(), At: r.stamp(), Revision: f.Revision, Opid: r.opid(), Because: "obsolete"}
	seedFlake(t, e, f.Id, f)
	before := acceptedTipForEndpoint(t, e)
	result, err := RecordFlake(flakeRequest(e, 1), flakeFixture())
	if err == nil || result.Publish.Outcome != OutcomeRejected || acceptedTipForEndpoint(t, e) != before {
		t.Fatalf("abandoned fix accepted or sighting leaked: %+v %v", result, err)
	}
	tree, _ := acceptedTreeForEndpoint(t, e)
	if len(tree.TrunkRed[0].Sightings) != 1 || len(tree.Live) != 0 {
		t.Fatalf("partial record: %+v", tree)
	}
}

func TestRecordFlakeRejectsUnconfirmedPublication(t *testing.T) {
	t.Parallel()
	for _, outcome := range []Outcome{OutcomeRejected, OutcomeExpired} {
		t.Run(string(outcome), func(t *testing.T) {
			t.Parallel()
			e, _ := fakeGoalEndpoint(t)
			r := flakeRequest(e, 1)
			before := acceptedTipForEndpoint(t, e)
			publish := func(e Endpoint, req PublishRequest) (PublishResult, error) {
				if outcome == OutcomeRejected {
					req.Validate = func(string) error { return errors.New("this change was refused") }
				} else {
					reads := 0
					req.now = func() time.Time {
						reads++
						if reads == 1 {
							return r.Now
						}
						return r.Now.Add(2 * DefaultPublishDeadline)
					}
					e = e.WithCASPublisher(func(Endpoint, string, string) (CASOutcome, error) {
						return CASRefused, errors.New("the shared list moved")
					})
				}
				return Publish(e, req)
			}
			result, err := recordFlake(r, flakeFixture(), publish)
			entry, readErr := ReadEntry(e.Root, r.opid())
			if err == nil || result.Publish.Outcome != outcome || readErr != nil || entry.Outcome != outcome || acceptedTipForEndpoint(t, e) != before {
				t.Fatalf("unconfirmed record: %+v %v journal=%+v %v", result, err, entry, readErr)
			}
		})
	}
}

func TestRecordFlakeExceptionKeepsOrdinaryMachineOpenRefused(t *testing.T) {
	t.Parallel()
	e, _ := fakeGoalEndpoint(t)
	if _, err := RecordFlake(flakeRequest(e, 1), flakeFixture()); err != nil {
		t.Fatal(err)
	}
	before := acceptedTipForEndpoint(t, e)
	risk := RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "A small known change."}
	_, err := OpenRisked(flakeRequest(e, 2), "ordinary-goal", "Do something.", OriginMain, "Start.", nil, nil, risk, 1, "", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "a seat opens only a defect") || acceptedTipForEndpoint(t, e) != before {
		t.Fatalf("ordinary machine open changed: %v", err)
	}
}
