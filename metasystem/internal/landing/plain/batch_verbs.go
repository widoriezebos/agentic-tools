package plain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// BatchAction keeps a person's decision and its impact on the selected batch.
type BatchAction struct {
	Kind    string    `json:"kind"`
	By      string    `json:"by"`
	At      string    `json:"at"`
	With    []GoalSHA `json:"with,omitempty"`
	Impact  string    `json:"impact"`
	Pending bool      `json:"pending,omitempty"`
	Attempt string    `json:"attempt,omitempty"`
}

// ActOnBatch records decisions under the queue lock. A live proof keeps its
// membership until the keeper can cancel it through the restart owner.
func ActOnBatch(install, kind, goal string, person ActProvenance, s ProveSeams) (batch *Batch, message string, err error) {
	err = withLock(install, func() error {
		var readErr error
		batch, readErr = ReadBatch(install)
		if readErr != nil {
			return readErr
		}
		if batch == nil {
			return errors.New("there is no current batch")
		}
		entries, err := Entries(install)
		if err != nil {
			return err
		}
		act := BatchAction{Kind: kind, By: person.Person, At: person.CheckedAt.UTC().Format(time.RFC3339Nano)}
		switch kind {
		case "seal":
			if batch.SealedAt != "" {
				message = "already sealed"
				return nil
			}
			batch.SealedAt = act.At
			act.Impact = "the admission window is closed; newcomers wait until the batch is reopened"
		case "reopen", "restart":
			git := checkoutGit(batch.Lane.Root, s)
			main, err := git.main()
			if err != nil {
				return err
			}
			entries, err = Landed(entries, func(sha string) (bool, error) { return git.contains(main, sha) })
			if err != nil {
				return err
			}
			state := "never handed in"
			for _, e := range entries {
				if e.Goal == goal {
					state = e.State
				}
				if e.State == StateWaiting && (kind == "restart" || e.Goal == goal) && !slices.Contains(batch.Members, GoalSHA{Goal: e.Goal, SHA: e.SHA}) {
					act.With = append(act.With, GoalSHA{Goal: e.Goal, SHA: e.SHA})
				}
			}
			if kind == "reopen" && len(act.With) == 0 {
				if slices.ContainsFunc(batch.Members, func(m GoalSHA) bool { return m.Goal == goal }) {
					message = "already a member"
					return nil
				}
				return fmt.Errorf("goal %s is %s; it has no waiting hand-in", goal, state)
			}
			running, _, alive, err := ReadRunning(install, s)
			if err != nil {
				return err
			}
			if kind == "restart" && len(act.With) == 0 && !alive && batch.State == BatchPrepared && batch.SealedAt == "" {
				message = "already a member"
				return nil
			}
			act.Pending = alive
			act.Impact = "the batch is reopened; admitted goals must be merged and gated before a new proof"
			if alive {
				act.Attempt = running.Attempt
				for _, old := range batch.Actions {
					if old.Pending && old.Attempt == act.Attempt && old.Kind == kind && slices.Equal(old.With, act.With) {
						message = "request already pending"
						return nil
					}
				}
				act.Impact = "recorded; nothing acts on it until the proof ends — run reopen again then"
			} else {
				for _, m := range act.With {
					i := slices.IndexFunc(batch.Members, func(old GoalSHA) bool { return old.Goal == m.Goal })
					if i >= 0 {
						batch.Members[i] = m
					} else {
						batch.Members = append(batch.Members, m)
					}
				}
				batch.State, batch.SealedAt, batch.ClosureReason = BatchPrepared, "", ""
				person.Kind, person.Subject = "selection", slices.Clone(batch.Members)
				if s.Pause != nil {
					if pause, paused := s.Pause(); paused && !pause.Unreadable() {
						person.StandingPause = &pause
					}
				}
				batch.Person = &person
			}
		default:
			return errors.New("unknown batch action")
		}
		batch.Actions = append(batch.Actions, act)
		return writeBatch(install, batch)
	})
	return batch, message, err
}

// PendingLine names the proof attempt a recorded request waits for.
func (act BatchAction) PendingLine() string {
	when, _ := time.Parse(time.RFC3339Nano, act.At)
	return fmt.Sprintf("%s requested by %s at %s for attempt %s; waits for the proof to end", act.Kind, act.By, when.Local().Format("15:04"), act.Attempt)
}

// BatchLines describes membership using the checkout's actual containment.
func BatchLines(batch *Batch, entries []Entry, checkout string, s ProveSeams) ([]string, error) {
	if batch == nil {
		return []string{"no current batch"}, nil
	}
	state := "open"
	if batch.State == BatchClosed {
		state = "closed"
	} else if batch.SealedAt != "" || !batch.Open && batch.State == BatchRunning {
		at := batch.SealedAt
		if at == "" {
			if running, recorded, _, err := ReadRunning(batch.Lane.Install, s); err != nil {
				return nil, err
			} else if recorded && running.BatchID == batch.ID {
				at = running.Since
			}
		}
		when, err := time.Parse(time.RFC3339Nano, at)
		state = "sealed; time unknown"
		if err == nil {
			state = "sealed since " + when.Local().Format("2006-01-02 15:04:05 MST")
		}
	}
	lines := []string{"batch " + batch.ID + ": " + state}
	for _, act := range batch.Actions {
		if act.Pending {
			lines = append(lines, act.PendingLine(), act.Impact)
			continue
		}
		when, _ := time.Parse(time.RFC3339Nano, act.At)
		goals := []string{}
		for _, m := range act.With {
			goals = append(goals, m.Goal)
		}
		lines = append(lines, fmt.Sprintf("%s by %s at %s with [%s]: %s", act.Kind, act.By, when.Local().Format("2006-01-02 15:04:05 MST"), strings.Join(goals, ", "), act.Impact))
	}
	head, err := s.git(checkout, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		git := checkoutGit(checkout, s)
		main, err := git.main()
		if err != nil {
			return nil, err
		}
		entries, err = Landed(entries, func(sha string) (bool, error) { return git.contains(main, sha) })
		if err != nil {
			return nil, err
		}
	}
	for _, m := range batch.Members {
		merged, err := batchContains(checkout, head, m.SHA, s)
		if err != nil {
			return nil, err
		}
		state := "not merged"
		if merged {
			state = "merged"
		}
		lines = append(lines, m.Goal+": "+state)
	}
	for _, e := range entries {
		if e.State != StateWaiting || slices.Contains(batch.Members, GoalSHA{Goal: e.Goal, SHA: e.SHA}) {
			continue
		}
		reason := e.Reason
		if reason == "" && len(e.After) > 0 {
			goals := []string{}
			for _, m := range e.After {
				goals = append(goals, m.Goal)
			}
			reason = "conflict; waits after " + strings.Join(goals, ", ")
		}
		if reason == "" {
			reason = "waiting for admission to the batch"
		}
		if reason == "waiting for admission to the batch" && state != "open" {
			reason = "the batch is sealed; waits for the next batch or a person's restart"
		}
		lines = append(lines, e.Goal+" waits: "+reason)
	}
	return lines, nil
}
