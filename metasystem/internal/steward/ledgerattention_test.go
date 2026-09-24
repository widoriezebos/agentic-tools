package steward

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func attentionGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir,
		"-c", "user.name=ledger-attention-fixture",
		"-c", "user.email=ledger-attention@example.invalid",
		"-c", "protocol.file.allow=always"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

type ledgerAttentionBed struct {
	origin, watcher, publisher string
	now                        time.Time
	sequence                   int
}

type attentionAuthorityReader struct{}

func (attentionAuthorityReader) Read(pid int64) (humanauthority.Snapshot, error) {
	if pid == 1 {
		// The process-tree root, as a kernel answers it: launchd, root-owned,
		// parent 0. The terminal walk continues to the root since
		// hp-terminal-grade-for-stopping-acts (798ce60f); a fixture that
		// answered parent 1 for pid 1 read as a cycle there.
		return humanauthority.Snapshot{
			Exact: identity.Exact{
				Pid: 1, StartedAt: time.Unix(1, 0), Argv: []string{"/sbin/launchd"}, ArgvKnown: true,
			},
			Executable: "/sbin/launchd", ExecutableKnown: true,
			OwnerUID: 0, OwnerKnown: true,
			ParentPID: 0, ParentKnown: true, TerminalKnown: true,
		}, nil
	}
	return humanauthority.Snapshot{
		Exact: identity.Exact{
			Pid: pid, StartedAt: time.Unix(pid, 0), Argv: []string{"attended-human-shell"}, ArgvKnown: true,
		},
		Executable: "/fixture/attended-human-shell", ExecutableKnown: true,
		OwnerUID: 501, OwnerKnown: true,
		ParentPID: 1, ParentKnown: true, TerminalID: "tty-ledger-attention", TerminalKnown: true,
	}, nil
}

func (attentionAuthorityReader) SessionLeader(pid int64) (int64, error) { return pid, nil }

func newLedgerAttentionBed(t *testing.T) *ledgerAttentionBed {
	t.Helper()
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	attentionGit(t, base, "init", "-q", "--bare", "-b", "main", origin)
	seed := filepath.Join(base, "seed")
	attentionGit(t, base, "clone", "-q", origin, seed)
	root := &goal.RootRecord{
		Identity: "01J5X000000000000000000000", FormatVersion: "1", SyncMode: goal.SyncRemote,
		MigrationEpoch: "2026-08-20T00:00:00Z", ManifestDigest: strings.Repeat("ab", 32), MigrationMode: "manifest", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-08-20T09:00:00Z", Opid: "01J5X0000000000000000000A0-mac-a-1a2b3c4d",
			Verb: "migrate", Actor: "mac-a+lin-1", Keep: -1,
		}},
	}
	if err := os.MkdirAll(filepath.Join(seed, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "plans", "goals", "backlog.md"), goal.RenderRoot(root), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := "metasystem.runtimes=fake\nsteward.ledger-attention-stale-minutes=30\n"
	if err := os.WriteFile(filepath.Join(seed, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	attentionGit(t, seed, "add", "plans/goals/backlog.md", "metasystem.conf")
	attentionGit(t, seed, "commit", "-qm", "seed shared ledger")
	attentionGit(t, seed, "push", "-q", "origin", "main")

	bed := &ledgerAttentionBed{
		origin: origin, watcher: filepath.Join(base, "watcher"), publisher: filepath.Join(base, "publisher"),
		now: time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC),
	}
	for _, clone := range []string{bed.watcher, bed.publisher} {
		attentionGit(t, base, "clone", "-q", origin, clone)
		attentionGit(t, clone, "config", "metasystem.goal.machine", "mac-a")
		attentionGit(t, clone, "config", "goal.sync-remote", "origin")
		attentionGit(t, clone, "config", "goal.sync-branch", "refs/heads/main")
		attentionGit(t, clone, "update-ref", goal.AcceptedRef, "origin/main")
	}
	adapterDir := filepath.Join(bed.publisher, "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapterDir, 0o755); err != nil {
		t.Fatal(err)
	}
	adapter := "#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match never-an-attended-human-shell'\n"
	if err := testexec.WriteFile(filepath.Join(adapterDir, "fake.sh"), []byte(adapter), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := humanauthority.Enroll(bed.publisher, 20, attentionAuthorityReader{}, "Wido", bed.now); err != nil {
		t.Fatalf("enroll approval fixture terminal: %v", err)
	}
	return bed
}

func (b *ledgerAttentionBed) request() goal.VerbRequest {
	b.sequence++
	return goal.VerbRequest{
		Endpoint: goal.Endpoint{Root: b.publisher, Remote: "origin", Branch: "refs/heads/main"},
		Actor:    goal.Actor{Machine: "mac-a", Lineage: "attention-fixture"},
		Ulid:     fmt.Sprintf("%026d", b.sequence),
		Now:      b.now.Add(time.Duration(b.sequence) * time.Minute), ClaimEpoch: 1,
	}
}

func (b *ledgerAttentionBed) open(t *testing.T, id string) string {
	t.Helper()
	result, err := goal.Open(b.request(), id, "Implement "+id+" safely.", goal.OriginMain, "Work on "+id+".")
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("open %s: %+v %v", id, result, err)
	}
	return result.Tip
}

func (b *ledgerAttentionBed) pin(t *testing.T, id, machine string) string {
	t.Helper()
	request := b.request()
	request.Actor.Human = "Wido"
	result, err := goal.SetPin(request, id, machine)
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("pin %s: %+v %v", id, result, err)
	}
	return result.Tip
}

