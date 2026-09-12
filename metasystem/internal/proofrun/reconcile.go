package proofrun

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Reconciliation actions, one per live attempt the pass looked at.
const (
	ReconcileReconciled     = "reconciled"      // the attempt is now terminal (failed) with its cause
	ReconcileCustodyPending = "custody-pending" // a recorded process could not be stopped; the reservation stays held
	ReconcileUnknown        = "unknown"         // a liveness could not be read; doubt authorizes nothing
	ReconcileObserved       = "observed"        // path 2 seen once; the next pass acts if it persists
	ReconcileLive           = "live"            // the launcher owns the attempt
)

// ReconcileOutcome is one line of the pass's account.
type ReconcileOutcome struct {
	AttemptID string
	Action    string
	Reason    string
}

// ReconcileOptions injects the clock and the process prover; the zero value
// reads the real kernel and the real clock.
type ReconcileOptions struct {
	Prober identity.Prober
	Now    time.Time
	Stop   StopOptions
	// ObservationWindow is how long path 2 (recorded processes ended, launcher
	// alive) must persist between two passes before the launcher is stopped.
	ObservationWindow time.Duration
	// GroupMembers lists the live pids of a process group; the default reads
	// the kernel. A recorded suite group with any live member keeps its
	// reservation: the recorded identities are the leader, the watchdog and
	// the launcher, and a child the leader spawned is a child all the same.
	GroupMembers func(pgid int64) ([]int64, error)
	Emit         func(string)
}

