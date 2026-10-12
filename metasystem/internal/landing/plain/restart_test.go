package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

var restartNow = time.Date(2026, 10, 11, 20, 24, 8, 0, time.UTC)

func restartBed(t *testing.T, minutes int, red bool) (string, *Batch, Running, ProveSeams, *int) {
	t.Helper()
	install := t.TempDir()
	if err := os.MkdirAll(filepath.Join(Dir(install), "proofs"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"queue.jsonl", "results.jsonl"} {
		data, err := os.ReadFile("testdata/restart/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(Dir(install), name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("proof.trunk-every=4h\n"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := Entries(install)
	if err != nil || len(entries) != 2 {
		t.Fatalf("queue: %+v %v", entries, err)
	}
	batch := &Batch{SealedAt: restartNow.Add(-time.Duration(minutes) * time.Minute).Format(time.RFC3339), ID: "batch", Base: "main", Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: entries[0].Goal, SHA: entries[0].SHA}}, State: BatchRunning, DepthClass: "full", Selector: PolicyValue{Value: "auto"}}
	running := Running{Attempt: "attempt", Tree: "tree", Commit: "commit", Since: restartNow.Add(-time.Duration(minutes) * time.Minute).Format(time.RFC3339), BatchID: batch.ID, BatchMembers: batch.Members, Pid: 4242, Process: "recorded-child"}
	if err := writeBatch(install, batch); err != nil {
		t.Fatal(err)
	}
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	notAncestor := exec.Command("/usr/bin/false").Run()
	ended := new(int)
	seams := ProveSeams{Now: func() time.Time { return restartNow }, Alive: func(Running) bool { return *ended == 0 },
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) {
			if red {
				return []goal.TrunkRedEntry{{Identity: "open", FixGoal: "shared-state-in-the-ledger", Opened: "2026-10-10T12:00:00Z"}}, nil
			}
			return nil, nil
		},
		Git: func(_ string, args ...string) (string, error) {
			switch args[0] {
			case "fetch", "cat-file":
				return "", nil
			case "rev-parse":
				return "main", nil
			case "merge-base":
				return "", notAncestor
			}
			t.Fatalf("unexpected git %v", args)
			return "", nil
		},
		EndProof: func(r Running) error {
			if !reflect.DeepEqual(r, running) {
				t.Fatalf("ended different identity: %+v", r)
			}
			cancel, err := readCancel(install, r)
			if err != nil || cancel == nil {
				t.Fatalf("child ended before cancellation: %+v %v", cancel, err)
			}
			*ended++
			return nil
		},
	}
	home := shadowHome(t, install, install)
	t.Cleanup(func() {
		effects := shadowSeams()
		effects.Now = seams.Now
		effects.Alive = seams.Alive
		effects.Incidents = seams.Incidents
		assertShadowAgrees(t, home, effects)
	})
	return install, batch, running, seams, ended
}

