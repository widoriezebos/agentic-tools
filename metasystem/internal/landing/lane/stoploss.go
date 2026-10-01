package lane

// What is left of the lane's budgets: the per-batch allowance that landing
// begin and landing prove still charge (lane runtime design r10 K10). The
// keeper's budgets (the session deadline, the crash breaker, the spend and
// its daily ceiling, the hit alerts) are gone with the simple lane (unit
// B); this file goes once begin and prove no longer charge (unit A).
//
// The store is read and written under the host lane flock: a caller of a
// ...Held function holds it (inside Gate).

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"
)

// The allowance (D4, K10).
const (
	AllowanceExecutions = 4
	AllowanceWindow     = 2 * time.Hour
	// ValidationWindow is a validation session's own launch-bound limit: a
	// session woken for no batch work is bound to its own launch.
	ValidationWindow = 2 * time.Hour
)

// The stop-loss refusal codes (the refusal register names their sites).
const (
	// CodeAllowanceSpent is an execution refused because its batch spent
	// its allowance (executions or time).
	CodeAllowanceSpent = "LANE_ALLOWANCE_SPENT"
	// CodeFreshSession is a begin refused because the session already took
	// up another batch: one fresh session per batch.
	CodeFreshSession = "LANE_ONE_SESSION_PER_BATCH"
)

// HitAllowance is the kind of a spent allowance's hit.
const HitAllowance = "allowance"

// The kinds of a session clock.
const (
	ClockBatch      = "batch"
	ClockValidation = "validation"
)

// StopLoss is the lane's budget store.
type StopLoss struct {
	Schema int `json:"schema"`
	// Grant is the allowance in force; a person's resume adds one.
	Grant     int    `json:"grant"`
	GrantedAt string `json:"grantedAt,omitempty"`
	GrantedBy string `json:"grantedBy,omitempty"`
	// Clock is the running session's deadline clock.
	Clock    *SessionClock    `json:"clock,omitempty"`
	Batches  []BatchAllowance `json:"batches,omitempty"`
	Launches []LaunchSpend    `json:"launches,omitempty"`
	Hits     []Hit            `json:"hits,omitempty"`
	// History is the typed audit of what the budgets did.
	History []StopLossEvent `json:"history,omitempty"`
}

// SessionClock is when the session's allowance began: the launch of the
// session that took the work up, kept by the recovery sessions after it.
type SessionClock struct {
	Grant  int    `json:"grant"`
	Kind   string `json:"kind"`
	Since  string `json:"since"`
	Launch string `json:"launch"`
}

// BatchAllowance is one batch's allowance under one grant.
type BatchAllowance struct {
	Batch      string   `json:"batch"`
	Grant      int      `json:"grant"`
	Since      string   `json:"since"`
	Launches   []string `json:"launches,omitempty"`
	Executions int      `json:"executions"`
}