// ReconcileAttempts is the checkout's liveness reconciliation of proof
// attempts, run after every job reaper pass. The launcher owns the one
// terminal commit while it lives; when it is provably dead (path 1), or when
// every process it recorded has ended while it still lives (path 2, seen on
// two passes), the attempt is terminalized as failed with its cause, after
// every recorded process is confirmed ended through the identity-safe stop
// ladder. A liveness that reads Unknown is named and left alone: doubt never
// authorizes a kill, and a reservation is released only after confirmed
// termination, never on elapsed time.
func ReconcileAttempts(root string, options ReconcileOptions) ([]ReconcileOutcome, error) {
	if options.Prober == nil {
		options.Prober = identity.KernelProber{}
	}
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	if options.ObservationWindow <= 0 {
		options.ObservationWindow = 30 * time.Second
	}
	if options.Stop.Prober == nil {
		options.Stop.Prober = options.Prober
	}
	if options.GroupMembers == nil {
		options.GroupMembers = liveGroupMembers
	}
	emit := options.Emit
	if emit == nil {
		emit = func(string) {}
	}
	attempts, err := ReadAttempts(root)
	if err != nil {
		return nil, err
	}
	sort.Slice(attempts, func(i, j int) bool { return attempts[i].AttemptID < attempts[j].AttemptID })
	var outcomes []ReconcileOutcome
	for _, attempt := range attempts {
		if attempt.Terminal != nil {
			// A launcher that committed its own terminal after an observation
			// leaves the marker behind; and its process records may still say
			// running when every process is gone (23 such records in the
			// audit). Both are tidied here, never signalled.
			clearObservation(root, attempt.AttemptID)
			settleTerminalRecords(root, attempt, options.Prober)
			continue
		}
		outcome := reconcileOne(root, attempt, options)
		if outcome.Action != ReconcileLive {
			emit(fmt.Sprintf("proof attempt %s: %s: %s", outcome.AttemptID, outcome.Action, outcome.Reason))
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, nil
}

func reconcileOne(root string, attempt Attempt, options ReconcileOptions) ReconcileOutcome {
	id := attempt.AttemptID
	launcherRef := attempt.Launcher.Ref()
	launcher := identity.AliveRef(options.Prober, launcherRef)
	if launcher == identity.Unknown {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileUnknown,
			Reason: fmt.Sprintf("launcher pid %d liveness is uninspectable", launcherRef.Pid)}
	}
	records := make([]Record, 0, len(attempt.ProcessKeys))
	for _, key := range attempt.ProcessKeys {
		record, readErr := ReadProcessRecord(root, key)
		if readErr != nil {
			return ReconcileOutcome{AttemptID: id, Action: ReconcileUnknown,
				Reason: fmt.Sprintf("process record %s is unreadable: %v", key, readErr)}
		}
		records = append(records, record)
	}

	if launcher == identity.Dead {
		// Path 1: the launcher is gone and can commit nothing. Confirm every
		// recorded process ended, and every member of the recorded suite
		// group with them, before the reservation is released.
		if reason, ok := stopRecorded(records, options); !ok {
			return ReconcileOutcome{AttemptID: id, Action: ReconcileCustodyPending, Reason: reason}
		}
		clearObservation(root, id)
		reason := fmt.Sprintf("reconciled: launcher pid %d started %d is dead; %d recorded processes confirmed ended; no terminal was committed",
			launcherRef.Pid, launcherRef.StartedAtSec, len(records))
		return finalizeReconciled(root, id, records, reason, options.Now)
	}

	// The launcher lives. Path 2 applies only when it published processes and
	// every one of them has ended: a launcher waiting on a child that no
	// longer exists commits nothing either.
	if len(records) == 0 || !allRecordedEnded(records, options.Prober) {
		clearObservation(root, id)
		return ReconcileOutcome{AttemptID: id, Action: ReconcileLive, Reason: "the launcher owns the attempt"}
	}
	firstSeen, seen := readObservation(root, id)
	if !seen {
		writeObservation(root, id, options.Now)
		return ReconcileOutcome{AttemptID: id, Action: ReconcileObserved,
			Reason: fmt.Sprintf("recorded processes ended while launcher pid %d lives; acted on if it persists past %s", launcherRef.Pid, options.ObservationWindow)}
	}
	if options.Now.Sub(firstSeen) < options.ObservationWindow {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileObserved,
			Reason: fmt.Sprintf("recorded processes ended while launcher pid %d lives since %s", launcherRef.Pid, firstSeen.UTC().Format(time.RFC3339))}
	}
	// The cancellation intent serializes against process publication (the
	// order CancelStopProof uses): a key the launcher publishes before the
	// intent lands is read again here and stopped with the rest.
	if err := RequestCancellation(root, id, "reconciled: recorded processes ended while the launcher lived"); err != nil {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileUnknown, Reason: "cancellation intent: " + err.Error()}
	}
	current, err := ReadAttempt(root, id)
	if err != nil {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileUnknown, Reason: "re-read: " + err.Error()}
	}
	if current.Terminal != nil {
		clearObservation(root, id)
		return ReconcileOutcome{AttemptID: id, Action: ReconcileLive, Reason: "a terminal landed meanwhile"}
	}
	if len(current.ProcessKeys) != len(records) {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileObserved, Reason: "a process was published during the pass; read again next pass"}
	}
	outcome := StopRecordedIdentity("launcher", attempt.Launcher, options.Stop)
	if outcome.Result == StopNotStopped {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileCustodyPending,
			Reason: fmt.Sprintf("launcher pid %d was not stopped: %s", launcherRef.Pid, outcome.Reason)}
	}
	if reason, ok := stopRecorded(records, options); !ok {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileCustodyPending, Reason: reason}
	}
	clearObservation(root, id)
	reason := fmt.Sprintf("reconciled: %d recorded processes ended without a terminal; launcher pid %d stopped after %s",
		len(records), launcherRef.Pid, options.Now.Sub(firstSeen).Round(time.Second))
	return finalizeReconciled(root, id, records, reason, options.Now)
}

// stopRecorded runs the stop ladder over every record and reports the first
// component that did not end; already-gone components are confirmations. A
// live member of the recorded suite group that the ladder does not name (a
// child the leader spawned) also keeps the reservation.
func stopRecorded(records []Record, options ReconcileOptions) (string, bool) {
	for _, record := range records {
		for _, outcome := range Stop(record, options.Stop) {
			if outcome.Result == StopNotStopped {
				return fmt.Sprintf("%s %s of record %s was not stopped: %s", outcome.Component,
					fmt.Sprint(outcome.Identity.Pid), record.Key(), outcome.Reason), false
			}
		}
		if record.SuiteProcess.Pgid > 0 {
			members, err := options.GroupMembers(record.SuiteProcess.Pgid)
			if err != nil {
				return fmt.Sprintf("suite group %d of record %s is uninspectable: %v", record.SuiteProcess.Pgid, record.Key(), err), false
			}
			if len(members) > 0 {
				return fmt.Sprintf("suite group %d of record %s still has live members %v", record.SuiteProcess.Pgid, record.Key(), members), false
			}
		}
	}
	return "", true
}

// liveGroupMembers reads the kernel for the live members of a process group.
func liveGroupMembers(pgid int64) ([]int64, error) {
	pids, err := identity.AllPids()
	if err != nil {
		return nil, err
	}
	var members []int64
	for _, pid := range pids {
		if group, err := syscall.Getpgid(int(pid)); err == nil && int64(group) == pgid {
			members = append(members, pid)
		}
	}
	return members, nil
}

