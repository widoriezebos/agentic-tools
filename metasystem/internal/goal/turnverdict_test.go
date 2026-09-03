package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func openItem(detail string) Item { return Item{Kind: "plan", Id: detail, Detail: detail} }

// The dual-slot block-once state machine — the canonical sequence:
// goal-block, open-work-block, clear, and NO re-block of the unchanged
// goal.
func TestVerdictDualSlotSequence(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "the goal", "Ship it.")

	// 1. Nothing scanned: the goal blocks once, byte-verbatim step.
	v, err := s.TurnVerdict(ScanResult{}, "session-1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !v.ShouldBlock || *v.BlockSource != "goal" || !strings.Contains(v.Display, "Ship it.") {
		t.Fatalf("goal did not block first: %+v", v)
	}

	// 2. Open work appears: open-work blocks once.
	scan := ScanResult{Open: []Item{openItem("plans/x.md next: do")}}
	v, _ = s.TurnVerdict(scan, "session-1", "", "")
	if !v.ShouldBlock || *v.BlockSource != "open-work" {
		t.Fatalf("open work did not block: %+v", v)
	}
	// Same signature again: reported, not re-blocked.
	v, _ = s.TurnVerdict(scan, "session-1", "", "")
	if v.ShouldBlock {
		t.Fatalf("unchanged open work re-blocked: %+v", v)
	}

	// 3. Work clears: the unchanged goal does NOT re-block (its revision
	// is spent), and the all-clear names the goal.
	v, _ = s.TurnVerdict(ScanResult{}, "session-1", "", "")
	if v.ShouldBlock {
		t.Fatalf("the spent goal revision re-blocked: %+v", v)
	}
	if !strings.Contains(v.Display, "NOTHING LEFT") || !strings.Contains(v.Display, "g") {
		t.Fatalf("all-clear does not name the goal: %s", v.Display)
	}

	// 4. The step changes: the new revision blocks once more.
	if _, err := s.SetNext(mainHolder, "Ship it harder."); err != nil {
		t.Fatal(err)
	}
	v, _ = s.TurnVerdict(ScanResult{}, "session-1", "", "")
	if !v.ShouldBlock || !strings.Contains(v.Display, "Ship it harder.") {
		t.Fatalf("a re-armed revision did not block: %+v", v)
	}

	// A different session has its own slots.
	v, _ = s.TurnVerdict(ScanResult{}, "session-2", "", "")
	if !v.ShouldBlock {
		t.Fatal("a fresh session inherited a spent revision")
	}
}

// Precedence — busy suppresses everything; human-waits suppress
// the goal clause; stale plans never block; unreadable vetoes both ways.
func TestPrecedenceLadder(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "the goal", "Ship it.")

	v, _ := s.TurnVerdict(ScanResult{Busy: []Item{{Kind: "mission", Id: "m1", Detail: "mission m1 [running]"}}}, "s", "", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "STILL WORKING") || strings.Contains(v.Display, "Ship it.") {
		t.Fatalf("busy precedence wrong: %+v", v)
	}

	v, _ = s.TurnVerdict(ScanResult{WaitingOnHuman: []Item{{Kind: "plan", Id: "w", Detail: "plans/w.md waits on the human"}}}, "s", "", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "WAITING ON THE HUMAN") || strings.Contains(v.Display, "Ship it.") {
		t.Fatalf("human-wait precedence wrong: %+v", v)
	}

	// Stale plans are warning-only: they ride Diagnostics/display via the
	// scanner, never the block path — an all-empty-but-stale scan lets
	// the goal block normally.
	v, _ = s.TurnVerdict(ScanResult{StalePlans: []Item{{Kind: "plan", Id: "old", Detail: "plans/old.md is stale"}}}, "s", "", "")
	if !v.ShouldBlock || *v.BlockSource != "goal" {
		t.Fatalf("stale plans changed the goal outcome: %+v", v)
	}

	// Unreadable vetoes the goal block AND the all-clear.
	v, _ = s.TurnVerdict(ScanResult{Unreadable: []string{"plans/broken.md: permission denied"}}, "s2", "", "")
	if v.ShouldBlock || strings.Contains(v.Display, "NOTHING LEFT") || !strings.Contains(v.Display, "unreadable") {
		t.Fatalf("unreadable veto wrong: %+v", v)
	}
	if len(v.Diagnostics) != 1 {
		t.Fatalf("unreadable path missing from diagnostics: %+v", v.Diagnostics)
	}
}

