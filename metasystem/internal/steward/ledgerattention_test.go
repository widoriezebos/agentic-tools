package steward

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
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

func (b *ledgerAttentionBed) claim(t *testing.T, id string) string {
	return b.claimAs(t, id, "mac-a")
}

func (b *ledgerAttentionBed) claimAs(t *testing.T, id, machine string) string {
	t.Helper()
	request := b.request()
	request.Actor.Machine = machine
	result, err := goal.Claim(request, id, goal.Budget{
		ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1,
	})
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("claim %s: %+v %v", id, result, err)
	}
	return result.Tip
}

func TestLedgerAttentionLocalAndPreBootstrapAreQuiet(t *testing.T) {
	local := convertedBed(t, "mac-a", nil)
	if report := RunLedgerAttention(local, time.Now().UTC()); report.Outcome != "local" || len(report.Pending) != 0 {
		t.Fatalf("local ledger attention: %+v", report)
	}
	pre := t.TempDir()
	attentionGit(t, pre, "init", "-q")
	if err := os.WriteFile(filepath.Join(pre, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if report := RunLedgerAttention(pre, time.Now().UTC()); report.Outcome != "pre-bootstrap" || len(report.Pending) != 0 {
		t.Fatalf("pre-bootstrap ledger attention: %+v", report)
	}
}

func TestLedgerAttentionInitializesFrontierAfterLocalOrPreBootstrapState(t *testing.T) {
	t.Run("local state", func(t *testing.T) {
		local := convertedBed(t, "mac-a", nil)
		if report := RunLedgerAttention(local, bedTime()); report.Outcome != "local" {
			t.Fatalf("local setup pass: %+v", report)
		}
		localState, _, err := loadLedgerAttentionState(local)
		if err != nil || localState.DiffedTip != "" {
			t.Fatalf("local setup state: %+v %v", localState, err)
		}
		bed := newLedgerAttentionBed(t)
		if err := saveLedgerAttentionState(bed.watcher, localState); err != nil {
			t.Fatal(err)
		}
		report := RunLedgerAttention(bed.watcher, bed.now)
		state, _, err := loadLedgerAttentionState(bed.watcher)
		if report.Outcome != "current" || report.Failure != "" || err != nil || state.DiffedTip == "" || state.ExaminedTip != state.DiffedTip {
			t.Fatalf("remote migrated ledger did not initialize over local state: report=%+v state=%+v err=%v", report, state, err)
		}
	})

	t.Run("pre-bootstrap state", func(t *testing.T) {
		bed := newLedgerAttentionBed(t)
		accepted := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef)
		attentionGit(t, bed.watcher, "update-ref", "-d", goal.AcceptedRef)
		if report := RunLedgerAttention(bed.watcher, bed.now); report.Outcome != "pre-bootstrap" {
			t.Fatalf("pre-bootstrap setup pass: %+v", report)
		}
		attentionGit(t, bed.watcher, "update-ref", goal.AcceptedRef, accepted)
		report := RunLedgerAttention(bed.watcher, bed.now.Add(time.Minute))
		state, _, err := loadLedgerAttentionState(bed.watcher)
		if report.Outcome != "current" || report.Failure != "" || err != nil || state.DiffedTip != accepted || state.ExaminedTip != accepted {
			t.Fatalf("migrated ledger did not initialize over pre-bootstrap state: report=%+v state=%+v err=%v", report, state, err)
		}
	})
}

func TestLedgerAttentionPreBootstrapSaveRetiresOldFrontier(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	baseTip := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	retiredTip := bed.open(t, "retired-before-bootstrap")
	advanced := RunLedgerAttention(bed.watcher, bed.now.Add(2*time.Minute))
	if len(advanced.Pending) != 1 || advanced.Pending[0].Tip != retiredTip {
		t.Fatalf("retired setup movement missing: %+v", advanced)
	}
	attentionGit(t, bed.publisher, "push", "-q", "--force", "origin", baseTip+":refs/heads/main")
	attentionGit(t, bed.watcher, "update-ref", "-d", goal.AcceptedRef)

	retired := RunLedgerAttention(bed.watcher, bed.now.Add(3*time.Minute))
	if retired.Outcome != "pre-bootstrap" || retired.Tip != "" || len(retired.Pending) != 0 {
		t.Fatalf("pre-bootstrap save retained retired attention: %+v", retired)
	}
	if cmd := exec.Command("git", "-C", bed.watcher, "rev-parse", "--verify", goal.AcceptedRef); cmd.Run() == nil {
		t.Fatal("pre-bootstrap attention resurrected the deleted accepted ref")
	}
	state, _, err := loadLedgerAttentionState(bed.watcher)
	if err != nil {
		t.Fatal(err)
	}
	if state.DiffedTip != "" || state.RemoteTip != "" || state.ExaminedTip != "" || state.Staged != nil || len(state.Pending) != 0 || len(state.Ready) != 0 || len(state.Pinned) != 0 || len(state.Queue) != 0 || state.JournalReady {
		t.Fatalf("pre-bootstrap save retained a retired frontier: %+v", state)
	}

	attentionGit(t, bed.watcher, "reflog", "expire", "--expire=now", "--all")
	attentionGit(t, bed.watcher, "gc", "--prune=now")
	if cmd := exec.Command("git", "-C", bed.watcher, "cat-file", "-e", retiredTip+"^{commit}"); cmd.Run() == nil {
		t.Fatalf("retired tip %s remained reachable in the watcher fixture", retiredTip)
	}
	attentionGit(t, bed.watcher, "update-ref", goal.AcceptedRef, baseTip)
	rebootstrapped := RunLedgerAttention(bed.watcher, bed.now.Add(4*time.Minute))
	state, _, err = loadLedgerAttentionState(bed.watcher)
	if rebootstrapped.Outcome != "current" || rebootstrapped.Failure != "" || len(rebootstrapped.Pending) != 0 || err != nil || state.DiffedTip != baseTip {
		t.Fatalf("re-bootstrap tried to walk the pruned retired frontier: report=%+v state=%+v err=%v", rebootstrapped, state, err)
	}
}

func bedTime() time.Time {
	return time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
}

func TestLedgerAttentionSurfacesMovementOnceAndAdvancesAccepted(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	if report := RunLedgerAttention(bed.watcher, bed.now); report.Outcome != "current" || len(report.Pending) != 0 {
		t.Fatalf("baseline pass: %+v", report)
	}
	remoteTip := bed.open(t, "claimable-a")
	report := RunLedgerAttention(bed.watcher, bed.now.Add(2*time.Minute))
	if report.Outcome != "advanced" || len(report.Pending) != 1 || strings.Join(report.Pending[0].Claimable, ",") != "claimable-a" {
		t.Fatalf("movement report: %+v", report)
	}
	if accepted := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef); accepted != remoteTip {
		t.Fatalf("accepted ref=%s remote=%s", accepted, remoteTip)
	}
	if err := PersistLedgerAttentionMark(bed.watcher, []string{report.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	if again := RunLedgerAttention(bed.watcher, bed.now.Add(3*time.Minute)); len(again.Pending) != 0 {
		t.Fatalf("surfaced change replayed after its mark: %+v", again)
	}
}

func TestLedgerAttentionFirstPassBaselinesBeforeFetchingMovedRemote(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	acceptedBefore := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef)
	remoteTip := bed.open(t, "first-pass-movement")
	report := RunLedgerAttention(bed.watcher, bed.now)
	if report.Outcome != "advanced" || len(report.Pending) != 1 || report.Pending[0].Tip != remoteTip {
		t.Fatalf("first pass erased a pre-existing remote movement: %+v", report)
	}
	state, _, err := loadLedgerAttentionState(bed.watcher)
	if err != nil || state.ExaminedTip != acceptedBefore || state.RemoteTip != remoteTip || state.MovedAt == "" {
		t.Fatalf("first pass did not retain the pre-fetch examination frontier: %+v %v", state, err)
	}
}

func TestLedgerAttentionBrokenAcceptedRefIsFailureNotBootstrap(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	common := attentionGit(t, bed.watcher, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err := os.WriteFile(filepath.Join(common, filepath.FromSlash(goal.AcceptedRef)), []byte("not-an-oid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := RunLedgerAttention(bed.watcher, bed.now)
	if report.Outcome != "failed" || report.FailureKind != ledgerAttentionFetchFailed || !strings.Contains(report.Failure, "accepted ref") {
		t.Fatalf("broken accepted state masqueraded as pre-bootstrap: %+v", report)
	}
	verdict := checkLedgerAttention(bed.watcher, bed.now.Add(time.Minute))
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
	bed := newLedgerAttentionBed(t)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	bed.open(t, "pin-target")
	opened := RunLedgerAttention(bed.watcher, bed.now.Add(2*time.Minute))
	if err := PersistLedgerAttentionMark(bed.watcher, []string{opened.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	bed.pin(t, "pin-target", "mac-a")
	pinned := RunLedgerAttention(bed.watcher, bed.now.Add(4*time.Minute))
	if len(pinned.Pending) != 1 || strings.Join(pinned.Pending[0].Pins, ",") != "pin-target" {
		t.Fatalf("local pin did not surface through schema-2 Pending.Pins: %+v", pinned)
	}
	if err := PersistLedgerAttentionMark(bed.watcher, []string{pinned.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	bed.pin(t, "pin-target", "mac-b")
	foreign := RunLedgerAttention(bed.watcher, bed.now.Add(6*time.Minute))
	for _, event := range foreign.Pending {
		if len(event.Pins) > 0 || len(event.Claimable) > 0 {
			t.Fatalf("foreign pin surfaced as local attention: %+v", foreign)
		}
	}
	bed.pin(t, "pin-target", "-")
	cleared := RunLedgerAttention(bed.watcher, bed.now.Add(8*time.Minute))
	if len(cleared.Pending) != 1 || strings.Join(cleared.Pending[0].Claimable, ",") != "pin-target" {
		t.Fatalf("clearing the foreign pin did not restore the local frontier: %+v", cleared)
	}
	if err := PersistLedgerAttentionMark(bed.watcher, []string{cleared.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	bed.claimAs(t, "pin-target", "mac-b")
	departed := RunLedgerAttention(bed.watcher, bed.now.Add(10*time.Minute))
	if len(departed.Pending) != 1 || strings.Join(departed.Pending[0].QueueWas, ",") != "pin-target" || len(departed.Pending[0].QueueNow) != 0 {
		t.Fatalf("a real queue departure did not surface its before/after sequence: %+v", departed)
	}
	if sameStrings([]string{"a", "b", "c"}, []string{"b", "a", "c"}) {
		t.Fatal("same-member queue reordering was treated as unchanged")
	}
}

func TestLedgerAttentionKeepsOneDigestAndNudgePerChange(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	bed.open(t, "change-one")
	bed.open(t, "change-two")
	report := RunLedgerAttention(bed.watcher, bed.now.Add(4*time.Minute))
	if len(report.Pending) != 2 || report.Pending[0].SourceID == report.Pending[1].SourceID {
		t.Fatalf("two remote changes were coalesced: %+v", report)
	}
	result := TickResult{LedgerAttention: report}
	if err := NarrateDigest(bed.watcher, Evidence{}, result, bed.now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	// A crash before the mark replays the same pending events; the digest's
	// source identity keeps one line per change rather than one per attempt.
	if err := NarrateDigest(bed.watcher, Evidence{}, result, bed.now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, event := range report.Pending {
		if err := QueueNotification(bed.watcher, ledgerAttentionNotification(event, "mac-a")); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(narratordigest.Path(bed.watcher))
	if err != nil || strings.Count(string(data), "source: ledger ") != 2 {
		t.Fatalf("per-change digest entries: %q %v", data, err)
	}
	pending, err := PendingNotifications(bed.watcher)
	if err != nil || len(pending) != 2 || pending[0].Nonce == pending[1].Nonce {
		t.Fatalf("per-change pending notifications: %+v %v", pending, err)
	}
}

func TestLedgerAttentionRetainsTransientFactAcrossSkippedMark(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	firstTip := bed.open(t, "transient-a")
	first := RunLedgerAttention(bed.watcher, bed.now.Add(2*time.Minute))
	if len(first.Pending) != 1 || first.Pending[0].Tip != firstTip {
		t.Fatalf("first transient fact: %+v", first)
	}
	// Simulate a crash after classification and accepted-ref advance by
	// deliberately skipping both surfacing and the mark.
	bed.claim(t, "transient-a")
	second := RunLedgerAttention(bed.watcher, bed.now.Add(4*time.Minute))
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
	bed := newLedgerAttentionBed(t)
	baseTip := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	forwardTip := bed.open(t, "rewind-visible")
	forward := RunLedgerAttention(bed.watcher, bed.now.Add(2*time.Minute))
	if len(forward.Pending) != 1 || forward.Pending[0].SourceID != forwardTip {
		t.Fatalf("initial forward event: %+v", forward)
	}
	if err := PersistLedgerAttentionMark(bed.watcher, []string{forward.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	attentionGit(t, bed.publisher, "push", "-q", "--force", "origin", baseTip+":refs/heads/main")
	endpoint, err := goal.ResolveEndpoint(bed.watcher)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := goal.RepairAcceptRemote(endpoint, "Wido"); err != nil {
		t.Fatal(err)
	}
	rewound := RunLedgerAttention(bed.watcher, bed.now.Add(4*time.Minute))
	if len(rewound.Pending) != 1 || rewound.Pending[0].Tip != baseTip || !strings.Contains(rewound.Pending[0].SourceID, "-epoch-1") {
		t.Fatalf("sanctioned rewind was not one direct, epoch-qualified transition: %+v", rewound)
	}
	if err := PersistLedgerAttentionMark(bed.watcher, []string{rewound.Pending[0].SourceID}); err != nil {
		t.Fatal(err)
	}
	attentionGit(t, bed.publisher, "push", "-q", "origin", forwardTip+":refs/heads/main")
	forwardAgain := RunLedgerAttention(bed.watcher, bed.now.Add(6*time.Minute))
	if len(forwardAgain.Pending) != 1 || forwardAgain.Pending[0].Tip != forwardTip || forwardAgain.Pending[0].SourceID == forward.Pending[0].SourceID {
		t.Fatalf("revisited commit was silently deduplicated after rewind: first=%+v later=%+v", forward, forwardAgain)
	}
}

func TestLedgerAttentionDurabilityRefusalDoesNotFetch(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	remoteTip := bed.open(t, "must-not-fetch")
	acceptedBefore := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef)
	previous := ledgerAttentionWriter
	ledgerAttentionWriter = func(string, string, string) (bool, error) { return false, nil }
	t.Cleanup(func() { ledgerAttentionWriter = previous })
	report := RunLedgerAttention(bed.watcher, bed.now)
	if report.Outcome != "failed" || report.FailureKind != ledgerAttentionStateWriteFailed {
		t.Fatalf("durability refusal was mislabeled: %+v", report)
	}
	if acceptedAfter := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef); acceptedAfter != acceptedBefore || acceptedAfter == remoteTip {
		t.Fatalf("fetch advanced past an uncommitted baseline: before=%s after=%s remote=%s", acceptedBefore, acceptedAfter, remoteTip)
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
	wrapper := filepath.Join(dir, "git")
	script := `#!/bin/sh
case " $* " in
  *" fetch "*)
    echo $$ > "$LEDGER_ATTENTION_GROUP_FILE"
    trap '' TERM
    while :; do sleep 1; done
    ;;
esac
exec "$LEDGER_ATTENTION_REAL_GIT" "$@"
`
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LEDGER_ATTENTION_REAL_GIT", realGit)
	t.Setenv("LEDGER_ATTENTION_GROUP_FILE", groupFile)
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
	if err != nil {
		t.Fatal(err)
	}
	groupID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || groupID < 1 {
		t.Fatalf("invalid transport group identity %q: %v", data, err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		err := syscall.Kill(-groupID, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("ledger transport group %d survived the 30-second hang failsafe", groupID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestLedgerAttentionRecoversDurableStageBeforeAcceptedCAS(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	acceptedBefore := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef)
	remoteTip := bed.open(t, "staged-before-cas")
	original := ledgerAttentionWriter
	ledgerAttentionWriter = func(path, contents, anchor string) (bool, error) {
		durable, err := original(path, contents, anchor)
		if err == nil && strings.Contains(contents, `"staged": {`) {
			return false, nil
		}
		return durable, err
	}
	t.Cleanup(func() { ledgerAttentionWriter = original })
	failed := RunLedgerAttention(bed.watcher, bed.now.Add(2*time.Minute))
	if failed.Outcome != "failed" || failed.FailureKind != ledgerAttentionStateWriteFailed {
		t.Fatalf("staged durability uncertainty was not refused: %+v", failed)
	}
	if accepted := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef); accepted != acceptedBefore {
		t.Fatalf("accepted ref moved before the staged frontier was proven durable: %s", accepted)
	}
	ledgerAttentionWriter = original
	recovered := RunLedgerAttention(bed.watcher, bed.now.Add(3*time.Minute))
	if len(recovered.Pending) != 1 || recovered.Pending[0].Tip != remoteTip {
		t.Fatalf("durable pre-CAS stage did not recover exactly once: %+v", recovered)
	}
	state, _, err := loadLedgerAttentionState(bed.watcher)
	if err != nil || state.Staged != nil || state.DiffedTip != remoteTip {
		t.Fatalf("recovered stage did not promote atomically: %+v %v", state, err)
	}
}

func TestLedgerAttentionCrashStageDoesNotResurrectHumanRetirement(t *testing.T) {
	bed := newLedgerAttentionBed(t)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	acceptedBefore := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef)
	retiredTip := bed.open(t, "retired-mid-capture")
	original := ledgerAttentionWriter
	ledgerAttentionWriter = func(path, contents, anchor string) (bool, error) {
		durable, err := original(path, contents, anchor)
		if err == nil && strings.Contains(contents, `"staged": {`) {
			return false, nil
		}
		return durable, err
	}
	t.Cleanup(func() { ledgerAttentionWriter = original })
	failed := RunLedgerAttention(bed.watcher, bed.now.Add(2*time.Minute))
	if failed.Outcome != "failed" || failed.FailureKind != ledgerAttentionStateWriteFailed {
		t.Fatalf("crash fixture did not stop after its durable stage: %+v", failed)
	}
	attentionGit(t, bed.publisher, "push", "-q", "--force", "origin", acceptedBefore+":refs/heads/main")
	endpoint, err := goal.ResolveEndpoint(bed.watcher)
	if err != nil {
		t.Fatal(err)
	}
	if repaired, err := goal.RepairAcceptRemote(endpoint, "Wido"); err != nil || repaired.Tip != acceptedBefore {
		t.Fatalf("human retirement repair: %+v %v", repaired, err)
	}
	ledgerAttentionWriter = original

	recovered := RunLedgerAttention(bed.watcher, bed.now.Add(3*time.Minute))
	if recovered.Outcome != "current" || recovered.Failure != "" || len(recovered.Pending) != 0 || recovered.Tip != acceptedBefore {
		t.Fatalf("crash recovery resurfaced the retired capture: %+v", recovered)
	}
	if accepted := attentionGit(t, bed.watcher, "rev-parse", goal.AcceptedRef); accepted != acceptedBefore || accepted == retiredTip {
		t.Fatalf("crash recovery resurrected retired accepted tip: accepted=%s retired=%s", accepted, retiredTip)
	}
	state, _, err := loadLedgerAttentionState(bed.watcher)
	if err != nil || state.Staged != nil || state.DiffedTip != acceptedBefore {
		t.Fatalf("retired durable stage survived recovery: %+v %v", state, err)
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
	bed := newLedgerAttentionBed(t)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	remoteTip := bed.open(t, "examine-me")
	moved := RunLedgerAttention(bed.watcher, bed.now.Add(2*time.Minute))
	if moved.Tip != remoteTip || moved.MovedAt.IsZero() {
		t.Fatalf("movement did not start the examination clock: %+v", moved)
	}
	opid := "journal-examines-remote"
	if _, err := goal.CreateEntry(bed.watcher, opid, "mac-a", "attention-fixture", goal.Intent{Verb: "edit"}); err != nil {
		t.Fatal(err)
	}
	attentionGit(t, bed.watcher, "config", "goal.sync-remote", filepath.Join(t.TempDir(), "unreachable.git"))
	empty := RunLedgerAttention(bed.watcher, bed.now.Add(3*time.Minute))
	if empty.Outcome != "failed" {
		t.Fatalf("offline fetch unexpectedly succeeded: %+v", empty)
	}
	state, _, err := loadLedgerAttentionState(bed.watcher)
	if err != nil || state.ExaminedTip == remoteTip || state.MovedAt == "" {
		t.Fatalf("a journal entry without a fetched tip falsely cleared attention: %+v %v", state, err)
	}
	if err := goal.RecordSteps(bed.watcher, opid, remoteTip, ""); err != nil {
		t.Fatal(err)
	}
	report := RunLedgerAttention(bed.watcher, bed.now.Add(4*time.Minute))
	if report.Outcome != "failed" {
		t.Fatalf("offline fetch unexpectedly succeeded: %+v", report)
	}
	state, _, err = loadLedgerAttentionState(bed.watcher)
	if err != nil || state.ExaminedTip != remoteTip || state.MovedAt != "" {
		t.Fatalf("journal evidence did not clear before offline fetch: %+v %v", state, err)
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
	if _, err := CompleteHookAttempt(root, hook.Generation, hook.AttemptSeq, ComponentOK, "EMITTED", line, payload, now.Add(time.Second)); err != nil {
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
	bed := newLedgerAttentionBed(t)
	_ = RunLedgerAttention(bed.watcher, bed.now)
	attentionGit(t, bed.watcher, "config", "goal.sync-remote", filepath.Join(t.TempDir(), "unreachable.git"))
	failureAt := bed.now.Add(time.Minute)
	if report := RunLedgerAttention(bed.watcher, failureAt); report.Outcome != "failed" {
		t.Fatalf("unreachable remote unexpectedly succeeded: %+v", report)
	}
	if early := checkLedgerAttention(bed.watcher, failureAt.Add(29*time.Minute)); early.Status != HealthAlive {
		t.Fatalf("a fresh fetch failure was not inside its patience window: %+v", early)
	}
	if mature := checkLedgerAttention(bed.watcher, failureAt.Add(30*time.Minute)); mature.Status != HealthUnknown || !strings.Contains(mature.Reason, "unreachable") {
		t.Fatalf("a persistent fetch failure did not mature to unknown: %+v", mature)
	}
}