func (b *ledgerAttentionBed) approve(t *testing.T, id string) string {
	t.Helper()
	request := b.request()
	request.Actor.Human = "Wido"
	proof, err := humanauthority.Prove(b.publisher, 20, attentionAuthorityReader{}, request.Now)
	if err != nil {
		t.Fatalf("prove approval fixture authority: %v", err)
	}
	budget := goal.Budget{
		ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1,
	}
	result, err := goal.Approve(request, []string{id}, &budget, &proof)
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("approve %s: %+v %v", id, result, err)
	}
	return result.Tip
}

func (b *ledgerAttentionBed) claim(t *testing.T, id string) string {
	return b.claimAs(t, id, "mac-a")
}

func (b *ledgerAttentionBed) claimAs(t *testing.T, id, machine string) string {
	t.Helper()
	request := b.request()
	request.Actor.Machine = machine
	result, err := goal.Claim(request, id)
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("claim %s: %+v %v", id, result, err)
	}
	return result.Tip
}

func TestLedgerAttentionLocalAndPreBootstrapAreQuiet(t *testing.T) {
	local := newAttentionPolicyBed(t)
	local.local = true
	if report := local.run(local.now, "endpoint"); report.Outcome != "local" || len(report.Pending) != 0 {
		t.Fatalf("local ledger attention: %+v", report)
	}
	pre := newAttentionPolicyBed(t)
	pre.migrated = false
	if report := pre.run(pre.now, "endpoint accepted"); report.Outcome != "pre-bootstrap" || len(report.Pending) != 0 {
		t.Fatalf("pre-bootstrap ledger attention: %+v", report)
	}
}

func TestLedgerAttentionInitializesFrontierAfterLocalOrPreBootstrapState(t *testing.T) {
	t.Run("local state", func(t *testing.T) {
		local := newAttentionPolicyBed(t)
		local.local = true
		if report := local.run(local.now, "endpoint"); report.Outcome != "local" {
			t.Fatalf("local setup pass: %+v", report)
		}
		localState := local.state()
		if localState.DiffedTip != "" {
			t.Fatalf("local setup state: %+v", localState)
		}
		bed := newAttentionPolicyBed(t)
		if err := saveLedgerAttentionState(bed.root, localState); err != nil {
			t.Fatal(err)
		}
		report := bed.run(bed.now, attentionBaselineCalls())
		state := bed.state()
		if report.Outcome != "current" || report.Failure != "" || state.DiffedTip != "base" || state.ExaminedTip != state.DiffedTip {
			t.Fatalf("remote migrated ledger did not initialize over local state: report=%+v state=%+v", report, state)
		}
	})
	t.Run("pre-bootstrap state", func(t *testing.T) {
		bed := newAttentionPolicyBed(t)
		bed.migrated = false
		if report := bed.run(bed.now, "endpoint accepted"); report.Outcome != "pre-bootstrap" {
			t.Fatalf("pre-bootstrap setup pass: %+v", report)
		}
		bed.migrated = true
		report := bed.run(bed.now.Add(time.Minute), attentionBaselineCalls())
		state := bed.state()
		if report.Outcome != "current" || report.Failure != "" || state.DiffedTip != "base" || state.ExaminedTip != "base" {
			t.Fatalf("migrated ledger did not initialize over pre-bootstrap state: report=%+v state=%+v", report, state)
		}
	})
}

func TestLedgerAttentionPreBootstrapSaveRetiresOldFrontier(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	_ = bed.run(bed.now, attentionBaselineCalls())
	retiredTip := "retired-tip"
	bed.move(retiredTip, attentionWorld(queuedAttentionGoal("retired-before-bootstrap")), "base")
	advanced := bed.run(bed.now.Add(2*time.Minute), attentionMoveCalls("base", retiredTip))
	if len(advanced.Pending) != 1 || advanced.Pending[0].Tip != retiredTip {
		t.Fatalf("retired setup movement missing: %+v", advanced)
	}
	bed.migrated = false
	retired := bed.run(bed.now.Add(3*time.Minute), "endpoint accepted")
	if retired.Outcome != "pre-bootstrap" || retired.Tip != "" || len(retired.Pending) != 0 {
		t.Fatalf("pre-bootstrap save retained retired attention: %+v", retired)
	}
	if bed.accepted != retiredTip {
		t.Fatal("pre-bootstrap pass changed the declared accepted tip")
	}
	state := bed.state()
	if state.DiffedTip != "" || state.RemoteTip != "" || state.ExaminedTip != "" || state.Staged != nil || len(state.Pending) != 0 || len(state.Ready) != 0 || len(state.Pinned) != 0 || len(state.Queue) != 0 || state.JournalReady {
		t.Fatalf("pre-bootstrap save retained a retired frontier: %+v", state)
	}
	delete(bed.worlds, retiredTip)
	bed.accepted, bed.remote, bed.migrated = "base", "base", true
	rebootstrapped := bed.run(bed.now.Add(4*time.Minute), attentionBaselineCalls())
	state = bed.state()
	if rebootstrapped.Outcome != "current" || rebootstrapped.Failure != "" || len(rebootstrapped.Pending) != 0 || state.DiffedTip != "base" {
		t.Fatalf("re-bootstrap tried to walk the retired frontier: report=%+v state=%+v", rebootstrapped, state)
	}
}

func bedTime() time.Time {
	return time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
}

