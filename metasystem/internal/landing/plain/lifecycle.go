package plain

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

type lifecycleRecord struct {
	kind, id, reason, to string
	retire               func() error
}

func lifecycleRecords(install string, status Status, now time.Time, seams ProveSeams) (records []lifecycleRecord, err error) {
	events, _, err := state.ReadEvents(Dir(install))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, event := range events {
		if event.RecordID != "" && event.Kind != "defect" {
			seen[event.RecordID] = true
		}
	}
	add := func(kind, id string, facts state.LifecycleFacts, retire func() error) {
		if due, reason := state.LifecycleDue(kind, facts, now); due && !seen[kind+":"+id] {
			seen[kind+":"+id] = true
			for _, row := range state.Lifecycle {
				if row.Kind == kind {
					records = append(records, lifecycleRecord{kind, kind + ":" + id, lifecycleLabel(kind, id, facts) + ": " + reason, row.To, retire})
				}
			}
		}
	}
	proofs, proofErr := Results(install)
	gates, gateErr := readLines[Result](gatesPath(install))
	running, recorded, _, runningErr := ReadRunning(install, seams)
	stops, stopErr := OpenStops(install)
	err = errors.Join(proofErr, gateErr, runningErr, stopErr)
	resultSets := [][]Result{proofs, gates}
	if recorded && !running.Checkpoint {
		r := Result{Attempt: running.Attempt, Trunk: running.Trunk, BatchID: running.BatchID, Goals: running.BatchMembers}
		if running.Gate {
			gates = append(gates, r)
		} else {
			proofs = append(proofs, r)
		}
	}
	for _, stop := range stops {
		add("loop-stop", stopKey(stop), stopLifecycleFacts(stop, status, proofs, gates), func() error {
			return closeStopsLocked(install, "subject gone", now, func(s Stop) bool { return stopKey(s) == stopKey(stop) }, func(path string, s Stop) error { return appendRawLine(path, s) })
		})
	}
	for _, q := range status.Raw.AllQuestions {
		if q.About != "lane" || q.Lineage != lane.AgentLineage {
			continue
		}
		premise := state.Unknown
		for _, fact := range status.Raw.QuestionHealth {
			if fact.ID == q.ID {
				premise = fact.Premise
			}
		}
		if q.State == "closed" && len(q.Facts) > 1 && strings.HasPrefix(q.Facts[1], "lane stop: ") {
			premise = state.Gone
		}
		if q.State != "open" && !(len(q.Facts) > 1 && strings.HasPrefix(q.Facts[1], "lane stop: ")) {
			continue
		}
		add("question", q.ID, state.LifecycleFacts{Premise: premise, Detail: q.ClosedBecause}, func() error {
			_, err := channel.Withdraw(install, q.ID, "its premise is gone", nil, channel.DestinationConfig{})
			return err
		})
	}
	paths, pathErr := filepath.Glob(filepath.Join(Dir(install), "fixes", "*.json"))
	err = errors.Join(err, pathErr)
	for _, path := range paths {
		var fix Fix
		err = errors.Join(err, readAgentRecord(install, "fixes/"+filepath.Base(path), &fix))
		if fix.State == "closed" {
			continue
		}
		f := state.LifecycleFacts{Word: fix.State}
		subject := stopLifecycleFacts(Stop{Loop: "lane-return", Subject: fix.Goal, Tree: fix.Tip}, status, proofs, gates)
		f.Subject, f.Subjects, f.Detail = subject.Subject, subject.Subjects, subject.Detail
		f.Merge = mergeFacts(&fix, fix.Commit, fix.Tip)
		f.Merge.WritersSettled = fix.State == "done" || fix.State == "abandoned"
		if recorded && fix.Tip == "" {
			f.Attempt, f.CurrentAttempt = fix.Attempt, running.Attempt
		}
		add("fix", filepath.Base(path), f, func() error { fix.State = "closed"; return writeRawFix(install, &fix, now) })
	}
	for i, e := range status.Queue {
		stamp := e.LandedAt
		if e.State == StateReturned {
			stamp = e.ReturnedAt
		}
		at, _ := time.Parse(time.RFC3339Nano, stamp)
		ordinal := i
		if i < len(status.Raw.Ordinals) {
			ordinal = status.Raw.Ordinals[i]
		}
		add("submission", fmt.Sprintf("%s/%s/%d", e.Goal, e.SHA, ordinal), state.LifecycleFacts{Word: e.State, At: at}, nil)
	}
	for _, results := range resultSets {
		latest := latestResults(results)
		for i, r := range results {
			at, _ := time.Parse(time.RFC3339Nano, r.At)
			add("result", r.Attempt+"/"+r.At, state.LifecycleFacts{At: at, Latest: latest[r.Trunk] == i}, nil)
		}
	}
	if p := status.AgentProgress; p != nil && p.Step != nil {
		add("agent-step", fmt.Sprintf("%s/%d/%s", p.Step.StartedAt, p.Step.PID, p.Step.Process), state.LifecycleFacts{ProcessGone: p.ProcessGone}, nil)
	}
	if b := status.Raw.Batch; b != nil {
		add("batch", b.ID+"/"+b.ClosedAt, state.LifecycleFacts{Word: b.State, Empty: len(b.Members) == 0}, nil)
	}
	return records, err
}