// settleTerminalRecords marks a terminal attempt's process records done once
// every process they name is provably dead; a record that still names a live
// or uninspectable process is left as it is.
func settleTerminalRecords(root string, attempt Attempt, prober identity.Prober) {
	for _, key := range attempt.ProcessKeys {
		record, err := ReadProcessRecord(root, key)
		if err != nil || record.Status == StatusDone {
			continue
		}
		if identity.AliveRef(prober, record.SuiteProcess.Ref()) != identity.Dead ||
			identity.AliveRef(prober, record.Watchdog.Ref()) != identity.Dead ||
			identity.AliveRef(prober, record.Launcher.Ref()) != identity.Dead {
			continue
		}
		_ = markDone(root, record, record.Launcher.Ref())
	}
}

func allRecordedEnded(records []Record, prober identity.Prober) bool {
	for _, record := range records {
		if record.Status == StatusDone {
			continue
		}
		if identity.AliveRef(prober, record.SuiteProcess.Ref()) != identity.Dead ||
			identity.AliveRef(prober, record.Watchdog.Ref()) != identity.Dead {
			return false
		}
	}
	return true
}

func finalizeReconciled(root, id string, records []Record, reason string, now time.Time) ReconcileOutcome {
	for _, record := range records {
		if record.Status != StatusDone {
			if err := markDone(root, record, record.Launcher.Ref()); err != nil {
				return ReconcileOutcome{AttemptID: id, Action: ReconcileUnknown,
					Reason: fmt.Sprintf("process record %s could not be marked done: %v", record.Key(), err)}
			}
		}
	}
	lock, err := AcquireMutation(root)
	if err != nil {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileUnknown, Reason: "proof mutation lock: " + err.Error()}
	}
	defer lock.Release()
	current, err := ReadAttempt(root, id)
	if err != nil {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileUnknown, Reason: "re-read: " + err.Error()}
	}
	if current.Terminal != nil {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileLive, Reason: "a terminal landed meanwhile"}
	}
	if _, err := FinalizeAttemptLocked(root, id, TerminalFailed, 1, reason, nil, now); err != nil {
		return ReconcileOutcome{AttemptID: id, Action: ReconcileUnknown, Reason: "terminal commit: " + err.Error()}
	}
	return ReconcileOutcome{AttemptID: id, Action: ReconcileReconciled, Reason: reason}
}

// The path-2 observation is a small file beside the attempt: the first pass
// that sees the shape records when; the pass that sees it persist acts.
func observationPath(root, id string) string {
	return filepath.Join(attemptsDir(root), id+".reconcile-observed")
}

func readObservation(root, id string) (time.Time, bool) {
	data, err := os.ReadFile(observationPath(root, id))
	if err != nil {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}

func writeObservation(root, id string, now time.Time) {
	_ = os.WriteFile(observationPath(root, id), []byte(now.UTC().Format(time.RFC3339Nano)+"\n"), 0o644)
}

func clearObservation(root, id string) {
	_ = os.Remove(observationPath(root, id))
}

// LiveAttemptSummary is the operator's line per live attempt.
type LiveAttemptSummary struct {
	AttemptID        string `json:"attemptId"`
	GoalID           string `json:"goalId"`
	StartedAt        string `json:"startedAt"`
	LauncherPid      int64  `json:"launcherPid"`
	LauncherLiveness string `json:"launcherLiveness"`
	ProcessRecords   int    `json:"processRecords"`
}

// LiveAttempts lists every attempt without a terminal and how its launcher
// reads now.
func LiveAttempts(root string, prober identity.Prober) ([]LiveAttemptSummary, error) {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	attempts, err := ReadAttempts(root)
	if err != nil {
		return nil, err
	}
	var live []LiveAttemptSummary
	for _, attempt := range attempts {
		if attempt.Terminal != nil {
			continue
		}
		live = append(live, LiveAttemptSummary{
			AttemptID: attempt.AttemptID, GoalID: attempt.GoalID, StartedAt: attempt.StartedAt,
			LauncherPid: attempt.Launcher.Pid, LauncherLiveness: identity.AliveRef(prober, attempt.Launcher.Ref()).String(),
			ProcessRecords: len(attempt.ProcessKeys),
		})
	}
	sort.Slice(live, func(i, j int) bool { return live[i].AttemptID < live[j].AttemptID })
	return live, nil
}