func TestSnapshotLedgerSeparatesApprovedReadyAwaitingAndPins(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	approved := func(id, opened, authority, reviewBy, pin string) *goal.GoalFile {
		file := approvedStewardGoal(id, "Work on "+id, "Continue.", opened)
		file.Pinned = pin
		if authority == goal.ApprovalAuthorityRelayed {
			file.Approved.Authority = authority
			file.Approved.ReviewBy = reviewBy
			event := &file.History[file.Approved.Revision-1]
			event.AuthorityOutcome = goal.AuthorityOutcomeTemporaryHumanWord
			event.AuthorityReviewBy = reviewBy
		}
		return file
	}
	projection := goal.Projection{
		Root: root,
		Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{
			"ready":   approved("ready", "2026-09-01T00:00:02Z", goal.ApprovalAuthorityProven, "", "mac-a"),
			"expired": approved("expired", "2026-09-01T00:00:00Z", goal.ApprovalAuthorityRelayed, "2026-09-01", "mac-a"),
			"queued": {
				Id: "queued", State: goal.StateQueued, Intent: "Wait for approval", Origin: goal.OriginMain,
				NextStep: "Wait.", OpenedAt: "2026-09-01T00:00:01Z", Pinned: "mac-a",
			},
			"foreign-pin": approved("foreign-pin", "2026-09-01T00:00:03Z", goal.ApprovalAuthorityProven, "", "mac-b"),
		}},
		Horizon: goal.ApprovalHorizon{Now: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)},
	}
	snapshot, err := snapshotLedger(projection, "mac-a")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(snapshot.Ready, ","); got != "ready" {
		t.Fatalf("approved ready frontier=%q, want ready", got)
	}
	if got := strings.Join(snapshot.Queue, ","); got != "expired,queued" {
		t.Fatalf("awaiting queue=%q, want the unranked identifier order expired,queued", got)
	}
	if got := strings.Join(snapshot.Pinned, ","); got != "expired,queued,ready" {
		t.Fatalf("local queued-or-approved pins=%q, want expired,queued,ready", got)
	}
}

func TestPriorityLedgerSnapshot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	queued := func(id, opened string, priority uint8, sequence uint64) *goal.GoalFile {
		return &goal.GoalFile{
			Id: id, State: goal.StateQueued, Intent: "Wait for " + id, Origin: goal.OriginMain,
			NextStep: "Wait.", OpenedAt: opened, Pinned: "mac-a", Priority: priority, Sequence: sequence,
		}
	}
	approved := func(id, opened string, priority uint8, sequence uint64) *goal.GoalFile {
		file := approvedStewardGoal(id, "Work on "+id, "Continue.", opened)
		file.Pinned, file.Priority, file.Sequence = "mac-a", priority, sequence
		return file
	}
	projection := goal.Projection{Root: root, Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{
		"z-queue-first": queued("z-queue-first", "2026-09-01T00:00:04Z", 1, 1),
		"z-ready-first": approved("z-ready-first", "2026-09-01T00:00:03Z", 1, 2),
		"a-queue-later": queued("a-queue-later", "2026-09-01T00:00:02Z", 2, 1),
		"a-ready-later": approved("a-ready-later", "2026-09-01T00:00:01Z", 2, 2),
	}}}

	snapshot, err := snapshotLedger(projection, "mac-a")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(snapshot.Ready, ","); got != "z-ready-first,a-ready-later" {
		t.Fatalf("ready projection sorted by age or identifier instead of rank: %q", got)
	}
	if got := strings.Join(snapshot.Queue, ","); got != "z-queue-first,a-queue-later" {
		t.Fatalf("waiting projection sorted by age or identifier instead of rank: %q", got)
	}
	if got := strings.Join(snapshot.Pinned, ","); got != "z-queue-first,z-ready-first,a-queue-later,a-ready-later" {
		t.Fatalf("pinned diagnostic did not filter the ordered traversal: %q", got)
	}
}

