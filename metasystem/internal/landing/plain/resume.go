package plain

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type ResumeRun struct {
	Lines   []string
	Agent   lane.AgentRun
	Changed bool
}

// ContinueSelection asks the keeper to execute a durable person's selection.
func ContinueSelection(install string, record lane.Record, keeper lane.AgentKeeper) lane.AgentRun {
	keeper.Explicit, keeper.PersonAct = true, true
	keeper.PersonSelection = func(r lane.Record) bool { _, err := RecordedPersonBatch(install, r, ""); return err == nil }
	keeper.Prepare = func(r lane.Record) error { _, err := RecordedPersonBatch(install, r, ""); return err }
	keeper.Continuation = func(r lane.Record) string { return PersonBatchContinuation(install, r, keeper.Home) }
	return keeper.Run()
}

// Resume recovers only work whose recorded owners permit the next transition.
func Resume(home string, person ActProvenance, now time.Time, s ProveSeams) (run ResumeRun, err error) {
	record, present, err := lane.Read(home)
	if err != nil || !present {
		return run, errors.Join(err, errors.New("no landing lane is registered"))
	}
	if s.ResumeKeeper == nil {
		return run, errors.New("the landing agent's keeper is unavailable")
	}
	s.Now = func() time.Time { return now }
	person.CheckedAt, person.Destination = now, record
	keeper := *s.ResumeKeeper
	if err := s.fetchMain(record.Root); err != nil {
		return run, err
	}
	blocked := false
	err = withLock(record.Install, func() error {
		held, err := lock.File(lane.LockPath(home), 0600, lock.Exclusive)
		if err != nil {
			return err
		}
		defer held.Release()
		current, ok, err := lane.Read(home)
		if err != nil || !ok || current != record {
			return errors.New("the lane registration changed")
		}
		if _, fenced, err := lane.ReadUnset(home); err != nil || fenced {
			return errors.New("the landing lane is being unset")
		}
		tip, err := s.git(record.Root, "rev-parse", "--verify", "MERGE_HEAD^{commit}")
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 128 {
				return nil
			}
			return err
		}
		fix, fixErr := readFixRecords(record.Install, false, true, true)
		head, headErr := s.git(record.Root, "rev-parse", "--verify", "HEAD^{commit}")
		if err := errors.Join(fixErr, headErr); err != nil {
			return err
		}
		blocked = true
		run.Lines = append(run.Lines, "the current merge is preserved; the agent continues it")
		facts := mergeFacts(fix, head, tip)
		if state.MergeOwnership(facts).Word == state.Unknown {
			return nil
		}
		settled, minutes, err := ResolutionWritersSettled(record.Install, *fix, s)
		if err != nil {
			return err
		}
		facts.WritersSettled = settled
		if state.MergeOwnership(facts).Word == state.Owned && !settled {
			run.Lines[0] = "the resolution job of " + fix.Goal + fmt.Sprintf(" still runs (%.1f min); give it back or wait", minutes)
			if fix.State == "resolving" && fix.Job == "" {
				run.Lines[0] = "the resolution of " + fix.Goal + " has no job; the agent continues it"
			}
			return nil
		}
		if fix.State == "resolving" {
			main, err := checkoutGit(record.Root, s).main()
			if err != nil {
				return err
			}
			move, err := mainMove(record.Root, fix.Commit, main, s)
			if err != nil {
				return err
			}
			facts.MainMoved = move.Kind == "code"
		}
		if state.MergeOwnership(facts).Word != state.Abandoned {
			return nil
		}
		if fix.State != "abandoned" {
			fix.State, fix.Reason = "abandoned", "superseded by main's code move; rebuild"
			if err := WriteFix(record.Install, fix); err != nil {
				return err
			}
		}
		if _, err := s.git(record.Root, "merge", "--abort"); err != nil {
			return err
		}
		// Abandoned resolutions retain their reason; ordinary readers exclude them.
		if err := WriteFix(record.Install, fix); err != nil {
			return err
		}
		blocked = false
		run.Changed = true
		run.Lines[0] = "aborted the stale merge of " + fix.Goal
		return nil
	})
	if err != nil {
		return run, err
	}
	if blocked {
		return resumeContinuation(home, record, keeper, run)
	}
	_, live, agentErr := keeper.Running()
	batch, batchErr := ReadBatch(record.Install)
	state, stateErr := lane.ReadAgentState(home)
	_, paused := lane.ReadPause(home)
	drain, drainErr := ReadDrain(record.Install)
	_, _, proofLive, proofErr := ReadRunning(record.Install, s)
	if err := errors.Join(agentErr, batchErr, stateErr, drainErr, proofErr); err != nil {
		return run, err
	}
	if live && !paused && drain == nil && state.Barren < 2 && len(run.Lines) == 0 {
		id := "none"
		if batch != nil {
			id = batch.ID
		}
		run.Lines = []string{"nothing to resume: the lane is running batch " + id}
		return run, nil
	}
	if proofLive {
		drainChanged, drainErr := ClearDrain(record.Install)
		pauseChanged, pauseErr := lane.ClearPause(home)
		run.Changed = run.Changed || drainChanged || pauseChanged
		run.Lines = append(run.Lines, "the running proof continues")
		return run, errors.Join(drainErr, pauseErr)
	}
	entries, queueErr := pending(record.Install, record.Root, s)
	policy, policyErr := s.batchPolicy()
	main, mainErr := checkoutGit(record.Root, s).main()
	incidents, incidentErr := s.incidents(record.Install, record.Root, main)
	results, resultErr := Results(record.Install)
	if err := errors.Join(queueErr, policyErr, mainErr, incidentErr, resultErr); err != nil {
		return run, err
	}
	slices.SortStableFunc(entries, func(a, b Entry) int { return strings.Compare(a.At, b.At) })
	if len(entries) > 0 && (state.Barren >= 2 || policy.Value == "person") {
		goals := []string{}
		for _, e := range entries {
			goals = append(goals, e.Goal)
		}
		batch, err = PersonSelection(record.Install, record.Root, record, goals, person, s)
		if err != nil {
			return run, err
		}
		run.Lines = append(run.Lines, "recorded the selection "+strings.Join(goals, ", "))
		run.Changed = true
	}
	if batch != nil && batch.State != BatchClosed {
		move, err := mainMove(record.Root, batch.Base, main, s, batch.Members...)
		if err != nil {
			return run, err
		}
		if move.Kind == "code" && (batch.SealedAt != "" || !batch.Open && batch.State == BatchRunning) {
			_, _, err = ActOnBatch(record.Install, "restart", "", person, s)
			if err != nil {
				return run, err
			}
			run.Lines = append(run.Lines, "restarted the batch on main "+Short(main))
			run.Changed = true
		} else if batch.Open {
			for _, e := range entries {
				if !slices.ContainsFunc(batch.Members, func(m GoalSHA) bool { return m.Goal == e.Goal }) {
					batch, _, err = ActOnBatch(record.Install, "reopen", e.Goal, person, s)
					if err != nil {
						return run, err
					}
					run.Lines = append(run.Lines, "admitted "+e.Goal)
					run.Changed = true
				}
			}
		}
	}
	proved := slices.ContainsFunc(results, func(r Result) bool {
		return r.Trunk && r.Commit == main && !r.ClassificationPending && (r.Result == Green || r.Result == Red)
	})
	clearHold := state.Barren >= 2 && len(entries) == 0
	if clearHold || len(openIncidents(incidents)) > 0 && !proved {
		s.Trunk, s.Gate, s.Impact, s.Fresh = true, false, false, clearHold
		person.Kind = "proof"
		s.Person, s.FenceCheck = &person, nil
		_, _, err := Start(record.Install, record.Root, s)
		if err != nil {
			return run, err
		}
		line := "proving main"
		if clearHold {
			line += " to clear the hold"
		}
		run.Lines = append(run.Lines, line)
		run.Changed = true
	}
	return resumeContinuation(home, record, keeper, run)
}