// Pre-adoption absence is advisory; post-adoption deletion is
// degraded with the all-clear vetoed and reconcile named.
func TestAbsenceAdvisoryVsDeletionDegraded(t *testing.T) {
	s := testStore(t)
	v, _ := s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.LedgerStatus != "absent" || v.ShouldBlock || !strings.Contains(v.Display, "`goal open` starts one") || !strings.Contains(v.Display, "NOTHING LEFT") {
		t.Fatalf("pre-adoption absence not advisory: %+v", v)
	}

	mustOpen(t, s, mainHolder, "g", "goal", "Do.")
	os.Remove(LedgerPath(s.Root))
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.LedgerStatus != "degraded" || v.ShouldBlock || strings.Contains(v.Display, "NOTHING LEFT") || !strings.Contains(v.Display, "reconcile") {
		t.Fatalf("post-adoption deletion not degraded: %+v", v)
	}
}

// The queued-only ledger blocks once naming the first queued
// goal, never a silent all-clear.
func TestQueuedOnlyVerdict(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "a", "goal a", "Do a.")
	mustOpen(t, s, mainHolder, "b", "goal b", "Do b.")
	// Reach queued-only via done --then then reopen of the done goal...
	// simpler: park current with --then b, then park b landing... park
	// requires successor. Reopen path: done a --then b, done b --and-none
	// refuses (queue empty is fine)... Construct directly instead: done
	// a --then b leaves b Current; reopen a; park b --then a... Simplest
	// real path: reopen drops Goal-free after everything concludes.
	if _, err := s.Done(mainHolder, "a", "landed", "b", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Done(mainHolder, "b", "landed", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reopen(mainHolder, "a", "Round two."); err != nil {
		t.Fatal(err)
	}
	ledger, _, _ := s.ReadLedger()
	if ledger.Current != nil || len(ledger.Queued) != 1 || ledger.Free != nil {
		t.Fatalf("not queued-only: %+v", ledger)
	}

	v, _ := s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.LedgerStatus != "queued-only" || !v.ShouldBlock || !strings.Contains(v.Display, "IDLE WITH BACKLOG") {
		t.Fatalf("queued-only verdict wrong: %+v", v)
	}
	// Legacy backlog is the same every-stop invariant as converted backlog.
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if !v.ShouldBlock || v.BlockSource == nil || *v.BlockSource != "idle-backlog" {
		t.Fatalf("the second unchanged legacy stop must still block: %+v", v)
	}
}

func TestQueuedOnlyVerdictIgnoresLegacyBlockOnceDigest(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "legacy-queue", "legacy queued goal", "Promote it.")
	if _, err := s.Done(mainHolder, "legacy-queue", "landed", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reopen(mainHolder, "legacy-queue", "Queue it again."); err != nil {
		t.Fatal(err)
	}
	first, digest := s.queuedFrontier()
	if first != "legacy-queue" || digest == "" {
		t.Fatalf("queued frontier missing: first=%q digest=%q", first, digest)
	}
	state := &verdictState{SchemaVersion: 1, Sessions: map[string]*sessionState{
		"legacy-queue-session": {
			LastTouched: s.nowISO(), BlockedGoalRevisions: []string{digest},
		},
	}}
	if err := s.saveVerdictState(state); err != nil {
		t.Fatal(err)
	}

	verdict, err := s.TurnVerdict(ScanResult{}, "legacy-queue-session", "", "")
	if err != nil || !verdict.ShouldBlock || verdict.BlockSource == nil || *verdict.BlockSource != "idle-backlog" ||
		!strings.Contains(verdict.Display, "legacy-queue") {
		t.Fatalf("a spent legacy queue digest must not suppress the invariant: %+v %v", verdict, err)
	}
}

func TestClaimedSessionReblocksOnceWhenTheSharedQueueChanges(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	mustGit(t, root, "config", "metasystem.goal.machine", "mac-a")
	request := verbReq(root, "01J5X00000000000000000TV10", "mac-a")
	if result, err := Open(request, "claimed-here", "Keep working here.", OriginMain, "Continue it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open claimed goal: %+v %v", result, err)
	}
	request.Ulid = "01J5X00000000000000000TV11"
	if result, err := claimApprovedForTest(t, request, "claimed-here", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim goal: %+v %v", result, err)
	}
	request.Ulid = "01J5X00000000000000000TV12"
	if result, err := Open(request, "queued-pin", "Wait in the queue.", OriginMain, "Claim later."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open queued goal: %+v %v", result, err)
	}
	store := &Store{Root: root, Now: func() time.Time { return request.Now }, Prober: installIdleLiveClaim(t, root, "lin-1")}
	first, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || !first.ShouldBlock {
		t.Fatalf("initial claimed world did not block: %+v %v", first, err)
	}
	spent, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || spent.ShouldBlock {
		t.Fatalf("unchanged claimed world reblocked: %+v %v", spent, err)
	}
	request.Ulid = "01J5X00000000000000000TV13"
	request.Actor.Human = "Wido"
	if result, err := SetPin(request, "queued-pin", "mac-a"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("pin queued goal: %+v %v", result, err)
	}
	changed, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || !changed.ShouldBlock || !strings.Contains(changed.Display, "shared goal queue changed") || !strings.Contains(changed.Display, "queued-pin") {
		t.Fatalf("queue change did not reblock the claimed session once: %+v %v", changed, err)
	}
	again, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || again.ShouldBlock {
		t.Fatalf("unchanged queue digest reblocked twice: %+v %v", again, err)
	}
	request.Ulid = "01J5X00000000000000000TV14"
	if result, err := SetPin(request, "queued-pin", "-"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("clear queued pin: %+v %v", result, err)
	}
	cleared, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || !cleared.ShouldBlock {
		t.Fatalf("pin clearing did not reblock the claimed session: %+v %v", cleared, err)
	}
	request.Ulid = "01J5X00000000000000000TV15"
	approveGoalForTest(t, request, "queued-pin", testBudget())
	request.Actor = Actor{Machine: "mac-b", Lineage: "turn-verdict-fixture"}
	if result, err := Claim(request, "queued-pin"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim the last queued goal elsewhere: %+v %v", result, err)
	}
	emptied, err := store.TurnVerdict(ScanResult{}, "claimed-queue-session", "", "")
	if err != nil || !emptied.ShouldBlock || !strings.Contains(emptied.Display, "now empty") {
		t.Fatalf("the final queue departure did not reblock the claimed session: %+v %v", emptied, err)
	}
}

