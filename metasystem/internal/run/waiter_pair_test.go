package run

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// pairProber replays a btime step: the same live process, constant pair,
// drifting seconds.
type pairProber struct{ shift int64 }

func (p pairProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{
		Pid: pid, StartedAt: time.Unix(5000+p.shift, 0),
		StartTicks: 4242, BootID: "boot-w",
	}, identity.Alive, nil
}

type restartProber struct{ deadPID int64 }

func (p restartProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == p.deadPID {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(6000, 0), StartTicks: 600, BootID: "boot-w"}, identity.Alive, nil
}

func TestWaitRestartRecoveryReplay(t *testing.T) {
	if binary := os.Getenv("METASYSTEM_WAIT_BINARY"); binary != "" {
		t.Run("installed process death resumes from rows", func(t *testing.T) {
			binary := testutil.InstalledWaitBinary(t, binary)
			root := t.TempDir()
			self := int64(os.Getpid())
			exact, state, err := (identity.KernelProber{}).Probe(self)
			if err != nil || state != identity.Alive {
				t.Fatalf("current process identity=%+v state=%s err=%v", exact, state, err)
			}
			announce := exec.Command(binary, "lease", "announce", "--root", root, "--session", "wait-restart-session", "--pid", strconv.FormatInt(self, 10), "--start", strconv.FormatInt(exact.StartedAt.Unix(), 10), "--start-ticks", strconv.FormatInt(exact.StartTicks, 10), "--boot-id", exact.BootID, "--tag", "wait-restart-test", "--runtime", "fake", "--owner-lineage", "wait-restart-lineage")
			if output, announceErr := announce.CombinedOutput(); announceErr != nil {
				t.Fatalf("announce installed restart holder: %v %s", announceErr, output)
			}
			jobs := filepath.Join(root, "artifacts", "agents", "jobs")
			if err := os.MkdirAll(jobs, 0o755); err != nil {
				t.Fatal(err)
			}
			jobPath := filepath.Join(jobs, "job-restart.json")
			if err := os.WriteFile(jobPath, []byte(`{"jobId":"job-restart","operationId":"reserve-restart","round":1,"status":"running","startedAt":"2026-09-13T12:00:00Z"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := os.CreateTemp(t.TempDir(), "installed-wait-output-*")
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			cmd := exec.Command(binary, "wait", "--root", root, "--job", "job-restart", "--timeout", "1m", "--json")
			cmd.Stdout, cmd.Stderr = output, output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			var row Waiter
			deadline := time.Now().Add(20 * time.Second)
			for row.WaitID == "" && time.Now().Before(deadline) {
				select {
				case waitErr := <-done:
					data, _ := os.ReadFile(output.Name())
					t.Fatalf("installed waiter exited before registration: %v output=%s", waitErr, data)
				default:
				}
				rows, failures := PendingWaitersForLineages(root, []string{"wait-restart-lineage"})
				if len(failures) != 0 {
					t.Fatalf("read pending installed waiter: %v", failures)
				}
				if len(rows) == 1 {
					row = rows[0]
					break
				}
				time.Sleep(25 * time.Millisecond)
			}
			if row.WaitID == "" {
				_ = cmd.Process.Kill()
				<-done
				t.Fatal("installed waiter did not publish its pending row within 20 seconds")
			}
			if err := cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			if waitErr := <-done; waitErr == nil {
				t.Fatal("killed installed waiter unexpectedly exited successfully")
			}
			if err := os.WriteFile(jobPath, []byte(`{"jobId":"job-restart","operationId":"reserve-restart","round":1,"status":"completed","startedAt":"2026-09-13T12:00:00Z"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			resume := exec.Command(binary, "wait", "--root", root, "--resume", row.WaitID, "--json")
			resumed, err := resume.CombinedOutput()
			if err != nil || !strings.Contains(string(resumed), `"exitCode":0`) {
				t.Fatalf("installed resume did not recover from the row: err=%v output=%s", err, resumed)
			}
		})
	}
	root := t.TempDir()
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	boot := time.Hour
	terminal := func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
		return SourceObservation{Incarnation: WaiterTarget{ProofDigest: "proof-digest"}, ExitCode: ExitGreen, Outcome: "success", Reason: "proof passed", Evidence: "attempt:a:proof-digest", TerminalStamp: "2026-09-13T11:59:00Z"}, nil
	}
	options := WaitOptions{
		Now:       func() time.Time { return now },
		BootClock: func() (string, time.Duration, error) { return "boot-w", boot, nil },
		Sleep:     func(context.Context, time.Duration) error { t.Fatal("terminal replay slept"); return nil },
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
		Observe: terminal,
	}
	store := &Store{Root: root, Prober: pairProber{}}
	owner := Caller{Class: "MAIN", MainId: "main-5000-1-abc123", OwnerLineage: "lineage-a", SessionId: "session-a"}
	result := store.Wait(context.Background(), WaitRequest{Selector: WaitSelector{Kind: "attempt", TargetID: "attempt-a"}, Owner: owner, RuntimeSession: "session-a", Timeout: time.Hour}, options)
	if result.ExitCode != ExitGreen {
		t.Fatalf("initial result = %+v", result)
	}
	rows, failures := PendingWaiters(root, owner.OwnerLineage)
	if len(rows) != 0 || len(failures) != 0 {
		t.Fatalf("terminal row appeared pending: rows=%+v failures=%v", rows, failures)
	}
	if matches, err := filepath.Glob(filepath.Join(WaitersDir(root), "*.pipe")); err != nil || len(matches) != 0 {
		t.Fatalf("terminal hint endpoint remains: %v %v", matches, err)
	}
	now = now.Add(5 * time.Minute)
	replayed := store.ResumeWait(context.Background(), result.WaitID, Caller{Class: "MAIN", MainId: owner.MainId, OwnerLineage: owner.OwnerLineage, SessionId: "session-b"}, "session-b", 0, options)
	if replayed.ExitCode != ExitGreen || replayed.SourceEvidence != result.SourceEvidence || replayed.ReturnedAt == result.ReturnedAt {
		t.Fatalf("replayed result = %+v; original=%+v", replayed, result)
	}
	if err := os.Remove(WaiterPointerPath(root, result.WaitID)); err != nil {
		t.Fatal(err)
	}
	if repaired := store.ResumeWait(context.Background(), result.WaitID, owner, "session-c", 0, options); repaired.ExitCode != ExitGreen || !repaired.PointerRepaired {
		t.Fatalf("missing pointer was not repaired from the durable row: %+v", repaired)
	}
	if _, err := os.Stat(WaiterPointerPath(root, result.WaitID)); err != nil {
		t.Fatalf("repaired pointer is absent: %v", err)
	}

	t.Run("pending-row-takeover", func(t *testing.T) {
		root := t.TempDir()
		now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		boot := time.Hour
		waitID := "0123456789abcdef0123456789abcdef"
		oldNonce := "fedcba9876543210fedcba9876543210"
		oldPID := int64(987654)
		selector := WaitSelector{Kind: "goal", TargetID: "goal-a", GoalID: "goal-a", Event: "human-act", After: strings.Repeat("a", 40)}
		incarnation := WaiterTarget{StartedAt: "ledger:" + selector.After, ProofDigest: "endpoint-a"}
		row := Waiter{
			SchemaVersion: 2, WaitID: waitID, Nonce: oldNonce, Kind: selector.Kind, TargetID: selector.TargetID,
			Pid: oldPID, PidStartedAt: 5000, PidStartTicks: 500, BootID: "boot-w",
			Session: "session-old", MainId: "main-old", OwnerLineage: "lineage-a", RuntimeSession: "session-old",
			Selector: selector, GoalID: selector.GoalID, Target: incarnation, OriginalCursor: selector.After,
			RegisteredAt: now.Add(-5 * time.Minute).Format(time.RFC3339Nano), Deadline: now.Add(55 * time.Minute).Format(time.RFC3339Nano),
			BootDeadlineNanos: (boot + 55*time.Minute).Nanoseconds(), DeadlineBootID: "boot-w", RemainingNanos: (55 * time.Minute).Nanoseconds(),
			LastObservedAt: now.Add(-time.Second).Format(time.RFC3339Nano), LastObservedBootNanos: (boot - time.Second).Nanoseconds(),
			State: "pending", Delivery: "blocking", Accelerator: "unavailable",
		}
		rowPath := WaiterPath(root, row.Kind, row.TargetID, OwnerDigest(row.MainId))
		if err := writeV2Waiter(rowPath, row); err != nil {
			t.Fatal(err)
		}
		if err := writeV2Pointer(root, row, rowPath); err != nil {
			t.Fatal(err)
		}
		enteredSleep := make(chan struct{}, 1)
		release := make(chan struct{})
		options := WaitOptions{
			Now:       func() time.Time { return now },
			BootClock: func() (string, time.Duration, error) { return "boot-w", boot, nil },
			Sleep: func(context.Context, time.Duration) error {
				enteredSleep <- struct{}{}
				<-release
				return context.Canceled
			},
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				return "blocking", false, nil
			},
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Pending: true, Incarnation: incarnation, LedgerTip: selector.After}, nil
			},
		}
		store := &Store{Root: root, Prober: restartProber{deadPID: oldPID}}
		newOwner := Caller{Class: "MAIN", MainId: "main-new", OwnerLineage: "lineage-a", SessionId: "session-new"}
		done := make(chan WaitResult, 1)
		go func() { done <- store.ResumeWait(context.Background(), waitID, newOwner, "session-new", 0, options) }()
		<-enteredSleep
		resumed, resumedPath, err := LoadWaiterByID(root, waitID)
		if err != nil || resumedPath == rowPath || resumed.ResumedBy == nil || resumed.ResumedBy.Session != "session-new" || resumed.OriginalCursor != selector.After || resumed.Nonce == oldNonce {
			t.Fatalf("resumed row=%+v path=%q err=%v", resumed, resumedPath, err)
		}
		if duplicate := store.ResumeWait(context.Background(), waitID, newOwner, "session-new", 0, options); duplicate.ExitCode != ExitWaiterBusy {
			t.Fatalf("duplicate resume=%+v", duplicate)
		}
		close(release)
		if interrupted := <-done; interrupted.ExitCode != ExitInterrupted {
			t.Fatalf("resumed waiter result=%+v", interrupted)
		}
	})

	t.Run("foreign row is not repaired", func(t *testing.T) {
		root := t.TempDir()
		waitID := "abcdefabcdefabcdefabcdefabcdefab"
		row := Waiter{
			SchemaVersion: 2, WaitID: waitID, Nonce: "1234567890abcdef1234567890abcdef", Kind: "job", TargetID: "job-a",
			OwnerDigest: OwnerDigest("main-owner"), MainId: "main-owner", OwnerLineage: "lineage-owner", State: "pending",
			Selector: WaitSelector{Kind: "job", TargetID: "job-a"}, Deadline: time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano),
		}
		rowPath := WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
		if err := writeV2Waiter(rowPath, row); err != nil {
			t.Fatal(err)
		}
		result := (&Store{Root: root, Prober: restartProber{deadPID: row.Pid}}).ResumeWait(context.Background(), waitID,
			Caller{Class: "MAIN", MainId: "main-foreign", OwnerLineage: "lineage-foreign", SessionId: "foreign"}, "foreign", 0, WaitOptions{})
		if result.ExitCode != 6 {
			t.Fatalf("foreign resume = %+v", result)
		}
		if _, err := os.Stat(WaiterPointerPath(root, waitID)); !os.IsNotExist(err) {
			t.Fatalf("foreign resume repaired pointer: %v", err)
		}
		stored, err := readV2Waiter(rowPath)
		if err != nil || stored.PointerRepaired {
			t.Fatalf("foreign row changed: %+v err=%v", stored, err)
		}
	})

	t.Run("explicit predecessor lineage is succeeded by main identifier", func(t *testing.T) {
		root := t.TempDir()
		now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		waitID := "00112233445566778899aabbccddeeff"
		row := Waiter{
			SchemaVersion: 2, WaitID: waitID, Nonce: "ffeeddccbbaa99887766554433221100", Kind: "job", TargetID: "job-a",
			OwnerDigest: OwnerDigest("main-old"), Pid: 91, MainId: "main-old", OwnerLineage: "mission-lineage", State: "pending",
			Selector: WaitSelector{Kind: "job", TargetID: "job-a"}, Target: WaiterTarget{Round: 1, StartedAt: "start"},
			RegisteredAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Deadline: now.Add(time.Hour).Format(time.RFC3339Nano),
			DeadlineBootID: "boot-w", BootDeadlineNanos: (2 * time.Hour).Nanoseconds(), RemainingNanos: time.Hour.Nanoseconds(),
		}
		rowPath := WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
		if err := writeV2Waiter(rowPath, row); err != nil {
			t.Fatal(err)
		}
		if err := writeV2Pointer(root, row, rowPath); err != nil {
			t.Fatal(err)
		}
		options := WaitOptions{
			SucceededMainID: "main-old", SucceededOwnerLineage: "mission-lineage",
			Now: func() time.Time { return now }, BootClock: func() (string, time.Duration, error) { return "boot-w", time.Hour, nil },
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				return "blocking", false, nil
			},
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Incarnation: row.Target, ExitCode: ExitGreen, Outcome: "completed", Evidence: "job:a"}, nil
			},
		}
		result := (&Store{Root: root, Prober: restartProber{deadPID: row.Pid}}).ResumeWait(context.Background(), waitID,
			Caller{Class: "MAIN", MainId: "main-new", OwnerLineage: "new-lineage", SessionId: "new"}, "new", 0, options)
		if result.ExitCode != ExitGreen {
			t.Fatalf("explicit-lineage succession = %+v", result)
		}
	})
}

