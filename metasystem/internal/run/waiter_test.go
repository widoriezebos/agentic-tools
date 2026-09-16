package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

func TestWatchSpeaksOnEveryConclusion(t *testing.T) {
	s := testStore(t)
	prober := fakeProber{verdicts: map[int64]identity.Liveness{}, starts: map[int64]int64{}}
	s.Prober = prober
	nonce := launchOne(t, s, "spoken")
	prober.verdicts[101] = identity.Alive
	prober.starts[101] = 5000
	if err := s.Bind("spoken", nonce, 101, 101); err != nil {
		t.Fatal(err)
	}
	record, _ := s.Read("spoken")
	if err := s.WriteSidecar("spoken", record.Generation, record.LaunchNonce, 0); err != nil {
		t.Fatal(err)
	}
	prober.verdicts[101] = identity.Dead
	if _, err := s.Assess("spoken"); err != nil {
		t.Fatal(err)
	}
	// The waiter proves its own owner's liveness from the MainId's
	// pid-start encoding.
	// The waiter records its OWN process identity through the store's
	// prober; the fake must recognize the live test process.
	self := int64(os.Getpid())
	prober.verdicts[self] = identity.Alive
	prober.starts[self] = 1
	owner := mainCaller
	var out strings.Builder
	rc := s.Watch("spoken", owner, 20*time.Millisecond, &out)
	if rc != ExitGreen {
		t.Fatalf("watch rc = %d (%s)", rc, out.String())
	}
	if !strings.Contains(out.String(), "run spoken green rc=0") {
		t.Fatalf("the conclusion was not spoken: %q", out.String())
	}
	var silent strings.Builder
	if rc := s.Watch("no-such-run", owner, 20*time.Millisecond, &silent); rc != ExitNoRecord || !strings.Contains(silent.String(), "no-record") {
		t.Fatalf("a missing record must still speak: rc=%d %q", rc, silent.String())
	}
}

type waitTestProber struct{ live bool }

func (p waitTestProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if !p.live {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(5000, 0), StartTicks: 77, BootID: "boot-test"}, identity.Alive, nil
}

type scriptedWaitHint struct {
	now       *time.Time
	boot      *time.Duration
	waitID    string
	nonce     string
	actions   []scriptedHintAction
	onTimeout func()
}

type scriptedHintAction struct {
	waitID      string
	nonce       string
	makeDurable func()
}

func (hint *scriptedWaitHint) Wait(_ context.Context, duration time.Duration) (bool, error) {
	for len(hint.actions) > 0 {
		action := hint.actions[0]
		hint.actions = hint.actions[1:]
		if action.waitID != hint.waitID || action.nonce != hint.nonce {
			continue
		}
		if action.makeDurable != nil {
			action.makeDurable()
		}
		return true, nil
	}
	*hint.now = hint.now.Add(duration)
	*hint.boot += duration
	if hint.onTimeout != nil {
		hint.onTimeout()
	}
	return false, nil
}

func (*scriptedWaitHint) Close() error { return nil }

func TestWaitHintsOnlyTriggerReads(t *testing.T) {
	run := func(t *testing.T, mode string) (WaitResult, []time.Time) {
		t.Helper()
		root := t.TempDir()
		now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
		boot := time.Hour
		durable := false
		deliveries := 0
		var readAt []time.Time
		options := WaitOptions{
			Now:       func() time.Time { return now },
			BootClock: func() (string, time.Duration, error) { return "boot-test", boot, nil },
			Sleep: func(_ context.Context, duration time.Duration) error {
				now = now.Add(duration)
				boot += duration
				if mode == "dropped" {
					durable = true
				}
				return nil
			},
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				deliveries++
				return "blocking", false, nil
			},
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				readAt = append(readAt, now)
				observation := SourceObservation{Pending: !durable, Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"}, Outcome: "running", Evidence: "run:r:g1"}
				if durable {
					observation.ExitCode, observation.Outcome, observation.Reason = ExitGreen, "green", "the durable record is green"
				}
				return observation, nil
			},
		}
		options.OpenHintReceiver = func(_ string, waitID, nonce string) (HintReceiver, error) {
			if mode == "dropped" {
				return nil, os.ErrNotExist
			}
			hint := &scriptedWaitHint{now: &now, boot: &boot, waitID: waitID, nonce: nonce, onTimeout: func() { durable = true }}
			switch mode {
			case "false-then-durable":
				hint.actions = []scriptedHintAction{
					// These matching notifications are queued as soon as the
					// receiver opens, before the registration is published.
					{waitID: waitID, nonce: nonce},
					{waitID: waitID, nonce: nonce},
					{waitID: waitID, nonce: nonce, makeDurable: func() { durable = true }},
				}
			case "hostile":
				hint.actions = []scriptedHintAction{{waitID: waitID, nonce: "wrong-nonce"}}
			}
			return hint, nil
		}
		result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(context.Background(), WaitRequest{
			Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "runtime-1", Timeout: time.Minute,
		}, options)
		row, _, err := LoadWaiterByID(root, result.WaitID)
		if err != nil {
			t.Fatal(err)
		}
		wantAccelerator := "fifo"
		if mode == "dropped" {
			wantAccelerator = "unavailable"
		}
		if row.Accelerator != wantAccelerator {
			t.Fatalf("accelerator = %q, want %q", row.Accelerator, wantAccelerator)
		}
		if deliveries != 1 {
			t.Fatalf("hint path caused %d wait deliveries, want one foreground delivery", deliveries)
		}
		return result, readAt
	}

	falseHintResult, falseHintReads := run(t, "false-then-durable")
	if falseHintResult.ExitCode != ExitGreen || len(falseHintReads) != 5 {
		t.Fatalf("false hint changed the outcome instead of triggering a read: result=%+v reads=%v", falseHintResult, falseHintReads)
	}
	if falseHintReads[2] != falseHintReads[1] || falseHintReads[3].Sub(falseHintReads[2]) < time.Second || falseHintReads[4].Sub(falseHintReads[3]) < time.Second {
		t.Fatalf("hint reads were not immediate then rate-limited: %v", falseHintReads)
	}

	hostileResult, hostileReads := run(t, "hostile")
	if hostileResult.ExitCode != ExitGreen || len(hostileReads) != 3 || hostileReads[2].Sub(hostileReads[1]) != 10*time.Second {
		t.Fatalf("wrong-nonce hint affected the durable result: result=%+v reads=%v", hostileResult, hostileReads)
	}

	droppedResult, droppedReads := run(t, "dropped")
	if droppedResult.ExitCode != ExitGreen || droppedResult.SourceEvidence != falseHintResult.SourceEvidence || len(droppedReads) != 3 || droppedReads[2].Sub(droppedReads[1]) != 10*time.Second {
		t.Fatalf("dropped hint changed more than latency: result=%+v reads=%v", droppedResult, droppedReads)
	}
}