func retireLifecycle(install string, status Status, now time.Time, seams ProveSeams, only string) error {
	return withLock(install, func() error {
		if status.Raw.Anchors.Lane.Install != "" {
			queue, err := readLines[Line](queuePath(install))
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(queue, status.Raw.Queue) {
				return &state.Stale{}
			}
		}
		records, err := lifecycleRecords(install, status, now, seams)
		if err != nil {
			return err
		}
		for _, record := range records {
			if only != "" && record.kind != only {
				continue
			}
			if record.retire != nil {
				s := seams
				s.TransitionBy, s.TransitionEvidence = "keeper", record.reason
				if err := transitionWrite(install, record.id, "keeper", now, record.to, record.retire, s); err != nil {
					return err
				}
				continue
			}
			mark := func(shadow *state.Record) {
				switch record.kind {
				case "agent-step":
					p := status.AgentProgress
					if p != nil && p.Step != nil && shadow.Agent.Step.StartedAt == p.Step.StartedAt && shadow.Agent.Step.Verb == p.Step.Verb && reflect.DeepEqual(shadow.Agent.Step.Args, p.Step.Args) {
						shadow.Agent.Word = record.to
					}
				case "batch":
					b := status.Raw.Batch
					if b != nil && shadow.Batch.ID == b.ID && shadow.Batch.Execution.Word == BatchClosed {
						shadow.Batch.Execution.Word = record.to
					}
				}
			}
			if _, err := state.Transition(Dir(install), now, record.id, "present", record.to, "keeper", record.reason, mark); err != nil {
				return err
			}
		}
		return nil
	})
}

// RefreshLifecycle runs after the keeper's shadow refresh and uses raw records.
func RefreshLifecycle(home string, now time.Time, seams ProveSeams) error {
	r, present, err := lane.Read(home)
	if err != nil || !present {
		return err
	}
	seams.Now, seams.ReadOnly, seams.BudgetHome = func() time.Time { return now }, true, home
	status := ObserveStatus(home, r, seams, nil)
	if len(status.Problems) > 0 {
		return errors.New(strings.Join(status.Problems, "; "))
	}
	if err := retireLifecycle(r.Install, status, now, seams, ""); err != nil {
		return err
	}
	_, err = RefreshShadow(home, now, "keeper", seams)
	return err
}

func lifecycleExpired(kind, word string, at, now time.Time) bool {
	due, _ := state.LifecycleDue(kind, state.LifecycleFacts{Word: word, At: at}, now)
	return due
}

func RecordStopPerson(install string, person *ActProvenance) error {
	return withLock(install, func() error { return writeAgentRecord(install, "stop-person.json", person) })
}

// Result files are append-only; each kind's last row remains current evidence.
func latestResults(results []Result) map[bool]int {
	latest := map[bool]int{}
	for i, result := range results {
		latest[result.Trunk] = i
	}
	return latest
}

func retainedResults(results []Result, now time.Time) []Result {
	latest := latestResults(results)
	var retained []Result
	for i, result := range results {
		at, _ := time.Parse(time.RFC3339Nano, result.At)
		due, _ := state.LifecycleDue("result", state.LifecycleFacts{At: at, Latest: latest[result.Trunk] == i}, now)
		if !due {
			retained = append(retained, result)
		}
	}
	return retained
}

func lifecycleLabel(kind, id string, facts state.LifecycleFacts) string {
	if kind == "loop-stop" {
		return "a loop stop of " + facts.At.Local().Format("2006-01-02 15:04") + " survived"
	}
	return kind + " " + id
}
