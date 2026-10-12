package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Restart retains the decision made against this batch's running attempt.
type Restart struct {
	Goal     string  `json:"goal"`
	Elapsed  float64 `json:"elapsed"`
	Expected float64 `json:"expected"`
	At       string  `json:"at"`
	Reason   string  `json:"reason"`
}

func expectedFullMinutes(install string) (float64, error) {
	results, err := Results(install)
	if err != nil {
		return 0, err
	}
	var minutes []float64
	seen := map[string]bool{}
	for _, r := range slices.Backward(results) {
		if seen[r.Attempt] {
			continue
		}
		seen[r.Attempt] = true
		if r.Result == Green && r.Scope == "full" && r.Minutes != nil && *r.Minutes >= 0 && r.ClassificationOf == "" && !strings.HasPrefix(r.Reason, "inherits green") {
			minutes = append(minutes, *r.Minutes)
			if len(minutes) == 5 {
				break
			}
		}
	}
	if len(minutes) == 0 {
		return 45, nil
	}
	slices.Sort(minutes)
	return (minutes[(len(minutes)-1)/2] + minutes[len(minutes)/2]) / 2, nil
}

func cancelPath(install, attempt string) string {
	return filepath.Join(Dir(install), "proofs", attempt+".cancel.json")
}

func readCancel(install string, running Running) (*Result, error) {
	data, err := os.ReadFile(cancelPath(install, running.Attempt))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.Attempt != running.Attempt || result.Tree != running.Tree || result.Result != "cancelled" {
		return nil, errors.New("the cancellation does not match the running check")
	}
	return &result, nil
}

func finishCancel(install string, running Running, result Result) error {
	mode := ProveSeams{Gate: running.Gate}
	results, err := readLines[Result](mode.resultsPath(install))
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(results, func(r Result) bool { return r.Attempt == running.Attempt }) {
		if err := appendLine(mode.resultsPath(install), result); err != nil {
			return err
		}
	}
	current, recorded, _, err := ReadRunning(install, ProveSeams{})
	if err != nil {
		return err
	}
	if recorded && current.Attempt == running.Attempt {
		return os.Remove(runningPath(install))
	}
	return nil
}

func (s ProveSeams) launchCheck(command *exec.Cmd, env []string, log string, done chan<- error) error {
	launch := gaterun.DetachedLaunch{Argv: command.Args, Dir: command.Dir, Env: env, Log: log, Completed: done}
	if s.checkInstall == "" {
		_, err := gaterun.LaunchDetached(launch)
		return err
	}
	return withLock(s.checkInstall, func() error {
		current, recorded, _, err := ReadRunning(s.checkInstall, s)
		if err != nil {
			return err
		}
		if !recorded || current.Attempt != s.checkAttempt {
			return errors.New("the check no longer owns this execution")
		}
		cancel, err := readCancel(s.checkInstall, current)
		if err != nil {
			return err
		}
		if cancel != nil {
			return errors.New("the check was cancelled before launch")
		}
		launch.Started = func(pid int64) error {
			process := processRef(pid)
			if process == "" {
				return errors.New("the command's process identity could not be recorded")
			}
			current.ProofProcess = process
			return writeRunning(s.checkInstall, current)
		}
		_, err = gaterun.LaunchDetached(launch)
		return err
	})
}