func TestWaitFIFOHintDelivery(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	waitID := "0123456789abcdef0123456789abcdef"
	nonce := "abcdef0123456789abcdef0123456789"
	rowPath := WaiterPath(root, "job", "job-a", "owner-a")
	hintPath := waiterHintPath(rowPath, nonce)
	receiver, err := OpenFIFOHintReceiver(hintPath, waitID, nonce)
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	defer os.Remove(hintPath)
	row := Waiter{SchemaVersion: 2, WaitID: waitID, Nonce: nonce, Kind: "job", TargetID: "job-a", OwnerDigest: "owner-a", State: "pending", Accelerator: "fifo", HintPath: hintPath}
	if err := writeV2Waiter(rowPath, row); err != nil {
		t.Fatal(err)
	}
	forgedPath := WaiterPath(root, "job", "job-a", "owner-b")
	if err := writeV2Waiter(forgedPath, Waiter{SchemaVersion: 2, WaitID: waitID, Nonce: nonce, Kind: "job", TargetID: "job-a", OwnerDigest: "owner-b", State: "pending", Accelerator: "fifo", HintPath: hintPath}); err != nil {
		t.Fatal(err)
	}
	writer, err := unix.Open(hintPath, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unix.Write(writer, []byte(waitID+" wrong-nonce\n")); err != nil {
		t.Fatal(err)
	}
	_ = unix.Close(writer)
	delivery, err := NotifyWaiters(root, WaitHint{Kind: "job", TargetID: "job-a"})
	if err != nil || delivery.Matched != 2 || delivery.Delivered != 1 {
		t.Fatalf("delivery=%+v err=%v", delivery, err)
	}
	hinted, err := receiver.Wait(context.Background(), time.Second)
	if err != nil || !hinted {
		t.Fatalf("valid nonce did not survive the preceding forged hint: hinted=%t err=%v", hinted, err)
	}
	row.State = "ready"
	if err := writeV2Waiter(rowPath, row); err != nil {
		t.Fatal(err)
	}
	forged, err := readV2Waiter(forgedPath)
	if err != nil {
		t.Fatal(err)
	}
	forged.State = "ready"
	if err := writeV2Waiter(forgedPath, forged); err != nil {
		t.Fatal(err)
	}
	late, err := NotifyWaiters(root, WaitHint{Kind: "job", TargetID: "job-a"})
	if err != nil || late.Matched != 0 || late.Delivered != 0 {
		t.Fatalf("late hint reached a terminal registration: delivery=%+v err=%v", late, err)
	}
}

func TestSupersedeStaleSessionRow(t *testing.T) {
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	target := WaiterTarget{Generation: 1, LaunchNonce: "run-nonce"}
	options := WaitOptions{Now: func() time.Time { return now }, BootClock: func() (string, time.Duration, error) { return "boot-test", time.Hour, nil },
		Sleep: func(context.Context, time.Duration) error { t.Fatal("terminal registration slept"); return nil },
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
		Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
			return SourceObservation{Incarnation: target, ExitCode: ExitGreen, Outcome: "green"}, nil
		}}
	for _, test := range []struct {
		name, session, main string
		supersedes          bool
	}{{"stale session", "s0", mainCaller.MainId, true}, {"matching session", "s1", mainCaller.MainId, false}, {"different owner", "s0", "main-other", false}} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			old := Waiter{SchemaVersion: 2, WaitID: strings.Repeat("a", 32), Nonce: strings.Repeat("b", 32), Kind: "run", TargetID: "r", OwnerDigest: OwnerDigest(mainCaller.MainId), Pid: 91, Session: test.session, RuntimeSession: test.session, MainId: test.main, State: "pending"}
			path := WaiterPath(root, old.Kind, old.TargetID, old.OwnerDigest)
			if err := writeV2Waiter(path, old); err != nil {
				t.Fatal(err)
			}
			result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(context.Background(), WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "s1", Timeout: time.Hour}, options)
			stored, err := readV2Waiter(path)
			if err != nil || test.supersedes != (result.ExitCode == ExitGreen) || (test.supersedes && (stored.Session != "s1" || stored.WaitID == old.WaitID)) || (!test.supersedes && stored.WaitID != old.WaitID) {
				t.Fatalf("supersedes=%t result=%+v row=%+v err=%v", test.supersedes, result, stored, err)
			}
		})
	}
}

func TestSupersedeVersusTerminalizeRace(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	old := Waiter{SchemaVersion: 2, WaitID: strings.Repeat("c", 32), Nonce: strings.Repeat("d", 32), Kind: "run", TargetID: "r", OwnerDigest: OwnerDigest(mainCaller.MainId), Pid: 91, Session: "s0", RuntimeSession: "s0", MainId: mainCaller.MainId, State: "pending", Deadline: now.Add(time.Hour).Format(time.RFC3339Nano)}
	path := WaiterPath(root, old.Kind, old.TargetID, old.OwnerDigest)
	if err := writeV2Waiter(path, old); err != nil {
		t.Fatal(err)
	}
	attempted, release := make(chan struct{}, 2), make(chan struct{})
	options := WaitOptions{Now: func() time.Time { return now }, BootClock: func() (string, time.Duration, error) { return "boot-test", time.Hour, nil }, Sleep: func(context.Context, time.Duration) error {
		select {
		case attempted <- struct{}{}:
		default:
		}
		<-release
		return nil
	},
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		}, Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
			return SourceObservation{Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"}, ExitCode: ExitGreen}, nil
		}}
	store := &Store{Root: root, Prober: waitTestProber{live: true}}
	registered, finished := make(chan WaitResult, 1), make(chan WaitResult, 1)
	if err := withWaiterLock(root, func() error {
		go func() {
			registered <- store.Wait(context.Background(), WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "s1", Timeout: time.Hour}, options)
		}()
		go func() {
			finished <- store.finishV2(context.Background(), path, old, options, ExitGreen, "ready", "old wait ended", "green", "old", "", "")
		}()
		<-attempted
		<-attempted
		close(release)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	newResult, oldResult := <-registered, <-finished
	stored, err := readV2Waiter(path)
	if err != nil || newResult.ExitCode != ExitGreen || stored.WaitID != newResult.WaitID || stored.State != "ready" || stored.Result == nil || stored.Session != "s1" {
		t.Fatalf("registration=%+v terminalization=%+v row=%+v err=%v", newResult, oldResult, stored, err)
	}
	if _, _, err := FindWaiterByID(root, old.WaitID); err == nil {
		t.Fatal("superseded wait remained observable as pending")
	}
}

