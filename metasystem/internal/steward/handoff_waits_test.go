package steward

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

const testHandoffNonce = "0123456789abcdef"

func writeHandoffWaiter(t *testing.T, root, name string, row run.Waiter) (string, []byte) {
	t.Helper()
	if row.SchemaVersion == 0 {
		row.SchemaVersion = 2
	}
	if row.WaitID == "" {
		row.WaitID = name + "-wait"
	}
	if row.Nonce == "" {
		row.Nonce = "0123456789abcdef0123456789abcdef"
	}
	if row.Kind == "" {
		row.Kind = "run"
	}
	if row.MainId == "" {
		row.MainId = "main-1"
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(run.WaitersDir(root), name+".json")
	writeTestFile(t, path, data)
	return path, data
}

func readHandoffWaiterTest(t *testing.T, path string) (run.Waiter, []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var row run.Waiter
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	return row, data
}

func TestEndInFlightWaitsRecordsSelectorAndTarget(t *testing.T) {
	root, now := handoffWaitsTestRoot(t, t.TempDir()), time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	selector := run.WaitSelector{Kind: "goal", TargetID: "goal-1", GoalID: "goal-1", Event: "human-act", After: "after", Verb: "answer", Question: "q-1", Chain: "chain-1", Poll: "channel"}
	target := run.WaiterTarget{StartedAt: "started", Round: 2, OperationID: "op-1", Generation: 3, LaunchNonce: "launch", ProofDigest: "proof"}
	tests := []struct {
		name       string
		pid, birth int64
	}{{"a-local", 41, 42}, {"b-remote", 41, 0}}
	for _, tc := range tests {
		writeHandoffWaiter(t, root, tc.name, run.Waiter{State: run.WaiterStatePending, Selector: selector, Target: target, Pid: tc.pid, PidStartedAt: tc.birth, Deadline: now.Add(10900 * time.Millisecond).Format(time.RFC3339Nano)})
	}
	got, err := EndInFlightWaits(root, handoffMainCaller(), testHandoffNonce, HandoffClock{Now: func() time.Time { return now }})
	if err != nil || len(got) != 2 {
		t.Fatalf("open work = %+v, err = %v", got, err)
	}
	for i, tc := range tests {
		wantPID, wantBirth := tc.pid, tc.birth
		if tc.birth == 0 {
			wantPID = 0
		}
		if got[i].RowID != tc.name || got[i].Selector != selector || got[i].Target != target || got[i].DeadlineLeftSeconds != 10 || got[i].Pid != wantPID || got[i].PidStartedAt != wantBirth {
			t.Errorf("%s open work = %+v", tc.name, got[i])
		}
	}
}

func TestEndInFlightWaitsGraceUsesInjectedClock(t *testing.T) {
	tests := []struct {
		name, after, registered string
		want, sleeps, changeAt  int
	}{{"becomes pending", run.WaiterStatePending, "", 1, 1, 1}, {"ends during grace", run.WaiterStateReady, "", 0, 1, 1}, {"grace expires", run.WaiterStateRegistering, "", 1, 20, 0}, {"malformed registration still gets grace", run.WaiterStatePending, "not-a-time", 1, 3, 3}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, now := handoffWaitsTestRoot(t, t.TempDir()), time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
			started := now
			registered := tc.registered
			if registered == "" {
				registered = now.Format(time.RFC3339Nano)
			}
			path, _ := writeHandoffWaiter(t, root, "grace", run.Waiter{State: run.WaiterStateRegistering, RegisteredAt: registered, Deadline: now.Add(time.Minute).Format(time.RFC3339Nano)})
			sleeps := 0
			clock := HandoffClock{Now: func() time.Time { return now }, Sleep: func(d time.Duration) {
				now, sleeps = now.Add(d), sleeps+1
				if sleeps == tc.changeAt && tc.changeAt != 0 {
					row, _ := readHandoffWaiterTest(t, path)
					row.State = tc.after
					writeHandoffWaiter(t, root, "grace", row)
				}
			}}
			got, err := EndInFlightWaits(root, handoffMainCaller(), testHandoffNonce, clock)
			row, _ := readHandoffWaiterTest(t, path)
			wantState := run.WaiterStateInterrupted
			if tc.want == 0 {
				wantState = run.WaiterStateReady
			}
			if err != nil || len(got) != tc.want || sleeps != tc.sleeps || now.Sub(started) != time.Duration(tc.sleeps)*250*time.Millisecond || row.State != wantState {
				t.Fatalf("got=%+v row=%s sleeps=%d advanced=%s err=%v", got, row.State, sleeps, now.Sub(started), err)
			}
		})
	}
}

