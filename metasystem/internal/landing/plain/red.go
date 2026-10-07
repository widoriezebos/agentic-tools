package plain

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// redContinuationLocked reads current advice after the red report is durable.
// A proof's person admission covers its execution, never classification.
func redContinuationLocked(install string, result Result, seams ProveSeams) error {
	value := PolicyValue{Value: "auto", Source: "default"}
	var err error
	if seams.Policy != nil {
		value, err = seams.Policy("landing.on-red")
	}
	if err == nil && value.Value == "auto" {
		return nil
	}
	reason := "the recorded red needs a person's classification act"
	if err != nil {
		reason += "; the red policy cannot be read: " + err.Error()
	}
	stop := Stop{Loop: "lane-classify", Subject: result.Attempt, ProofAttempt: result.Attempt, Tree: result.Tree, BatchID: result.BatchID, Scope: result.Scope, Trunk: result.Trunk, Decision: "stop", Handoff: "ask lane", Cause: result.Cause, Class: reason, Evidence: result.Log, At: seams.now().Format(time.RFC3339Nano), Required: []string{"metasystem", "landing", "prove", "--classify", result.Attempt}}
	open, readErr := OpenStops(install)
	if readErr != nil {
		return readErr
	}
	for _, previous := range open {
		if previous.Loop == stop.Loop && previous.Subject == stop.Subject {
			return &Refusal{Code: "LANE_RED_PERSON", Reason: reason, Next: stop.Command()}
		}
	}
	if err := appendLine(stopsPath(install), stop); err != nil {
		return err
	}
	return &Refusal{Code: "LANE_RED_PERSON", Reason: reason, Next: stop.Command()}
}

