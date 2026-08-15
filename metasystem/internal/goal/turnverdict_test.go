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

// GOAL-04: the dual-slot block-once state machine — the G-01 sequence:
// goal-block, open-work-block, clear, and NO re-block of the unchanged
// goal.
func TestVerdictDualSlotSequence(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "the goal", "Ship it.")

	// 1. Nothing scanned: the goal blocks once, byte-verbatim step.
	v, err := s.TurnVerdict(ScanResult{}, "session-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if !v.ShouldBlock || *v.BlockSource != "goal" || !strings.Contains(v.Display, "Ship it.") {
		t.Fatalf("goal did not block first: %+v", v)
	}

	// 2. Open work appears: open-work blocks once.
	scan := ScanResult{Open: []Item{openItem("plans/x.md next: do")}}
	v, _ = s.TurnVerdict(scan, "session-1", "")
	if !v.ShouldBlock || *v.BlockSource != "open-work" {
		t.Fatalf("open work did not block: %+v", v)
	}
	// Same signature again: reported, not re-blocked.
	v, _ = s.TurnVerdict(scan, "session-1", "")
	if v.ShouldBlock {
		t.Fatalf("unchanged open work re-blocked: %+v", v)
	}

	// 3. Work clears: the unchanged goal does NOT re-block (its revision
	// is spent), and the all-clear names the goal.
	v, _ = s.TurnVerdict(ScanResult{}, "session-1", "")
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
	v, _ = s.TurnVerdict(ScanResult{}, "session-1", "")
	if !v.ShouldBlock || !strings.Contains(v.Display, "Ship it harder.") {
		t.Fatalf("a re-armed revision did not block: %+v", v)
	}

	// A different session has its own slots.
	v, _ = s.TurnVerdict(ScanResult{}, "session-2", "")
	if !v.ShouldBlock {
		t.Fatal("a fresh session inherited a spent revision")
	}
}

// GOAL-07: precedence — busy suppresses everything; human-waits suppress
// the goal clause; stale plans never block; unreadable vetoes both ways.
func TestPrecedenceLadder(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "the goal", "Ship it.")

	v, _ := s.TurnVerdict(ScanResult{Busy: []Item{{Kind: "mission", Id: "m1", Detail: "mission m1 [running]"}}}, "s", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "STILL WORKING") || strings.Contains(v.Display, "Ship it.") {
		t.Fatalf("busy precedence wrong: %+v", v)
	}

	v, _ = s.TurnVerdict(ScanResult{WaitingOnHuman: []Item{{Kind: "plan", Id: "w", Detail: "plans/w.md waits on the human"}}}, "s", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "WAITING ON THE HUMAN") || strings.Contains(v.Display, "Ship it.") {
		t.Fatalf("human-wait precedence wrong: %+v", v)
	}

	// Stale plans are warning-only: they ride Diagnostics/display via the
	// scanner, never the block path — an all-empty-but-stale scan lets
	// the goal block normally.
	v, _ = s.TurnVerdict(ScanResult{StalePlans: []Item{{Kind: "plan", Id: "old", Detail: "plans/old.md is stale"}}}, "s", "")
	if !v.ShouldBlock || *v.BlockSource != "goal" {
		t.Fatalf("stale plans changed the goal outcome: %+v", v)
	}

	// GOAL-17: unreadable vetoes the goal block AND the all-clear.
	v, _ = s.TurnVerdict(ScanResult{Unreadable: []string{"plans/broken.md: permission denied"}}, "s2", "")
	if v.ShouldBlock || strings.Contains(v.Display, "NOTHING LEFT") || !strings.Contains(v.Display, "unreadable") {
		t.Fatalf("unreadable veto wrong: %+v", v)
	}
	if len(v.Diagnostics) != 1 {
		t.Fatalf("unreadable path missing from diagnostics: %+v", v.Diagnostics)
	}
}

// GOAL-10: pre-adoption absence is advisory; post-adoption deletion is
// degraded with the all-clear vetoed and reconcile named.
func TestAbsenceAdvisoryVsDeletionDegraded(t *testing.T) {
	s := testStore(t)
	v, _ := s.TurnVerdict(ScanResult{}, "s", "")
	if v.LedgerStatus != "absent" || v.ShouldBlock || !strings.Contains(v.Display, "`goal open` starts one") || !strings.Contains(v.Display, "NOTHING LEFT") {
		t.Fatalf("pre-adoption absence not advisory: %+v", v)
	}

	mustOpen(t, s, mainHolder, "g", "goal", "Do.")
	os.Remove(LedgerPath(s.Root))
	v, _ = s.TurnVerdict(ScanResult{}, "s", "")
	if v.LedgerStatus != "degraded" || v.ShouldBlock || strings.Contains(v.Display, "NOTHING LEFT") || !strings.Contains(v.Display, "reconcile") {
		t.Fatalf("post-adoption deletion not degraded: %+v", v)
	}
}