func TestLedgerAttentionSurfacesMovementOnceAndAdvancesAccepted(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	if report := bed.run(bed.now, attentionBaselineCalls()); report.Outcome != "current" || len(report.Pending) != 0 {
		t.Fatalf("baseline pass: %+v", report)
	}
	bed.move("opened", attentionWorld(queuedAttentionGoal("claimable-a")), "base")
	queued := bed.run(bed.now.Add(2*time.Minute), attentionMoveCalls("base", "opened"))
	if queued.Outcome != "advanced" || len(queued.Pending) != 1 || strings.Join(queued.Pending[0].QueueNow, ",") != "claimable-a" || len(queued.Pending[0].Claimable) != 0 {
		t.Fatalf("queued movement report: %+v", queued)
	}
	if err := PersistLedgerAttentionMark(bed.root, []string{queued.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	bed.move("approved", attentionWorld(approvedAttentionGoal("claimable-a")), "opened")
	report := bed.run(bed.now.Add(4*time.Minute), attentionMovedCalls("opened", "approved"))
	if report.Outcome != "advanced" || len(report.Pending) != 1 || strings.Join(report.Pending[0].Claimable, ",") != "claimable-a" {
		t.Fatalf("approval movement report: %+v", report)
	}
	if bed.accepted != "approved" {
		t.Fatalf("accepted ref=%s remote=%s", bed.accepted, bed.remote)
	}
	if err := PersistLedgerAttentionMark(bed.root, []string{report.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	again := bed.run(bed.now.Add(5*time.Minute), "endpoint accepted machine entries capture sync accepted validate cleanup")
	if len(again.Pending) != 0 {
		t.Fatalf("surfaced change replayed after its mark: %+v", again)
	}
	if len(bed.state().Pending) != 0 {
		t.Fatal("marked events remained on disk")
	}
}

func TestLedgerAttentionFirstPassBaselinesBeforeFetchingMovedRemote(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	bed.move("remote-moved", attentionWorld(queuedAttentionGoal("first-pass-movement")), "base")
	report := bed.run(bed.now, "endpoint accepted machine project:base capture sync accepted gates validate entries project:base changes:base>remote-moved project:remote-moved advance:remote-moved entries cleanup")
	if report.Outcome != "advanced" || len(report.Pending) != 1 || report.Pending[0].Tip != "remote-moved" {
		t.Fatalf("first pass erased a pre-existing remote movement: %+v", report)
	}
	state := bed.state()
	if state.ExaminedTip != "base" || state.RemoteTip != "remote-moved" || state.MovedAt == "" {
		t.Fatalf("first pass did not retain the pre-fetch examination frontier: %+v", state)
	}
}

func TestLedgerAttentionBrokenAcceptedRefIsFailureNotBootstrap(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	bed.fail("accepted", "accepted ref is unreadable")
	report := bed.run(bed.now, "endpoint accepted")
	if report.Outcome != "failed" || report.FailureKind != ledgerAttentionFetchFailed || !strings.Contains(report.Failure, "accepted ref") {
		t.Fatalf("broken accepted state masqueraded as pre-bootstrap: %+v", report)
	}
	verdict := checkLedgerAttention(bed.root, bed.now.Add(time.Minute))
	if verdict.Status != HealthAlive || !strings.Contains(verdict.Reason, "last fetch failed") || strings.Contains(verdict.Reason, "examined at the canonical tip") {
		t.Fatalf("a first failure with no canonical tip claimed an examination: %+v", verdict)
	}
}

func TestLedgerAttentionValidationRefusalLeavesAcceptedUntouched(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	acceptedBefore := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef)
	if err := os.WriteFile(filepath.Join(bed.publisher, "plans", "goals", "backlog.md"), []byte("not a goal root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	attentionGit(t, bed.publisher, "add", "plans/goals/backlog.md")
	attentionGit(t, bed.publisher, "commit", "-qm", "publish invalid ledger")
	attentionGit(t, bed.publisher, "push", "-q", "origin", "main")
	report := RunLedgerAttention(bed.watcher, bed.now.Add(time.Minute))
	if report.Outcome != "failed" || report.FailureKind != ledgerAttentionFetchFailed {
		t.Fatalf("invalid captured ledger did not fail closed: %+v", report)
	}
	if acceptedAfter := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef); acceptedAfter != acceptedBefore {
		t.Fatalf("validation refusal advanced accepted ref: before=%s after=%s", acceptedBefore, acceptedAfter)
	}
}

func TestLedgerAttentionPinsAndQueueSequenceChanges(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	_ = bed.run(bed.now, attentionBaselineCalls())
	queued := queuedAttentionGoal("pin-target")
	bed.move("opened", attentionWorld(queued), "base")
	opened := bed.run(bed.now.Add(2*time.Minute), attentionMoveCalls("base", "opened"))
	if err := PersistLedgerAttentionMark(bed.root, []string{opened.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	pinnedFile := queuedAttentionGoal("pin-target")
	pinnedFile.Pinned = "mac-a"
	bed.move("pinned", attentionWorld(pinnedFile), "opened")
	pinned := bed.run(bed.now.Add(4*time.Minute), attentionMovedCalls("opened", "pinned"))
	if len(pinned.Pending) != 1 || strings.Join(pinned.Pending[0].Pins, ",") != "pin-target" {
		t.Fatalf("local pin did not surface through schema-2 Pending.Pins: %+v", pinned)
	}
	if err := PersistLedgerAttentionMark(bed.root, []string{pinned.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	approvedFile := approvedAttentionGoal("pin-target")
	approvedFile.Pinned = "mac-a"
	bed.move("approved", attentionWorld(approvedFile), "pinned")
	approved := bed.run(bed.now.Add(6*time.Minute), attentionMovedCalls("pinned", "approved"))
	if len(approved.Pending) != 1 || strings.Join(approved.Pending[0].Claimable, ",") != "pin-target" || strings.Join(approved.Pending[0].QueueWas, ",") != "pin-target" || len(approved.Pending[0].QueueNow) != 0 {
		t.Fatalf("approval did not move the goal from awaiting to the local ready frontier: %+v", approved)
	}
	if err := PersistLedgerAttentionMark(bed.root, []string{approved.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	foreignFile := approvedAttentionGoal("pin-target")
	foreignFile.Pinned = "mac-b"
	bed.move("foreign", attentionWorld(foreignFile), "approved")
	foreign := bed.run(bed.now.Add(8*time.Minute), attentionMovedCalls("approved", "foreign"))
	for _, event := range foreign.Pending {
		if len(event.Pins) > 0 || len(event.Claimable) > 0 {
			t.Fatalf("foreign pin surfaced as local attention: %+v", foreign)
		}
	}
	bed.move("cleared", attentionWorld(approvedAttentionGoal("pin-target")), "foreign")
	cleared := bed.run(bed.now.Add(10*time.Minute), attentionMovedCalls("foreign", "cleared"))
	if len(cleared.Pending) != 1 || strings.Join(cleared.Pending[0].Claimable, ",") != "pin-target" {
		t.Fatalf("clearing the foreign pin did not restore the local frontier: %+v", cleared)
	}
	if err := PersistLedgerAttentionMark(bed.root, []string{cleared.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	if sameStrings([]string{"a", "b", "c"}, []string{"b", "a", "c"}) {
		t.Fatal("same-member queue reordering was treated as unchanged")
	}
}

func TestLedgerAttentionKeepsOneDigestAndNudgePerChange(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	_ = bed.run(bed.now, attentionBaselineCalls())
	bed.world("change-one", attentionWorld(queuedAttentionGoal("change-one")))
	pinned := queuedAttentionGoal("change-two")
	pinned.Pinned = "mac-a"
	bed.move("change-two", attentionWorld(queuedAttentionGoal("change-one"), pinned), "base",
		goal.LedgerChange{Tip: "change-one", Consecutive: true}, goal.LedgerChange{Tip: "change-two", Consecutive: true})
	report := bed.run(bed.now.Add(4*time.Minute), attentionMoveCalls("base", "change-one", "change-two"))
	if len(report.Pending) != 2 || report.Pending[0].SourceID == report.Pending[1].SourceID {
		t.Fatalf("two remote changes were coalesced: %+v", report)
	}
	var discoveryCalls []string
	readMachine := func(root string) (string, error) {
		if root != bed.root {
			t.Fatalf("narration machine root = %q, want %q", root, bed.root)
		}
		discoveryCalls = append(discoveryCalls, "machine")
		return "mac-a", nil
	}
	resolveLayout := func(root string) (stateroot.Layout, error) {
		if root != bed.root {
			t.Fatalf("digest layout root = %q, want %q", root, bed.root)
		}
		discoveryCalls = append(discoveryCalls, "layout")
		return stateroot.Layout{GitRoot: bed.root, RepositoryRoot: bed.root, InstallationRoot: bed.root}, nil
	}
	result := TickResult{LedgerAttention: report}
	for _, when := range []time.Time{bed.now.Add(4 * time.Minute), bed.now.Add(5 * time.Minute)} {
		if err := narrateDigestWithReaders(bed.root, Evidence{}, result, when, readMachine, resolveLayout); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := strings.Join(discoveryCalls, ","), "machine,layout,layout,layout,layout,machine,layout,layout,layout"; got != want {
		t.Fatalf("narration discovery calls = %q, want %q", got, want)
	}
	for _, event := range report.Pending {
		if err := QueueNotification(bed.root, ledgerAttentionNotification(event, "mac-a")); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(bed.root, "records", "narrator-digest.log"))
	if err != nil || strings.Count(string(data), "source: ledger ") != 2 {
		t.Fatalf("per-change digest entries: %q %v", data, err)
	}
	for _, event := range report.Pending {
		if count := strings.Count(string(data), "(source: ledger "+event.SourceID+")"); count != 1 {
			t.Fatalf("digest source %q appears %d times in %q", event.SourceID, count, data)
		}
	}
	if !strings.Contains(string(data), "pin(s) addressed to mac-a: change-two") {
		t.Fatalf("digest omitted the enrolled machine name: %q", data)
	}
	pending, err := PendingNotifications(bed.root)
	if err != nil || len(pending) != 2 || pending[0].Nonce == pending[1].Nonce {
		t.Fatalf("per-change pending notifications: %+v %v", pending, err)
	}
	notifications := map[string]PendingNotification{}
	for _, item := range pending {
		notifications[item.Nonce] = item
	}
	for _, event := range report.Pending {
		if _, ok := notifications["ledger-attention-"+event.SourceID]; !ok {
			t.Fatalf("missing notification for %q: %+v", event.SourceID, pending)
		}
	}
	if !strings.Contains(notifications["ledger-attention-"+report.Pending[1].SourceID].Message, "pin(s) addressed to mac-a: change-two") {
		t.Fatalf("notification omitted the enrolled machine name: %+v", pending)
	}
}

func TestLedgerAttentionRetainsTransientFactAcrossSkippedMark(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	_ = bed.run(bed.now, attentionBaselineCalls())
	bed.move("opened", attentionWorld(queuedAttentionGoal("transient-a")), "base")
	queued := bed.run(bed.now.Add(2*time.Minute), attentionMoveCalls("base", "opened"))
	if len(queued.Pending) != 1 {
		t.Fatalf("queued transient setup: %+v", queued)
	}
	if err := PersistLedgerAttentionMark(bed.root, []string{queued.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	approved := approvedAttentionGoal("transient-a")
	bed.move("approved", attentionWorld(approved), "opened")
	first := bed.run(bed.now.Add(4*time.Minute), attentionMovedCalls("opened", "approved"))
	if len(first.Pending) != 1 || first.Pending[0].Tip != "approved" {
		t.Fatalf("first transient fact: %+v", first)
	}
	claimed := approvedAttentionGoal("transient-a")
	claimed.State = goal.StateClaimed
	claimed.Claimed = &goal.ClaimRecord{Machine: "mac-a", Lineage: "attention-fixture", At: "2026-09-01T08:05:00Z", Revision: 3}
	claimed.Revision = 3
	claimed.History = append(claimed.History, goal.HistoryLine{At: "2026-09-01T08:05:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAX-mac-a-00000000", Verb: "claim", Actor: "mac-a+attention-fixture", Targets: []string{"transient-a"}, Keep: -1})
	bed.move("claimed", attentionWorld(claimed), "approved")
	second := bed.run(bed.now.Add(6*time.Minute), attentionMovedCalls("approved", "claimed"))
	found := false
	for _, event := range second.Pending {
		if event.SourceID == first.Pending[0].SourceID && strings.Join(event.Claimable, ",") == "transient-a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("later claim erased an unsurfaced earlier claimable fact: %+v", second)
	}
}

func TestLedgerAttentionRewindUsesDirectTransitionAndFreshEventIdentity(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	_ = bed.run(bed.now, attentionBaselineCalls())
	bed.move("forward", attentionWorld(queuedAttentionGoal("rewind-visible")), "base")
	forward := bed.run(bed.now.Add(2*time.Minute), attentionMoveCalls("base", "forward"))
	if len(forward.Pending) != 1 || forward.Pending[0].SourceID != "forward" {
		t.Fatalf("initial forward event: %+v", forward)
	}
	if err := PersistLedgerAttentionMark(bed.root, []string{forward.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	bed.changes["forward>base"] = []goal.LedgerChange{{Tip: "base", Consecutive: false}}
	bed.accepted, bed.remote = "base", "base"
	bed.entries = append(bed.entries, goal.Entry{Opid: "repair-rewind", Intent: goal.Intent{Verb: "repair-accept-remote", Args: map[string]string{"newTip": "base"}}, Phase: goal.PhaseTerminal, Outcome: goal.OutcomeConfirmed})
	bed.repairBaseline = []string{"repair-rewind"}
	rewound := bed.run(bed.now.Add(4*time.Minute), "endpoint accepted machine entries project:forward changes:forward>base project:base entries capture sync accepted validate entries cleanup")
	if len(rewound.Pending) != 1 || rewound.Pending[0].Tip != "base" || !strings.Contains(rewound.Pending[0].SourceID, "-epoch-1") {
		t.Fatalf("sanctioned rewind was not one direct, epoch-qualified transition: %+v", rewound)
	}
	if err := PersistLedgerAttentionMark(bed.root, []string{rewound.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	bed.move("forward", attentionWorld(queuedAttentionGoal("rewind-visible")), "base")
	bed.stageTime["forward"] = bed.now.Add(6 * time.Minute)
	forwardAgain := bed.run(bed.now.Add(6*time.Minute), attentionMoveCalls("base", "forward"))
	if len(forwardAgain.Pending) != 1 || forwardAgain.Pending[0].Tip != "forward" || forwardAgain.Pending[0].SourceID == forward.Pending[0].SourceID {
		t.Fatalf("revisited commit was silently deduplicated after rewind: first=%+v later=%+v", forward, forwardAgain)
	}
}

func TestLedgerAttentionDurabilityRefusalDoesNotFetch(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	bed.move("must-not-fetch", attentionWorld(queuedAttentionGoal("must-not-fetch")), "base")
	report := bed.runWithWriter(bed.now, "endpoint accepted machine project:base", func(string, string, string) (bool, error) { return false, nil })
	if report.Outcome != "failed" || report.FailureKind != ledgerAttentionStateWriteFailed {
		t.Fatalf("durability refusal was mislabeled: %+v", report)
	}
	if bed.accepted != "base" || bed.accepted == bed.remote {
		t.Fatalf("fetch advanced past an uncommitted baseline: before=%s remote=%s", bed.accepted, bed.remote)
	}
}

func TestLedgerAttentionFetchTimeoutLeavesAcceptedAndTransportDead(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	bed.open(t, "timeout-must-not-advance")
	acceptedBefore := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	groupFile := filepath.Join(dir, "group")
	lifetimeFIFO := filepath.Join(dir, "lifetime")
	if err := syscall.Mkfifo(lifetimeFIFO, 0o600); err != nil {
		t.Fatal(err)
	}
	lifetime, err := os.OpenFile(lifetimeFIFO, os.O_RDONLY|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lifetime.Close()
	wrapper := filepath.Join(dir, "git")
	script := `#!/bin/sh
case " $* " in
  *" fetch "*)
	    exec 9>"$LEDGER_ATTENTION_LIFETIME_FIFO"
    echo $$ > "$LEDGER_ATTENTION_GROUP_FILE"
    trap '' TERM
    while :; do sleep 1; done
    ;;
esac
exec "$LEDGER_ATTENTION_REAL_GIT" "$@"
`
	if err := testexec.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEDGER_ATTENTION_REAL_GIT", realGit)
	t.Setenv("LEDGER_ATTENTION_GROUP_FILE", groupFile)
	t.Setenv("LEDGER_ATTENTION_LIFETIME_FIFO", lifetimeFIFO)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	previousBudget := ledgerAttentionFetchBudget
	ledgerAttentionFetchBudget = 300 * time.Millisecond
	t.Cleanup(func() { ledgerAttentionFetchBudget = previousBudget })
	result, tickErr := RunTick(bed.watcher, TickConfig{}, fakeCensus{})
	if tickErr != nil {
		t.Fatalf("ledger timeout stopped the steward's later duties: %v", tickErr)
	}
	if result.LedgerAttention.Outcome != "failed" || !strings.Contains(result.LedgerAttention.Failure, "timed out") || result.Health.Schema == 0 {
		t.Fatalf("blocking ledger transport did not fail quietly while health continued: %+v", result)
	}
	if accepted := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef); accepted != acceptedBefore {
		t.Fatalf("timed-out fetch advanced accepted ref: before=%s after=%s", acceptedBefore, accepted)
	}
	data, err := os.ReadFile(groupFile)
	if err != nil || strings.TrimSpace(string(data)) == "" {
		t.Fatalf("transport wrapper did not publish its group identity: %q %v", data, err)
	}
	if err := syscall.SetNonblock(int(lifetime.Fd()), false); err != nil {
		t.Fatal(err)
	}
	if inherited, err := io.ReadAll(lifetime); err != nil || len(inherited) != 0 {
		t.Fatalf("transport lifetime pipe did not close cleanly: bytes=%q err=%v", inherited, err)
	}
}

func TestLedgerAttentionRecoversDurableStageBeforeAcceptedCAS(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	_ = bed.run(bed.now, attentionBaselineCalls())
	bed.move("staged-tip", attentionWorld(queuedAttentionGoal("staged-before-cas")), "base")
	stagedUncertain := func(path, contents, anchor string) (bool, error) {
		durable, err := atomicfile.WriteText(path, contents, anchor)
		if err == nil && strings.Contains(contents, `"staged": {`) {
			return false, nil
		}
		return durable, err
	}
	failed := bed.runWithWriter(bed.now.Add(2*time.Minute), "endpoint accepted machine capture sync accepted gates validate entries project:base changes:base>staged-tip project:staged-tip cleanup", stagedUncertain)
	if failed.Outcome != "failed" || failed.FailureKind != ledgerAttentionStateWriteFailed {
		t.Fatalf("staged durability uncertainty was not refused: %+v", failed)
	}
	if bed.accepted != "base" {
		t.Fatalf("accepted ref moved before staged frontier was proven durable: %s", bed.accepted)
	}
	if stage := bed.state().Staged; stage == nil || stage.Tip != "staged-tip" {
		t.Fatalf("writer did not leave a durable staged file: %+v", stage)
	}
	recovered := bed.run(bed.now.Add(3*time.Minute), "endpoint accepted machine entries advance:staged-tip ancestor:staged-tip>staged-tip accepted capture sync accepted validate entries cleanup")
	if len(recovered.Pending) != 1 || recovered.Pending[0].Tip != "staged-tip" {
		t.Fatalf("durable pre-CAS stage did not recover exactly once: %+v", recovered)
	}
	state := bed.state()
	if state.Staged != nil || state.DiffedTip != "staged-tip" {
		t.Fatalf("recovered stage did not promote atomically: %+v", state)
	}
}

func TestLedgerAttentionCrashStageDoesNotResurrectHumanRetirement(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	_ = bed.run(bed.now, attentionBaselineCalls())
	acceptedBefore := bed.accepted
	retiredTip := "retired-tip"
	bed.move(retiredTip, attentionWorld(queuedAttentionGoal("retired-mid-capture")), "base")
	stagedUncertain := func(path, contents, anchor string) (bool, error) {
		durable, err := atomicfile.WriteText(path, contents, anchor)
		if err == nil && strings.Contains(contents, `"staged": {`) {
			return false, nil
		}
		return durable, err
	}
	failed := bed.runWithWriter(bed.now.Add(2*time.Minute), "endpoint accepted machine capture sync accepted gates validate entries project:base changes:base>retired-tip project:retired-tip cleanup", stagedUncertain)
	if failed.Outcome != "failed" || failed.FailureKind != ledgerAttentionStateWriteFailed {
		t.Fatalf("crash fixture did not stop after its durable stage: %+v", failed)
	}
	if bed.accepted != acceptedBefore {
		t.Fatalf("accepted ref moved before staged frontier was proven durable: %s", bed.accepted)
	}
	if stage := bed.state().Staged; stage == nil || stage.Tip != retiredTip {
		t.Fatalf("writer did not leave a durable retired stage: %+v", stage)
	}
	bed.entries = append(bed.entries, goal.Entry{Opid: "repair-retirement", Intent: goal.Intent{Verb: "repair-accept-remote", Args: map[string]string{"newTip": acceptedBefore}}, Phase: goal.PhaseTerminal, Outcome: goal.OutcomeConfirmed})
	bed.accepted, bed.remote = acceptedBefore, acceptedBefore

	recovered := bed.run(bed.now.Add(3*time.Minute), "endpoint accepted machine entries accepted capture sync accepted validate cleanup")
	if recovered.Outcome != "current" || recovered.Failure != "" || len(recovered.Pending) != 0 || recovered.Tip != acceptedBefore {
		t.Fatalf("crash recovery resurfaced the retired capture: %+v", recovered)
	}
	if accepted := bed.accepted; accepted != acceptedBefore || accepted == retiredTip {
		t.Fatalf("crash recovery resurrected retired accepted tip: accepted=%s retired=%s", accepted, retiredTip)
	}
	state := bed.state()
	if state.Staged != nil || state.DiffedTip != acceptedBefore {
		t.Fatalf("retired durable stage survived recovery: %+v", state)
	}
}

func TestTickRecordsLedgerStateWriteFailureAndRunsLaterDuties(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	previous := ledgerAttentionWriter
	ledgerAttentionWriter = func(string, string, string) (bool, error) { return false, nil }
	t.Cleanup(func() { ledgerAttentionWriter = previous })
	result, err := RunTick(bed.watcher, TickConfig{}, fakeCensus{})
	if err != nil {
		t.Fatalf("ledger-attention failure stopped the steward tick: %v", err)
	}
	if result.LedgerAttention.FailureKind != ledgerAttentionStateWriteFailed || result.Health.Schema == 0 {
		t.Fatalf("tick did not retain failure and later health duties: %+v", result)
	}
	record, err := loadComponentEvidence(ComponentEvidencePath(bed.watcher, "ledger-attention"))
	if err != nil || record.Result != ComponentError || record.Outcome != ledgerAttentionStateWriteFailed {
		t.Fatalf("component evidence mislabeled the state refusal: %+v %v", record, err)
	}
}

func TestLedgerAttentionJournalClearsBeforeOfflineFetch(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	_ = bed.run(bed.now, attentionBaselineCalls())
	bed.move("examine-tip", attentionWorld(queuedAttentionGoal("examine-me")), "base")
	moved := bed.run(bed.now.Add(2*time.Minute), attentionMoveCalls("base", "examine-tip"))
	if moved.Tip != "examine-tip" || moved.MovedAt.IsZero() {
		t.Fatalf("movement did not start the examination clock: %+v", moved)
	}
	opid := "journal-examines-remote"
	bed.entries = []goal.Entry{{Opid: opid}}
	bed.fail("capture", "offline fetch unavailable")
	empty := bed.run(bed.now.Add(3*time.Minute), "endpoint accepted machine entries capture")
	if empty.Outcome != "failed" {
		t.Fatalf("offline fetch unexpectedly succeeded: %+v", empty)
	}
	state := bed.state()
	if state.ExaminedTip == "examine-tip" || state.MovedAt == "" {
		t.Fatalf("a journal entry without a fetched tip falsely cleared attention: %+v", state)
	}
	bed.entries[0].FetchedOid = "examine-tip"
	report := bed.run(bed.now.Add(4*time.Minute), "endpoint accepted machine entries ancestor:examine-tip>examine-tip capture")
	if report.Outcome != "failed" {
		t.Fatalf("offline fetch unexpectedly succeeded: %+v", report)
	}
	state = bed.state()
	if state.ExaminedTip != "examine-tip" || state.MovedAt != "" {
		t.Fatalf("journal evidence did not clear before offline fetch: %+v", state)
	}
}

func TestLedgerAttentionDoesNotInferExaminationFromHookTiming(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC)
	remoteTip := strings.Repeat("a", 40)
	state := ledgerAttentionState{
		Schema: ledgerAttentionStateSchema, RemoteTip: remoteTip, ExaminedTip: strings.Repeat("b", 40),
		MovedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), JournalReady: true,
	}
	hook, err := BeginHookAttempt(root, identity.Ref{Pid: 99101, StartedAtSec: 100}, "after-rewind", now)
	if err != nil {
		t.Fatal(err)
	}
	line := "HEALTH unknown — ledger-attention=dead"
	payload := `{"systemMessage":"HEALTH unknown — ledger-attention=dead"}`
	if _, err := CompleteHookAttempt(root, hook.Generation, hook.AttemptSeq, ComponentOK, "EMITTED", line, payload, nil, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	cleared, err := clearLedgerAttentionFromJournal(root, &state)
	if err != nil || cleared || state.ExaminedTip == remoteTip || state.MovedAt == "" {
		t.Fatalf("post-movement hook timing falsely proved examination across a possible accepted-ref rewind: cleared=%t state=%+v err=%v", cleared, state, err)
	}
}

func TestLedgerAttentionHealthDeadOutranksPersistentFetchFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nsteward.ledger-attention-stale-minutes=30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	old := now.Add(-47 * time.Minute).Format(time.RFC3339Nano)
	state := ledgerAttentionState{
		Schema: ledgerAttentionStateSchema, LastOutcome: "failed", LastFailure: "remote offline", FailingSince: old,
		RemoteTip: strings.Repeat("a", 40), ExaminedTip: strings.Repeat("b", 40), MovedAt: old,
	}
	if err := saveLedgerAttentionState(root, state); err != nil {
		t.Fatal(err)
	}
	verdict := checkLedgerAttention(root, now)
	if verdict.Status != HealthDead || !verdict.NoAutomaticRemedy || !strings.Contains(verdict.Reason, "47m") || !strings.Contains(verdict.Remedy, "journaling goal verb") {
		t.Fatalf("known unexamined movement did not outrank unknown reachability: %+v", verdict)
	}
}

func TestLedgerAttentionFetchFailureMaturesFromAliveToUnknown(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	if report := bed.run(bed.now, attentionBaselineCalls()); report.Outcome != "current" {
		t.Fatalf("baseline did not establish a reachable ledger: %+v", report)
	}
	bed.fail("capture", "unreachable remote")
	failureAt := bed.now.Add(time.Minute)
	if report := bed.run(failureAt, "endpoint accepted machine capture"); report.Outcome != "failed" {
		t.Fatalf("unreachable remote unexpectedly succeeded: %+v", report)
	}
	state := bed.state()
	if state.LastOutcome != "failed" || state.LastFailure != "unreachable remote" || state.FailingSince != failureAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("fetch failure was not persisted: %+v", state)
	}
	if early := checkLedgerAttention(bed.root, failureAt.Add(29*time.Minute)); early.Status != HealthAlive {
		t.Fatalf("a fresh fetch failure was not inside its patience window: %+v", early)
	}
	if mature := checkLedgerAttention(bed.root, failureAt.Add(30*time.Minute)); mature.Status != HealthUnknown || !strings.Contains(mature.Reason, "unreachable") {
		t.Fatalf("a persistent fetch failure did not mature to unknown: %+v", mature)
	}
}
