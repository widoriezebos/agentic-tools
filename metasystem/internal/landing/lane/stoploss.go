package lane

// The lane's budgets (lane runtime design r10 §2, K10; Wido D4), kept by
// the keeper in host state outside every session (host/landing-stoploss.json)
// so they persist across sessions and batches:
//
//   - Allowance: per batch, 4 executions of any subject and 2 h measured
//     from the launch of the session that took the batch up, covering its
//     composition and surviving a recovery session; one fresh session per
//     batch. Exhausting it refuses the next execution and never what is
//     already admitted and green: publish reads no allowance.
//   - Deadline: the keeper cancels a session at its deadline and settles
//     custody (K9). A validation session has its own launch-bound limit.
//   - Crash breaker: 3 unsuccessful recovery starts in 6 h. A fresh session
//     never counts, however it ends.
//   - Spend: every ended landing launch, cancelled and failed ones too, is
//     reconciled once by launch id; usage that can't be read holds the next
//     launch. The daily ceiling resets at local midnight.
//   - Hit: the lane is paused, an alert opened (through the steward's
//     OpenAlert, by the keeper), and a person's resume grants a fresh
//     allowance, keeping the history.
//
// The store is read and written under the host lane flock: a caller of a
// ...Held function holds it (inside Gate, or the keeper's decision).

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The allowance, the deadline and the breaker (D4, K10).
const (
	AllowanceExecutions = 4
	AllowanceWindow     = 2 * time.Hour
	// ValidationWindow is a validation session's own launch-bound limit: a
	// session woken for no batch work is bound to its own launch.
	ValidationWindow = 2 * time.Hour
	BreakerFailures  = 3
	BreakerWindow    = 6 * time.Hour
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

// StopLossBy is who a stop-loss pause names; WatchBy the lane watch's.
const (
	StopLossBy = "the landing lane's stop-loss"
	WatchBy    = "the landing lane's watch"
)

// The kinds of a hit, in the record and the alert's work identity.
const (
	HitAllowance    = "allowance"
	HitDeadline     = "deadline"
	HitBreaker      = "crash-breaker"
	HitUsageUnknown = "usage-unknown"
	HitCeiling      = "daily-ceiling"
	HitWatch        = "watch"
)

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

// LaunchEnd is what a reap read of an ended landing launch.
type LaunchEnd struct {
	Launch string
	// State is the launch's terminal state; Completed is "completed".
	State     string
	Completed bool
	Usage     Usage
}

// Hit is one budget the lane hit (or the watch's finding): the lane was
// paused and an alert is owed until Alerted.
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

// StopLossPath is the budget store, for a person and an alert's evidence.
func StopLossPath(home string) string { return stopLossPath(home) }

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

// hitHeld records a hit and pauses the lane for the stop-loss; a hit for
// work already hit is not recorded twice. The caller holds the lane flock.
func (s *StopLoss) hitHeld(home, kind, work, message, by string, now time.Time) error {
	s.recordHit(kind, work, message, now)
	_, err := setPauseLocked(home, by, now)
	return err
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

// StartLaunchHeld records a landing launch the keeper started and sets the
// session clock: a session woken for batch work starts the allowance's
// clock at its launch, and a recovery session (an unfinished batch woke
// it) keeps the clock of the session before it; any other session (a
// validation) is bound to its own launch. The caller holds the lane flock.
func StartLaunchHeld(home, launch string, reasons []string, startedAt time.Time) error {
	return updateStopLossHeld(home, startedAt, func(store *StopLoss) error {
		recovery := slices.Contains(reasons, WakeUnfinishedBatch)
		batchWork := recovery || slices.Contains(reasons, WakeQueued)
		clock := SessionClock{Grant: store.Grant, Kind: ClockValidation, Since: stamp(startedAt), Launch: launch}
		if batchWork {
			clock.Kind = ClockBatch
			if recovery && store.Clock != nil && store.Clock.Grant == store.Grant && store.Clock.Kind == ClockBatch {
				clock.Since = store.Clock.Since
			}
		}
		store.Clock = &clock
		store.Launches = append(store.Launches, LaunchSpend{Launch: launch, Grant: store.Grant, StartedAt: stamp(startedAt),
			Reasons: slices.Clone(reasons), Recovery: recovery})
		store.record(startedAt, "start", "", launch, strings.Join(reasons, ","))
		return nil
	})
}

// FailedStartHeld records a start the launcher refused: an unsuccessful
// recovery start when an unfinished batch woke it, which the breaker
// counts. The caller holds the lane flock.
func FailedStartHeld(home string, reasons []string, now time.Time, why string) error {
	if !slices.Contains(reasons, WakeUnfinishedBatch) {
		return nil
	}
	return updateStopLossHeld(home, now, func(store *StopLoss) error {
		store.Launches = append(store.Launches, LaunchSpend{Launch: "start-failed-" + stamp(now), Grant: store.Grant, StartedAt: stamp(now),
			Reasons: slices.Clone(reasons), Recovery: true, EndedAt: stamp(now), State: "start-failed", Unsuccessful: true,
			Usage: &Usage{Known: true, Source: "none"}})
		store.record(now, "start-failed", "", "", why)
		return store.breakerHeld(home, now)
	})
}

// RecordLaunchEndHeld reconciles one ended landing launch, once by its id:
// its end and its usage. Usage that can't be read is a hit (the next launch
// is held until a person resumes the lane); an unsuccessful recovery start
// counts toward the breaker. A launch already reconciled is unchanged. A
// keeper's Reap calls it, under the lane flock.
func RecordLaunchEndHeld(home string, end LaunchEnd, now time.Time) error {
	return updateStopLossHeld(home, now, func(store *StopLoss) error {
		index := slices.IndexFunc(store.Launches, func(spend LaunchSpend) bool { return spend.Launch == end.Launch })
		if index < 0 {
			store.Launches = append(store.Launches, LaunchSpend{Launch: end.Launch, Grant: store.Grant})
			index = len(store.Launches) - 1
		}
		spend := &store.Launches[index]
		if spend.Usage != nil {
			return nil
		}
		usage := end.Usage
		spend.EndedAt, spend.State, spend.Unsuccessful, spend.Usage = stamp(now), end.State, !end.Completed, &usage
		if usage.Known {
			store.record(now, "reconciled", "", end.Launch, fmt.Sprintf("%s, %d tokens from its %s", end.State, usage.Tokens, usage.Source))
		} else {
			message := fmt.Sprintf("the token use of landing session %s (%s) can't be read, so no further session starts", end.Launch, end.State)
			if usage.Why != "" {
				message += " (" + usage.Why + ")"
			}
			if err := store.hitHeld(home, HitUsageUnknown, "usage-unknown:"+end.Launch, message, StopLossBy, now); err != nil {
				return err
			}
		}
		if spend.Recovery && spend.Unsuccessful {
			return store.breakerHeld(home, now)
		}
		return nil
	})
}

// breakerHeld trips the crash breaker at 3 unsuccessful recovery starts in
// 6 h under the grant in force.
func (s *StopLoss) breakerHeld(home string, now time.Time) error {
	var failures []string
	for _, spend := range s.Launches {
		started, err := time.Parse(time.RFC3339, spend.StartedAt)
		if spend.Grant != s.Grant || !spend.Recovery || !spend.Unsuccessful || err != nil || now.Sub(started) >= BreakerWindow {
			continue
		}
		failures = append(failures, spend.Launch)
	}
	if len(failures) < BreakerFailures {
		return nil
	}
	message := fmt.Sprintf("%d landing sessions in a row failed to recover an unfinished batch within %s, so no further session starts", len(failures), BreakerWindow)
	return s.hitHeld(home, HitBreaker, fmt.Sprintf("crash-breaker:%d:%s", s.Grant, failures[len(failures)-1]), message, StopLossBy, now)
}

// StopLossHoldHeld is the keeper's budget hold before a start: usage that
// can't be read (until a person resumes), and the daily token ceiling,
// counted from local midnight or the person's latest resume, whichever is
// later; reaching it is a hit. ceiling <= 0 imposes none. The caller holds
// the lane flock (a keeper hold runs under it).
func StopLossHoldHeld(home string, now time.Time, ceiling int64) (string, error) {
	store, err := ReadStopLoss(home)
	if err != nil {
		return "", err
	}
	for _, spend := range store.Launches {
		if spend.Usage != nil && !spend.Usage.Known && spend.AcceptedBy == "" {
			return fmt.Sprintf("the token use of landing session %s can't be read; a person resumes the lane with metasystem landing start", spend.Launch), nil
		}
	}
	if ceiling <= 0 {
		return "", nil
	}
	local := now.Local()
	from := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	if granted, err := time.Parse(time.RFC3339, store.GrantedAt); err == nil && granted.After(from) {
		from = granted
	}
	var spent int64
	for _, spend := range store.Launches {
		ended, err := time.Parse(time.RFC3339, spend.EndedAt)
		if err != nil || spend.Usage == nil || ended.Before(from) {
			continue
		}
		spent += spend.Usage.Tokens
	}
	if spent < ceiling {
		return "", nil
	}
	reason := fmt.Sprintf("the landing agent used %d tokens today, and its daily ceiling is %d", spent, ceiling)
	message := reason + ", so the lane is stopped until a person resumes it; the count starts again from the resume, or from local midnight"
	work := fmt.Sprintf("daily-ceiling:%s:%d", from.Format("2006-01-02"), store.Grant)
	return reason, updateStopLossHeld(home, now, func(store *StopLoss) error {
		return store.hitHeld(home, HitCeiling, work, message, StopLossBy, now)
	})
}

// SessionDeadlineHeld is when the running launch's clock runs out; false
// when the store keeps no clock for it (a launch started before the
// budgets). The caller holds the lane flock.
func SessionDeadlineHeld(home, launch string) (time.Time, bool, error) {
	store, err := ReadStopLoss(home)
	if err != nil {
		return time.Time{}, false, err
	}
	deadline, ok := store.sessionDeadline(launch)
	return deadline, ok, nil
}

// DeadlineHitHeld records that the keeper cancelled launch at its deadline
// and settled custody (settleErr when it could not): a hit. The caller
// holds the lane flock.
func DeadlineHitHeld(home, launch string, deadline, now time.Time, settleErr error) error {
	return updateStopLossHeld(home, now, func(store *StopLoss) error {
		message := fmt.Sprintf("landing session %s ran out of its time at %s and was stopped", launch, localClock(deadline))
		if settleErr != nil {
			message += "; the test runs it started could not all be ended (" + settleErr.Error() + ")"
		}
		store.record(now, "deadline", "", launch, message)
		// The deadline pauses the lane (design r10 K10 "Hit: pause"). That
		// a pause also holds publishing green work is a recorded deviation
		// from R8-08, pending Wido's ruling (lane-runtime-design.md §2 K10).
		return store.hitHeld(home, HitDeadline, "deadline:"+launch, message, StopLossBy, now)
	})
}

// LaunchTookUpBatch says whether launch took up a batch (it began one):
// its session ended its batch's work, so its reasons hold no cooldown.
func LaunchTookUpBatch(store StopLoss, launch string) bool {
	return launch != "" && slices.ContainsFunc(store.Batches, func(allowance BatchAllowance) bool {
		return slices.Contains(allowance.Launches, launch)
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

// MarkAlerted records that the alert of the hit for work was opened.
func MarkAlerted(home, work string, now time.Time) error {
	return withLock(home, func() error {
		return updateStopLossHeld(home, now, func(store *StopLoss) error {
			for index := range store.Hits {
				if store.Hits[index].Work == work && store.Hits[index].Alerted == "" {
					store.Hits[index].Alerted = stamp(now)
				}
			}
			return nil
		})
	})
}

// DeliverHits opens the alert of every hit still owed, through alert (the
// steward's OpenAlert at the lane installation), and records each opened.
// It runs outside the lane flock. It returns a line for the steward when it
// opened or failed one.
func DeliverHits(home string, now time.Time, alert func(Hit) error) string {
	if alert == nil {
		return ""
	}
	pending, err := PendingHits(home)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		return "the landing lane's stop-loss alerts can't be read: " + err.Error()
	}
	var lines []string
	for _, hit := range pending {
		if err := alert(hit); err != nil {
			lines = append(lines, "the landing lane's stop-loss alert for "+hit.Work+" could not be opened: "+err.Error())
			continue
		}
		if err := MarkAlerted(home, hit.Work, now); err != nil {
			lines = append(lines, "the landing lane's stop-loss alert for "+hit.Work+" was opened but not recorded: "+err.Error())
			continue
		}
		lines = append(lines, "the landing lane stopped itself: "+hit.Message)
	}
	return strings.Join(lines, "\n")
}