func TestClaimedSessionBaselinesAnUnchangedQueueWithoutFalseChange(t *testing.T) {
	_, root := oneClone(t)
	seedLedger(t, root)
	mustGit(t, root, "config", "metasystem.goal.machine", "mac-a")
	request := verbReq(root, "01J5X00000000000000000TV20", "mac-a")
	if result, err := Open(request, "steady-claim", "Keep the steady claim.", OriginMain, "Continue it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open steady claim: %+v %v", result, err)
	}
	request.Ulid = "01J5X00000000000000000TV21"
	if result, err := claimApprovedForTest(t, request, "steady-claim", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim steady goal: %+v %v", result, err)
	}
	store := &Store{Root: root, Now: func() time.Time { return request.Now }, Prober: installIdleLiveClaim(t, root, "lin-1")}
	first, err := store.TurnVerdict(ScanResult{}, "fresh-steady-session", "", "")
	if err != nil || !first.ShouldBlock || strings.Contains(first.Display, "shared goal queue changed") {
		t.Fatalf("a fresh session falsely described its empty queue baseline as a change: %+v %v", first, err)
	}
	state, err := store.loadVerdictState()
	if err != nil {
		t.Fatal(err)
	}
	state.Sessions["fresh-steady-session"].ObservedQueueDigest = ""
	if err := store.saveVerdictState(state); err != nil {
		t.Fatal(err)
	}
	rollout, err := store.TurnVerdict(ScanResult{}, "fresh-steady-session", "", "")
	if err != nil || rollout.ShouldBlock || strings.Contains(rollout.Display, "shared goal queue changed") {
		t.Fatalf("an upgraded pre-existing session falsely described its steady queue as a change: %+v %v", rollout, err)
	}
}