// ClassifyAttempt attributes the immutable saved red. It claims the lane's
// execution slot and may admit one required full check, never a return.
func ClassifyAttempt(install, checkout, attempt string, seams ProveSeams) (result Result, err error) {
	var running Running
	err = withLock(install, func() error {
		if current, recorded, alive, err := ReadRunning(install, seams); err != nil {
			return err
		} else if recorded && alive {
			return &Busy{Running: current}
		} else if recorded && current.Admission != nil {
			// Recover a lost full execution before replacing its slot. Its
			// admission remains evidence that this classification spent its credit.
			if _, _, _, err := checkState(install, checkout, "", seams); err != nil {
				return err
			}
		}
		found := false
		for _, gate := range []bool{false, true} {
			mode := seams
			mode.Gate = gate
			results, err := readLines[Result](mode.resultsPath(install))
			if err != nil {
				return err
			}
			for _, r := range results {
				if r.Attempt == attempt && r.Result == Red {
					result, seams.Gate, seams.Trunk, found = r, gate, r.Trunk, true
				}
			}
		}
		if !found {
			return fmt.Errorf("no recorded red exists for attempt %s", attempt)
		}
		if seams.Person == nil {
			if err := redContinuationLocked(install, result, seams); err != nil {
				return err
			}
		}
		result.ClassificationPolicy = PolicyValue{Value: "auto", Source: "default"}
		if seams.Policy != nil {
			policy, readErr := seams.Policy("landing.on-red")
			result.ClassificationPolicy = policy
			if readErr != nil {
				result.ClassificationWarning = "the red policy cannot be read: " + readErr.Error()
			}
		}
		running = Running{Attempt: attempt + "-classify-" + seams.newID(), Commit: result.Commit, Tree: result.Tree, BatchID: result.BatchID, BatchMembers: result.BatchMembers, Gate: seams.Gate, Trunk: result.Trunk, Since: seams.now().Format(time.RFC3339Nano), Pid: int64(os.Getpid()), Process: processRef(int64(os.Getpid()))}
		// Store the act before any attribution command; the raw result stays intact.
		if seams.Person != nil {
			act := *seams.Person
			act.Kind = "classification"
			act.Subject = result.Goals
			result.ClassificationPerson = &act
			running.Person = &act
		}
		if err := writeRunning(install, running); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	defer func() {
		err = errors.Join(err, withLock(install, func() error {
			current, ok, _, readErr := ReadRunning(install, seams)
			if readErr != nil {
				return readErr
			}
			if ok && current.Attempt == running.Attempt {
				return os.Remove(runningPath(install))
			}
			return nil
		}))
	}()
	command := ""
	if seams.CommandForCommit == nil {
		return result, fmt.Errorf("the recorded commit's check declaration cannot be read")
	}
	if seams.Gate && seams.GateCommandForCommit != nil {
		seams.CommandForCommit = seams.GateCommandForCommit
	}
	command, err = seams.CommandForCommit(result.Commit)
	if err != nil {
		return result, err
	}
	tree := filepath.Join(proofTrees(install), running.Attempt)
	if err = os.MkdirAll(proofTrees(install), 0755); err != nil {
		return result, err
	}
	if _, err = seams.git(checkout, "worktree", "add", "--detach", tree, result.Commit); err != nil {
		return result, err
	}
	defer func() {
		if tree == "" {
			return
		}
		_, removeErr := seams.git(checkout, "worktree", "remove", "--force", tree)
		err = errors.Join(err, removeErr)
	}()
	dir := tree
	if rel, relErr := filepath.Rel(checkout, install); relErr == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		dir = filepath.Join(tree, rel)
	}
	// Replay ids follow the original report; no full admission is copied.
	original := running
	original.Attempt, original.Log = result.Attempt, result.Log
	decision := scopeDecision{scopeRecord: scopeRecord{Scope: result.Scope, Base: result.Base, ScopeReason: result.ScopeReason}}
	for i := len(result.Executions) - 1; i >= 0; i-- {
		admission := result.Executions[i]
		if admission.Scope == result.Scope {
			for _, group := range admission.Groups {
				decision.affected.Groups = append(decision.affected.Groups, testpolicy.AffectedGroup{Group: testpolicy.Group{ID: group}})
			}
			break
		}
	}
	result = continueRed(seams, install, checkout, command, dir, original, decision, io.Discard, result, Result{}, func(red, prior Result) Result {
		if seams.Gate {
			return replayGate(seams, install, checkout, command, original, red, prior)
		}
		return replayBatch(seams, install, checkout, command, original, red, prior)
	})
	err = withLock(install, func() error {
		if err := appendLine(seams.resultsPath(install), result); err != nil {
			return err
		}
		if err := recordProofStop(install, result); err != nil {
			return err
		}
		if result.ClassificationPerson != nil {
			return closeSubjectStopsLocked(install, "lane-classify", attempt, "person classified recorded red", seams.now())
		}
		return nil
	})
	if err == nil && result.Repeat == "allowed" && seams.Person != nil && !seams.Gate && result.Scope == "full" {
		ref := "HEAD"
		if result.Trunk {
			ref = "origin/main"
			if _, err := seams.git(checkout, "fetch", "origin", "main"); err != nil {
				return result, err
			}
		}
		head, readErr := seams.git(checkout, "rev-parse", "--verify", ref+"^{commit}")
		if readErr != nil {
			return result, readErr
		}
		if head != result.Commit {
			return result, nil
		}
		// Attribution grants at most one new full admission, within the retained
		// allowance. A historical red cannot authorize a replacement tree.
		boundErr := withLock(install, func() error {
			observations, skipped, err := countedLines[Result](seams.resultsPath(install))
			if err != nil {
				return err
			}
			if skipped != 0 {
				return holdProofLocked(install, running, seams, "LANE_PROOF_PERSON", "the classification continuation history cannot be read; request another check explicitly")
			}
			for _, observation := range observations {
				if observation.ClassificationOf == result.Attempt {
					return holdProofLocked(install, running, seams, "LANE_PROOF_PERSON", "classification already admitted its next full check; request another check explicitly")
				}
			}
			if !result.Trunk {
				if err := seams.checkBudget(install, checkout, result.Commit); err != nil {
					return holdProofLocked(install, running, seams, "LANE_PROOF_BUDGET", "classification retained the spent full-check allowance; request a fresh check explicitly")
				}
			}
			current, recorded, _, err := ReadRunning(install, seams)
			if err != nil {
				return err
			}
			if !recorded || current.Attempt != running.Attempt {
				return fmt.Errorf("the classification no longer owns its execution slot")
			}
			return os.Remove(runningPath(install))
		})
		if boundErr != nil {
			result.ClassificationWarning += "; " + boundErr.Error()
			return result, nil
		}
		if _, err = seams.git(checkout, "worktree", "remove", "--force", tree); err != nil {
			return result, err
		}
		tree = ""
		log, logErr := os.Create(filepath.Join(Dir(install), "proofs", running.Attempt+"-full.log"))
		if logErr != nil {
			return result, logErr
		}
		seams.ClassificationOf = result.Attempt
		declaration := seams.CommandForCommit
		seams.CommandForCommit = func(commit string) (string, error) {
			if commit != result.Commit {
				return "", &Refusal{Code: "LANE_PROOF_SUBJECT", Reason: "classification cannot authorize a replacement check target", Next: proofCommand(false, result.Trunk)}
			}
			return declaration(commit)
		}
		full, fullErr := Run(install, checkout, command, "", log, seams)
		closeErr := log.Close()
		result.NextFull = &full
		// The fresh red's own request remains pending; the classification act
		// has completed and supplies no permission for another execution.
		var refusal *Refusal
		if errors.As(fullErr, &refusal) && refusal.Code == "LANE_PROOF_BUDGET" {
			result.NextFull = nil
			result.ClassificationWarning += "; " + refusal.Error()
		} else if !errors.As(fullErr, &refusal) || refusal.Code != "LANE_RED_PERSON" {
			err = fullErr
		}
		err = errors.Join(err, closeErr)
	}
	return result, err
}
