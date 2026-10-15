package proofrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// Install the observer once before tests run; each counter belongs to one
// private fixture path. Parallel tests never replace the opener.
var attemptOpenCounters sync.Map

func init() {
	readAttemptRecord = func(path string) ([]byte, error) {
		if value, ok := attemptOpenCounters.Load(path); ok {
			value.(*atomic.Int64).Add(1)
		}
		return os.ReadFile(path)
	}
	openAttemptRecord = func(path string) (*os.File, error) {
		if value, ok := attemptOpenCounters.Load(path); ok {
			value.(*atomic.Int64).Add(1)
		}
		return os.Open(path)
	}
}
func countAttemptOpens(t *testing.T, root, id string) *atomic.Int64 {
	t.Helper()
	path, err := AttemptPath(root, id)
	if err != nil {
		t.Fatal(err)
	}
	counter := new(atomic.Int64)
	attemptOpenCounters.Store(path, counter)
	t.Cleanup(func() { attemptOpenCounters.Delete(path) })
	return counter
}
func saveAttemptShape(t *testing.T, root string, attempt Attempt, suffix string) {
	t.Helper()
	data, err := json.Marshal(attempt)
	if err != nil {
		t.Fatal(err)
	}
	path, err := AttemptPath(root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, suffix...), 0600); err != nil {
		t.Fatal(err)
	}
}
func requireSettled(t *testing.T, root, id string, want bool) {
	t.Helper()
	_, err := os.Stat(settledPath(root, id))
	if (err == nil) != want || err != nil && !os.IsNotExist(err) {
		t.Fatalf("settled %s = %v, want %v", id, err, want)
	}
}
func terminalShape(a Attempt, now time.Time) Attempt {
	a.EndedAt = now.Add(time.Minute).Format(time.RFC3339Nano)
	a.ObservedMinutes = 1
	a.Terminal = &AttemptTerminal{Result: TerminalSuccess, At: a.EndedAt}
	return a
}

func TestAttemptHeadStopsBeforeEveryPayloadAndAtObjectEnd(t *testing.T) {
	t.Parallel()
	for _, terminal := range []bool{false, true} {
		for _, payload := range []string{"", "deliveryReceipt", "deliveryReceiptBytes", "testResult", "load"} {
			t.Run(fmt.Sprintf("terminal-%v/%s", terminal, payload), func(t *testing.T) {
				t.Parallel()
				now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
				root, a, _ := reconcileFixture(t, now)
				if terminal {
					a = terminalShape(a, now)
					a.Terminal.Reason = "retained terminal"
				}
				a.Load = nil
				data, err := json.Marshal(a)
				if err != nil {
					t.Fatal(err)
				}
				if payload != "" {
					data = append(data[:len(data)-1], []byte(`,"`+payload+`":`)...)
				}
				path, _ := AttemptPath(root, a.AttemptID)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				head, err := ReadAttemptHead(root, a.AttemptID)
				if err != nil || !reflect.DeepEqual(head, headOf(a)) {
					t.Fatalf("head=%+v err=%v", head, err)
				}
				if payload != "" {
					if _, err := ReadAttempt(root, a.AttemptID); err == nil {
						t.Fatal("full reader accepted absent payload value")
					}
				}
			})
		}
	}
}