// A goal-free declaration over a moved world blocks
// once; renewal re-arms the all-clear.
func TestGoalFreeStaleness(t *testing.T) {
	s := testStore(t)
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	v, _ := s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "goal-free declared") {
		t.Fatalf("fresh declaration did not read all-clear: %+v", v)
	}

	// The world moves.
	os.MkdirAll(filepath.Join(s.Root, "plans"), 0o755)
	os.WriteFile(filepath.Join(s.Root, "plans", "new-work.md"), []byte("x"), 0o644)
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if !v.ShouldBlock || !strings.Contains(v.Display, "predates new work") {
		t.Fatalf("stale declaration did not block: %+v", v)
	}
	// Once per world.
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.ShouldBlock {
		t.Fatalf("stale declaration re-blocked: %+v", v)
	}
	// A FURTHER world change blocks once more.
	os.WriteFile(filepath.Join(s.Root, "plans", "even-newer.md"), []byte("y"), 0o644)
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if !v.ShouldBlock {
		t.Fatalf("a further world change did not block: %+v", v)
	}
	// Renewal restores the all-clear.
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "goal-free declared") {
		t.Fatalf("renewal did not restore the all-clear: %+v", v)
	}
}

// The sessions map caps at 128 oldest-evicted, session ids
// normalize, and concurrent verdicts serialize under the flock.
func TestSessionMapCapAndHygiene(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	base := time.Unix(1786800000, 0)
	for i := 0; i < 140; i++ {
		tick := base.Add(time.Duration(i) * time.Minute)
		s.Now = func() time.Time { return tick }
		if _, err := s.TurnVerdict(ScanResult{}, fmt.Sprintf("session-%03d", i), "", ""); err != nil {
			t.Fatal(err)
		}
	}
	state, err := s.loadVerdictState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Sessions) != maxSessions {
		t.Fatalf("sessions map holds %d, cap is %d", len(state.Sessions), maxSessions)
	}
	if _, oldest := state.Sessions["session-000"]; oldest {
		t.Fatal("the oldest session survived eviction")
	}

	// A path-shaped session id normalizes to its sha256.
	if _, err := s.TurnVerdict(ScanResult{}, "../../etc/passwd", "", ""); err != nil {
		t.Fatal(err)
	}
	state, _ = s.loadVerdictState()
	for id := range state.Sessions {
		if strings.Contains(id, "/") || strings.Contains(id, "..") {
			t.Fatalf("unnormalized session id stored: %q", id)
		}
	}

	// 30-day expiry drops dormant sessions on any write.
	s.Now = func() time.Time { return base.Add(40 * 24 * time.Hour) }
	if _, err := s.TurnVerdict(ScanResult{}, "fresh", "", ""); err != nil {
		t.Fatal(err)
	}
	state, _ = s.loadVerdictState()
	if len(state.Sessions) != 1 {
		t.Fatalf("expiry kept %d sessions", len(state.Sessions))
	}
}

// The watchdog protocol — changed surfaces, same suppresses,
// clear resets, same surfaces again; concurrent calls surface exactly
// once.
func TestWatchdogProtocol(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	digest := strings.Repeat("a", 64)
	v, _ := s.TurnVerdict(ScanResult{}, "s", digest, "")
	if !v.SurfaceWatchdog {
		t.Fatal("new digest did not surface")
	}
	v, _ = s.TurnVerdict(ScanResult{}, "s", digest, "")
	if v.SurfaceWatchdog {
		t.Fatal("same digest surfaced twice")
	}
	// No findings clears the slot...
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if v.SurfaceWatchdog {
		t.Fatal("clear surfaced")
	}
	// ...so the same digest surfaces again (recover-then-warn-again).
	v, _ = s.TurnVerdict(ScanResult{}, "s", digest, "")
	if !v.SurfaceWatchdog {
		t.Fatal("post-clear digest did not re-surface")
	}

	// Concurrent Stop calls: exactly one surfaces a fresh digest.
	fresh := strings.Repeat("b", 64)
	var wg sync.WaitGroup
	surfaced := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := s.TurnVerdict(ScanResult{}, "s", fresh, "")
			if err == nil {
				surfaced <- v.SurfaceWatchdog
			}
		}()
	}
	wg.Wait()
	close(surfaced)
	count := 0
	for got := range surfaced {
		if got {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("fresh digest surfaced %d times, want exactly 1", count)
	}
}

// Unreadable-veto tail: inventory failure (as Unreadable) vetoes even when the
// ledger is goal-free-fresh — no all-clear over unknown activity.
func TestInventoryFailureVetoes(t *testing.T) {
	s := testStore(t)
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	v, _ := s.TurnVerdict(ScanResult{Unreadable: []string{"runners/m1.json: runner liveness unknown"}}, "s", "", "")
	if v.ShouldBlock || strings.Contains(v.Display, "NOTHING LEFT") {
		t.Fatalf("inventory failure did not veto: %+v", v)
	}
}