// LaunchSpend is one landing launch: how it started and, once reaped, how
// it ended and what it spent.
type LaunchSpend struct {
	Launch    string   `json:"launch"`
	Grant     int      `json:"grant"`
	StartedAt string   `json:"startedAt,omitempty"`
	Reasons   []string `json:"reasons,omitempty"`
	// Recovery is a start that resumes an unfinished batch.
	Recovery bool   `json:"recovery,omitempty"`
	EndedAt  string `json:"endedAt,omitempty"`
	State    string `json:"state,omitempty"`
	// Unsuccessful is a launch that did not complete, or did not start.
	Unsuccessful bool   `json:"unsuccessful,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
	// AcceptedBy is the person whose resume went past unknown usage.
	AcceptedBy string `json:"acceptedBy,omitempty"`
}

// Usage is a launch's reconciled token use.
type Usage struct {
	Known  bool  `json:"known"`
	Tokens int64 `json:"tokens"`
	// Source is measure (the adapter's own) or transcript.
	Source string `json:"source,omitempty"`
	Why    string `json:"why,omitempty"`
}

// Hit is a batch's spent allowance, recorded with the alert it owed.
type Hit struct {
	At      string `json:"at"`
	Kind    string `json:"kind"`
	Work    string `json:"work"`
	Message string `json:"message"`
	Alerted string `json:"alerted,omitempty"`
}

// StopLossEvent is one entry of the typed audit.
type StopLossEvent struct {
	At     string `json:"at"`
	Verb   string `json:"verb"`
	Batch  string `json:"batch,omitempty"`
	Launch string `json:"launch,omitempty"`
	Detail string `json:"detail,omitempty"`
}

const (
	stopLossSchema = 1
	// historyCap bounds the audit; the oldest entries go first.
	historyCap = 1000
	// keepFor bounds what the store keeps of ended launches and allowances
	// of earlier grants; open budgets are always kept.
	keepFor = 8 * 24 * time.Hour
)

func stopLossPath(home string) string { return filepath.Join(HostDir(home), "landing-stoploss.json") }

// ReadStopLoss reads the budget store; the zero store when none is kept.
// An unreadable store is an error, which every gate holds on.
func ReadStopLoss(home string) (StopLoss, error) {
	var store StopLoss
	ok, err := readJSON(stopLossPath(home), &store)
	if err != nil {
		return StopLoss{}, fmt.Errorf("the landing lane's budget store can't be read, so nothing starts: %w", err)
	}
	if !ok {
		return StopLoss{Schema: stopLossSchema}, nil
	}
	if store.Schema != stopLossSchema {
		return StopLoss{}, fmt.Errorf("the landing lane's budget store %s has schema %d, not %d", stopLossPath(home), store.Schema, stopLossSchema)
	}
	return store, nil
}

func writeStopLoss(home string, store StopLoss, now time.Time) error {
	store.Schema = stopLossSchema
	cutoff := now.Add(-keepFor)
	store.Launches = slices.DeleteFunc(store.Launches, func(spend LaunchSpend) bool {
		ended, err := time.Parse(time.RFC3339, spend.EndedAt)
		open := spend.Usage != nil && !spend.Usage.Known && spend.AcceptedBy == ""
		return err == nil && ended.Before(cutoff) && !open
	})
	store.Batches = slices.DeleteFunc(store.Batches, func(allowance BatchAllowance) bool {
		since, err := time.Parse(time.RFC3339, allowance.Since)
		return allowance.Grant != store.Grant && err == nil && since.Before(cutoff)
	})
	store.Hits = slices.DeleteFunc(store.Hits, func(hit Hit) bool {
		at, err := time.Parse(time.RFC3339, hit.At)
		return hit.Alerted != "" && err == nil && at.Before(cutoff)
	})
	if len(store.History) > historyCap {
		store.History = store.History[len(store.History)-historyCap:]
	}
	return writeJSON(home, stopLossPath(home), store)
}

func (s *StopLoss) record(now time.Time, verb, batch, launch, detail string) {
	s.History = append(s.History, StopLossEvent{At: stamp(now), Verb: verb, Batch: batch, Launch: launch, Detail: detail})
}

func stamp(now time.Time) string { return now.UTC().Format(time.RFC3339) }

// updateStopLossHeld reads, changes and writes the store; the caller holds
// the lane flock.
func updateStopLossHeld(home string, now time.Time, change func(*StopLoss) error) error {
	store, err := ReadStopLoss(home)
	if err != nil {
		return err
	}
	changeErr := change(&store)
	if err := writeStopLoss(home, store, now); err != nil {
		return errors.Join(changeErr, err)
	}
	return changeErr
}

// recordHit records a hit whose alert is owed, without pausing the lane: an
// allowance hit. A hit for work already hit is not recorded twice.
func (s *StopLoss) recordHit(kind, work, message string, now time.Time) {
	if !slices.ContainsFunc(s.Hits, func(hit Hit) bool { return hit.Work == work }) {
		s.Hits = append(s.Hits, Hit{At: stamp(now), Kind: kind, Work: work, Message: message})
		s.record(now, "hit", "", "", kind+": "+message)
	}
}

// allowanceWork is the hit identity of batch's spent allowance under grant.
func allowanceWork(batch string, grant int) string {
	return fmt.Sprintf("allowance:%s:%d", batch, grant)
}

// AllowanceSpent names a batch whose allowance under the grant in force is
// spent, for a person's landing start to grant a fresh one; false when none
// is. An unreadable store is an error.
func AllowanceSpent(home string) (string, bool, error) {
	store, err := ReadStopLoss(home)
	if err != nil {
		return "", false, err
	}
	for _, hit := range store.Hits {
		if hit.Kind != HitAllowance {
			continue
		}
		for _, allowance := range store.Batches {
			if allowance.Grant == store.Grant && hit.Work == allowanceWork(allowance.Batch, store.Grant) {
				return allowance.Batch, true, nil
			}
		}
	}
	return "", false, nil
}

// allowance finds or opens batch's allowance under the grant in force.
func (s *StopLoss) allowance(batch string, since time.Time) *BatchAllowance {
	for index := range s.Batches {
		if s.Batches[index].Batch == batch && s.Batches[index].Grant == s.Grant {
			return &s.Batches[index]
		}
	}
	s.Batches = append(s.Batches, BatchAllowance{Batch: batch, Grant: s.Grant, Since: stamp(since)})
	return &s.Batches[len(s.Batches)-1]
}

// currentLaunch is the landing launch the keeper started and has not yet
// reaped: the session a kernel operation belongs to. Empty when none runs
// (an operation a person runs by hand, or a test).
func currentLaunch(home string) (string, error) {
	state, err := ReadAgentState(home)
	if err != nil {
		return "", errors.New(UnreadableAgentRecord(home))
	}
	if state.ReapedAt != "" {
		return "", nil
	}
	return state.Launch, nil
}

// sessionDeadline is when the clock of launch runs out; false when the
// clock is not that launch's.
func (s *StopLoss) sessionDeadline(launch string) (time.Time, bool) {
	clock := s.Clock
	if clock == nil || launch == "" || clock.Launch != launch || clock.Grant != s.Grant {
		return time.Time{}, false
	}
	since, err := time.Parse(time.RFC3339, clock.Since)
	if err != nil {
		return time.Time{}, false
	}
	if clock.Kind == ClockValidation {
		return since.Add(ValidationWindow), true
	}
	return since.Add(AllowanceWindow), true
}

// allowanceRefusal is an execution refused for a spent allowance.
func allowanceRefusal(message string) *Refusal {
	return &Refusal{Code: CodeAllowanceSpent, Message: message + "; work already proven green still publishes, and a person grants a fresh allowance",
		Fix: "a person checks the alert and grants a fresh allowance: metasystem landing start", Argv: []string{"metasystem", "landing", "start"}}
}

// BindBatchHeld takes batch up for the running session at landing begin:
// the batch's allowance starts at the session's clock, and a session that
// already took up another batch is refused (one fresh session per batch).
// The caller holds the lane flock (inside Gate).
func BindBatchHeld(home, batch string, now time.Time) error {
	launch, err := currentLaunch(home)
	if err != nil {
		return err
	}
	// A repeat whose binding holds is success with no second record
	// (R-129-ui): the store is not written again.
	if current, err := ReadStopLoss(home); err == nil && current.boundTo(batch, launch) {
		return nil
	}
	return updateStopLossHeld(home, now, func(store *StopLoss) error {
		if launch != "" {
			for _, other := range store.Batches {
				if other.Grant == store.Grant && other.Batch != batch && slices.Contains(other.Launches, launch) {
					return &Refusal{Code: CodeFreshSession,
						Message: fmt.Sprintf("this landing session already took up batch %s, and each batch gets a fresh session, so batch %s was not begun", other.Batch, batch),
						Fix:     "end this session; the keeper starts a fresh one for the batch: metasystem landing status", Argv: []string{"metasystem", "landing", "status"}}
				}
			}
		}
		allowance := store.allowance(batch, store.clockSince(launch, now))
		if launch != "" && !slices.Contains(allowance.Launches, launch) {
			allowance.Launches = append(allowance.Launches, launch)
		}
		store.record(now, "begin", batch, launch, "")
		return nil
	})
}

// boundTo is whether batch's allowance under the grant in force already
// holds launch (any allowance when no session runs) and no other batch
// of that grant holds it: binding again would change nothing.
func (s *StopLoss) boundTo(batch, launch string) bool {
	bound := false
	for _, allowance := range s.Batches {
		if allowance.Grant != s.Grant {
			continue
		}
		if allowance.Batch == batch {
			bound = launch == "" || slices.Contains(allowance.Launches, launch)
		} else if launch != "" && slices.Contains(allowance.Launches, launch) {
			return false
		}
	}
	return bound
}

// clockSince is when the running session's allowance began: its clock, or
// now when no clock is the launch's.
func (s *StopLoss) clockSince(launch string, now time.Time) time.Time {
	if s.Clock != nil && launch != "" && s.Clock.Launch == launch && s.Clock.Grant == s.Grant {
		if since, err := time.Parse(time.RFC3339, s.Clock.Since); err == nil {
			return since
		}
	}
	return now
}

// ChargeHeld charges one execution (landing prove, any subject) to batch's
// allowance before its child starts. A spent allowance (its executions, or
// the session's time) refuses the execution and records the hit, whose
// alert asks a person for a fresh allowance. It does not pause the lane
// (R8-08): a pause would refuse publish and the agent's returns, and work
// already admitted and green must still publish; every further execution
// of the batch is refused here until a person grants a fresh allowance.
// The caller holds the lane flock (inside Gate).
func ChargeHeld(home, batch string, now time.Time) error {
	launch, err := currentLaunch(home)
	if err != nil {
		return err
	}
	return updateStopLossHeld(home, now, func(store *StopLoss) error {
		allowance := store.allowance(batch, store.clockSince(launch, now))
		if launch != "" && !slices.Contains(allowance.Launches, launch) {
			allowance.Launches = append(allowance.Launches, launch)
		}
		if deadline, ok := store.sessionDeadline(launch); ok && !now.Before(deadline) {
			message := fmt.Sprintf("batch %s's session ran out of its %s at %s, so no further test run was started", batch, AllowanceWindow, localClock(deadline))
			store.recordHit(HitAllowance, allowanceWork(batch, store.Grant), message, now)
			return allowanceRefusal(message)
		}
		if allowance.Executions >= AllowanceExecutions {
			message := fmt.Sprintf("batch %s used its allowance of %d test runs, so a further one was not started", batch, AllowanceExecutions)
			store.record(now, "prove-refused", batch, launch, message)
			store.recordHit(HitAllowance, allowanceWork(batch, store.Grant), message, now)
			return allowanceRefusal(message)
		}
		allowance.Executions++
		store.record(now, "prove", batch, launch, fmt.Sprintf("execution %d of %d", allowance.Executions, AllowanceExecutions))
		return nil
	})
}

// Grant is a person's resume: a fresh allowance (a new grant, so every
// batch's executions, the breaker and the daily count start again) and
// their word past usage that could not be read. The history is kept. A
// session the keeper has not seen end keeps its clock: a session whose
// deadline cancel failed stays bounded, and the keeper cancels it again.
func Grant(home, by string, now time.Time) error {
	return withLock(home, func() error {
		live, err := currentLaunch(home)
		if err != nil {
			return err
		}
		return updateStopLossHeld(home, now, func(store *StopLoss) error {
			store.Grant++
			store.GrantedAt, store.GrantedBy = stamp(now), by
			if store.Clock != nil && live != "" && store.Clock.Launch == live {
				store.Clock.Grant = store.Grant
			} else {
				store.Clock = nil
			}
			for index := range store.Launches {
				if usage := store.Launches[index].Usage; usage != nil && !usage.Known && store.Launches[index].AcceptedBy == "" {
					store.Launches[index].AcceptedBy = by
				}
			}
			store.record(now, "grant", "", "", fmt.Sprintf("allowance %d granted by %s", store.Grant, by))
			return nil
		})
	})
}

// PendingHits are the hits whose alert is still owed.
func PendingHits(home string) ([]Hit, error) {
	var pending []Hit
	err := withLock(home, func() error {
		store, err := ReadStopLoss(home)
		if err != nil {
			return err
		}
		for _, hit := range store.Hits {
			if hit.Alerted == "" {
				pending = append(pending, hit)
			}
		}
		return nil
	})
	return pending, err
}