func TestWaitRunTerminalsAndDeadline(t *testing.T) {
	terminalCases := []struct {
		name string
		code int
	}{{"green", ExitGreen}, {"red", ExitRed}, {"unknown", ExitEndedUnknown}, {"launch-failed", ExitLaunchFailed}}
	for _, test := range terminalCases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
			boot := 2 * time.Hour
			reads := 0
			options := WaitOptions{
				Now:       func() time.Time { return now },
				BootClock: func() (string, time.Duration, error) { return "boot-test", boot, nil },
				Sleep:     func(context.Context, time.Duration) error { t.Fatal("terminal wait slept"); return nil },
				Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
					return "blocking", false, nil
				},
				Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
					reads++
					observation := SourceObservation{Incarnation: WaiterTarget{Generation: 3, LaunchNonce: "0123456789abcdef0123456789abcdef"}, Evidence: "run:r:g3", Outcome: test.name}
					if reads == 1 {
						observation.Pending = true
						return observation, nil
					}
					observation.ExitCode, observation.Reason = test.code, "recorded terminal"
					return observation, nil
				},
			}
			store := &Store{Root: root, Prober: waitTestProber{live: true}}
			result := store.Wait(context.Background(), WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "runtime-1", Timeout: time.Hour}, options)
			if result.ExitCode != test.code || result.SchemaVersion != 2 || result.WaitID == "" {
				t.Fatalf("result = %+v", result)
			}
			row, path, err := LoadWaiterByID(root, result.WaitID)
			if err != nil || row.State != "ready" || row.Delivery != "blocking" || row.Accelerator != "unavailable" || row.Result == nil || filepath.Base(path) == "" {
				t.Fatalf("retained row = %+v path=%q err=%v", row, path, err)
			}
			pointer, err := os.ReadFile(WaiterPointerPath(root, result.WaitID))
			if err != nil || strings.TrimSpace(string(pointer)) != filepath.Base(path) {
				t.Fatalf("pointer = %q err=%v", pointer, err)
			}
			encoded, _ := json.Marshal(row)
			if !strings.Contains(string(encoded), `"schemaVersion":2`) {
				t.Fatalf("version-2 row missing: %s", encoded)
			}
		})
	}

	t.Run("deadline", func(t *testing.T) {
		root := t.TempDir()
		now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
		boot := 2 * time.Hour
		options := WaitOptions{
			Now:       func() time.Time { return now },
			BootClock: func() (string, time.Duration, error) { return "boot-test", boot, nil },
			Sleep: func(_ context.Context, duration time.Duration) error {
				now = now.Add(duration)
				boot += duration
				return nil
			},
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				return "blocking", false, nil
			},
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Pending: true, Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"}, Outcome: "running", Evidence: "run:r:g1"}, nil
			},
		}
		result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(context.Background(), WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "runtime-1", Timeout: 3 * time.Second}, options)
		if result.ExitCode != ExitWaitDeadline || result.Reason != "this wait reached its deadline" {
			t.Fatalf("deadline result = %+v", result)
		}
	})

	t.Run("same-key-live-refusal", func(t *testing.T) {
		root := t.TempDir()
		now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
		enteredSleep := make(chan struct{}, 1)
		release := make(chan struct{})
		options := WaitOptions{
			Now:       func() time.Time { return now },
			BootClock: func() (string, time.Duration, error) { return "boot-test", time.Hour, nil },
			Sleep: func(context.Context, time.Duration) error {
				select {
				case enteredSleep <- struct{}{}:
				default:
				}
				<-release
				return context.Canceled
			},
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				return "blocking", false, nil
			},
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Pending: true, Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"}, Outcome: "running", Evidence: "run:r:g1"}, nil
			},
		}
		store := &Store{Root: root, Prober: waitTestProber{live: true}}
		request := WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "runtime-1", Timeout: time.Hour}
		firstDone := make(chan WaitResult, 1)
		go func() { firstDone <- store.Wait(context.Background(), request, options) }()
		<-enteredSleep
		second := store.Wait(context.Background(), request, options)
		if second.ExitCode != ExitWaiterBusy || !strings.Contains(second.Reason, "live waiter") {
			t.Fatalf("same-key result = %+v", second)
		}
		close(release)
		if first := <-firstDone; first.ExitCode != ExitInterrupted {
			t.Fatalf("first waiter cleanup result = %+v", first)
		}
	})
}