func (s ProveSeams) endProof(r Running) error {
	if s.EndProof != nil {
		return s.EndProof(r)
	}
	if r.Pid > 0 && r.Process == "" {
		return errors.New("the wrapper's process identity is missing; the batch stays sealed")
	}
	var refs []identity.Ref
	for _, value := range []string{r.Process, r.ProofProcess} {
		if value == "" {
			continue
		}
		ref, err := identity.ParseRef(value)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		// The group can outlive its recorded leader, so signal the group directly.
		if err := syscall.Kill(-int(ref.Pid), syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
	}
	deadline := s.now().Add(5 * time.Second)
	for {
		stopped := true
		for _, ref := range refs {
			stopped = stopped && identity.LiveRef(identity.KernelProber{}, ref) == identity.Dead && errors.Is(syscall.Kill(-int(ref.Pid), 0), syscall.ESRCH)
		}
		if stopped {
			return nil
		}
		if !s.now().Before(deadline) {
			return errors.New("the check's processes have not stopped; the batch stays sealed")
		}
		if s.WaitPoll != nil {
			s.WaitPoll()
		} else {
			time.Sleep(25 * time.Millisecond)
		}
	}
}

func incidentFix(e Entry, incidents []goal.TrunkRedEntry) bool {
	return slices.ContainsFunc(incidents, func(i goal.TrunkRedEntry) bool {
		return i.Closed == nil && i.EntryClass() == goal.TrunkRedClassTrunkRed && i.FixGoal == e.Goal && (e.Fix == "" || e.Fix == i.Identity)
	})
}

// restartBatchLocked runs at selection, under the queue lock, before the
// keeper decides whether the detached proof still holds the agent.
func restartBatchLocked(install, checkout, main string, batch *Batch, s ProveSeams) error {
	batch.Waiting = nil
	running, recorded, alive, err := ReadRunning(install, s)
	if err != nil {
		return err
	}
	if !recorded || running.Gate || running.Impact || running.Trunk || running.BatchID != batch.ID || running.Admission != nil && running.Admission.Scope != "full" || running.Person != nil || running.ClassificationOf != "" {
		return writeBatch(install, batch)
	}
	last, found, err := ResultFor(install, running.Tree)
	if err != nil {
		return err
	}
	classifying := running.ClassificationOf != "" || found && last.Attempt == running.Attempt && last.ClassificationPending
	if found && last.Attempt == running.Attempt && !classifying {
		return writeBatch(install, batch)
	}
	cancel, err := readCancel(install, running)
	if err != nil {
		return err
	}
	if !alive && cancel == nil {
		return writeBatch(install, batch)
	}
	expected, err := expectedFullMinutes(install)
	if err != nil {
		return err
	}
	elapsed := elapsedMinutes(running.Since, s.now().Format(time.RFC3339))
	if elapsed == nil {
		return errors.New("the running check's elapsed time cannot be read")
	}
	incidents, err := s.incidents(install, checkout, main)
	if err != nil {
		return err
	}
	fixProving := slices.ContainsFunc(batch.Members, func(m GoalSHA) bool { return incidentFix(Entry{Goal: m.Goal}, incidents) })
	preempted := false
	newcomer := ""
	entries, err := batchEntries(install)
	if err != nil {
		return err
	}
	commonWhy, ordinaryWhy := "", ""
	switch {
	case batch.Person != nil:
		commonWhy = "this batch was selected by " + batch.Person.Person
	case fixProving:
		commonWhy = "the fix of main is proving"
	case classifying && cancel == nil:
		commonWhy = "a red is being classified; the newcomer waits"
	}
	if len(batch.Restarts) > 0 {
		ordinaryWhy = "this batch already restarted"
	} else if *elapsed >= expected/2 {
		ordinaryWhy = "proof at " + proofMinutes(*elapsed, expected) + ", past the restart point"
	}
	var newcomers []GoalSHA
	for _, e := range entries {
		if e.State != StateWaiting || e.Records || e.Held || len(e.After) > 0 || elapsedMinutes(running.Since, e.At) == nil || slices.ContainsFunc(batch.Members, func(m GoalSHA) bool { return m.Goal == e.Goal }) {
			continue
		}
		landed, err := batchContains(checkout, main, e.SHA, s)
		if err != nil {
			return err
		}
		if landed {
			continue
		}
		why := commonWhy
		fix := incidentFix(e, incidents)
		if why == "" && !fix && cancel == nil {
			why = ordinaryWhy
		}
		if why != "" {
			batch.Waiting = append(batch.Waiting, e.Goal+" waits: "+why)
		} else {
			newcomers = append(newcomers, GoalSHA{Goal: e.Goal, SHA: e.SHA})
			if newcomer == "" || fix {
				newcomer = e.Goal
			}
			preempted = preempted || fix
		}
	}
	if len(newcomers) == 0 {
		return writeBatch(install, batch)
	}
	reason := "restart for " + newcomer
	if preempted {
		reason = "restart: main is red; " + newcomer + " may be its fix"
	}
	if cancel == nil && running.ProofProcess != "" {
		ref, err := identity.ParseRef(running.ProofProcess)
		if err != nil {
			return err
		}
		// Let the wrapper publish an exited command's result before making a decision.
		if identity.LiveRef(identity.KernelProber{}, ref) != identity.Alive {
			return writeBatch(install, batch)
		}
	}
	if cancel == nil {
		r := timedResult(Result{Result: "cancelled", Reason: "cancelled: restart for " + newcomer, Attempt: running.Attempt, Tree: running.Tree, Commit: running.Commit, Log: running.Log, BatchID: running.BatchID, BatchMembers: running.BatchMembers, Goals: running.BatchMembers}, running.Since, s.now())
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err := atomicfile.WriteFile(cancelPath(install, running.Attempt), append(data, '\n'), 0600, install); err != nil {
			return err
		}
		cancel = &r
	}
	if !atomicfile.Confirm(cancelPath(install, running.Attempt), install) {
		return errors.New("the cancellation could not be made durable; the batch stays sealed")
	}
	if err := s.endProof(running); err != nil {
		return err
	}
	batch.Restarts = append(batch.Restarts, Restart{Goal: newcomer, Elapsed: *elapsed, Expected: expected, At: s.now().Format(time.RFC3339), Reason: reason})
	batch.Members = append(batch.Members, newcomers...)
	batch.Waiting = nil
	batch.State, batch.Open, batch.SealedAt = BatchPrepared, true, ""
	if err := writeBatch(install, batch); err != nil {
		return err
	}
	return finishCancel(install, running, *cancel)
}

func proofMinutes(elapsed, expected float64) string {
	return fmt.Sprintf("%.1f of %.1f min", elapsed, expected)
}