func resumeContinuation(home string, record lane.Record, keeper lane.AgentKeeper, run ResumeRun) (ResumeRun, error) {
	batch, err := ReadBatch(record.Install)
	if err != nil {
		return run, err
	}
	changed, err := ClearDrain(record.Install)
	run.Changed = run.Changed || changed
	if err != nil {
		return run, err
	}
	changed, err = lane.ClearPause(home)
	run.Changed = run.Changed || changed
	if err != nil {
		return run, err
	}
	if batch != nil && batch.Person != nil {
		run.Agent = ContinueSelection(record.Install, record, keeper)
	} else {
		keeper.Explicit, keeper.PersonAct = true, true
		run.Agent = keeper.Run()
	}
	line := run.Agent.Line
	if run.Agent.Outcome == lane.AgentStarted {
		line = "started the agent"
		run.Changed = true
	}
	run.Lines = append(run.Lines, line)
	return run, nil
}

func resolutionSettled(fix Fix, s ProveSeams) (bool, float64, error) {
	if fix.Job == "" {
		return fix.State == "abandoned", 0, nil
	}
	store := launch.Store{}
	if s.ResolutionStore != nil {
		store = *s.ResolutionStore
	}
	record, err := store.Read(fix.Job)
	if err != nil {
		return false, 0, fmt.Errorf("read resolution job %s: %w", fix.Job, err)
	}
	minutes := 0.0
	if elapsed := elapsedMinutes(record.StartedAt, s.now().Format(time.RFC3339)); elapsed != nil {
		minutes = *elapsed
	}
	return record.State.Terminal() && !record.OutputOwnerUnproven, minutes, nil
}

// ResolutionWritersSettled permits discarding a merge only after its recorded
// job, landing agent and regeneration have relinquished the checkout.
func ResolutionWritersSettled(install string, fix Fix, s ProveSeams) (bool, float64, error) {
	settled, minutes, jobErr := resolutionSettled(fix, s)
	if s.ResumeKeeper == nil || s.ResumeKeeper.Running == nil {
		return false, minutes, errors.Join(jobErr, errors.New("the landing agent's keeper is unavailable"))
	}
	_, live, agentErr := s.ResumeKeeper.Running()
	regen, regenErr := ReadRunningRegeneration(install, s)
	return settled && !live && (regen == nil || regen.State != "running"), minutes, errors.Join(jobErr, agentErr, regenErr)
}

func mergeFacts(fix *Fix, head, tip string) state.MergeFacts {
	if fix == nil {
		return state.MergeFacts{Head: head, MergeTip: tip}
	}
	return state.MergeFacts{Present: true, Job: fix.Job, State: fix.State, Checkpoint: fix.Commit, Tip: fix.Tip, Head: head, MergeTip: tip, Closed: fix.State == "closed"}
}