func TestWaitPollFailureIsRecordedWithoutEndingTheWait(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	boot := time.Hour
	reads := 0
	checkedPending := false
	owner := mainCaller
	options := WaitOptions{
		Now:       func() time.Time { return now },
		BootClock: func() (string, time.Duration, error) { return "boot-test", boot, nil },
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
		Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
			reads++
			observation := SourceObservation{
				Pending: true, Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"},
				Outcome: "running", Evidence: "run:r:g1", PollAt: now.UTC().Format(time.RFC3339Nano),
			}
			if reads < 3 {
				observation.PollError = "provider temporarily unavailable"
				return observation, nil
			}
			observation.Pending = false
			observation.ExitCode = ExitGreen
			observation.Outcome = "green"
			observation.Reason = "the durable source recorded the answer"
			return observation, nil
		},
		Sleep: func(context.Context, time.Duration) error {
			rows, failures := PendingWaiters(root, owner.OwnerLineage)
			if len(failures) != 0 || len(rows) != 1 || rows[0].LastPollError != "provider temporarily unavailable" || rows[0].LastPollAt == "" {
				t.Fatalf("provider outage was not retained on the pending row: rows=%+v failures=%v", rows, failures)
			}
			checkedPending = true
			now = now.Add(10 * time.Second)
			boot += 10 * time.Second
			return nil
		},
	}
	result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(context.Background(), WaitRequest{
		Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: owner, RuntimeSession: "runtime-1", Timeout: time.Minute,
	}, options)
	if result.ExitCode != ExitGreen || !checkedPending {
		t.Fatalf("provider outage ended or bypassed the durable wait: result=%+v checkedPending=%t", result, checkedPending)
	}
	row, _, err := LoadWaiterByID(root, result.WaitID)
	if err != nil || row.LastPollAt == "" || row.LastPollError != "" {
		t.Fatalf("later successful poll metadata was not retained with the result: row=%+v err=%v", row, err)
	}
}

func TestWaitLockClockAndFetchBounds(t *testing.T) {
	t.Run("freshness", func(t *testing.T) {
		root := t.TempDir()
		start := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
		now := start
		boot := time.Hour
		reads := 0
		options := WaitOptions{
			Now:       func() time.Time { return now },
			BootClock: func() (string, time.Duration, error) { return "boot-test", boot, nil },
			Sleep: func(_ context.Context, duration time.Duration) error {
				now = now.Add(duration)
				boot += duration
				return nil
			},
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				return "blocking", false, nil
			},
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				reads++
				return SourceObservation{Pending: true, Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"}, Outcome: "running", Evidence: "run:r:g1", Temporary: reads > 1}, nil
			},
		}
		result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(context.Background(), WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "runtime-1", Timeout: time.Minute}, options)
		if result.ExitCode != ExitWaiterIO || now.Sub(start) > 40*time.Second {
			t.Fatalf("transient failure bound result=%+v elapsed=%s", result, now.Sub(start))
		}
	})

	t.Run("lock", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(WaitersDir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		lock, err := os.OpenFile(filepath.Join(WaitersDir(root), ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			t.Fatal(err)
		}
		defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		start := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
		now := start
		boot := time.Hour
		delivered := false
		options := WaitOptions{
			Now:       func() time.Time { return now },
			BootClock: func() (string, time.Duration, error) { return "boot-test", boot, nil },
			Sleep: func(_ context.Context, duration time.Duration) error {
				now = now.Add(duration)
				boot += duration
				return nil
			},
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				delivered = true
				return "blocking", false, nil
			},
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Pending: true, Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"}}, nil
			},
		}
		result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(context.Background(), WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "runtime-1", Timeout: time.Hour}, options)
		if result.ExitCode != ExitWaiterIO || delivered || now.Sub(start) > 5*time.Second+20*time.Millisecond {
			t.Fatalf("lock bound result=%+v delivered=%t elapsed=%s", result, delivered, now.Sub(start))
		}
	})

	t.Run("wall-clock-regression", func(t *testing.T) {
		root := t.TempDir()
		start := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
		now := start
		boot := time.Hour
		options := WaitOptions{
			Now:       func() time.Time { return now },
			BootClock: func() (string, time.Duration, error) { return "boot-test", boot, nil },
			Sleep: func(_ context.Context, duration time.Duration) error {
				now = now.Add(-time.Minute)
				boot += duration
				return nil
			},
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				return "blocking", false, nil
			},
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Pending: true, Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"}}, nil
			},
		}
		result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(context.Background(), WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "runtime-1", Timeout: time.Hour}, options)
		if result.ExitCode != ExitWaiterIO || result.SourceOutcome != "clock-drift" {
			t.Fatalf("wall regression result=%+v", result)
		}
	})
}

