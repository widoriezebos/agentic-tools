package goal

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func detachedWaitFixture(t *testing.T, kind string, readyBacklog bool) *pendingWaitVerdictFixture {
	t.Helper()
	fixture := newPendingWaitVerdictFixture(t, "job", readyBacklog)
	fixture.row.Kind = kind
	fixture.row.TargetID = fixture.row.WaitID
	fixture.row.Target = metarun.WaiterTarget{}
	fixture.row.GoalID = ""
	fixture.row.Selector = metarun.WaitSelector{Kind: kind, TargetID: fixture.row.WaitID}
	fixture.row.LastObservedAt = ""
	fixture.row.LastObservedBootNanos = 0
	fixture.row.DeadlineBootID = ""
	fixture.row.BootDeadlineNanos = 0
	fixture.row.RegisteredBootID = pendingWaitBootID
	fixture.row.RegisteredBootNanos = fixture.bootElapsed.Nanoseconds()
	fixture.row.Label = "compile the release"
	fixture.row.Question = ""
	fixture.row.JobID = ""
	fixture.row.Delivery = "harness"
	if kind == "human" {
		fixture.row.Pid = 0
		fixture.row.PidStartedAt = 0
		fixture.row.PidStartedAtMicro = 0
		fixture.row.PidStartTicks = 0
		fixture.row.BootID = ""
		fixture.row.RegisteredBootID = ""
		fixture.row.RegisteredBootNanos = 0
		fixture.row.Label = ""
		fixture.row.Question = "May this ship?"
		fixture.row.Selector.Question = fixture.row.Question
		fixture.row.Delivery = "human"
	}
	fixture.writeRow(t)
	return fixture
}

func TestLocalWaitAllowsTheStopWhileItsProcessLives(t *testing.T) {
	t.Parallel()
	fixture := detachedWaitFixture(t, "local", true)
	verdict := fixture.verdict(t, fixture.scan)
	waitLine := "WAITING: compile the release (pid 42) until " + fixture.row.Deadline
	if verdict.ShouldBlock || verdict.BlockSource != nil || strings.Count(verdict.Display, "WAITING:") != 1 ||
		!strings.Contains(verdict.Display, waitLine) {
		t.Fatalf("live local wait verdict=%+v", verdict)
	}
}

func TestLocalWaitOfADeadProcessDoesNotAllowTheStop(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		mutate     func(*pendingWaitVerdictFixture)
		wantReason string
	}{
		{name: "gone", wantReason: "liveness=dead", mutate: func(f *pendingWaitVerdictFixture) {
			prober := f.store.Prober.(idleFixtureProber)
			delete(prober, 42)
		}},
		{name: "reused pid", wantReason: "liveness=dead", mutate: func(f *pendingWaitVerdictFixture) {
			prober := f.store.Prober.(idleFixtureProber)
			prober[42] = identity.Exact{Pid: 42, StartedAt: time.Unix(201, 0), StartTicks: 421, BootID: pendingWaitBootID}
		}},
		{name: "invalid identity", wantReason: "mode=invalid", mutate: func(f *pendingWaitVerdictFixture) {
			f.row.PidStartTicks = 0
		}},
		{name: "changed boot", wantReason: "row-boot-clock", mutate: func(f *pendingWaitVerdictFixture) {
			f.row.RegisteredBootID = "earlier-boot"
		}},
		{name: "deadline", wantReason: "row-wall-clock", mutate: func(f *pendingWaitVerdictFixture) {
			f.row.Deadline = f.store.Now().Format(time.RFC3339Nano)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := detachedWaitFixture(t, "local", true)
			test.mutate(fixture)
			fixture.writeRow(t)
			verdict := fixture.verdict(t, fixture.scan)
			if !verdict.ShouldBlock || verdict.BlockSource == nil || strings.Contains(verdict.Display, "WAITING: compile") {
				t.Fatalf("invalid local wait changed the decision: %+v", verdict)
			}
			joined := strings.Join(verdict.Diagnostics, "\n")
			if !strings.Contains(joined, test.wantReason) {
				t.Fatalf("drop reason %q missing from %q", test.wantReason, joined)
			}
		})
	}
}

