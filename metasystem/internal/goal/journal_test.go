package goal

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func testIntentFor(verb string) Intent {
	return Intent{
		Verb: verb, Targets: []string{"fix-it"},
		Args: map[string]string{"reason": "because"},
	}
}

func TestJournalEntryIsDurableBeforeAction(t *testing.T) {
	root := t.TempDir()
	in := Intent{
		Verb: "edit", Targets: []string{"fix-it"},
		Args:   map[string]string{"keep": "5", "conclusion": "done well"},
		Deltas: []FieldDelta{{Target: "fix-it", Field: "intent", Old: "a", New: "b"}},
	}
	e, err := CreateEntry(root, "op-1", "mac-a", "lineage-1", in)
	if err != nil {
		t.Fatal(err)
	}
	if e.Phase != PhaseCreated || e.CreatedAt == "" {
		t.Fatalf("created is the first durable phase: %+v", e)
	}
	back, err := ReadEntry(root, "op-1")
	if err != nil {
		t.Fatal(err)
	}
	// The stored intent must be COMPLETE enough to rebuild without
	// the original process (R8-03).
	if back.Intent.Verb != "edit" || back.Intent.Args["conclusion"] != "done well" ||
		len(back.Intent.Deltas) != 1 || back.Intent.Deltas[0].New != "b" {
		t.Fatalf("the normalized command intent must round-trip whole: %+v", back.Intent)
	}
	if back.Owner.Pid != int64(os.Getpid()) {
		t.Fatalf("the creator owns the entry: %+v", back.Owner)
	}
}