func TestWaitRegistrationAndTypedFailureExits(t *testing.T) {
	baseOptions := func() WaitOptions {
		return WaitOptions{
			BootClock: func() (string, time.Duration, error) { return "boot-test", time.Hour, nil },
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Pending: true, Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"}, Outcome: "running", Evidence: "run:r:g1"}, nil
			},
			Sleep: func(context.Context, time.Duration) error { return context.Canceled },
		}
	}
	request := WaitRequest{Selector: WaitSelector{Kind: "run", TargetID: "r"}, Owner: mainCaller, RuntimeSession: "runtime-1", Timeout: time.Hour}

	t.Run("adapter failure leaves a visible registering row and returns 65", func(t *testing.T) {
		root := t.TempDir()
		options := baseOptions()
		options.Deliver = func(ctx context.Context, waitID, _ string, _ time.Time, _ string) (string, bool, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 10*time.Second+time.Second {
				t.Fatalf("adapter delivery was not capped at ten seconds: %v", deadline)
			}
			row, _, err := LoadWaiterByID(root, waitID)
			if err != nil || row.State != "registering" {
				t.Fatalf("pre-delivery row = %+v err=%v", row, err)
			}
			return "", false, context.DeadlineExceeded
		}
		result := (&Store{Root: root, Prober: waitTestProber{live: true}}).Wait(context.Background(), request, options)
		if result.ExitCode != ExitWaiterIO {
			t.Fatalf("adapter failure = %+v", result)
		}
		row, _, err := LoadWaiterByID(root, result.WaitID)
		if err != nil || row.State != "failed" {
			t.Fatalf("failed registration row = %+v err=%v", row, err)
		}
	})

	t.Run("action infrastructure failure is 65", func(t *testing.T) {
		options := baseOptions()
		options.Deliver = func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		}
		options.Actionable = func(context.Context, Waiter) (string, bool, error) { return "", false, os.ErrNotExist }
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(context.Background(), request, options)
		if result.ExitCode != ExitWaiterIO {
			t.Fatalf("infrastructure failure = %+v", result)
		}
	})

	t.Run("only an actual change is 6", func(t *testing.T) {
		options := baseOptions()
		options.Deliver = func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		}
		options.Actionable = func(context.Context, Waiter) (string, bool, error) { return "new goal became claimable", true, nil }
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(context.Background(), request, options)
		if result.ExitCode != 6 {
			t.Fatalf("actionable change = %+v", result)
		}
	})

	t.Run("a matched event wins over a frontier change in the same cycle", func(t *testing.T) {
		options := baseOptions()
		options.Deliver = func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		}
		calls := 0
		options.Observe = func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
			calls++
			observation := SourceObservation{Incarnation: WaiterTarget{Generation: 1, LaunchNonce: "n"}, ClaimableRead: true, Evidence: "run:r:g1"}
			if calls == 1 {
				observation.Pending, observation.Outcome = true, "running"
				return observation, nil
			}
			observation.Pending, observation.ExitCode, observation.Reason, observation.Outcome = false, 0, "the run ended green", "green"
			observation.ClaimableGoals = []ClaimableGoal{{ID: "fresh", Revision: 1}}
			return observation, nil
		}
		options.Sleep = func(context.Context, time.Duration) error { return nil }
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(context.Background(), request, options)
		if result.ExitCode != 0 || result.SourceOutcome != "green" {
			t.Fatalf("a frontier change outranked the matched event: %+v", result)
		}
	})

	t.Run("interrupt wins an action-check race", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		options := baseOptions()
		options.Deliver = func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		}
		options.Actionable = func(context.Context, Waiter) (string, bool, error) { cancel(); return "", false, os.ErrNotExist }
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(ctx, request, options)
		if result.ExitCode != ExitInterrupted {
			t.Fatalf("interrupt race = %+v", result)
		}
	})

	t.Run("wait deadline during action check is 124", func(t *testing.T) {
		start := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
		now := start
		boot := time.Hour
		options := baseOptions()
		options.Now = func() time.Time { return now }
		options.BootClock = func() (string, time.Duration, error) { return "boot-test", boot, nil }
		options.Deliver = func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		}
		options.Actionable = func(ctx context.Context, _ Waiter) (string, bool, error) {
			if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 10*time.Second+time.Second {
				t.Fatalf("actionable check did not have the ten-second ceiling: %v", deadline)
			}
			now = start.Add(5 * time.Second)
			boot += 5 * time.Second
			return "", false, context.DeadlineExceeded
		}
		shortRequest := request
		shortRequest.Timeout = 5 * time.Second
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(context.Background(), shortRequest, options)
		if result.ExitCode != ExitWaitDeadline || result.SourceOutcome != "wait-deadline" {
			t.Fatalf("action deadline = %+v", result)
		}
	})
}