// Unwatched work blocks once before Busy can hide it,
// keys on lifecycle tags so a reused job id re-arms, and run warnings ride
// above the ladder.
func TestUnwatchedAndWarnings(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	job := JobFact{Id: "j1", MainId: "main-1", StartedAt: "2026-08-15T10:00:00Z", Status: "running"}
	busy := []Item{{Kind: "job", Id: "j1", Detail: "impl j1 [running, codex]"}}

	// Unwatched job, Busy present: the block fires DESPITE Busy.
	v, _ := s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{job}}, "s", "", "main-1")
	if !v.ShouldBlock || *v.BlockSource != "unwatched-work" || !strings.Contains(v.Display, "unwatched") {
		t.Fatalf("unwatched did not block before Busy: %+v", v)
	}
	// Same set again: reported once, no re-block; Busy shows.
	v, _ = s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{job}}, "s", "", "main-1")
	if v.ShouldBlock || !strings.Contains(v.Display, "STILL WORKING") {
		t.Fatalf("unwatched re-blocked or Busy hidden: %+v", v)
	}
	// The SAME job id with a NEW startedAt is a new incarnation
	// and re-arms.
	reused := job
	reused.StartedAt = "2026-08-15T11:00:00Z"
	v, _ = s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{reused}}, "s", "", "main-1")
	if !v.ShouldBlock {
		t.Fatalf("a reused job id did not re-arm: %+v", v)
	}
	// A watched job never blocks; a foreign main's job never blocks us.
	watched := reused
	watched.WaiterLive = true
	v, _ = s.TurnVerdict(ScanResult{Jobs: []JobFact{watched}}, "s", "", "main-1")
	if v.BlockSource != nil && *v.BlockSource == "unwatched-work" {
		t.Fatalf("a watched job blocked as unwatched: %+v", v)
	}
	foreign := JobFact{Id: "j9", MainId: "main-other", StartedAt: "x", Status: "running"}
	v, _ = s.TurnVerdict(ScanResult{Jobs: []JobFact{foreign}}, "s-f", "", "main-1")
	if v.BlockSource != nil && *v.BlockSource == "unwatched-work" {
		t.Fatalf("a foreign job blocked as unwatched: %+v", v)
	}

	// Run warnings above the ladder: red with continuation verbatim,
	// even while Busy (mixed-state display test Busy+RunRed).
	red := RunFact{Id: "r1", MainId: "main-1", Generation: 1, Nonce: "n", Status: "red", ExpectRed: "read the log at /tmp/r1.log"}
	v, _ = s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{watched}, Runs: []RunFact{red}}, "s", "", "main-1")
	if !strings.Contains(v.Display, "went red") || !strings.Contains(v.Display, "read the log at /tmp/r1.log") || !strings.Contains(v.Display, "STILL WORKING") {
		t.Fatalf("Busy hid the red warning or the continuation: %s", v.Display)
	}
	// Busy+RunUnreadable: both visible.
	v, _ = s.TurnVerdict(ScanResult{Busy: busy, Jobs: []JobFact{watched}, RunUnreadable: []string{"runs/x.json: torn"}}, "s", "", "main-1")
	if !strings.Contains(v.Display, "runs/x.json: torn") || !strings.Contains(v.Display, "STILL WORKING") {
		t.Fatalf("Busy hid the run-unreadable line: %s", v.Display)
	}
}

// Greens surface exactly once per session in terminal-sequence
// order, and any unreadable run record freezes the cursor so a delayed
// green is never skipped.
func TestGreenPrefixConsistency(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	greenB := RunFact{Id: "run-b", Status: "green", TerminalSeq: 11, ExpectGreen: "ship B"}
	// Turn 1: B green at seq 11, but run A's record is unreadable —
	// cursor FROZEN, nothing surfaces.
	v, _ := s.TurnVerdict(ScanResult{Runs: []RunFact{greenB}, RunUnreadable: []string{"runs/run-a.json: unreadable"}}, "s", "", "")
	if strings.Contains(v.Display, "finished green") {
		t.Fatalf("green surfaced through the freeze: %s", v.Display)
	}
	// Turn 2: A recovered and concluded green at seq 10 — BOTH surface,
	// in order, once.
	greenA := RunFact{Id: "run-a", Status: "green", TerminalSeq: 10, ExpectGreen: "ship A"}
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{greenA, greenB}}, "s", "", "")
	aIdx := strings.Index(v.Display, "run-a finished green")
	bIdx := strings.Index(v.Display, "run-b finished green")
	if aIdx < 0 || bIdx < 0 || aIdx > bIdx {
		t.Fatalf("greens missing or out of order: %s", v.Display)
	}
	// Turn 3: neither resurfaces.
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{greenA, greenB}}, "s", "", "")
	if strings.Contains(v.Display, "finished green") {
		t.Fatalf("a surfaced green repeated: %s", v.Display)
	}
	// A fresh session gets its own cursor.
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{greenA, greenB}}, "s2", "", "")
	if !strings.Contains(v.Display, "run-a finished green") {
		t.Fatalf("a fresh session saw no greens: %s", v.Display)
	}
}