func TestAttemptHeadRefusalsAlsoProtectReconciliation(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*Attempt){
		"newer-schema":      func(a *Attempt) { a.SchemaVersion = IdentityAttemptSchemaVersion + 1 },
		"older-schema":      func(a *Attempt) { a.SchemaVersion = 0 },
		"foreign-root":      func(a *Attempt) { a.ControlRoot = "/foreign" },
		"filename-identity": func(a *Attempt) { a.AttemptID = "different" },
		"launcher":          func(a *Attempt) { a.Launcher = ProcessIdentity{Pid: 123} },
		"end-mismatch":      func(a *Attempt) { a.Terminal.At = a.StartedAt },
		"end-missing":       func(a *Attempt) { a.EndedAt = ""; a.Terminal.At = "" },
		"start-malformed":   func(a *Attempt) { a.StartedAt = "invalid" },
		"end-malformed": func(a *Attempt) {
			a.StartedAt = "0001-01-01T00:00:00Z"
			a.EndedAt = "invalid"
			a.Terminal.At = a.EndedAt
		},
		"end-before-start":  func(a *Attempt) { a.EndedAt = "2020-01-01T00:00:00Z"; a.Terminal.At = a.EndedAt },
		"observed-zero":     func(a *Attempt) { a.ObservedMinutes = 0 },
		"success-nonzero":   func(a *Attempt) { a.Terminal.ExitStatus = 1 },
		"failure-zero":      func(a *Attempt) { a.Terminal.Result = TerminalFailed },
		"success-cancelled": func(a *Attempt) { a.CancellationIntent = "cancel" },
		"unknown-result":    func(a *Attempt) { a.Terminal.Result = "bogus" },
		"live-end":          func(a *Attempt) { a.Terminal = nil; a.ObservedMinutes = 0 },
		"live-observed":     func(a *Attempt) { a.Terminal = nil; a.EndedAt = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
			root, a, record := reconcileFixture(t, now)
			originalID := a.AttemptID
			a = terminalShape(a, now)
			mutate(&a)
			data, _ := json.Marshal(a)
			path, _ := AttemptPath(root, originalID)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadAttemptHead(root, originalID); err == nil {
				t.Fatal("head accepted invalid record")
			}
			if _, err := ReadAttempt(root, originalID); err == nil {
				t.Fatal("full reader accepted invalid record")
			}
			probe := &mapProbe{states: map[int64]identity.Liveness{1001: identity.Alive}}
			live, err := LiveAttempts(root, probe)
			if err != nil || len(live) != 1 || live[0].Unreadable == "" {
				t.Fatalf("unreadable summary = %+v, %v", live, err)
			}
			if n := liveAttemptsOtherThan(loadReaders{prober: probe}, root, ""); n != 0 {
				t.Fatalf("invalid head counted: %d", n)
			}
			options := reconcileOptions(probe, now.Add(time.Hour), func(int, syscall.Signal) error { t.Error("invalid record signalled"); return nil })
			var emitted []string
			options.Emit = func(s string) { emitted = append(emitted, s) }
			for i := 0; i < 2; i++ {
				if _, err := ReconcileAttempts(root, options); err != nil {
					t.Fatal(err)
				}
				requireSettled(t, root, originalID, false)
			}
			if len(emitted) != 2 || !strings.Contains(emitted[0], "record skipped") {
				t.Fatalf("refusals = %v", emitted)
			}
			if after, err := ReadProcessRecord(root, record.Key()); err != nil || after.Status != StatusRunning {
				t.Fatalf("process changed: %+v %v", after, err)
			}
		})
	}
}

func TestReconcileRequiresFullValidationBeforeAnyAction(t *testing.T) {
	t.Parallel()
	for _, terminal := range []bool{false, true} {
		for _, bad := range []string{"newer-result", "unknown-field", "trailing-data"} {
			t.Run(fmt.Sprintf("terminal-%v/%s", terminal, bad), func(t *testing.T) {
				t.Parallel()
				now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
				root, a, record := reconcileFixture(t, now)
				if terminal {
					a = terminalShape(a, now)
				}
				suffix := ""
				if bad == "newer-result" {
					a.TestResult = &TestResult{SchemaVersion: TestResultSchemaVersion + 1}
				}
				if bad == "trailing-data" {
					suffix = " {}"
				}
				saveAttemptShape(t, root, a, suffix)
				if bad == "unknown-field" {
					path, _ := AttemptPath(root, a.AttemptID)
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					data = append(data[:len(data)-1], []byte(`,"unexpected":true}`)...)
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := ReadAttemptHead(root, a.AttemptID); err != nil {
					t.Fatalf("valid head refused: %v", err)
				}
				signals := 0
				states := map[int64]identity.Liveness{}
				if !terminal {
					states[1002] = identity.Alive
				}
				options := reconcileOptions(&mapProbe{states: states}, now.Add(time.Hour), func(int, syscall.Signal) error { signals++; return nil })
				var emitted []string
				options.Emit = func(s string) { emitted = append(emitted, s) }
				for i := 0; i < 2; i++ {
					if _, err := ReconcileAttempts(root, options); err != nil {
						t.Fatal(err)
					}
					requireSettled(t, root, a.AttemptID, false)
				}
				if signals != 0 || len(emitted) != 2 || !strings.Contains(emitted[0], "record skipped") {
					t.Fatalf("signals=%d refusals=%v", signals, emitted)
				}
				if after, err := ReadProcessRecord(root, record.Key()); err != nil || after.Status != StatusRunning {
					t.Fatalf("process changed: %+v %v", after, err)
				}
			})
		}
	}
}

func TestReconcileSettlementRetriesUnreadableAndUnwritableProcesses(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"unreadable", "unwritable", "alive", "unknown"} {
		t.Run(bad, func(t *testing.T) {
			t.Parallel()
			now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
			root, a, record := reconcileFixture(t, now)
			a = terminalShape(a, now)
			saveAttemptShape(t, root, a, "")
			path, _ := ProcessRecordPath(root, record.AttemptID, record.LaunchID)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			probe := &mapProbe{states: map[int64]identity.Liveness{}}
			switch bad {
			case "unreadable":
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "unwritable":
				if err := os.Chmod(filepath.Dir(path), 0500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(filepath.Dir(path), 0700) })
			case "alive":
				probe.states[1002] = identity.Alive
			case "unknown":
				probe.states[1002] = identity.Unknown
			}
			counter := countAttemptOpens(t, root, a.AttemptID)
			for i := 0; i < 2; i++ {
				if _, err := ReconcileAttempts(root, reconcileOptions(probe, now, nil)); err != nil {
					t.Fatal(err)
				}
				requireSettled(t, root, a.AttemptID, false)
			}
			if counter.Load() != 2 {
				t.Fatalf("retry opens=%d", counter.Load())
			}
			if bad == "unwritable" {
				if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			delete(probe.states, 1002)
			if _, err := ReconcileAttempts(root, reconcileOptions(probe, now, nil)); err != nil {
				t.Fatal(err)
			}
			requireSettled(t, root, a.AttemptID, true)
		})
	}
}