func TestWaitSavedResultRenewalClasses(t *testing.T) {
	now := time.Date(2026, 9, 13, 14, 0, 0, 0, time.UTC)
	target := WaiterTarget{Generation: 3, LaunchNonce: "run-nonce"}
	owner := Caller{Class: "MAIN", MainId: "main-renew", OwnerLineage: "lineage-renew", SessionId: "session-renew"}
	selector := WaitSelector{Kind: "run", TargetID: "run-renew"}
	writeSaved := func(t *testing.T, code int) (string, *Store, string) {
		t.Helper()
		root := t.TempDir()
		waitID := "0123456789abcdef0123456789abcdef"
		row := Waiter{
			SchemaVersion: 2, WaitID: waitID, Nonce: "fedcba9876543210fedcba9876543210", Kind: selector.Kind, TargetID: selector.TargetID,
			OwnerDigest: OwnerDigest(owner.MainId), Pid: 91, PidStartedAt: 5000, PidStartTicks: 77, BootID: "boot-old",
			Session: owner.SessionId, MainId: owner.MainId, OwnerLineage: owner.OwnerLineage, RuntimeSession: owner.SessionId,
			Selector: selector, Target: target, RegisteredAt: now.Add(-time.Hour).Format(time.RFC3339Nano), Deadline: now.Add(time.Hour).Format(time.RFC3339Nano),
			DeadlineBootID: "boot-old", BootDeadlineNanos: (2 * time.Hour).Nanoseconds(), State: "ready", Delivery: "blocking",
		}
		row.Result = &WaitResult{SchemaVersion: 2, WaitID: waitID, Selector: selector, TargetIncarnation: target, ExitCode: code,
			Reason: "saved result", SourceOutcome: "saved", RegisteredAt: row.RegisteredAt, Deadline: row.Deadline, ReturnedAt: now.Add(-time.Minute).Format(time.RFC3339Nano)}
		path := WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
		if err := writeV2Waiter(path, row); err != nil {
			t.Fatal(err)
		}
		if err := writeV2Pointer(root, row, path); err != nil {
			t.Fatal(err)
		}
		return root, &Store{Root: root, Prober: waitTestProber{live: true}}, row.Nonce
	}
	options := func(deliveries *int) WaitOptions {
		return WaitOptions{
			Now:       func() time.Time { return now },
			BootClock: func() (string, time.Duration, error) { return "boot-test", time.Hour, nil },
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				*deliveries++
				return "blocking", false, nil
			},
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Incarnation: target, ExitCode: ExitGreen, Reason: "renewed source completed", Outcome: "green", Evidence: "run:renewed"}, nil
			},
		}
	}

	for _, code := range []int{ExitGreen, ExitRed, ExitEndedUnknown, ExitLaunchFailed, ExitNoRecord} {
		t.Run("source-terminal-"+strconv.Itoa(code), func(t *testing.T) {
			root, store, nonce := writeSaved(t, code)
			deliveries := 0
			plain := store.ResumeWait(context.Background(), "0123456789abcdef0123456789abcdef", owner, owner.SessionId, 0, options(&deliveries))
			explicit := store.ResumeWait(context.Background(), plain.WaitID, owner, owner.SessionId, time.Hour, options(&deliveries))
			row, _, err := LoadWaiterByID(root, plain.WaitID)
			if plain.ExitCode != code || explicit.ExitCode != code || deliveries != 0 || err != nil || row.Nonce != nonce || len(row.Renewals) != 0 {
				t.Fatalf("terminal code %d renewed: plain=%+v explicit=%+v deliveries=%d row=%+v err=%v", code, plain, explicit, deliveries, row, err)
			}
		})
	}
	t.Run("a renewal of a ledger wait reads from the floor kept before a matched event", func(t *testing.T) {
		// A registration that matched its event and then ended renewable
		// keeps the floor before the event as its last checked tip; the
		// renewal reads from there and finds the event again.
		root := t.TempDir()
		waitID := "abcdef0123456789abcdef0123456789"
		goalSelector := WaitSelector{Kind: "goal", TargetID: "goal-renew", Event: "human-act", After: "cursor-a"}
		row := Waiter{
			SchemaVersion: 2, WaitID: waitID, Nonce: "0123456789abcdef0123456789abcdef", Kind: goalSelector.Kind, TargetID: goalSelector.TargetID,
			OwnerDigest: OwnerDigest(owner.MainId), Pid: 91, PidStartedAt: 5000, PidStartTicks: 77, BootID: "boot-old",
			Session: owner.SessionId, MainId: owner.MainId, OwnerLineage: owner.OwnerLineage, RuntimeSession: owner.SessionId,
			Selector: goalSelector, Target: WaiterTarget{OperationID: "goal-renew"}, OriginalCursor: "cursor-a", LastCheckedTip: "cursor-a", RegisteredAt: now.Add(-time.Hour).Format(time.RFC3339Nano),
			Deadline: now.Add(time.Hour).Format(time.RFC3339Nano), DeadlineBootID: "boot-old", BootDeadlineNanos: (2 * time.Hour).Nanoseconds(), State: "ready", Delivery: "blocking",
		}
		row.Result = &WaitResult{SchemaVersion: 2, WaitID: waitID, Selector: goalSelector, ExitCode: ExitWaiterIO, Reason: "delivery failed after the event was found",
			SourceOutcome: "transport-failure", RegisteredAt: row.RegisteredAt, Deadline: row.Deadline, ReturnedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), LedgerTip: "commit-event"}
		path := WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
		if err := writeV2Waiter(path, row); err != nil {
			t.Fatal(err)
		}
		if err := writeV2Pointer(root, row, path); err != nil {
			t.Fatal(err)
		}
		deliveries := 0
		renewOptions := options(&deliveries)
		var floors []string
		renewOptions.Observe = func(_ context.Context, _ WaitSelector, _ WaiterTarget, floor string) (SourceObservation, error) {
			floors = append(floors, floor)
			if floor == "cursor-a" {
				return SourceObservation{Incarnation: WaiterTarget{OperationID: "goal-renew"}, ExitCode: ExitGreen, Reason: "the act was recorded", Outcome: "human-act", Evidence: "goal:renew", LedgerTip: "commit-event"}, nil
			}
			return SourceObservation{Incarnation: WaiterTarget{OperationID: "goal-renew"}, Pending: true, LedgerTip: floor}, nil
		}
		store := &Store{Root: root, Prober: waitTestProber{live: true}}
		result := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, time.Hour, renewOptions)
		if result.ExitCode != ExitGreen || len(floors) == 0 || floors[0] != "cursor-a" {
			t.Fatalf("the renewal did not read from the kept floor: floors=%v result=%+v", floors, result)
		}
	})

	t.Run("a renewal of a ledger wait reads from the last checked tip, not the original cursor", func(t *testing.T) {
		// A wait that advanced past many accepted changes before its
		// deadline renews with one read from where it stopped, so a
		// renewal's cost grows with the changes since the last check and
		// not with the age of the wait.
		root := t.TempDir()
		waitID := "fedcba9876543210fedcba9876543210"
		goalSelector := WaitSelector{Kind: "goal", TargetID: "goal-tip", GoalID: "goal-tip", Event: "human-act", After: "cursor-a"}
		row := Waiter{
			SchemaVersion: 2, WaitID: waitID, Nonce: "0123456789abcdef0123456789abcdef", Kind: goalSelector.Kind, TargetID: goalSelector.TargetID,
			OwnerDigest: OwnerDigest(owner.MainId), Pid: 91, PidStartedAt: 5000, PidStartTicks: 77, BootID: "boot-old",
			Session: owner.SessionId, MainId: owner.MainId, OwnerLineage: owner.OwnerLineage, RuntimeSession: owner.SessionId,
			Selector: goalSelector, Target: WaiterTarget{OperationID: "goal-tip"}, OriginalCursor: "cursor-a", LastCheckedTip: "commit-40", RegisteredAt: now.Add(-time.Hour).Format(time.RFC3339Nano),
			Deadline: now.Add(time.Hour).Format(time.RFC3339Nano), DeadlineBootID: "boot-old", BootDeadlineNanos: (2 * time.Hour).Nanoseconds(), State: "deadline", Delivery: "blocking",
		}
		row.Result = &WaitResult{SchemaVersion: 2, WaitID: waitID, Selector: goalSelector, ExitCode: ExitWaitDeadline, Reason: "this wait reached its deadline",
			SourceOutcome: "wait-deadline", RegisteredAt: row.RegisteredAt, Deadline: row.Deadline, ReturnedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), LedgerTip: "commit-40"}
		path := WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
		if err := writeV2Waiter(path, row); err != nil {
			t.Fatal(err)
		}
		if err := writeV2Pointer(root, row, path); err != nil {
			t.Fatal(err)
		}
		deliveries := 0
		renewOptions := options(&deliveries)
		var floors []string
		renewOptions.Observe = func(_ context.Context, _ WaitSelector, _ WaiterTarget, floor string) (SourceObservation, error) {
			floors = append(floors, floor)
			if floor == "commit-40" {
				return SourceObservation{Incarnation: WaiterTarget{OperationID: "goal-tip"}, ExitCode: ExitGreen, Reason: "the act was recorded", Outcome: "human-act", Evidence: "goal:tip", LedgerTip: "commit-41"}, nil
			}
			return SourceObservation{Incarnation: WaiterTarget{OperationID: "goal-tip"}, Pending: true, LedgerTip: floor}, nil
		}
		store := &Store{Root: root, Prober: waitTestProber{live: true}}
		result := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, time.Hour, renewOptions)
		if result.ExitCode != ExitGreen || len(floors) != 1 || floors[0] != "commit-40" {
			t.Fatalf("the renewal did not read once from the last checked tip: floors=%v result=%+v", floors, result)
		}
	})

	t.Run("a registration that matched and then failed keeps the floor before the event", func(t *testing.T) {
		root := t.TempDir()
		goalSelector := WaitSelector{Kind: "goal", TargetID: "goal-keep", GoalID: "goal-keep", Event: "human-act", After: "cursor-a"}
		renewOptions := options(new(int))
		renewOptions.Observe = func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
			return SourceObservation{Incarnation: WaiterTarget{OperationID: "goal-keep"}, ExitCode: ExitGreen, Reason: "the act was recorded", Outcome: "human-act", Evidence: "goal:keep", LedgerTip: "commit-event"}, nil
		}
		renewOptions.Deliver = func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "", false, context.DeadlineExceeded
		}
		store := &Store{Root: root, Prober: waitTestProber{live: true}}
		result := store.Wait(context.Background(), WaitRequest{Selector: goalSelector, Owner: owner, RuntimeSession: owner.SessionId, Timeout: time.Hour}, renewOptions)
		row, _, err := LoadWaiterByID(root, result.WaitID)
		if result.ExitCode != ExitWaiterIO || err != nil || row.LastCheckedTip != "cursor-a" {
			t.Fatalf("a failed registration checked the matched event off: result=%+v row=%+v err=%v", result, row, err)
		}
	})

	for _, code := range []int{ExitWaitDeadline, ExitInterrupted, 6, ExitWaiterIO} {
		t.Run("renewable-"+strconv.Itoa(code), func(t *testing.T) {
			root, store, nonce := writeSaved(t, code)
			deliveries := 0
			plain := store.ResumeWait(context.Background(), "0123456789abcdef0123456789abcdef", owner, owner.SessionId, 0, options(&deliveries))
			if plain.ExitCode != code || deliveries != 0 {
				t.Fatalf("plain replay code %d = %+v deliveries=%d", code, plain, deliveries)
			}
			renewed := store.ResumeWait(context.Background(), plain.WaitID, owner, owner.SessionId, time.Hour, options(&deliveries))
			row, _, err := LoadWaiterByID(root, plain.WaitID)
			if renewed.ExitCode != ExitGreen || deliveries != 1 || err != nil || row.Nonce == nonce || len(row.Renewals) != 1 || row.Renewals[0].Result.ExitCode != code || row.Renewals[0].RenewedAt != now.Format(time.RFC3339Nano) {
				t.Fatalf("renewable code %d: renewed=%+v deliveries=%d row=%+v err=%v", code, renewed, deliveries, row, err)
			}
		})
	}
	t.Run("changed boot requires an explicit renewal", func(t *testing.T) {
		root := t.TempDir()
		waitID := "abcdefabcdefabcdefabcdefabcdefab"
		row := Waiter{
			SchemaVersion: 2, WaitID: waitID, Nonce: "1234567890abcdef1234567890abcdef", Kind: selector.Kind, TargetID: selector.TargetID,
			OwnerDigest: OwnerDigest(owner.MainId), Pid: 91, PidStartedAt: 5000, PidStartTicks: 77, BootID: "boot-old",
			Session: owner.SessionId, MainId: owner.MainId, OwnerLineage: owner.OwnerLineage, RuntimeSession: owner.SessionId,
			Selector: selector, Target: target, RegisteredAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Deadline: now.Add(time.Hour).Format(time.RFC3339Nano),
			DeadlineBootID: "boot-old", BootDeadlineNanos: (2 * time.Hour).Nanoseconds(), RemainingNanos: time.Hour.Nanoseconds(), State: "pending", Delivery: "blocking",
		}
		path := WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
		if err := writeV2Waiter(path, row); err != nil {
			t.Fatal(err)
		}
		if err := writeV2Pointer(root, row, path); err != nil {
			t.Fatal(err)
		}
		deliveries := 0
		renewOptions := options(&deliveries)
		renewOptions.BootClock = func() (string, time.Duration, error) { return "boot-new", time.Minute, nil }
		store := &Store{Root: root, Prober: restartProber{deadPID: row.Pid}}
		plain := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, 0, renewOptions)
		if plain.ExitCode != ExitWaitDeadline || deliveries != 0 {
			t.Fatalf("plain changed-boot resume = %+v deliveries=%d", plain, deliveries)
		}
		renewed := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, time.Hour, renewOptions)
		stored, _, err := LoadWaiterByID(root, waitID)
		if renewed.ExitCode != ExitGreen || deliveries != 1 || err != nil || len(stored.Renewals) != 1 || stored.Renewals[0].Result.ExitCode != ExitWaitDeadline {
			t.Fatalf("changed-boot renewal = %+v deliveries=%d row=%+v err=%v", renewed, deliveries, stored, err)
		}
	})
}