// A HUMAN caller (empty mainId) owns human-launched runs (null
// coordinates): the unwatched rule fires for them too.
func TestHumanOwnsHumanRuns(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")
	humanRun := RunFact{Id: "h-run", MainId: "", Generation: 1, Nonce: "n", Status: "running"}
	v, _ := s.TurnVerdict(ScanResult{Runs: []RunFact{humanRun}}, "hs", "", "")
	if !v.ShouldBlock || *v.BlockSource != "unwatched-work" {
		t.Fatalf("a human's unwatched run did not block: %+v", v)
	}
	// A MAIN caller does NOT own the human's run.
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{humanRun}}, "hs2", "", "main-1")
	if v.BlockSource != nil && *v.BlockSource == "unwatched-work" {
		t.Fatalf("a main owned the human's run: %+v", v)
	}
}

// The green cursor trusts the DISK read inside the
// verdict flock, not the scanner's snapshot — a scan that predates a
// run's conclusion must not let the cursor advance past it, and a torn
// record on disk freezes the cursor even when the scan looked clean.
func TestGreenCursorRereadsDisk(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")
	writeGreen := func(id string, seq int) {
		record := `{"schemaVersion":1,"runId":"` + id + `","kind":"suite","display":"x","custody":"wrapped",` +
			`"generation":1,"pid":null,"pidStartedAt":null,"pgid":null,` +
			`"launchNonce":"` + strings.Repeat("ef", 16) + `","log":"/tmp/x.log","startedAt":"2026-08-15T10:00:00Z",` +
			`"sessionId":"s","goalId":"","staleAfterMin":30,"windDownMin":10,` +
			`"endedAt":"2026-08-15T10:05:00Z","terminalSeq":` + fmt.Sprintf("%d", seq) + `,` +
			`"evidence":{"mode":"exit-sidecar"},"expect":{"green":"ship ` + id + `","red":"","hung":"","unknown":""},` +
			`"status":"green","acked":false}`
		dir := filepath.Join(s.Root, "artifacts", "agents", "runs")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(record), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeGreen("disk-run", 5)

	// The scan snapshot is STALE — it never saw disk-run — yet the green
	// surfaces because the verdict re-reads the records under its flock.
	v, _ := s.TurnVerdict(ScanResult{}, "s", "", "")
	if !strings.Contains(v.Display, "disk-run finished green") || !strings.Contains(v.Display, "ship disk-run") {
		t.Fatalf("the stale scan hid the on-disk green: %s", v.Display)
	}
	// Once surfaced, never repeated.
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if strings.Contains(v.Display, "finished green") {
		t.Fatalf("the disk green repeated: %s", v.Display)
	}

	// A torn record ON DISK freezes the cursor even when the scan carries
	// a clean green fact.
	writeGreen("late-run", 6)
	dir := filepath.Join(s.Root, "artifacts", "agents", "runs")
	if err := os.WriteFile(filepath.Join(dir, "torn.json"), []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, _ = s.TurnVerdict(ScanResult{Runs: []RunFact{{Id: "late-run", Status: "green", TerminalSeq: 6, ExpectGreen: "x"}}}, "s", "", "")
	if strings.Contains(v.Display, "finished green") {
		t.Fatalf("green surfaced through an on-disk freeze: %s", v.Display)
	}
	if err := os.Remove(filepath.Join(dir, "torn.json")); err != nil {
		t.Fatal(err)
	}
	v, _ = s.TurnVerdict(ScanResult{}, "s", "", "")
	if !strings.Contains(v.Display, "late-run finished green") {
		t.Fatalf("the frozen green never surfaced after recovery: %s", v.Display)
	}
}