func TestLiveAttemptSettlementMarkerDoesNotAuthorizeVerdicts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	for _, state := range []identity.Liveness{identity.Alive, identity.Unknown} {
		t.Run(state.String(), func(t *testing.T) {
			t.Parallel()
			root, a, _ := reconcileFixture(t, now)
			counter := countAttemptOpens(t, root, a.AttemptID)
			options := reconcileOptions(&mapProbe{states: map[int64]identity.Liveness{1001: state, 1002: identity.Alive}}, now, nil)
			for i := 0; i < 2; i++ {
				out, err := ReconcileAttempts(root, options)
				if err != nil || len(out) != 1 {
					t.Fatalf("pass=%+v %v", out, err)
				}
				requireSettled(t, root, a.AttemptID, false)
			}
			if counter.Load() != 2 {
				t.Fatalf("live opens=%d", counter.Load())
			}
			if err := os.WriteFile(settledPath(root, a.AttemptID), []byte("settled"), 0600); err != nil {
				t.Fatal(err)
			}
			observation, err := ObserveAttempt(context.Background(), root, metarun.WaitSelector{TargetID: a.AttemptID}, metarun.WaiterTarget{}, "")
			if err != nil || !observation.Pending || counter.Load() != 3 {
				t.Fatalf("verdict=%+v err=%v opens=%d", observation, err, counter.Load())
			}
			path, _ := AttemptPath(root, a.AttemptID)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if _, err := ReconcileAttempts(root, options); err != nil {
				t.Fatal(err)
			}
			requireSettled(t, root, a.AttemptID, false)
		})
	}
}