func TestWaitSavedResultReplayEvidence(t *testing.T) {
	now := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	owner := Caller{Class: "MAIN", MainId: "main-replay", OwnerLineage: "lineage-replay", SessionId: "session-replay"}
	target := WaiterTarget{StartedAt: "ledger:cursor", ProofDigest: "endpoint-proof"}
	original := strings.Repeat("0", 40)
	event := strings.Repeat("a", 40)
	current := strings.Repeat("b", 40)
	selector := WaitSelector{Kind: "goal", TargetID: "goal-replay", GoalID: "goal-replay", Event: "human-act", After: original}

	writeSaved := func(t *testing.T, code int, evidence, lastChecked string) (*Store, string) {
		t.Helper()
		root := t.TempDir()
		waitID := "abcdefabcdefabcdefabcdefabcdefab"
		row := Waiter{
			SchemaVersion: 2, WaitID: waitID, Nonce: "0123456789abcdef0123456789abcdef", Kind: selector.Kind, TargetID: selector.TargetID,
			OwnerDigest: OwnerDigest(owner.MainId), Pid: 91, PidStartedAt: 5000, PidStartTicks: 77, BootID: "boot-old",
			Session: owner.SessionId, MainId: owner.MainId, OwnerLineage: owner.OwnerLineage, RuntimeSession: owner.SessionId,
			Selector: selector, Target: target, OriginalCursor: original, LastCheckedTip: lastChecked,
			RegisteredAt: now.Add(-time.Hour).Format(time.RFC3339Nano), Deadline: now.Add(time.Hour).Format(time.RFC3339Nano),
			DeadlineBootID: "boot-old", BootDeadlineNanos: (2 * time.Hour).Nanoseconds(), State: "ready", Delivery: "blocking",
		}
		row.Result = &WaitResult{
			SchemaVersion: 2, WaitID: waitID, Selector: selector, TargetIncarnation: target, ExitCode: code,
			Reason: "saved result", SourceOutcome: "answer", SourceEvidence: evidence, RegisteredAt: row.RegisteredAt,
			Deadline: row.Deadline, ReturnedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), LedgerTip: event,
		}
		path := WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest)
		if err := writeV2Waiter(path, row); err != nil {
			t.Fatal(err)
		}
		if err := writeV2Pointer(root, row, path); err != nil {
			t.Fatal(err)
		}
		return &Store{Root: root, Prober: waitTestProber{live: true}}, waitID
	}

	t.Run("replay starts at the saved event floor", func(t *testing.T) {
		store, waitID := writeSaved(t, ExitGreen, "ledger:"+event+":operation:answer-op", event)
		floors := []string{}
		deliveries := 0
		options := WaitOptions{
			Now: func() time.Time { return now },
			Observe: func(_ context.Context, _ WaitSelector, _ WaiterTarget, floor string) (SourceObservation, error) {
				floors = append(floors, floor)
				return SourceObservation{Pending: true, Incarnation: target, Outcome: "pending", Evidence: "ledger:" + current, LedgerTip: current}, nil
			},
			Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
				deliveries++
				return "blocking", false, nil
			},
		}
		plain := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, 0, options)
		explicit := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, time.Hour, options)
		if plain.ExitCode != ExitGreen || explicit.ExitCode != ExitGreen || deliveries != 0 || len(floors) != 2 || floors[0] != event || floors[1] != event {
			t.Fatalf("aged replay plain=%+v explicit=%+v deliveries=%d floors=%v", plain, explicit, deliveries, floors)
		}
	})

	t.Run("an earlier version-2 row can use its saved result tip", func(t *testing.T) {
		store, waitID := writeSaved(t, ExitGreen, "ledger:"+event+":operation:answer-op", "")
		floor := ""
		result := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, 0, WaitOptions{
			Now: func() time.Time { return now },
			Observe: func(_ context.Context, _ WaitSelector, _ WaiterTarget, observedFloor string) (SourceObservation, error) {
				floor = observedFloor
				return SourceObservation{Pending: true, Incarnation: target, Outcome: "pending", Evidence: "ledger:" + current, LedgerTip: current}, nil
			},
		})
		if result.ExitCode != ExitGreen || floor != event {
			t.Fatalf("older row replay=%+v floor=%q", result, floor)
		}
	})

	t.Run("removed evidence is invalid", func(t *testing.T) {
		store, waitID := writeSaved(t, ExitGreen, "ledger:"+event+":operation:answer-op", event)
		result := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, 0, WaitOptions{
			Now: func() time.Time { return now },
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Incarnation: target, ExitCode: ExitNoRecord, Outcome: "invalid-source", Reason: "the accepted branch no longer contains the act"}, nil
			},
		})
		if result.ExitCode != ExitNoRecord || result.SourceOutcome != "invalid-source" {
			t.Fatalf("removed evidence replay=%+v", result)
		}
	})

	t.Run("a saved actionable exit replays from a pending observation", func(t *testing.T) {
		store, waitID := writeSaved(t, 6, "ledger:"+event, event)
		result := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, 0, WaitOptions{
			Now: func() time.Time { return now },
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Pending: true, Incarnation: target, Outcome: "pending", Evidence: "ledger:" + current, LedgerTip: current}, nil
			},
		})
		if result.ExitCode != 6 || result.Reason != "saved result" {
			t.Fatalf("saved actionable replay=%+v", result)
		}
	})

	t.Run("a replaced incarnation remains invalid", func(t *testing.T) {
		store, waitID := writeSaved(t, ExitGreen, "ledger:"+event+":operation:answer-op", event)
		result := store.ResumeWait(context.Background(), waitID, owner, owner.SessionId, 0, WaitOptions{
			Now: func() time.Time { return now },
			Observe: func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
				return SourceObservation{Pending: true, Incarnation: WaiterTarget{StartedAt: "ledger:other", ProofDigest: "other-endpoint"}}, nil
			},
		})
		if result.ExitCode != ExitNoRecord {
			t.Fatalf("replaced source replay=%+v", result)
		}
	})
}

