package steward

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fakeCensus struct {
	workers Workers
	err     error
}

func (f fakeCensus) Workers(string) (Workers, error) { return f.workers, f.err }

// gitRepoWithCurrentGoal builds a real repository with an owned goal,
// so marks and open work read from genuine sources.
func gitRepoWithCurrentGoal(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := "# Goals\n\n## Current goal: fix-it — Repair the thing\n- Origin: main\n- Next step: Repair it.\n"
	if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "baseline")
	return root
}

func tickN(t *testing.T, root string, cfg TickConfig, census WorkerCensus, n int) TickResult {
	t.Helper()
	var last TickResult
	for i := 0; i < n; i++ {
		r, err := RunTick(root, cfg, census)
		if err != nil {
			t.Fatal(err)
		}
		last = r
	}
	return last
}

func TestQuietTicksAgeIntoLiveIdleNotification(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	r := tickN(t, root, TickConfig{StaleTicks: 3}, census, 3)
	if r.Decision.Verdict != VerdictHealthy {
		t.Fatalf("inside the threshold a live worker is healthy: %+v", r.Decision)
	}
	r = tickN(t, root, TickConfig{StaleTicks: 3}, census, 1)
	if r.Decision.Verdict != VerdictStalledIdle || r.Decision.Action != ActNotify {
		t.Fatalf("past the threshold a live-idle worker is notified, never displaced: %+v", r.Decision)
	}
}

func TestCommitResetsTheAging(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	tickN(t, root, TickConfig{StaleTicks: 2}, census, 2)
	// Real progress: a commit moves HEAD.
	cmd := exec.Command("git", "-C", root, "commit", "-q", "--allow-empty", "-m", "progress")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
	r := tickN(t, root, TickConfig{StaleTicks: 2}, census, 1)
	if r.Evidence.TicksSinceAdvance != 0 || r.Decision.Verdict != VerdictHealthy {
		t.Fatalf("a commit is progress: %+v", r)
	}
}

func TestProvenDeathRevivesRegardlessOfFreshEvidence(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{CensusComplete: true}}
	r := tickN(t, root, TickConfig{}, census, 1)
	if r.Decision.Verdict != VerdictStalledDead || r.Decision.Action != ActRevive {
		t.Fatalf("dead seconds after a fresh commit is still dead: %+v", r.Decision)
	}
}

func TestUnreadableCensusNeverSpawns(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{err: os.ErrPermission}
	r := tickN(t, root, TickConfig{}, census, 1)
	if r.Decision.Verdict != VerdictUnknown || r.Decision.Action != ActNotify {
		t.Fatalf("an unreadable census cannot prove death: %+v", r.Decision)
	}
}

func TestOpenIntentSuppressesASecondRevival(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	it := testIntent("live-one")
	if err := MintIntent(root, it); err != nil {
		t.Fatal(err)
	}
	census := fakeCensus{workers: Workers{CensusComplete: true}}
	r := tickN(t, root, TickConfig{}, census, 1)
	if r.Decision.Action != ActNotify {
		t.Fatalf("an open continuation suppresses dispatch: %+v", r.Decision)
	}
}

func TestGoalFreeRepositoryTicksQuietly(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	free := "# Goals\n\n## Goal-free: declared 2026-08-15T10:00:00Z by human over abc123\n"
	if err := os.WriteFile(filepath.Join(root, "plans", "goals.md"), []byte(free), 0o644); err != nil {
		t.Fatal(err)
	}
	r := tickN(t, root, TickConfig{}, fakeCensus{}, 1)
	if r.Decision.Verdict != VerdictNoWork || r.Decision.Action != ActNone {
		t.Fatalf("goal-free needs nothing: %+v", r.Decision)
	}
}

func TestNotifyVerdictsReachTheQueue(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	live := fakeCensus{workers: Workers{Live: 1, CensusComplete: true}}
	tickN(t, root, TickConfig{StaleTicks: 1}, live, 2) // ages past the threshold
	pending, err := PendingNotifications(root)
	if err != nil || len(pending) == 0 {
		t.Fatalf("a live-idle verdict is an incident the operator hears about: %v %v", pending, err)
	}
	found := false
	for _, n := range pending {
		if strings.Contains(n.Message, "stalled-idle") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the incident names its verdict: %v", pending)
	}
	// The standing condition holds ONE pending message per verdict.
	before := len(pending)
	tickN(t, root, TickConfig{StaleTicks: 1}, live, 3)
	after, _ := PendingNotifications(root)
	if len(after) != before {
		t.Fatalf("a repeating verdict overwrites its one message: %d -> %d", before, len(after))
	}
}

func TestDeliveredRevivalMessageCompletesTheGate(t *testing.T) {
	root := reviveRepo(t)
	if out, err := gitConfig(root, "metasystem.steward.notify-command", "exit 1"); err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	if err := PrepareIntent(root, testIntent("dg-1")); err != nil {
		t.Fatal(err)
	}
	// The channel returns; the runner's ordinary drain delivers the
	// queued revival message — the intent's gate must complete here,
	// not strand behind its own successful delivery.
	if out, err := gitConfig(root, "metasystem.steward.notify-command", "true"); err != nil {
		t.Fatalf("config: %v\n%s", err, out)
	}
	if _, err := DeliverPending(root); err != nil {
		t.Fatal(err)
	}
	live, _ := LiveIntents(root)
	if len(live) != 1 || !live[0].Notified {
		t.Fatalf("the drained delivery completes the intent's gate: %+v", live)
	}
	launched := 0
	out, err := CompleteRevival(root, TickConfig{}, deadCensus(), "dg-1", func(Intent) error { launched++; return nil })
	if err != nil || !out.Launched || launched != 1 {
		t.Fatalf("the gate-complete intent launches exactly once: %+v %d %v", out, launched, err)
	}
}

func TestReviveVerdictQueuesTheIncidentToo(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := fakeCensus{workers: Workers{CensusComplete: true}}
	r := tickN(t, root, TickConfig{}, census, 1)
	if r.Decision.Action != ActRevive {
		t.Fatalf("this world revives: %+v", r.Decision)
	}
	// The invariant says DEAD with open work is notified within one
	// tick. The revival's own gated message must not be the only
	// channel — a revival failing before its mint would loop silently.
	pending, err := PendingNotifications(root)
	if err != nil || len(pending) == 0 {
		t.Fatalf("the revive verdict is an incident the operator hears about: %v %v", pending, err)
	}
	found := false
	for _, n := range pending {
		if strings.Contains(n.Message, "stalled-dead") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the incident names its verdict: %v", pending)
	}
}