func TestJournalRefusesADuplicateOpid(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateEntry(root, "op-2", "m", "l", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateEntry(root, "op-2", "m", "l", testIntentFor("claim")); err == nil {
		t.Fatal("a duplicate opid must refuse")
	}
}

func TestJournalPhasesAreMonotonic(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateEntry(root, "op-3", "m", "l", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	if err := MarkPushed(root, "op-3", "old-tip", 1, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := MarkTerminal(root, "op-3", OutcomeConfirmed, "opid in history"); err != nil {
		t.Fatal(err)
	}
	if err := MarkPushed(root, "op-3", "old-tip", 2, time.Now()); err == nil {
		t.Fatal("terminal → pushed must refuse; the machine never walks backward")
	}
	if err := MarkTerminal(root, "op-3", OutcomeLost, "x"); err == nil {
		t.Fatal("a second terminalization must refuse")
	}
	if err := RecordSteps(root, "op-3", "oid", ""); err == nil {
		t.Fatal("a terminal entry's steps never change")
	}
}

func TestPushedBlocksOwnCloneMutations(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateEntry(root, "op-4", "m", "l", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	if _, blocking, _ := PushedBlocking(root); blocking {
		t.Fatal("a created entry does not block; nothing has left the process")
	}
	if err := MarkPushed(root, "op-4", "tip", 1, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	e, blocking, err := PushedBlocking(root)
	if err != nil || !blocking || e.Opid != "op-4" {
		t.Fatalf("a pushed entry blocks until terminal: %v %v %v", e.Opid, blocking, err)
	}
	if err := MarkTerminal(root, "op-4", OutcomeConfirmed, "landed"); err != nil {
		t.Fatal(err)
	}
	if _, blocking, _ := PushedBlocking(root); blocking {
		t.Fatal("a terminal entry no longer blocks")
	}
}

// spawnForeignOwner starts a live process and rewrites the entry's
// owner to it, so this test process is NOT the owner.
func spawnForeignOwner(t *testing.T, root, opid string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	e, err := ReadEntry(root, opid)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := ownerOf(int64(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	e.Owner = owner
	if err := writeEntry(root, e); err != nil {
		t.Fatal(err)
	}
	return cmd
}

// ownerOf captures an arbitrary live process's identity, the way
// SelfOwner does for the caller.
func ownerOf(pid int64) (OwnerIdentity, error) {
	live, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		return OwnerIdentity{}, err
	}
	return OwnerIdentity{
		Pid: pid, StartTicks: live.StartTicks,
		BootID: live.BootID, PidStartedAt: live.StartedAt.Unix(),
	}, nil
}

func TestALiveOwnersEntryIsNeverTouched(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateEntry(root, "op-5", "m", "l", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	spawnForeignOwner(t, root, "op-5")
	if err := MarkTerminal(root, "op-5", OutcomeAbandoned, "not ours"); err == nil ||
		!strings.Contains(err.Error(), "live process") {
		t.Fatalf("a live owner's entry is never advanced by a stranger: %v", err)
	}
	if _, err := TakeOver(root, "op-5"); err == nil ||
		!strings.Contains(err.Error(), "never displaced") {
		t.Fatalf("a live owner is never displaced: %v", err)
	}
}

func TestADeadOwnersEntryIsTakenOverAndCompleted(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateEntry(root, "op-6", "m", "l", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	cmd := spawnForeignOwner(t, root, "op-6")
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	e, err := TakeOver(root, "op-6")
	if err != nil {
		t.Fatalf("a provably dead owner's entry is recovered, not orphaned: %v", err)
	}
	if e.Owner.Pid != int64(os.Getpid()) {
		t.Fatalf("the recovering process takes ownership: %+v", e.Owner)
	}
	// The stored intent is what recovery completes from.
	if e.Intent.Verb != "claim" || len(e.Intent.Targets) != 1 {
		t.Fatalf("the takeover carries the whole stored intent: %+v", e.Intent)
	}
	if err := MarkTerminal(root, "op-6", OutcomeConfirmed, "rebuilt push landed"); err != nil {
		t.Fatalf("the new owner finishes the operation: %v", err)
	}
}

func TestConfirmedLateCorrectsOnlyTerminals(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateEntry(root, "op-7", "m", "l", testIntentFor("done")); err != nil {
		t.Fatal(err)
	}
	if err := CorrectLate(root, "op-7", "opid seen"); err == nil {
		t.Fatal("a non-terminal entry follows the ordinary recovery rule, not the late correction")
	}
	if err := MarkTerminal(root, "op-7", OutcomeLost, "competitor won"); err != nil {
		t.Fatal(err)
	}
	// The pre-rewind push landed after all: the opid is in canonical
	// history, and the belief corrects to the evidence.
	if err := CorrectLate(root, "op-7", "opid in canonical history after repair"); err != nil {
		t.Fatal(err)
	}
	e, _ := ReadEntry(root, "op-7")
	if e.Outcome != OutcomeConfirmedLate {
		t.Fatalf("lost corrects to confirmed-late on canonical evidence: %+v", e)
	}
	// A confirmed entry stands.
	if _, err := CreateEntry(root, "op-8", "m", "l", testIntentFor("done")); err != nil {
		t.Fatal(err)
	}
	if err := MarkTerminal(root, "op-8", OutcomeConfirmed, "landed"); err != nil {
		t.Fatal(err)
	}
	if err := CorrectLate(root, "op-8", "x"); err != nil {
		t.Fatal(err)
	}
	e, _ = ReadEntry(root, "op-8")
	if e.Outcome != OutcomeConfirmed {
		t.Fatalf("confirmed never regresses: %+v", e)
	}
}

func TestTheOneRecoveryRule(t *testing.T) {
	created := Entry{Phase: PhaseCreated}
	pushed := Entry{Phase: PhasePushed}
	terminalLost := Entry{Phase: PhaseTerminal, Outcome: OutcomeLost}
	terminalOk := Entry{Phase: PhaseTerminal, Outcome: OutcomeConfirmed}

	cases := []struct {
		name         string
		e            Entry
		post         Postcondition
		alive, owns  bool
		pastDeadline bool
		want         RecoveryAction
	}{
		{"present confirms", pushed, PostconditionPresent, true, true, false, ActionConfirm},
		{"present confirms a dead owner's too", pushed, PostconditionPresent, false, false, false, ActionConfirm},
		{"competitor means lost", pushed, PostconditionCompetitor, true, true, false, ActionLost},
		{"absent + dead owner completes from intent (created)", created, PostconditionAbsent, false, false, false, ActionComplete},
		{"absent + dead owner completes from intent (pushed, past deadline included)", pushed, PostconditionAbsent, false, false, true, ActionComplete},
		{"absent + live foreign owner is left alone", pushed, PostconditionAbsent, true, false, true, ActionLeaveToOwner},
		{"live owner abandons its own never-pushed work", created, PostconditionAbsent, true, true, false, ActionAbandonOwn},
		{"live owner expires its own loop at the deadline", pushed, PostconditionAbsent, true, true, true, ActionExpireOwn},
		{"live owner inside the deadline keeps retrying", pushed, PostconditionAbsent, true, true, false, ActionKeepRetrying},
		{"terminalized entry corrects late on canonical evidence", terminalLost, PostconditionPresent, true, true, false, ActionConfirmLate},
		{"a correct terminal needs nothing", terminalOk, PostconditionPresent, true, true, false, ActionNothingToDo},
	}
	for _, c := range cases {
		if got := ClassifyRecovery(c.e, c.post, c.alive, c.owns, c.pastDeadline); got != c.want {
			t.Fatalf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestDeadlineReadsTheEntryStamp(t *testing.T) {
	e := Entry{Deadline: time.Now().Add(-time.Second).UTC().Format(time.RFC3339)}
	if !PastDeadline(e, time.Now()) {
		t.Fatal("a passed stamp is past")
	}
	e.Deadline = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if PastDeadline(e, time.Now()) {
		t.Fatal("a future stamp is not past")
	}
	if PastDeadline(Entry{}, time.Now()) {
		t.Fatal("no stamp, no expiry — created entries have no deadline")
	}
}