// GOAL-20: the queued-only ledger blocks once naming the first queued
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

	v, _ := s.TurnVerdict(ScanResult{}, "s", "")
	if v.LedgerStatus != "queued-only" || !v.ShouldBlock || !strings.Contains(v.Display, "goal promote a") {
		t.Fatalf("queued-only verdict wrong: %+v", v)
	}
	// Once.
	v, _ = s.TurnVerdict(ScanResult{}, "s", "")
	if v.ShouldBlock {
		t.Fatalf("queued-only re-blocked: %+v", v)
	}
}

// GOAL-01 + GOAL-15: a goal-free declaration over a moved world blocks
// once; renewal re-arms the all-clear.
func TestGoalFreeStaleness(t *testing.T) {
	s := testStore(t)
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	v, _ := s.TurnVerdict(ScanResult{}, "s", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "goal-free declared") {
		t.Fatalf("fresh declaration did not read all-clear: %+v", v)
	}

	// The world moves.
	os.MkdirAll(filepath.Join(s.Root, "plans"), 0o755)
	os.WriteFile(filepath.Join(s.Root, "plans", "new-work.md"), []byte("x"), 0o644)
	v, _ = s.TurnVerdict(ScanResult{}, "s", "")
	if !v.ShouldBlock || !strings.Contains(v.Display, "predates new work") {
		t.Fatalf("stale declaration did not block: %+v", v)
	}
	// Once per world.
	v, _ = s.TurnVerdict(ScanResult{}, "s", "")
	if v.ShouldBlock {
		t.Fatalf("stale declaration re-blocked: %+v", v)
	}
	// A FURTHER world change blocks once more.
	os.WriteFile(filepath.Join(s.Root, "plans", "even-newer.md"), []byte("y"), 0o644)
	v, _ = s.TurnVerdict(ScanResult{}, "s", "")
	if !v.ShouldBlock {
		t.Fatalf("a further world change did not block: %+v", v)
	}
	// Renewal restores the all-clear.
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	v, _ = s.TurnVerdict(ScanResult{}, "s", "")
	if v.ShouldBlock || !strings.Contains(v.Display, "goal-free declared") {
		t.Fatalf("renewal did not restore the all-clear: %+v", v)
	}
}

// GOAL-04: the sessions map caps at 128 oldest-evicted, session ids
// normalize, and concurrent verdicts serialize under the flock.
func TestSessionMapCapAndHygiene(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	base := time.Unix(1786800000, 0)
	for i := 0; i < 140; i++ {
		tick := base.Add(time.Duration(i) * time.Minute)
		s.Now = func() time.Time { return tick }
		if _, err := s.TurnVerdict(ScanResult{}, fmt.Sprintf("session-%03d", i), ""); err != nil {
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
	if _, err := s.TurnVerdict(ScanResult{}, "../../etc/passwd", ""); err != nil {
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
	if _, err := s.TurnVerdict(ScanResult{}, "fresh", ""); err != nil {
		t.Fatal(err)
	}
	state, _ = s.loadVerdictState()
	if len(state.Sessions) != 1 {
		t.Fatalf("expiry kept %d sessions", len(state.Sessions))
	}
}

// GOAL-04: the watchdog protocol — changed surfaces, same suppresses,
// clear resets, same surfaces again; concurrent calls surface exactly
// once.
func TestWatchdogProtocol(t *testing.T) {
	s := testStore(t)
	mustOpen(t, s, mainHolder, "g", "goal", "Do.")

	digest := strings.Repeat("a", 64)
	v, _ := s.TurnVerdict(ScanResult{}, "s", digest)
	if !v.SurfaceWatchdog {
		t.Fatal("new digest did not surface")
	}
	v, _ = s.TurnVerdict(ScanResult{}, "s", digest)
	if v.SurfaceWatchdog {
		t.Fatal("same digest surfaced twice")
	}
	// No findings clears the slot...
	v, _ = s.TurnVerdict(ScanResult{}, "s", "")
	if v.SurfaceWatchdog {
		t.Fatal("clear surfaced")
	}
	// ...so the same digest surfaces again (recover-then-warn-again).
	v, _ = s.TurnVerdict(ScanResult{}, "s", digest)
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
			v, err := s.TurnVerdict(ScanResult{}, "s", fresh)
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

// GOAL-17 tail: inventory failure (as Unreadable) vetoes even when the
// ledger is goal-free-fresh — no all-clear over unknown activity.
func TestInventoryFailureVetoes(t *testing.T) {
	s := testStore(t)
	if _, err := s.DeclareFree(human); err != nil {
		t.Fatal(err)
	}
	v, _ := s.TurnVerdict(ScanResult{Unreadable: []string{"runners/m1.json: runner liveness unknown"}}, "s", "")
	if v.ShouldBlock || strings.Contains(v.Display, "NOTHING LEFT") {
		t.Fatalf("inventory failure did not veto: %+v", v)
	}
}
