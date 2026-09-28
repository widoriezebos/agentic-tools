package adapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// fixtureStartGateRoot is a root whose committed configuration selects the
// fake runtime, the only root where the fixture expiry event is honored.
func fixtureStartGateRoot(t *testing.T, runtimes string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"),
		[]byte("metasystem.runtimes="+runtimes+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// ownFixtureKey encodes a fixture key whose owner is this test process, or
// another pid carrying the same start identity when pid is non-zero.
func ownFixtureKey(t *testing.T, pid int64) string {
	t.Helper()
	exact, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe own process: state=%v err=%v", state, err)
	}
	owner := exact.Ref()
	if pid != 0 {
		owner.Pid = pid
	}
	key, err := identity.EncodeKey(identity.FixtureKey{Owner: owner, Test: "start-gate", Nonce: "0123abcd"})
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func envOf(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestWaitStartGateFileRefusesIncompleteRequests(t *testing.T) {
	t.Parallel()
	gate := filepath.Join(t.TempDir(), "gate")
	for _, row := range []struct {
		name          string
		path          string
		timeout, poll time.Duration
	}{
		{"no path", "", time.Hour, time.Millisecond},
		{"zero timeout", gate, 0, time.Millisecond},
		{"negative poll", gate, time.Hour, -time.Millisecond},
	} {
		err := WaitStartGateFile("", row.path, row.timeout, row.poll, envOf(nil))
		if err == nil || !strings.Contains(err.Error(), "requires a path, a positive timeout and a positive poll interval") {
			t.Errorf("%s: incomplete request = %v, want the named refusal", row.name, err)
		}
	}
}

func TestWaitStartGateFileOpensOnAPublishedGate(t *testing.T) {
	t.Parallel()
	gate := filepath.Join(t.TempDir(), "gate")
	if err := os.WriteFile(gate, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WaitStartGateFile("", gate, time.Hour, time.Millisecond, envOf(nil)); err != nil {
		t.Fatalf("an already-published gate did not open: %v", err)
	}
}

// The gate is published by another party after the wait began; a poll tick
// sees it. The timeout is far beyond the test, so only the gate can end the
// wait successfully.
func TestWaitStartGateFileOpensWhenTheGateAppearsLater(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gate := filepath.Join(dir, "gate")
	done := make(chan error, 1)
	go func() { done <- WaitStartGateFile("", gate, time.Hour, time.Millisecond, envOf(nil)) }()
	if err := os.WriteFile(filepath.Join(dir, "gate.tmp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "gate.tmp"), gate); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("a gate published during the wait did not open it: %v", err)
	}
}

func TestWaitStartGateFileFixtureExpiryEventAuthority(t *testing.T) {
	t.Parallel()
	event := filepath.Join(t.TempDir(), "expire")
	if err := os.WriteFile(event, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(t.TempDir(), "never-published")
	fakeRoot := fixtureStartGateRoot(t, "fake")

	t.Run("no root", func(t *testing.T) {
		t.Parallel()
		err := WaitStartGateFile("", gate, time.Hour, time.Millisecond,
			envOf(map[string]string{StartGateFixtureExpiryEventEnvironment: event}))
		if err == nil || !strings.Contains(err.Error(), "requires a fake-runtime root") {
			t.Fatalf("expiry event honored without a root: %v", err)
		}
	})
	t.Run("real runtime root", func(t *testing.T) {
		t.Parallel()
		err := WaitStartGateFile(fixtureStartGateRoot(t, "claude"), gate, time.Hour, time.Millisecond,
			envOf(map[string]string{StartGateFixtureExpiryEventEnvironment: event}))
		if err == nil || !strings.Contains(err.Error(), "requires a fake-runtime root") {
			t.Fatalf("expiry event honored in a real-runtime root: %v", err)
		}
	})
	for _, row := range []struct {
		name  string
		owner string
	}{
		{"missing owner", ""},
		{"malformed owner", "not-a-key"},
		{"owner not the live process", ownFixtureKey(t, 1<<30)},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			err := WaitStartGateFile(fakeRoot, gate, time.Hour, time.Millisecond, envOf(map[string]string{
				StartGateFixtureExpiryEventEnvironment: event,
				identity.FixtureOwnerEnv:               row.owner,
			}))
			if err == nil || !strings.Contains(err.Error(), "requires its exact owner") {
				t.Fatalf("expiry event honored with %s: %v", row.name, err)
			}
		})
	}
	t.Run("live exact owner expires the gate", func(t *testing.T) {
		t.Parallel()
		err := WaitStartGateFile(fakeRoot, gate, time.Hour, time.Millisecond, envOf(map[string]string{
			StartGateFixtureExpiryEventEnvironment: event,
			identity.FixtureOwnerEnv:               ownFixtureKey(t, 0),
		}))
		if !errors.Is(err, ErrStartGateExpired) {
			t.Fatalf("published expiry event did not expire the gate: %v", err)
		}
	})
}

func TestLiveFixtureOwnerNamesTheRefusal(t *testing.T) {
	t.Parallel()
	if err := liveFixtureOwner(ownFixtureKey(t, 0)); err != nil {
		t.Fatalf("this live process was refused as its own owner: %v", err)
	}
	if err := liveFixtureOwner("a|b"); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed owner = %v, want the malformed refusal", err)
	}
	if err := liveFixtureOwner(ownFixtureKey(t, 1<<30)); err == nil || !strings.Contains(err.Error(), "does not match the live exact process") {
		t.Fatalf("absent owner = %v, want the live-identity refusal", err)
	}
}

func TestStartGateCancellationAndBackwardClock(t *testing.T) {
	t.Parallel()
	start := time.Unix(500, 0)
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	timerReady := make(chan struct{})
	events := make(chan struct{}, 1)
	checks := make(chan struct{}, 4)
	type outcome struct {
		result StartGateResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := WaitStartGate(ctx, time.Minute, StartGateDependencies{
			// The second reading is earlier than the first: elapsed clamps
			// to zero rather than going negative.
			Now: func() time.Time {
				calls++
				if calls == 1 {
					return start
				}
				return start.Add(-time.Second)
			},
			After:  func(time.Duration) <-chan time.Time { close(timerReady); return make(chan time.Time) },
			Events: events,
			Exists: func() bool { checks <- struct{}{}; return false },
		})
		done <- outcome{result, err}
	}()
	<-timerReady
	<-checks // the initial existence check
	// A spurious event with no gate keeps waiting.
	events <- struct{}{}
	<-checks
	cancel()
	got := <-done
	if !errors.Is(got.err, context.Canceled) || got.result.Opened || got.result.ElapsedMS != 0 {
		t.Fatalf("cancelled wait = %+v, %v; want not opened, zero elapsed, context.Canceled", got.result, got.err)
	}
}

func TestStartGateReportsElapsedFromTheInjectedClock(t *testing.T) {
	t.Parallel()
	start := time.Unix(900, 0)
	readings := []time.Time{start, start.Add(1500 * time.Millisecond)}
	result, err := WaitStartGate(context.Background(), time.Minute, StartGateDependencies{
		Now: func() time.Time {
			now := readings[0]
			readings = readings[1:]
			return now
		},
		After:  func(time.Duration) <-chan time.Time { t.Fatal("an open gate armed its timer"); return nil },
		Exists: func() bool { return true },
	})
	if err != nil || !result.Opened || result.ElapsedMS != 1500 {
		t.Fatalf("open gate = %+v, %v; want opened after 1500 ms", result, err)
	}
}