// Issue #1 run-package pairing: a pair-bearing waiter stays LIVE under
// clock drift (registration reports busy, LiveWaiter true), and cleanup
// still removes its own record after drift instead of orphaning it.
func TestWaiterPairSurvivesDrift(t *testing.T) {
	root := t.TempDir()
	store := &Store{Root: root, Prober: pairProber{0}}
	owner := Caller{MainId: "main-5000-1-abc123", SessionId: "s"}
	target := WaiterTarget{Generation: 1, LaunchNonce: "n"}
	if err := store.RegisterWaiter("supervise", "demo", owner, target); err != nil {
		t.Fatalf("register: %v", err)
	}
	drifted := &Store{Root: root, Prober: pairProber{4}}
	if err := drifted.RegisterWaiter("supervise", "demo", owner, target); err == nil {
		t.Fatal("a live pair-bearing waiter must read busy under drift")
	}
	if !LiveWaiter(root, pairProber{4}, "supervise", "demo", owner.MainId, target) {
		t.Fatal("pair-bearing waiter read dead under drift")
	}
	drifted.RemoveWaiter("supervise", "demo", owner)
	if LiveWaiter(root, pairProber{4}, "supervise", "demo", owner.MainId, target) {
		t.Fatal("cleanup under drift orphaned the waiter record")
	}
}