func TestRestartSelectionAndWaitReasons(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		minutes    int
		red, prior bool
		wait       string
	}{
		{name: "young", minutes: 12},
		{name: "late", minutes: 31, wait: "proof at 31.0 of 44.0 min, past the restart point"},
		{name: "halfway", minutes: 22, wait: "proof at 22.0 of 44.0 min, past the restart point"},
		{name: "already restarted", minutes: 12, prior: true, wait: "this batch already restarted"},
		{name: "main red", minutes: 40, red: true},
		{name: "main red after restart", minutes: 40, red: true, prior: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			install, batch, running, seams, ended := restartBed(t, tc.minutes, tc.red)
			if tc.prior {
				batch.Restarts = []Restart{{Goal: "earlier", Expected: 44, Elapsed: 12, At: running.Since}}
				if err := writeBatch(install, batch); err != nil {
					t.Fatal(err)
				}
			}
			selected, err := SelectBatch(install, install, batch.Lane, seams)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wait != "" {
				if *ended != 0 || selected.Open || len(selected.Members) != 1 || len(selected.Waiting) != 1 || selected.Waiting[0] != "shared-state-in-the-ledger waits: "+tc.wait {
					t.Fatalf("wait: %+v ended=%d", selected, *ended)
				}
				if _, err := os.Stat(cancelPath(install, running.Attempt)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("waiting wrote cancel: %v", err)
				}
			} else {
				if *ended != 1 || !selected.Open || selected.State != BatchPrepared || selected.SealedAt != "" || len(selected.Members) != 2 || len(selected.Restarts) != 1+boolInt(tc.prior) {
					t.Fatalf("restart: %+v ended=%d", selected, *ended)
				}
				decision := selected.Restarts[len(selected.Restarts)-1]
				if decision.Goal != "shared-state-in-the-ledger" || decision.Elapsed != float64(tc.minutes) || decision.Expected != 44 || decision.At != restartNow.Format(time.RFC3339) {
					t.Fatalf("decision %+v", decision)
				}
				if tc.red && decision.Reason != "restart: main is red; shared-state-in-the-ledger may be its fix" {
					t.Fatalf("red reason %+v", decision)
				}
				last, ok, err := LastResult(install)
				if err != nil || !ok || last.Result != "cancelled" || last.Reason != "cancelled: restart for shared-state-in-the-ledger" || last.Attempt != running.Attempt {
					t.Fatalf("cancelled result %+v %v", last, err)
				}
				_, recorded, _, err := ReadRunning(install, seams)
				if err != nil || recorded {
					t.Fatalf("old running record survives: %v %v", recorded, err)
				}
				reasons, err := WakeReasons(install, install, restartNow.Add(-time.Minute), restartNow, seams)
				if err != nil || !slices.Contains(reasons, WakeQueued) || !slices.Contains(reasons, WakeProofFinished) {
					t.Fatalf("keeper wake %v %v", reasons, err)
				}
				hold, err := ProofHold(install, seams)
				if err != nil || hold != "" {
					t.Fatalf("proof hold %s %v", hold, err)
				}
			}
			status := readStatus(t.TempDir(), batch.Lane, lane.View{Root: &batch.Lane.Root}, seams, laneGit{main: func() (string, error) { return "main", nil }, contains: func(string, string) (bool, error) { return false, nil }, brought: func(string, string) ([]string, error) { return nil, nil }})
			reason := tc.wait
			if reason == "" {
				reason = selected.Restarts[len(selected.Restarts)-1].Reason
			}
			if !strings.Contains(status.Summary, reason) {
				t.Fatalf("status omits decision: %s", status.Summary)
			}
		})
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestRestartExcludesInadmissibleNewcomers(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"records", "held", "returned", "landed", "queued before proof"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			install, batch, running, seams, ended := restartBed(t, 12, false)
			data, err := os.ReadFile(queuePath(install))
			if err != nil {
				t.Fatal(err)
			}
			var lines []Line
			for _, raw := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				var l Line
				if err := json.Unmarshal([]byte(raw), &l); err != nil {
					t.Fatal(err)
				}
				lines = append(lines, l)
			}
			switch name {
			case "queued before proof":
				since, err := time.Parse(time.RFC3339, running.Since)
				if err != nil {
					t.Fatal(err)
				}
				lines[1].At = since.Add(-time.Second).Format(time.RFC3339)
			case "records":
				lines[1].Records = true
			case "held":
				held := lines[1]
				held.Outcome = StateWaiting
				held.Held = true
				held.After = batch.Members
				lines = append(lines, held)
				lines[1].Held = true
				lines[1].After = batch.Members
			case "returned":
				lines[1].Outcome = StateReturned
			case "landed":
				original := seams.Git
				seams.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "merge-base" && args[2] == lines[1].SHA {
						return "", nil
					}
					return original(dir, args...)
				}
			}
			if err := os.Remove(queuePath(install)); err != nil {
				t.Fatal(err)
			}
			for _, l := range lines {
				if err := appendLine(queuePath(install), l); err != nil {
					t.Fatal(err)
				}
			}
			selected, err := SelectBatch(install, install, batch.Lane, seams)
			if err != nil || *ended != 0 || len(selected.Members) != 1 || len(selected.Restarts) != 0 {
				t.Fatalf("excluded arrival %+v ended=%d err=%v", selected, *ended, err)
			}
		})
	}
}

func TestCancelledLostProcessNeverBecomesRed(t *testing.T) {
	t.Parallel()
	for _, cancelled := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancel record", false: "lost child"}[cancelled], func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			b.seams.Now = func() time.Time { return restartNow }
			running := Running{Attempt: "dead", Commit: "commit", Tree: "tree", Since: restartNow.Add(-time.Minute).Format(time.RFC3339), Trunk: true}
			if err := os.MkdirAll(Dir(b.install), 0700); err != nil {
				t.Fatal(err)
			}
			if err := writeRunning(b.install, running); err != nil {
				t.Fatal(err)
			}
			if cancelled {
				r := Result{Result: "cancelled", Reason: "cancelled: restart for G", Attempt: running.Attempt, Tree: running.Tree}
				data, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(cancelPath(b.install, running.Attempt)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(cancelPath(b.install, running.Attempt), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			b.seams.Alive = func(Running) bool { return false }
			b.seams.RecordMain = func([]Result) error { t.Fatal("cancelled child opened an incident"); return nil }
			err := withLock(b.install, func() error { _, _, _, err := checkState(b.install, b.checkout, "", b.seams); return err })
			if err != nil {
				t.Fatal(err)
			}
			got, ok, err := LastResult(b.install)
			want := Red
			if cancelled {
				want = "cancelled"
			}
			if err != nil || !ok || got.Result != want || cancelled && got.Cause != nil {
				t.Fatalf("result %+v err=%v", got, err)
			}
			if cancelled {
				stops, err := OpenStops(b.install)
				if err != nil || len(stops) != 0 {
					t.Fatalf("cancelled proof opened stop %+v %v", stops, err)
				}
			}
		})
	}
}

