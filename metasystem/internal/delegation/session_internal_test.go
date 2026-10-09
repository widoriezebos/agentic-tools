package delegation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// stubLease records the lease operations an internal step reaches.
type stubLease struct {
	calls []string
}

func (l *stubLease) Classify(Invocation) (lease.ClassifyResult, error) {
	return lease.ClassifyResult{Class: lease.ClassHuman}, nil
}
func (l *stubLease) RequireHolder(Invocation, *int64) (lease.HolderView, error) {
	return lease.HolderView{Class: lease.ClassHuman, Holder: true}, nil
}
func (l *stubLease) Renew(Invocation) (lease.RenewResult, error) { return lease.RenewResult{}, nil }
func (l *stubLease) Held(_ Invocation, _ *int64, fn func() error) error {
	l.calls = append(l.calls, "held")
	return fn()
}
func (l *stubLease) Authorize(_ Invocation, mode AuthorityMode, job string) error {
	l.calls = append(l.calls, fmt.Sprintf("authorize %s %s", mode, job))
	return nil
}

type stubHost struct{ wait WaitOutcome }

func (stubHost) UnitLaunchStatus(string) (string, error) { return "", os.ErrNotExist }
func (stubHost) CancelUnitLaunch(string) error           { return os.ErrNotExist }

func (h stubHost) WaitJob(context.Context, string, string, int64) WaitOutcome { return h.wait }
func (stubHost) WatchJob(context.Context, string, string, int64, string) int  { return 0 }
func (stubHost) BreachStopOrderingHuman(context.Context, string, int64, time.Time) (string, error) {
	return "", errors.New("no enrolled person")
}
func (stubHost) ExtendBudget(context.Context, ExtendBudgetRequest) (string, int) {
	return "", 1
}

// stubGoal is a checkout with no accepted goal ledger.
type stubGoal struct{}

func (stubGoal) Binding(string) (GoalBinding, error) { return GoalBinding{}, nil }
func (stubGoal) LedgerIdentity() string              { return "" }
func (stubGoal) BreachStop(string, uint64, time.Time, string) (goal.StopBatch, error) {
	return goal.StopBatch{}, errors.New("no accepted goal ledger")
}

type stubClock struct{ now time.Time }

func (c *stubClock) Now() time.Time        { return c.now }
func (c *stubClock) Sleep(d time.Duration) { c.now = c.now.Add(d) }

func internalSession(t *testing.T, ports Ports) (*session, *bytes.Buffer) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"artifacts/agents/jobs", "artifacts/agents/record-locks", "artifacts/agents/hb", "artifacts/agents/supervision"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if ports.Goal == nil {
		ports.Goal = stubGoal{}
	}
	var stderr bytes.Buffer
	life := &Lifecycle{ports: ports, root: root, repoScope: root, engine: "/engine"}
	return life.newSession(context.Background(), Request{
		Invocation: Invocation{CallerPid: int64(os.Getpid())}, LockTag: "internal-test-tag", Stderr: &stderr,
	}), &stderr
}

// authority-regression WC-3: the wait re-enters reaping only through the
// lease-held entry, once, for every terminal verdict, and maps the waiter's
// verdict to the dispatcher's exit codes.
func TestWaitReapsOnlyThroughTheLeaseHeldEntry(t *testing.T) {
	t.Parallel()
	for waiter, want := range map[int]int{0: 0, 1: 3, 2: 4, 3: 8, 4: 5, 9: 9} {
		held := &stubLease{}
		s, _ := internalSession(t, Ports{Lease: held, Host: stubHost{wait: WaitOutcome{Code: waiter}}, Clock: &stubClock{now: time.Unix(1790000000, 0)}})
		if got := s.waitForJob("job-a"); got != want {
			t.Fatalf("waiter %d mapped to %d, want %d", waiter, got, want)
		}
		reaped := waiter <= 3
		if reaped != (strings.Join(held.calls, ";") == "held;authorize holder-only ") {
			t.Fatalf("waiter %d: lease calls %v", waiter, held.calls)
		}
	}
}

// delegate-caps AUTH-R2-005: a cap at or above the live watcher's attested
// ceiling refuses by name, whatever the configuration raised.
func TestJobCapMustStayBelowTheAttestedWatcherCeiling(t *testing.T) {
	t.Parallel()
	s, stderr := internalSession(t, Ports{Clock: &stubClock{now: time.Now()}, Git: ownerGit{}})
	conf := "dispatch.cap-min=120\ndispatch.cap-max=900\ncap.min.implementer.fake.fake-model=500\n"
	if err := os.WriteFile(filepath.Join(s.root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1790000000, 0)
	s.l.ports.Clock = &stubClock{now: now}
	heartbeat := filepath.Join(s.agents, "supervision", "watcher.heartbeat.json")
	writeJSON := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON(heartbeat, fmt.Sprintf(`{"pid":4242,"pidStartedAt":7,"instanceTag":"w","loadedCapMin":330,"observedAtEpoch":%d}`, now.Unix()))
	// The state claims a raised ceiling; only the watcher's own attested
	// loadedCapMin counts.
	writeJSON(filepath.Join(s.agents, "supervision", "state.json"), fmt.Sprintf(
		`{"derivedWatcherCapMin":999,"intervalSec":60,"components":{"watcher":{"pid":4242,"pidStartedAt":7,"instanceTag":"w","heartbeat":%q}}}`, heartbeat))
	output := filepath.Join(t.TempDir(), "cap.json")
	err := s.authorizeJobCap("job-a", "implementer", "fake", "fake-model", "", "", "", "dispatch", output)
	if ExitCode(err) != 1 || !strings.Contains(stderr.String(), "dispatch refused: its 500m cap is not below the watcher's 330m ceiling\nre-arm supervision with --max-cap above 500") {
		t.Fatalf("exit %d stderr %q", ExitCode(err), stderr.String())
	}
	stderr.Reset()
	if err := s.authorizeJobCap("job-b", "implementer", "fake", "fake-model", "", "", "300", "dispatch", output); err != nil {
		t.Fatalf("a cap below the attested ceiling refused: %v %s", err, stderr.String())
	}
}