func TestAttemptHeadsAndSettlementBoundedReads(t *testing.T) {
	t.Parallel()
	// TotalAlloc is process-wide: isolate this measurement from parallel tests.
	const child = "METASYSTEM_ATTEMPT_BYTES_CHILD"
	if os.Getenv(child) != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestAttemptHeadsAndSettlementBoundedReads$", "-test.v", "-test.timeout=30m")
		command.Env = append(os.Environ(), child+"=1")
		output, err := command.CombinedOutput()
		t.Log(string(output))
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	root, base, record := reconcileFixture(t, now)
	oldPath, _ := AttemptPath(root, base.AttemptID)
	if err := os.Remove(oldPath); err != nil {
		t.Fatal(err)
	}
	counters := make([]*atomic.Int64, 301)
	for i := 0; i <= 300; i++ {
		a := base
		a.AttemptID = fmt.Sprintf("proof-%04d", i)
		r := record
		r.AttemptID = a.AttemptID
		if i < 300 {
			r.Status = StatusDone
			a = terminalShape(a, now)
			a.DeliveryReceiptBytes = bytes.Repeat([]byte("x"), 1<<20)
		}
		if err := writeRecord(r); err != nil {
			t.Fatal(err)
		}
		a.ProcessKeys = []string{r.Key()}
		if i == 300 {
			r.LaunchID = "launch2"
			if err := writeRecord(r); err != nil {
				t.Fatal(err)
			}
			a.ProcessKeys = append(a.ProcessKeys, r.Key())
		}
		result := componentAttemptResult(a.AttemptID, "group", strings.Repeat("b", 64), "passed")
		if i == 300 {
			result.Uncertainty = []string{strings.Repeat("x", 1<<20)}
			result.RecomputeDelivery()
		}
		a.TestResult = &result
		if err := writeAttempt(a); err != nil {
			t.Fatal(err)
		}
		counters[i] = countAttemptOpens(t, root, a.AttemptID)
	}
	probe := &mapProbe{states: map[int64]identity.Liveness{}}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	live, err := LiveAttempts(root, probe)
	n := liveAttemptsOtherThan(loadReaders{prober: probe}, root, "")
	runtime.ReadMemStats(&after)
	if err != nil || len(live) != 1 || live[0].AttemptID != "proof-0300" || live[0].LauncherLiveness != "dead" || n != 0 {
		t.Fatalf("live=%+v count=%d err=%v", live, n, err)
	}
	if delta := after.TotalAlloc - before.TotalAlloc; delta >= 8<<20 {
		t.Fatalf("first head pass allocated %d bytes", delta)
	} else {
		t.Logf("first head pass: %d bytes", delta)
	}
	for _, c := range counters {
		if c.Swap(0) != 2 {
			t.Fatal("counts did not open each head once per reader")
		}
	}
	for pass := 1; pass <= 3; pass++ {
		runtime.ReadMemStats(&before)
		outcomes, err := ReconcileAttempts(root, reconcileOptions(probe, now.Add(time.Hour), nil))
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		if pass == 1 && (len(outcomes) != 1 || outcomes[0].Action != ReconcileReconciled) {
			t.Fatalf("live transition=%+v", outcomes)
		}
		for i, c := range counters {
			opens := c.Swap(0)
			if i < 300 {
				want := int64(0)
				if pass == 1 {
					want = 1
				}
				if opens != want {
					t.Fatalf("pass %d record %d opens=%d want=%d", pass, i, opens, want)
				}
			} else if pass == 2 && opens != 1 || pass == 3 && opens != 0 {
				t.Fatalf("pass %d newly terminal opens=%d", pass, opens)
			}
			if i < 300 || pass >= 2 {
				requireSettled(t, root, fmt.Sprintf("proof-%04d", i), true)
			}
		}
		if pass == 3 {
			if delta := after.TotalAlloc - before.TotalAlloc; delta >= 8<<20 {
				t.Fatalf("settled pass allocated %d bytes", delta)
			} else {
				t.Logf("settled pass: %d bytes; all 301 record opens=0", delta)
			}
		}
	}
	// Scratch recovery still opens the full record despite its settled marker.
	scratchProbe := newScratchProbe(t)
	scratch := crashedScratch(t, root, scratchProbe, func(r *ScratchRecord) { r.Attempt = "proof-0000" })
	runtime.ReadMemStats(&before)
	outcomes := ReconcileScratch(root, ScratchOptions{Prober: scratchProbe, GroupMembers: func(int64) ([]int64, error) { return nil, nil }})
	runtime.ReadMemStats(&after)
	if counters[0].Load() != 1 || scratchOutcome(outcomes, scratch.Run).Action != ReconcileScratchRemoved {
		t.Fatalf("scratch full opens=%d outcomes=%+v", counters[0].Load(), outcomes)
	}
	t.Logf("scratch recovery unchanged: %d full record open, %d bytes (outside bound)", counters[0].Load(), after.TotalAlloc-before.TotalAlloc)
}

func TestReconcileScratchSurvivesAnUnreadableAttemptsDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	probe := newScratchProbe(t)
	scratch := crashedScratch(t, root, probe, nil)
	if err := os.MkdirAll(filepath.Dir(attemptsDir(root)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attemptsDir(root), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	var emitted []string
	outcomes, err := ReconcileAttempts(root, ReconcileOptions{Prober: probe,
		GroupMembers: func(int64) ([]int64, error) { return nil, nil }, Emit: func(line string) { emitted = append(emitted, line) }})
	if err != nil || scratchOutcome(outcomes, scratch.Run).Action != ReconcileScratchRemoved {
		t.Fatalf("scratch recovery=%+v err=%v", outcomes, err)
	}
	if len(emitted) < 1 || !strings.Contains(emitted[0], "proof attempt record skipped:") {
		t.Fatalf("listing failure=%v", emitted)
	}
}