func TestCancellationWinsOverCompletingRedProof(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	b.seams.Command = func(cmd *exec.Cmd) error {
		r, ok, _, err := ReadRunning(b.install, b.seams)
		if err != nil || !ok {
			t.Fatalf("running %v %v", ok, err)
		}
		cancel := Result{Result: "cancelled", Attempt: r.Attempt, Tree: r.Tree, Reason: "cancelled: restart for G"}
		data, err := json.Marshal(cancel)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(cancelPath(b.install, r.Attempt)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cancelPath(b.install, r.Attempt), data, 0600); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(cmd.Stdout, "LANDING-FAILED\tu/a\tTestA\nLANDING-CHECKED\t1\n")
		return exec.Command("/usr/bin/false").Run()
	}
	b.seams.RecordMain = func([]Result) error { t.Fatal("cancelled completion published incident"); return nil }
	b.seams.Trunk = true
	originalGit := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify origin/main^{commit}" {
			return b.git.commit, nil
		}
		return originalGit(dir, args...)
	}
	r, err := Run(b.install, b.checkout, "fixture", "", b.output, b.seams)
	if err != nil || r.Result != "cancelled" || r.Cause != nil {
		t.Fatalf("completion %+v %v", r, err)
	}
	results, err := Results(b.install)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Attempt == "attempt" && r.Result != "cancelled" {
			t.Fatalf("cancelled attempt became %+v", r)
		}
	}
}

func TestRestartEndsRealDetachedIdentity(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, _ := restartBed(t, 12, false)
	done := make(chan error, 1)
	pid, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: []string{"/bin/sleep", "600"}, Completed: done})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = (ProveSeams{}).endProof(running) })
	running.Pid, running.Process = pid, processRef(pid)
	if running.Process == "" {
		t.Fatal("missing child identity")
	}
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	seams.Alive, seams.EndProof = nil, nil
	selected, err := SelectBatch(install, install, batch.Lane, seams)
	if err != nil {
		t.Fatal(err)
	}
	exit := <-done
	var killed *exec.ExitError
	if !errors.As(exit, &killed) || killed.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL || !selected.Open {
		t.Fatalf("detached child exit=%v batch=%+v", exit, selected)
	}
	t.Logf("real detached child %d ended by SIGKILL; batch reopened with %d members", pid, len(selected.Members))
}

func TestRestartExpectedMinutesDefaultAndMedian(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		values []float64
		want   float64
	}{
		{"no history", nil, 45}, {"one", []float64{20}, 20}, {"zero", []float64{0}, 0}, {"even", []float64{20, 40}, 30}, {"odd", []float64{10, 20, 30}, 20}, {"last five", []float64{100, 38.8, 67.5, 45, 41, 44}, 44},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			if err := os.MkdirAll(Dir(install), 0700); err != nil {
				t.Fatal(err)
			}
			for i, value := range tc.values {
				v := value
				if err := appendLine(resultsPath(install), Result{Attempt: fmt.Sprint(i), Result: Green, Scope: "full", Minutes: &v}); err != nil {
					t.Fatal(err)
				}
			}
			got, err := expectedFullMinutes(install)
			if err != nil || got != tc.want {
				t.Fatalf("median %v: %g %v, want %g", tc.values, got, err, tc.want)
			}
		})
	}
}

// Saved lane records exercise the restart against its recorded queue and history.
func TestRestartSavedLaneRecords(t *testing.T) {
	t.Parallel()
	install, batch, _, seams, ended := restartBed(t, 12, false)
	selected, err := SelectBatch(install, install, batch.Lane, seams)
	if err != nil || *ended != 1 || len(selected.Restarts) != 1 || selected.Restarts[0].Expected != 44 {
		t.Fatalf("real shapes restart %+v %v", selected, err)
	}
}