func TestEndInFlightWaitsUnendableRow(t *testing.T) {
	root, now := handoffWaitsTestRoot(t, t.TempDir()), time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	writeHandoffWaiter(t, root, "blocked", run.Waiter{State: run.WaiterStatePending, Deadline: now.Add(time.Minute).Format(time.RFC3339Nano)})
	if err := os.Mkdir(filepath.Join(run.WaitersDir(root), ".lock"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := EndInFlightWaits(root, handoffMainCaller(), testHandoffNonce, HandoffClock{Now: func() time.Time { return now }})
	var refusal *HandoffRefusal
	if got != nil || !errors.As(err, &refusal) || refusal.Code != "HANDOFF_WAIT_UNENDABLE" || refusal.Detail != "row=blocked" {
		t.Fatalf("got=%+v refusal=%+v err=%v", got, refusal, err)
	}
}

func TestEndInFlightWaitsSelectsByWaiterStateClass(t *testing.T) {
	original := run.WaiterStates
	defer func() { run.WaiterStates = original }()
	for i := range run.WaiterStates {
		if run.WaiterStates[i].Name == run.WaiterStatePending {
			run.WaiterStates[i].Class = run.WaiterStateEnded
		}
		if run.WaiterStates[i].Name == run.WaiterStateReady {
			run.WaiterStates[i].Class = run.WaiterStateInFlight
		}
	}
	root, now := handoffWaitsTestRoot(t, t.TempDir()), time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	paths, before := map[string]string{}, map[string][]byte{}
	for _, state := range run.WaiterStates {
		paths[state.Name], before[state.Name] = writeHandoffWaiter(t, root, state.Name, run.Waiter{State: state.Name, RegisteredAt: now.Add(-time.Minute).Format(time.RFC3339Nano), Deadline: now.Add(time.Minute).Format(time.RFC3339Nano)})
	}
	got, err := EndInFlightWaits(root, handoffMainCaller(), testHandoffNonce, HandoffClock{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	ended := 0
	for _, state := range run.WaiterStates {
		row, after := readHandoffWaiterTest(t, paths[state.Name])
		if state.Class == run.WaiterStateInFlight {
			ended++
			if row.State != run.WaiterStateInterrupted {
				t.Errorf("in-flight class %q stayed %q", state.Name, row.State)
			}
		} else if !bytes.Equal(after, before[state.Name]) {
			t.Errorf("ended class %q changed on disk", state.Name)
		}
	}
	if len(got) != ended {
		t.Fatalf("returned %d rows, ended %d", len(got), ended)
	}
}

func TestRecordOnlyHandoffStillRefusesInFlightWaits(t *testing.T) {
	root := handoffWaitsTestRoot(t, t.TempDir())
	installHandoffFixtureFiles(t, root)
	path := filepath.Join(run.WaitersDir(root), "record-only.json")
	for _, state := range run.WaiterStates {
		_, before := writeHandoffWaiter(t, root, "record-only", run.Waiter{State: state.Name})
		file := capturedGoal("claimed")
		snapshot := handoffGoalSnapshot{claimed: []string{file.Id}, accepted: map[string]*goal.GoalFile{file.Id: file}}
		reads := 0
		_, err := captureHandoffWithReader(root, handoffMainCaller(), HandoffRecord{}, handoffCaptureNow, func(gotRoot string, gotNow time.Time) (handoffGoalSnapshot, error) {
			reads++
			if gotRoot != root || gotNow != handoffCaptureNow {
				t.Errorf("goal reader received root=%q clock=%s; want root=%q clock=%s", gotRoot, gotNow, root, handoffCaptureNow)
			}
			return snapshot, nil
		})
		if reads != 1 {
			t.Errorf("%q goal reader calls=%d, want 1", state.Name, reads)
		}
		var refusal *HandoffRefusal
		if state.Class == run.WaiterStateInFlight && (!errors.As(err, &refusal) || refusal.Code != "HANDOFF_WAIT_IN_FLIGHT") {
			t.Errorf("in-flight class %q returned %v", state.Name, err)
		}
		if state.Class == run.WaiterStateEnded && err != nil {
			t.Errorf("ended class %q returned %v", state.Name, err)
		}
		row, after := readHandoffWaiterTest(t, path)
		if !bytes.Equal(after, before) || row.State != state.Name || row.InterruptedBy != "" {
			t.Errorf("record-only path changed %q: row=%+v", state.Name, row)
		}
	}
}

// handoffWaitsTestRoot resolves a test root through its symlinks. On
// macOS a t.TempDir() root is a /var/folders path that resolves into
// /private/var, and the handoff reference check correctly reads the
// difference as an escape. Production reaches captureHandoff through
// handoffCanonicalRoot, which resolves it; a witness that calls captureHandoff
// directly must resolve the root itself.
func handoffWaitsTestRoot(t *testing.T, dir string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return root
}