func TestInterruptWaiterRow(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	path := WaiterPath(root, "run", "interrupt-me", "owner")
	row := Waiter{SchemaVersion: 2, WaitID: strings.Repeat("a", 32), Nonce: strings.Repeat("b", 32), Kind: "run",
		State: WaiterStatePending, LastCheckedTip: "tip-7", Deadline: now.Add(time.Hour).Format(time.RFC3339Nano)}
	if err := writeV2Waiter(path, row); err != nil {
		t.Fatal(err)
	}
	store := &Store{Root: root}
	options := WaitOptions{Now: func() time.Time { return now }}
	got, err := store.InterruptWaiterRow(path, "handoff 0123456789abcdef", options)
	if err != nil || got.State != WaiterStateInterrupted || got.InterruptedBy != "handoff 0123456789abcdef" || got.Result == nil ||
		got.Result.ExitCode != ExitInterrupted || got.Result.Reason != "wait command was interrupted" || got.Result.SourceOutcome != "interrupted" {
		t.Fatalf("interrupt transition: row=%+v result=%+v err=%v", got, got.Result, err)
	}
	if _, err := store.InterruptWaiterRow(path, "again", WaitOptions{}); err == nil || err.Error() != fmt.Sprintf("waiter row %s is already ended (state %q)", path, WaiterStateInterrupted) {
		t.Fatalf("second interrupt error = %v", err)
	}
	store.finishV2(context.Background(), path, got, options, ExitWaitDeadline, WaiterStateDeadline, "deadline", "wait-deadline", "", got.LastCheckedTip, "")
	if preserved, err := readV2Waiter(path); err != nil || preserved.InterruptedBy != "handoff 0123456789abcdef" {
		t.Fatalf("ordinary terminal transition cleared interruptedBy: %q, err=%v", preserved.InterruptedBy, err)
	}
}