func TestRestartSecondArrivalWaitsAfterActualRestart(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, ended := restartBed(t, 12, false)
	first, err := SelectBatch(install, install, batch.Lane, seams)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := HandIn(install, Line{Goal: "third", SHA: "third-sha", At: restartNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	first.State, first.Open = BatchRunning, false
	if err := writeBatch(install, first); err != nil {
		t.Fatal(err)
	}
	running.Attempt, running.Since, running.BatchMembers = "second", restartNow.Add(-time.Minute).Format(time.RFC3339), first.Members
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	seams.Alive = func(Running) bool { return true }
	next, err := SelectBatch(install, install, batch.Lane, seams)
	if err != nil || *ended != 1 || len(next.Restarts) != 1 || len(next.Members) != 2 || !slices.Contains(next.Waiting, "third waits: this batch already restarted") {
		t.Fatalf("second arrival %+v ended=%d error=%v", next, *ended, err)
	}
}

func TestRestartCancellationFailuresKeepTheBatchRecoverable(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"cancel write", "signal", "malformed cancel", "scoped proof", "missing wrapper identity"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			install, batch, running, seams, ended := restartBed(t, 12, false)
			switch name {
			case "missing wrapper identity":
				running.Process = ""
				seams.EndProof = nil
				if err := writeRunning(install, running); err != nil {
					t.Fatal(err)
				}
			case "cancel write":
				if err := os.Mkdir(cancelPath(install, running.Attempt), 0700); err != nil {
					t.Fatal(err)
				}
			case "signal":
				seams.EndProof = func(Running) error { return errors.New("signal failed") }
			case "malformed cancel":
				if err := os.WriteFile(cancelPath(install, running.Attempt), []byte(`{"attempt":"another","result":"cancelled"}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "scoped proof":
				running.Admission = &ExecutionAdmission{Scope: "scoped"}
				if err := writeRunning(install, running); err != nil {
					t.Fatal(err)
				}
			}
			_, err := SelectBatch(install, install, batch.Lane, seams)
			if (err == nil) != (name == "scoped proof") {
				t.Fatalf("unexpected selection error: %v", err)
			}
			saved, readErr := ReadBatch(install)
			if readErr != nil || *ended != 0 || saved.Open || len(saved.Restarts) != 0 || len(saved.Members) != 1 {
				t.Fatalf("failed cancellation changed batch %+v %v", saved, readErr)
			}
			if name == "signal" {
				cancel, err := readCancel(install, running)
				if err != nil || cancel == nil {
					t.Fatalf("lost cancellation on failed signal: %v", err)
				}
				seams.EndProof = func(Running) error { *ended++; return nil }
				seams.Alive = func(Running) bool { return false }
				next, err := SelectBatch(install, install, batch.Lane, seams)
				if err != nil || !next.Open || len(next.Members) != 2 || *ended != 1 {
					t.Fatalf("retry %+v %v", next, err)
				}
			}
		})
	}
}

func TestRestartWakeDoesNotValidateTheSelection(t *testing.T) {
	t.Parallel()
	install, batch, _, seams, _ := restartBed(t, 12, false)
	batch.Lane.CustodyEpoch = 0
	if err := writeBatch(install, batch); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBatch(install); err == nil {
		t.Fatal("the selection unexpectedly accepted an incomplete registration")
	}
	reasons, err := WakeReasons(install, install, time.Time{}, restartNow, seams)
	if err != nil || !slices.Contains(reasons, WakeQueued) {
		t.Fatalf("selection validation blocked the queue wake: %v %v", reasons, err)
	}
}

func TestRestartPreservesPersonSelectedMembers(t *testing.T) {
	t.Parallel()
	for _, red := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary main", true: "red main"}[red], func(t *testing.T) {
			t.Parallel()
			install, batch, _, seams, ended := restartBed(t, 12, red)
			batch.Person = &ActProvenance{Kind: "selection", Person: "Wido", Root: install, CheckedAt: restartNow, TerminalGeneration: 1, Destination: batch.Lane, Subject: batch.Members}
			batch.Person.TerminalRef.PID = 1
			if err := writeBatch(install, batch); err != nil {
				t.Fatal(err)
			}
			selected, err := SelectBatch(install, install, batch.Lane, seams)
			if err != nil || *ended != 0 || len(selected.Members) != 1 || selected.Open {
				t.Fatalf("automatic arrival changed a person's selection: %+v ended=%d error=%v", selected, *ended, err)
			}
			if _, err := ReadBatch(install); err != nil {
				t.Fatalf("person selection became unreadable: %v", err)
			}
			if !slices.Contains(selected.Waiting, "shared-state-in-the-ledger waits: this batch was selected by Wido") {
				t.Fatalf("missing wait reason: %v", selected.Waiting)
			}
		})
	}
}

func TestRestartOnlyAnIncidentsFixPreempts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, fix, binding, wait string
		minutes                  int
		prior                    bool
	}{
		{name: "unrelated young", fix: "another", minutes: 10},
		{name: "unrelated late", fix: "another", minutes: 40, wait: "proof at 40.0 of 44.0 min, past the restart point"},
		{name: "unrelated after restart", fix: "another", minutes: 10, prior: true, wait: "this batch already restarted"},
		{name: "fix proving", fix: "lane-rehearsal-before-landing", minutes: 10, wait: "the fix of main is proving"},
		{name: "different incident binding", fix: "shared-state-in-the-ledger", binding: "another", minutes: 40, wait: "proof at 40.0 of 44.0 min, past the restart point"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			install, batch, running, seams, ended := restartBed(t, tc.minutes, true)
			lines, err := readLines[Line](queuePath(install))
			if err != nil {
				t.Fatal(err)
			}
			lines[1].At = running.Since
			if tc.binding != "" {
				lines[1].Fix = tc.binding
			}
			var queue []byte
			for _, line := range lines {
				data, err := json.Marshal(line)
				if err != nil {
					t.Fatal(err)
				}
				queue = append(queue, append(data, '\n')...)
			}
			if err := os.WriteFile(queuePath(install), queue, 0600); err != nil {
				t.Fatal(err)
			}
			if tc.prior {
				batch.Restarts = []Restart{{Goal: "earlier"}}
				if err := writeBatch(install, batch); err != nil {
					t.Fatal(err)
				}
			}
			seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
				return []goal.TrunkRedEntry{{Identity: "open", FixGoal: tc.fix}}, nil
			}
			selected, err := SelectBatch(install, install, batch.Lane, seams)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wait == "" {
				if *ended != 1 || !selected.Open || strings.Contains(selected.Restarts[len(selected.Restarts)-1].Reason, "main is red") {
					t.Fatalf("unrelated arrival did not follow the ordinary rule: %+v ended=%d", selected, *ended)
				}
			} else if *ended != 0 || selected.Open || !slices.Contains(selected.Waiting, "shared-state-in-the-ledger waits: "+tc.wait) {
				t.Fatalf("wait: %+v ended=%d", selected, *ended)
			}
		})
	}
}

func TestRestartStopsTheWholeDetachedCheckBeforeReopening(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, _ := restartBed(t, 12, false)
	wrapperDone := make(chan error, 1)
	wrapper, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: []string{"/bin/sleep", "600"}, Completed: wrapperDone})
	if err != nil {
		t.Fatal(err)
	}
	running.Pid, running.Process = wrapper, processRef(wrapper)
	t.Cleanup(func() { _ = (ProveSeams{}).endProof(Running{Process: running.Process}) })
	// Use the same native command launcher as a full check, including its durable identity.
	seams.checkInstall, seams.checkAttempt = install, running.Attempt
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(install, "child.pid")
	proofDone := make(chan error, 1)
	command := exec.Command(os.Args[0], "-test.run=^TestRestartProofWorker$", "-test.timeout=30m")
	command.Env = append(os.Environ(), "PLAIN_RESTART_WORKER_PID="+childPath)
	if err := seams.launchCheck(command, os.DevNull, proofDone); err != nil {
		t.Fatal(err)
	}
	running, _, _, err = ReadRunning(install, seams)
	if err != nil || running.ProofProcess == "" {
		t.Fatalf("command identity was not recorded: %+v %v", running, err)
	}
	var child int64
	testenv.Await(t, "the command's long child", func() bool {
		data, _ := os.ReadFile(childPath)
		_, err := fmt.Sscan(string(data), &child)
		return err == nil && child > 0
	})

	childRef, err := identity.ParseRef(processRef(child))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = (ProveSeams{}).endProof(running)
		_ = identity.SignalExact(identity.KernelProber{}, childRef, syscall.SIGKILL)
	})
	seams.Alive, seams.EndProof = nil, nil
	proofRef, _ := identity.ParseRef(running.ProofProcess)
	selected, err := SelectBatch(install, install, batch.Lane, seams)
	if err != nil {
		t.Fatal(err)
	}
	wrapperRef, _ := identity.ParseRef(running.Process)
	for _, ref := range []identity.Ref{wrapperRef, proofRef, childRef} {
		if identity.LiveRef(identity.KernelProber{}, ref) != identity.Dead {
			t.Fatalf("batch reopened with process %d still running", ref.Pid)
		}
	}
	if !selected.Open || len(selected.Members) != 2 {
		t.Fatalf("batch did not reopen: %+v", selected)
	}
	<-wrapperDone
	<-proofDone
	t.Log("recorded wrapper, detached command, and long child were gone before reopen")
}

func TestRestartProofWorker(t *testing.T) {
	t.Parallel()
	path := os.Getenv("PLAIN_RESTART_WORKER_PID")
	if path == "" {
		t.Skip("only runs inside the detached fixture proof")
	}
	child := exec.Command("/bin/sleep", "600")
	// The child remains in the proof command's process group.
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	if err := os.WriteFile(path, []byte(fmt.Sprint(child.Process.Pid)), 0600); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
}

func TestRestartCannotReopenUntilTerminationIsConfirmed(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, _ := restartBed(t, 12, false)
	done := make(chan error, 1)
	wrapper, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: []string{"/bin/sleep", "600"}, Completed: done})
	if err != nil {
		t.Fatal(err)
	}
	running.Pid, running.Process = wrapper, processRef(wrapper)
	t.Cleanup(func() { _ = (ProveSeams{}).endProof(Running{Process: running.Process}) })
	child := exec.Command("/bin/sleep", "600")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	process := processRef(int64(child.Process.Pid))
	running.ProofProcess = process
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
		_ = (ProveSeams{}).endProof(Running{Process: running.Process})
	})
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	seams.Alive, seams.EndProof = nil, nil
	clock := restartNow
	seams.Now = func() time.Time { return clock }
	seams.WaitPoll = func() { clock = clock.Add(time.Second) }
	_, err = SelectBatch(install, install, batch.Lane, seams)
	if err == nil || !strings.Contains(err.Error(), "have not stopped") {
		t.Fatalf("unconfirmed termination: %v", err)
	}
	saved, err := ReadBatch(install)
	if err != nil || saved.Open || saved.State != BatchRunning || len(saved.Restarts) != 0 {
		t.Fatalf("partial cancellation reopened: %+v %v", saved, err)
	}
	cancel, err := readCancel(install, running)
	if err != nil || cancel == nil {
		t.Fatalf("lost durable cancellation: %v", err)
	}
	seams.Alive = func(Running) bool { return false }
	if _, _, _, err := checkState(install, install, "", seams); err == nil {
		t.Fatal("lost-process recovery forgot the surviving command")
	}
	if _, recorded, _, err := ReadRunning(install, seams); err != nil || !recorded {
		t.Fatalf("lost running identity: %v", err)
	}
	_ = child.Process.Kill()
	_ = child.Wait()
	seams.Alive = nil
	selected, err := SelectBatch(install, install, batch.Lane, seams)
	if err != nil || !selected.Open {
		t.Fatalf("confirmed retry: %+v %v", selected, err)
	}
	<-done
}

func TestRestartCannotLaunchAfterCancellationOrLossOfOwnership(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"cancelled", "another attempt", "identity write failed"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			install, _, running, seams, _ := restartBed(t, 12, false)
			seams.checkInstall, seams.checkAttempt = install, running.Attempt
			switch kind {
			case "cancelled":
				data, _ := json.Marshal(Result{Attempt: running.Attempt, Tree: running.Tree, Result: "cancelled"})
				if err := os.WriteFile(cancelPath(install, running.Attempt), data, 0600); err != nil {
					t.Fatal(err)
				}
			case "another attempt":
				seams.checkAttempt = "another"
			case "identity write failed":
				seams.Alive = func(Running) bool {
					if err := os.Remove(runningPath(install)); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(runningPath(install), 0700); err != nil {
						t.Fatal(err)
					}
					return true
				}

			}
			marker := filepath.Join(install, "started")
			done := make(chan error, 1)
			err := seams.launchCheck(exec.Command("/bin/sh", "-c", `echo started > "$1"; sleep 600`, "check", marker), os.DevNull, done)
			if err == nil {
				t.Fatal("an unowned command launched")
			}
			if kind != "identity write failed" {
				if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("unowned command ran: %v", err)
				}
			}
		})
	}
}

func TestRestartBeforeTheCommandLaunches(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, _ := restartBed(t, 12, false)
	done := make(chan error, 1)
	pid, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: []string{"/bin/sleep", "600"}, Completed: done})
	if err != nil {
		t.Fatal(err)
	}
	running.Pid, running.Process = pid, processRef(pid)
	t.Cleanup(func() { _ = (ProveSeams{}).endProof(Running{Process: running.Process}) })
	running.Admission = &ExecutionAdmission{State: "claimed", Scope: "full"}
	if err := writeRunning(install, running); err != nil {
		t.Fatal(err)
	}
	seams.Alive, seams.EndProof = nil, nil
	selected, err := SelectBatch(install, install, batch.Lane, seams)
	if err != nil || !selected.Open {
		t.Fatalf("unlaunched command blocked cancellation: %+v %v", selected, err)
	}
	<-done
}

func TestRestartRunRecordsTheUnlaunchedCommand(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	inspected := false
	b.seams.Command = func(command *exec.Cmd) error {
		current, recorded, _, err := ReadRunning(b.install, b.seams)
		if err != nil || !recorded || current.ProofProcess != "" {
			t.Fatalf("new check cannot distinguish its unlaunched command: %+v %v", current, err)
		}
		inspected = true
		return command.Run()
	}
	_, err := Run(b.install, b.checkout, passingEvidenceReport(), "", b.output, b.seams)
	if err != nil || !inspected {
		t.Fatalf("new command ownership was not recorded: %v inspected=%v", err, inspected)
	}
}

func TestRestartCancelledWrapperLeavesWorktreeForConfirmedCleanup(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	removed := false
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "worktree" && args[1] == "remove" {
			removed = true
		}
		return git(dir, args...)
	}
	b.seams.Command = func(command *exec.Cmd) error {
		running, recorded, _, err := ReadRunning(b.install, b.seams)
		if err != nil || !recorded {
			t.Fatalf("running command: %v", err)
		}
		cancel := Result{Attempt: running.Attempt, Tree: running.Tree, Result: "cancelled", Reason: "cancelled: restart for G"}
		data, err := json.Marshal(cancel)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(cancelPath(b.install, running.Attempt)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cancelPath(b.install, running.Attempt), data, 0600); err != nil {
			t.Fatal(err)
		}
		return errors.New("the command ended while its process group is being stopped")
	}
	result, err := Run(b.install, b.checkout, "fixture", "", b.output, b.seams)
	if err != nil || result.Result != "cancelled" {
		t.Fatalf("cancelled completion: %+v %v", result, err)
	}
	if removed {
		t.Fatal("the wrapper removed the worktree before cancellation confirmed termination")
	}
	if _, recorded, _, err := ReadRunning(b.install, b.seams); err != nil || !recorded {
		t.Fatalf("the wrapper discarded identities before cancellation confirmed termination: %v", err)
	}
}

func TestRestartCancelledTreeCanBeProvenAgainAfterReturn(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, _ := restartBed(t, 12, false)
	if _, err := SelectBatch(install, install, batch.Lane, seams); err != nil {
		t.Fatal(err)
	}
	newcomer := batch.Members[len(batch.Members)-1]
	if err := appendLine(queuePath(install), Line{Goal: newcomer.Goal, SHA: newcomer.SHA, Outcome: StateReturned, At: restartNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if err := withLock(install, func() error {
		previous, found, err := seams.checkBound(install, running.Tree)
		if err != nil || !found || previous.Result != "cancelled" {
			t.Fatalf("old tree cannot be proven again: %+v %v", previous, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	b := newScopeBed(t)
	cancelled, _, err := LastResult(install)
	if err != nil {
		t.Fatal(err)
	}
	cancelled.Tree, cancelled.Commit = b.git.tree, b.git.commit
	b.save(t, cancelled)
	b.seams.NewID = func() string { return "replacement" }
	result := b.run(t)
	if result.Result != Green || result.Attempt == cancelled.Attempt {
		t.Fatalf("old tree was not proved again: %+v", result)
	}
}

func TestRestartWaitsForExitedCommandToPublishFinishedResult(t *testing.T) {
	t.Parallel()
	b := newScopeBed(t)
	b.seams.Now = func() time.Time { return restartNow }
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }
	notAncestor := exec.Command("/usr/bin/false").Run()
	tick := b.seams
	tick.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "merge-base" {
			return "", notAncestor
		}
		return "main", nil
	}
	report := b.seams.Command
	b.seams.Command = func(command *exec.Cmd) error {
		running, recorded, _, err := ReadRunning(b.install, b.seams)
		if err != nil || !recorded {
			t.Fatalf("running: %v", err)
		}
		done := make(chan error, 1)
		wrapper, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: []string{"/bin/sleep", "600"}, Completed: done})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = syscall.Kill(-int(wrapper), syscall.SIGKILL); <-done })
		exited := make(chan error, 1)
		pid, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: []string{"/usr/bin/true"}, Completed: exited, Started: func(pid int64) error {
			process := processRef(pid)
			running.ProofProcess = process
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		if err := <-exited; err != nil {
			t.Fatal(err)
		}
		if pid == 0 {
			t.Fatal("missing exited command")
		}
		batch := &Batch{ID: "finishing", Base: "main", State: BatchRunning, Lane: lane.Record{Root: b.checkout, Install: b.install, CustodyEpoch: 1}, Members: []GoalSHA{{Goal: "old", SHA: "old-sha"}}, Selector: PolicyValue{Value: "auto"}}
		running.Pid, running.Process = wrapper, processRef(wrapper)
		running.BatchID, running.BatchMembers = batch.ID, batch.Members
		running.Since = restartNow.Add(-12 * time.Minute).Format(time.RFC3339)
		running.Admission.Scope = "full"
		if err := writeRunning(b.install, running); err != nil {
			t.Fatal(err)
		}
		if err := writeBatch(b.install, batch); err != nil {
			t.Fatal(err)
		}
		if err := appendLine(queuePath(b.install), Line{Goal: "old", SHA: "old-sha", At: running.Since}); err != nil {
			t.Fatal(err)
		}
		if err := appendLine(queuePath(b.install), Line{Goal: "new", SHA: "new-sha", At: restartNow.Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
		if _, err := SelectBatch(b.install, b.checkout, batch.Lane, tick); err != nil {
			t.Fatalf("finishing proof wedged the lane: %v", err)
		}
		if _, err := os.Stat(cancelPath(b.install, running.Attempt)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("finished command was cancelled: %v", err)
		}
		saved, err := ReadBatch(b.install)
		if err != nil || saved.Open || len(saved.Restarts) != 0 {
			t.Fatalf("finishing command recorded a restart: %+v %v", saved, err)
		}
		return report(command)
	}
	result := b.run(t)
	if result.Result != Green {
		t.Fatalf("finished result discarded: %+v", result)
	}
	if _, recorded, _, err := ReadRunning(b.install, b.seams); err != nil || recorded {
		t.Fatalf("finished proof wedged running record: %v", err)
	}
}

func TestRestartMedianCountsEachAttemptOnce(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0700); err != nil {
		t.Fatal(err)
	}
	for i, v := range []float64{38.8, 67.5, 45, 41, 44} {
		r := Result{Attempt: fmt.Sprint(i), Result: Green, Scope: "full", Minutes: &v}
		if err := appendLine(resultsPath(install), r); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			for range 4 {
				if err := appendLine(resultsPath(install), r); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if got, err := expectedFullMinutes(install); err != nil || got != 44 {
		t.Fatalf("duplicate attempts biased median: %g %v", got, err)
	}
}

func TestRestartClassificationWaitAndStaleReasons(t *testing.T) {
	t.Parallel()
	install, batch, running, seams, ended := restartBed(t, 12, false)
	if err := appendLine(resultsPath(install), Result{Attempt: running.Attempt, Tree: running.Tree, Result: Red, ClassificationPending: true}); err != nil {
		t.Fatal(err)
	}
	selected, err := SelectBatch(install, install, batch.Lane, seams)
	if err != nil || *ended != 0 || !slices.Contains(selected.Waiting, "shared-state-in-the-ledger waits: a red is being classified; the newcomer waits") {
		t.Fatalf("classification wait: %+v %v", selected, err)
	}
	if err := appendLine(resultsPath(install), Result{Attempt: running.Attempt, Tree: running.Tree, Result: Green}); err != nil {
		t.Fatal(err)
	}
	selected, err = SelectBatch(install, install, batch.Lane, seams)
	if err != nil || len(selected.Waiting) != 0 {
		t.Fatalf("completed proof retains stale reasons: %+v %v", selected, err)
	}
}

func TestRestartBatchWireFields(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(Batch{Open: false, SealedAt: "stamp"})
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record["open"] != false || record["sealed_at"] != "stamp" {
		t.Fatalf("batch fields diverged: %s", data)
	}
	data, err = json.Marshal(Batch{})
	if err != nil {
		t.Fatal(err)
	}
	record = nil
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if _, exists := record["sealed_at"]; exists {
		t.Fatalf("unsealed batch carries a seal stamp: %s", data)
	}
	install := t.TempDir()
	batch := &Batch{ID: "batch", Base: "main", Open: true, State: BatchRunning, Lane: lane.Record{Root: install, Install: install, CustodyEpoch: 1}}
	if err := writeBatch(install, batch); err != nil {
		t.Fatal(err)
	}
	read, err := ReadBatch(install)
	if err != nil || read == nil || !read.Open || read.SealedAt != "" {
		t.Fatalf("batch reader rejected an absent seal stamp: %+v %v", read, err)
	}
}

func TestRestartEndsGroupWhoseCommandLeaderExited(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	childPath := filepath.Join(install, "child.pid")
	done := make(chan error, 1)
	var process string
	pid, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{
		Argv: []string{"/bin/sh", "-c", `sleep 600 & echo $! > "$1"`, "proof", childPath}, Completed: done,
		Started: func(pid int64) error { process = processRef(pid); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-int(pid), syscall.SIGKILL) })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(childPath)
	if err != nil {
		t.Fatal(err)
	}
	var child int64
	if _, err := fmt.Sscan(string(data), &child); err != nil {
		t.Fatal(err)
	}
	ref, err := identity.ParseRef(processRef(child))
	if err != nil {
		t.Fatal(err)
	}
	clock := restartNow
	seams := ProveSeams{Now: func() time.Time { return clock }, WaitPoll: func() {
		testenv.Await(t, "the surviving command group stopped", func() bool {
			return identity.LiveRef(identity.KernelProber{}, ref) == identity.Dead && errors.Is(syscall.Kill(-int(pid), 0), syscall.ESRCH)
		})
		clock = clock.Add(time.Second)
	}}
	if err := seams.endProof(Running{ProofProcess: process}); err != nil {
		t.Fatal(err)
	}
	if identity.LiveRef(identity.KernelProber{}, ref) != identity.Dead {
		t.Fatal("the command's child survived")
	}
}

func TestRestartClearsWaitReasonsAfterPreemption(t *testing.T) {
	t.Parallel()
	install, batch, _, seams, _ := restartBed(t, 40, false)
	if _, _, err := HandIn(install, Line{Goal: "fix", SHA: "fix-sha", At: restartNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{Identity: "open", FixGoal: "fix"}}, nil
	}
	selected, err := SelectBatch(install, install, batch.Lane, seams)
	if err != nil || !selected.Open || len(selected.Waiting) != 0 {
		t.Fatalf("reopened batch retained the cancelled proof's waits: %+v %v", selected, err)
	}
}