type countingWaitProber struct {
	base  identity.Prober
	reads []int64
}

func (p *countingWaitProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	p.reads = append(p.reads, pid)
	return p.base.Probe(pid)
}

func TestHumanWaitAllowsTheStopUntilItsDeadline(t *testing.T) {
	t.Parallel()
	fixture := detachedWaitFixture(t, "human", true)
	counter := &countingWaitProber{base: fixture.store.Prober}
	fixture.store.Prober = counter
	verdict := fixture.verdict(t, fixture.scan)
	if verdict.ShouldBlock || !strings.Contains(verdict.Display, "WAITING: human answer to May this ship? until "+fixture.row.Deadline) {
		t.Fatalf("pending human wait verdict=%+v", verdict)
	}
	for _, pid := range counter.reads {
		if pid == 0 {
			t.Fatalf("human wait probed a tracked process: reads=%v", counter.reads)
		}
	}
	waits, reasons := fixture.store.registeredWaits(readClaimableWorkForTest(t, fixture), pendingWaitSession, pendingWaitMainID, false)
	if len(reasons) != 0 || len(waits) != 1 || !waits[0].humanAct || !waits[0].detached || !waits.hasWorkInFlight() {
		t.Fatalf("human wait classification=%+v reasons=%v", waits, reasons)
	}

	expired := detachedWaitFixture(t, "human", true)
	expired.row.Deadline = expired.store.Now().Format(time.RFC3339Nano)
	expired.writeRow(t)
	if verdict := expired.verdict(t, expired.scan); !verdict.ShouldBlock || strings.Contains(verdict.Display, "WAITING: human") {
		t.Fatalf("expired human wait verdict=%+v", verdict)
	}

	ended := detachedWaitFixture(t, "human", true)
	epoch := int64(7)
	owner := metarun.Caller{Class: "MAIN", MainId: pendingWaitMainID, OwnerLineage: pendingWaitLineage, ClaimEpoch: &epoch, SessionId: pendingWaitSession}
	row, err := (&metarun.Store{Root: ended.root}).EndDetachedWait(ended.row.WaitID, owner, pendingWaitSession,
		metarun.WaitOptions{Now: ended.store.Now})
	if err != nil || row.State != metarun.WaiterStateInterrupted {
		t.Fatalf("end human wait row=%+v err=%v", row, err)
	}
	if verdict := ended.verdict(t, ended.scan); !verdict.ShouldBlock || strings.Contains(verdict.Display, "WAITING: human") {
		t.Fatalf("ended human wait verdict=%+v", verdict)
	}
}

func readClaimableWorkForTest(t *testing.T, fixture *pendingWaitVerdictFixture) ClaimableBudgetedWork {
	t.Helper()
	work, err := readClaimableBudgetedWork(fixture.root, fixture.store.Now(), fixture.store.Prober, fixture.store.projectionDeps)
	if err != nil {
		t.Fatal(err)
	}
	return work
}

func TestLocalWaitCoversTheJobItNames(t *testing.T) {
	t.Parallel()
	fixture := detachedWaitFixture(t, "local", false)
	fixture.row.JobID = "background-job"
	fixture.writeRow(t)
	scan := fixture.scan
	scan.Jobs = []JobFact{{Id: "background-job", MainId: pendingWaitMainID, StartedAt: "2026-09-18T09:00:00Z", Status: "running"}}
	if verdict := fixture.verdict(t, scan); verdict.ShouldBlock || strings.Contains(verdict.Display, "unwatched") {
		t.Fatalf("named job was not covered: %+v", verdict)
	}
	if err := os.Remove(metarun.WaiterPath(fixture.root, fixture.row.Kind, fixture.row.TargetID, fixture.row.OwnerDigest)); err != nil {
		t.Fatal(err)
	}
	without := fixture.verdict(t, scan)
	if !without.ShouldBlock || without.BlockSource == nil || *without.BlockSource != "unwatched-work" {
		t.Fatalf("job without local wait was not refused: %+v", without)
	}
}
