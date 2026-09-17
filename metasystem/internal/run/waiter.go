package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The waiter layer: the blocking watch verb and its owner-keyed records.
// A waiter is the wake path — a live process any runtime's background
// facility can hold — and its record is the fact the turn verdict reads.
// Records are exclusive PER (kind, id, owner) by liveness: a foreign
// waiter neither satisfies your unwatched rule nor blocks your own
// watching; a live same-owner waiter from a DEAD lifecycle is
// replaceable, because its own watch is about to exit on the terminal
// record it is actually reading.

// Watch exit codes: outcomes 0-4, operational failures 64-66, disjoint.
const (
	ExitGreen         = 0
	ExitRed           = 1
	ExitEndedUnknown  = 2
	ExitLaunchFailed  = 3
	ExitNoRecord      = 4
	ExitWaiterBusy    = 64
	ExitWaiterIO      = 65
	ExitWaiterUnknown = 66
	ExitInvalidWait   = 67
	ExitWaitDeadline  = 124
	ExitInterrupted   = 130
)

// WaiterTarget pins the waiter to one lifecycle.
type WaiterTarget struct {
	StartedAt   string `json:"startedAt,omitempty"`           // jobs
	Round       int64  `json:"round,omitempty"`               // jobs
	OperationID string `json:"operationId,omitempty"`         // jobs
	Generation  int    `json:"generation,omitempty"`          // runs
	LaunchNonce string `json:"launchNonce,omitempty"`         // runs
	ProofDigest string `json:"proofIdentityDigest,omitempty"` // attempts
}

// WaitSelector is the complete durable predicate. Resume never replaces any
// field: After stays the match floor while the row's last checked tip keeps
// repeated source reads incremental.
type WaitSelector struct {
	Kind     string `json:"kind"`
	TargetID string `json:"targetId"`
	Path     string `json:"path,omitempty"`
	Until    string `json:"until,omitempty"`
	GoalID   string `json:"goalId,omitempty"`
	Event    string `json:"event,omitempty"`
	After    string `json:"after,omitempty"`
	Verb     string `json:"verb,omitempty"`
	Question string `json:"question,omitempty"`
	Chain    string `json:"chain,omitempty"`
	Poll     string `json:"poll,omitempty"`
}

// SourceObservation is the typed fact supplied by a source owner. Pending
// observations carry the pinned incarnation and may advance LastTip; a final
// observation carries the exit and the evidence identity to retain.
type SourceObservation struct {
	Pending        bool
	ExitCode       int
	Reason         string
	Outcome        string
	Evidence       string
	LedgerTip      string
	TerminalStamp  string
	Incarnation    WaiterTarget
	Temporary      bool
	ClaimableGoals []ClaimableGoal
	ClaimableRead  bool
	PollError      string
	PollAt         string
}

// WaitResult is both the one command result and the terminal replay payload.
type WaitResult struct {
	SchemaVersion         int          `json:"schemaVersion"`
	WaitID                string       `json:"waitId"`
	Nonce                 string       `json:"nonce,omitempty"`
	Selector              WaitSelector `json:"selector"`
	TargetIncarnation     WaiterTarget `json:"targetIncarnation"`
	ExitCode              int          `json:"exitCode"`
	Reason                string       `json:"reason"`
	SourceOutcome         string       `json:"sourceOutcome"`
	SourceEvidence        string       `json:"sourceEvidence"`
	RegisteredAt          string       `json:"registeredAt"`
	Deadline              string       `json:"deadline"`
	ObservedAt            string       `json:"observedAt"`
	ReturnedAt            string       `json:"returnedAt"`
	LedgerTip             string       `json:"ledgerTip,omitempty"`
	TerminalStamp         string       `json:"terminalStamp,omitempty"`
	ReplayCommand         string       `json:"replayCommand"`
	PointerRepaired       bool         `json:"pointerRepaired,omitempty"`
	Runtime               string       `json:"runtime,omitempty"`
	Mode                  string       `json:"mode,omitempty"`
	AtEntry               bool         `json:"atEntry,omitempty"`
	State                 string       `json:"state,omitempty"`
	PrevObservedAt        string       `json:"prevObservedAt,omitempty"`
	RegisteredBootNanos   int64        `json:"registeredBootNanos,omitempty"`
	RegisteredBootID      string       `json:"registeredBootId,omitempty"`
	PrevObservedBootNanos int64        `json:"prevObservedBootNanos,omitempty"`
	PrevObservedBootID    string       `json:"prevObservedBootId,omitempty"`
	ObservedBootNanos     int64        `json:"observedBootNanos,omitempty"`
	ObservedBootID        string       `json:"observedBootId,omitempty"`
	ReturnedBootNanos     int64        `json:"returnedBootNanos,omitempty"`
	ReturnedBootID        string       `json:"returnedBootId,omitempty"`
	ReturnEventFailed     bool         `json:"returnEventFailed,omitempty"`
}

// ResumeIdentity records the process that lawfully replaced a dead waiter.
type ResumeIdentity struct {
	Session           string `json:"session"`
	Pid               int64  `json:"pid"`
	PidStartedAt      int64  `json:"pidStartedAt"`
	PidStartedAtMicro int64  `json:"pidStartedAtExactMicro,omitempty"`
	PidStartTicks     int64  `json:"pidStartTicks,omitempty"`
	BootID            string `json:"bootId,omitempty"`
}

// WaitRenewal retains the result that ended the preceding bounded period and
// the instant an explicit timeout started the next registration.
type WaitRenewal struct {
	Result    WaitResult `json:"result"`
	RenewedAt string     `json:"renewedAt"`
}

// ClaimableGoal is the level captured at registration. Only a later addition
// or revision change is an actionable event; an existing backlog is not.
type ClaimableGoal struct {
	ID       string `json:"id"`
	Revision uint64 `json:"revision"`
}

// Waiter is one registered waiter record.
type Waiter struct {
	SchemaVersion         int             `json:"schemaVersion,omitempty"`
	WaitID                string          `json:"waitId,omitempty"`
	Nonce                 string          `json:"nonce,omitempty"`
	Kind                  string          `json:"kind"`
	TargetID              string          `json:"targetId,omitempty"`
	OwnerDigest           string          `json:"ownerDigest,omitempty"`
	Pid                   int64           `json:"pid"`
	PidStartedAt          int64           `json:"pidStartedAt"`
	PidStartedAtMicro     int64           `json:"pidStartedAtExactMicro,omitempty"`
	PidStartTicks         int64           `json:"pidStartTicks,omitempty"`
	BootID                string          `json:"bootId,omitempty"`
	Session               string          `json:"session"`
	MainId                string          `json:"mainId"`
	OwnerLineage          string          `json:"ownerLineage,omitempty"`
	ClaimEpoch            *int64          `json:"claimEpoch,omitempty"`
	RuntimeSession        string          `json:"runtimeSession,omitempty"`
	Runtime               string          `json:"runtime,omitempty"`
	Mode                  string          `json:"mode,omitempty"`
	AtEntry               bool            `json:"atEntry,omitempty"`
	Selector              WaitSelector    `json:"selector,omitempty"`
	GoalID                string          `json:"goalId,omitempty"`
	Target                WaiterTarget    `json:"target"`
	OriginalCursor        string          `json:"originalCursor,omitempty"`
	LastCheckedTip        string          `json:"lastCheckedTip,omitempty"`
	SourceResultIdentity  string          `json:"sourceResultIdentity,omitempty"`
	RegisteredAt          string          `json:"registeredAt,omitempty"`
	Deadline              string          `json:"deadline,omitempty"`
	BootDeadlineNanos     int64           `json:"bootDeadlineNanos,omitempty"`
	DeadlineBootID        string          `json:"deadlineBootId,omitempty"`
	RemainingNanos        int64           `json:"remainingNanos,omitempty"`
	LastObservedAt        string          `json:"lastObservedAt,omitempty"`
	LastObservedBootNanos int64           `json:"lastObservedBootNanos,omitempty"`
	LastObservedBootID    string          `json:"lastObservedBootId,omitempty"`
	RegisteredBootNanos   int64           `json:"registeredBootNanos,omitempty"`
	RegisteredBootID      string          `json:"registeredBootId,omitempty"`
	OpenWorkSignature     string          `json:"openWorkSignature,omitempty"`
	ClaimableGoals        []ClaimableGoal `json:"claimableGoals,omitempty"`
	LastPollError         string          `json:"lastPollError,omitempty"`
	LastPollAt            string          `json:"lastPollAt,omitempty"`
	State                 string          `json:"state,omitempty"`
	Delivery              string          `json:"delivery,omitempty"`
	Accelerator           string          `json:"accelerator,omitempty"`
	HintPath              string          `json:"hintPath,omitempty"`
	Result                *WaitResult     `json:"result,omitempty"`
	Renewals              []WaitRenewal   `json:"renewals,omitempty"`
	ResumedBy             *ResumeIdentity `json:"resumedBy,omitempty"`
	InterruptedBy         string          `json:"interruptedBy,omitempty"`
	PointerRepaired       bool            `json:"pointerRepaired,omitempty"`
}

// WaitRequest contains the caller-owned registration inputs.
type WaitRequest struct {
	Selector          WaitSelector
	Owner             Caller
	RuntimeSession    string
	Timeout           time.Duration
	OpenWorkSignature string
}

// WaitOptions carries the bounded external ports and deterministic clocks.
// Tests inject them; production supplies the source owner and adapter only.
type WaitOptions struct {
	Observe    func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error)
	Deliver    func(context.Context, string, string, time.Time, string) (answer string, declined bool, err error)
	Claimable  func(context.Context, Waiter) ([]ClaimableGoal, error)
	Actionable func(context.Context, Waiter) (string, bool, error)
	// OpenWorkSignature reads only plan open steps. Explicit renewals use it
	// to baseline the new bounded registration without running a report scan.
	OpenWorkSignature func(context.Context) (string, error)
	// SucceededMainID is supplied only after the command layer proves the
	// current lease's latest takeover names this dead predecessor.
	SucceededMainID       string
	SucceededOwnerLineage string
	Now                   func() time.Time
	BootClock             func() (string, time.Duration, error)
	Sleep                 func(context.Context, time.Duration) error
	OpenHintReceiver      OpenHintReceiver
	Runtime               string
	EmitEvent             func(root, event, summary string, fields map[string]string) error
}

type finishStamps struct {
	AtEntry           bool
	ObservedBootNanos int64
	ObservedBootID    string
}

var waitEvents = &events.Emitter{Component: "run", Pid: int64(os.Getpid())}

// WaitersDir is the one namespace for job and run waiters alike.
func WaitersDir(root string) string { return filepath.Join(root, "artifacts", "agents", "waiters") }

// WaiterPath keys a record by kind, public id, and owner digest.
func WaiterPath(root, kind, id, ownerDigest string) string {
	return filepath.Join(WaitersDir(root), fmt.Sprintf("%s-%s-%s.json", kind, id, ownerDigest))
}

// withWaiterLock bounds every waiter-record mutation.
func withWaiterLock(root string, fn func() error) error {
	dir := WaitersDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("waiter lock is busy")
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return fn()
}

// waiterError carries the pinned operational exit code.
type waiterError struct {
	code    int
	message string
}

func (e *waiterError) Error() string { return e.message }

// WaiterExitCode maps an error from this layer to its pinned code.
func WaiterExitCode(err error) int {
	if we, ok := err.(*waiterError); ok {
		return we.code
	}
	return ExitWaiterIO
}

// RegisterWaiter writes the caller's waiter record. Exclusive per key by
// liveness; a live same-key waiter on the SAME lifecycle refuses (64); a
// live waiter on a dead lifecycle, a dead owner, or a mismatched target
// is replaced by identity-checked compare-and-delete; an owner whose
// liveness is unknowable refuses (66) — unknown never authorizes.
func (s *Store) RegisterWaiter(kind, id string, owner Caller, target WaiterTarget) error {
	digest := OwnerDigest(owner.MainId)
	path := WaiterPath(s.Root, kind, id, digest)
	return withWaiterLock(s.Root, func() error {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			var existing Waiter
			if json.Unmarshal(data, &existing) != nil {
				// A malformed record cannot be identity-checked; refuse
				// rather than clobber (66).
				return &waiterError{ExitWaiterUnknown, "existing waiter record is unreadable; refusing to replace what cannot be identity-checked"}
			}
			switch identity.AliveRef(s.prober(), identity.Ref{Pid: existing.Pid, StartedAtSec: existing.PidStartedAt,
				StartTicks: existing.PidStartTicks, BootID: existing.BootID}) {
			case identity.Alive:
				if existing.Target == target {
					return &waiterError{ExitWaiterBusy, "a live waiter already watches this lifecycle for this owner"}
				}
				// Live but watching a DEAD lifecycle: replaceable;
				// fall through to the write.
			case identity.Unknown:
				return &waiterError{ExitWaiterUnknown, "existing waiter liveness unknown; refusing"}
			}
		case !os.IsNotExist(err):
			return &waiterError{ExitWaiterIO, err.Error()}
		}
		self := int64(os.Getpid())
		exact, state, _ := s.prober().Probe(self)
		if state != identity.Alive {
			return &waiterError{ExitWaiterUnknown, "own identity unreadable"}
		}
		record := Waiter{
			Kind: kind, Pid: self, PidStartedAt: exact.StartedAt.Unix(),
			PidStartTicks: exact.StartTicks, BootID: exact.BootID,
			Session: owner.SessionId, MainId: owner.MainId, Target: target,
		}
		encoded, err := json.MarshalIndent(record, "", " ")
		if err != nil {
			return &waiterError{ExitWaiterIO, err.Error()}
		}
		if err := atomicWrite(path, append(encoded, '\n')); err != nil {
			return &waiterError{ExitWaiterIO, err.Error()}
		}
		return nil
	})
}

// RemoveWaiter deletes the caller's own record — compare-and-delete on
// identity, so one waiter can never delete another's registration.
func (s *Store) RemoveWaiter(kind, id string, owner Caller) {
	path := WaiterPath(s.Root, kind, id, OwnerDigest(owner.MainId))
	_ = withWaiterLock(s.Root, func() error {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var existing Waiter
		if json.Unmarshal(data, &existing) != nil {
			return nil
		}
		self := int64(os.Getpid())
		exact, state, _ := s.prober().Probe(self)
		same := false
		if existing.Pid == self && state == identity.Alive {
			// The pair decides when both sides carry it:
			// cleanup must not orphan its own record after clock drift.
			if existing.PidStartTicks > 0 && existing.BootID != "" &&
				exact.StartTicks > 0 && exact.BootID != "" {
				same = existing.PidStartTicks == exact.StartTicks && existing.BootID == exact.BootID
			} else {
				same = existing.PidStartedAt == exact.StartedAt.Unix()
			}
		}
		if same {
			os.Remove(path)
		}
		return nil
	})
}

// LiveWaiter reports whether a live identity-verified waiter of the given
// owner watches the given current lifecycle. Scans retain this display fact;
// turn-ending decisions authenticate registered waits through their gate.
func LiveWaiter(root string, prober identity.Prober, kind, id, mainId string, target WaiterTarget) bool {
	data, err := os.ReadFile(WaiterPath(root, kind, id, OwnerDigest(mainId)))
	if err != nil {
		return false
	}
	var waiter Waiter
	if json.Unmarshal(data, &waiter) != nil {
		return false
	}
	if waiter.Target != target {
		return false
	}
	return identity.AliveRef(prober, identity.Ref{Pid: waiter.Pid, StartedAtSec: waiter.PidStartedAt,
		StartTicks: waiter.PidStartTicks, BootID: waiter.BootID}) == identity.Alive
}

var waitIdentifierRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// WaiterPointerPath is the durable wait-identifier lookup used by resume.
func WaiterPointerPath(root, waitID string) string {
	return filepath.Join(WaitersDir(root), "by-id", waitID)
}

func defaultWaitNow() time.Time { return time.Now().UTC() }

func defaultWaitSleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizeWaitOptions(options WaitOptions) WaitOptions {
	if options.Now == nil {
		options.Now = defaultWaitNow
	}
	if options.BootClock == nil {
		options.BootClock = identity.BootClock
	}
	if options.Sleep == nil {
		options.Sleep = defaultWaitSleep
	}
	if options.EmitEvent == nil {
		options.EmitEvent = waitEvents.EmitChecked
	}
	return options
}

// ValidWaitID reports whether a value can name a version-2 registration.
func ValidWaitID(value string) bool { return nonceRe.MatchString(value) }

// ValidateWaitSelector checks the public selector independently of caller
// classification, so malformed input cannot be mistaken for an authority
// refusal.
func ValidateWaitSelector(selector WaitSelector) error {
	if selector.Kind != "job" && selector.Kind != "run" && selector.Kind != "attempt" && selector.Kind != "goal" && selector.Kind != "path" {
		return fmt.Errorf("wait needs a job, run, attempt, goal, or path selector")
	}
	if !waitIdentifierRe.MatchString(selector.TargetID) {
		return fmt.Errorf("wait target identifier is invalid")
	}
	if selector.Kind == "path" {
		if selector.Path == "" || !filepath.IsAbs(selector.Path) {
			return fmt.Errorf("wait path must be absolute")
		}
		if filepath.Clean(selector.Path) != selector.Path {
			return fmt.Errorf("wait path must be clean")
		}
		if selector.Until != "present" && selector.Until != "absent" {
			return fmt.Errorf("wait path --until must be present or absent")
		}
		if selector.TargetID != PathWaitTargetID(selector.Path) {
			return fmt.Errorf("wait path target identifier does not match the cleaned path")
		}
		if selector.GoalID != "" || selector.Event != "" || selector.After != "" || selector.Verb != "" || selector.Question != "" || selector.Chain != "" || selector.Poll != "" {
			return fmt.Errorf("ledger event arguments apply only to goal waits")
		}
	} else if selector.Path != "" || selector.Until != "" {
		return fmt.Errorf("path wait arguments apply only to path waits")
	} else if selector.Kind == "goal" {
		if selector.GoalID == "" || selector.GoalID != selector.TargetID {
			return fmt.Errorf("goal wait requires one matching goal identifier")
		}
		if selector.Event != "landing" && selector.Event != "human-act" {
			return fmt.Errorf("goal wait event must be landing or human-act")
		}
		if selector.After == "" {
			return fmt.Errorf("goal wait requires the accepted ledger cursor supplied by --after")
		}
		if selector.Event == "human-act" && selector.Verb == "answer" && selector.Question == "" {
			return fmt.Errorf("an answer wait requires a question identifier")
		}
		if selector.Question != "" && selector.Verb != "answer" {
			return fmt.Errorf("a question selector applies only to an answer wait")
		}
		if selector.Event != "human-act" && (selector.Verb != "" || selector.Question != "") {
			return fmt.Errorf("verb and question selectors apply only to human-act waits")
		}
		if selector.Event != "landing" && selector.Chain != "" {
			return fmt.Errorf("a chain selector applies only to landing waits")
		}
		if selector.Poll != "" && selector.Poll != "channel" {
			return fmt.Errorf("goal wait poll must be channel when present")
		}
		if selector.Poll == "channel" && (selector.Event != "human-act" || selector.Verb != "answer" || selector.Question == "") {
			return fmt.Errorf("channel polling applies only to an answer wait with a question identifier")
		}
	} else if selector.Event != "" || selector.After != "" || selector.Verb != "" || selector.Question != "" || selector.Chain != "" || selector.Poll != "" {
		return fmt.Errorf("ledger event arguments apply only to goal waits")
	}
	return nil
}

func validateWaitRequest(request WaitRequest) error {
	if err := ValidateWaitSelector(request.Selector); err != nil {
		return err
	}
	if request.Owner.MainId == "" || request.Owner.OwnerLineage == "" || request.Owner.SessionId == "" || request.RuntimeSession == "" {
		return fmt.Errorf("wait registration requires a classified owner, lineage, session, and runtime session")
	}
	if request.Timeout <= 0 || request.Timeout > 24*time.Hour {
		return fmt.Errorf("wait timeout must be positive and no longer than 24 hours")
	}
	return nil
}

func invalidWaitResult(reason string) WaitResult {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return WaitResult{SchemaVersion: 2, ExitCode: ExitInvalidWait, Reason: reason, SourceOutcome: "invalid-arguments", ObservedAt: now, ReturnedAt: now}
}

func waitResult(row Waiter, code int, state, reason, outcome, evidence, ledgerTip, terminalStamp string, now time.Time) WaitResult {
	result := WaitResult{
		SchemaVersion: 2, WaitID: row.WaitID, Nonce: row.Nonce, Selector: row.Selector, TargetIncarnation: row.Target,
		ExitCode: code, Reason: reason, SourceOutcome: outcome, SourceEvidence: evidence,
		RegisteredAt: row.RegisteredAt, Deadline: row.Deadline,
		ObservedAt: now.UTC().Format(time.RFC3339Nano), ReturnedAt: now.UTC().Format(time.RFC3339Nano),
		LedgerTip: ledgerTip, TerminalStamp: terminalStamp,
		ReplayCommand:   WaitResumeCommand(row),
		PointerRepaired: row.PointerRepaired,
		Runtime:         row.Runtime, Mode: row.Mode, AtEntry: row.AtEntry, State: state,
		PrevObservedAt:      row.LastObservedAt,
		RegisteredBootNanos: row.RegisteredBootNanos, RegisteredBootID: row.RegisteredBootID,
		PrevObservedBootNanos: row.LastObservedBootNanos, PrevObservedBootID: row.LastObservedBootID,
	}
	return result
}

// WaitResumeCommand keeps recovery on the command family that owns any
// provider work recorded in the selector.
func WaitResumeCommand(row Waiter) string {
	if row.Selector.Poll == "channel" {
		return "metasystem channel wait --resume " + row.WaitID
	}
	return "metasystem wait --resume " + row.WaitID
}

func withWaiterLockBounded(ctx context.Context, root string, deadline time.Time, now func() time.Time, sleep func(context.Context, time.Duration) error, fn func() error) error {
	dir := WaitersDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	lockDeadline := now().Add(5 * time.Second)
	if deadline.Before(lockDeadline) {
		lockDeadline = deadline
	}
	for {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		}
		if !now().Before(lockDeadline) {
			return fmt.Errorf("waiter lock did not become available before the wait deadline")
		}
		remaining := lockDeadline.Sub(now())
		pause := 20 * time.Millisecond
		if remaining < pause {
			pause = remaining
		}
		if err := sleep(ctx, pause); err != nil {
			return err
		}
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return fn()
}

func readV2Waiter(path string) (Waiter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Waiter{}, err
	}
	var row Waiter
	if err := json.Unmarshal(data, &row); err != nil {
		return Waiter{}, fmt.Errorf("waiter row is unreadable: %w", err)
	}
	if row.SchemaVersion != 2 || row.WaitID == "" || row.Nonce == "" || row.State == "" {
		return Waiter{}, fmt.Errorf("waiter row is not a version-2 registration")
	}
	return row, nil
}

func writeV2Waiter(path string, row Waiter) error {
	encoded, err := json.MarshalIndent(row, "", " ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(encoded, '\n'))
}

func writeV2Pointer(root string, row Waiter, rowPath string) error {
	pointer := WaiterPointerPath(root, row.WaitID)
	return atomicWrite(pointer, []byte(filepath.Base(rowPath)+"\n"))
}

func removeV2Pointer(root string, row Waiter) {
	if row.WaitID != "" {
		_ = os.Remove(WaiterPointerPath(root, row.WaitID))
	}
}

func currentProcessIdentity(prober identity.Prober) (identity.Exact, error) {
	exact, state, err := prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return identity.Exact{}, fmt.Errorf("own process identity is uncertain")
	}
	return exact, nil
}

func processMicrosecondIdentity(exact identity.Exact) int64 {
	return exact.Ref().StartedAtUnixMicro
}

func existingWaitRefuses(prober identity.Prober, row Waiter) error {
	if row.SchemaVersion == 2 && row.State != "pending" && row.State != "registering" {
		return nil
	}
	switch identity.AliveRef(prober, identity.Ref{Pid: row.Pid, StartedAtSec: row.PidStartedAt, StartedAtUnixMicro: row.PidStartedAtMicro, StartTicks: row.PidStartTicks, BootID: row.BootID}) {
	case identity.Alive:
		return &waiterError{ExitWaiterBusy, "a live waiter already owns this target for this owner"}
	case identity.Unknown:
		return &waiterError{ExitWaiterUnknown, "existing waiter identity is uncertain; refusing replacement"}
	}
	return nil
}

func supersedesStaleSession(existing, replacement Waiter) bool {
	return existing.SchemaVersion == 2 && (existing.State == "pending" || existing.State == "registering") &&
		existing.MainId != "" && existing.MainId == replacement.MainId && existing.OwnerDigest == replacement.OwnerDigest &&
		existing.Session != "" && existing.Session == existing.RuntimeSession &&
		replacement.Session != "" && replacement.Session == replacement.RuntimeSession && existing.Session != replacement.Session
}

func (s *Store) initialObservation(ctx context.Context, selector WaitSelector, options WaitOptions, deadline time.Time) (SourceObservation, error) {
	if options.Observe == nil {
		return SourceObservation{}, fmt.Errorf("wait source reader is unavailable")
	}
	remaining := deadline.Sub(options.Now())
	if remaining <= 0 {
		return SourceObservation{}, context.DeadlineExceeded
	}
	if remaining > 10*time.Second {
		remaining = 10 * time.Second
	}
	readCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	return readWaitSource(readCtx, selector, WaiterTarget{}, selector.After, options)
}

func (s *Store) observe(ctx context.Context, row Waiter, options WaitOptions, deadline time.Time) (SourceObservation, error) {
	remaining := deadline.Sub(options.Now())
	if remaining <= 0 {
		return SourceObservation{}, context.DeadlineExceeded
	}
	if remaining > 10*time.Second {
		remaining = 10 * time.Second
	}
	readCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	return readWaitSource(readCtx, row.Selector, row.Target, row.LastCheckedTip, options)
}

func readWaitSource(ctx context.Context, selector WaitSelector, pinned WaiterTarget, lastTip string, options WaitOptions) (SourceObservation, error) {
	observation, err := options.Observe(ctx, selector, pinned, lastTip)
	if observation.PollError != "" && observation.PollAt == "" {
		observation.PollAt = options.Now().UTC().Format(time.RFC3339Nano)
	}
	return observation, err
}

func waitSelectorHasIncarnation(selector WaitSelector) bool { return selector.Kind != "path" }

func deliverBeforeDeadline(ctx context.Context, deadline time.Time, options WaitOptions, waitID, nonce, session string) (string, bool, error) {
	remaining := deadline.Sub(options.Now())
	if remaining <= 0 {
		return "", false, context.DeadlineExceeded
	}
	if remaining > 10*time.Second {
		remaining = 10 * time.Second
	}
	deliveryCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	return options.Deliver(deliveryCtx, waitID, nonce, deadline, session)
}

func (s *Store) persistV2(ctx context.Context, rowPath string, expectNonce string, deadline time.Time, options WaitOptions, mutate func(*Waiter)) (Waiter, error) {
	var updated Waiter
	err := withWaiterLockBounded(ctx, s.Root, deadline, options.Now, options.Sleep, func() error {
		current, err := readV2Waiter(rowPath)
		if err != nil {
			return err
		}
		if current.WaitID == "" || current.Nonce != expectNonce {
			return fmt.Errorf("wait registration changed while it was being updated")
		}
		mutate(&current)
		if err := writeV2Waiter(rowPath, current); err != nil {
			return err
		}
		updated = current
		return nil
	})
	return updated, err
}

func waitEventFields(row Waiter) map[string]string {
	return map[string]string{
		"waitId": row.WaitID, "nonce": row.Nonce, "kind": row.Kind, "targetId": row.TargetID,
		"runtime": row.Runtime, "runtimeSession": row.RuntimeSession, "mainId": row.MainId,
		"mode": row.Mode, "registeredAt": row.RegisteredAt,
		"registeredBootNanos": strconv.FormatInt(row.RegisteredBootNanos, 10), "registeredBootId": row.RegisteredBootID,
		"accelerator": row.Accelerator,
	}
}

func emitWaitRegistered(root string, row Waiter, options WaitOptions) {
	_ = options.EmitEvent(root, "wait-registered", "wait registration became durable", waitEventFields(row))
}

func waitReturnedFields(row Waiter, result WaitResult) map[string]string {
	fields := waitEventFields(row)
	fields["mode"], fields["atEntry"], fields["state"] = result.Mode, strconv.FormatBool(result.AtEntry), result.State
	fields["exitCode"], fields["sourceOutcome"], fields["sourceEvidence"] = strconv.Itoa(result.ExitCode), result.SourceOutcome, result.SourceEvidence
	fields["ledgerTip"], fields["terminalStamp"] = result.LedgerTip, result.TerminalStamp
	fields["prevObservedAt"], fields["observedAt"], fields["returnedAt"] = result.PrevObservedAt, result.ObservedAt, result.ReturnedAt
	fields["prevObservedBootNanos"], fields["prevObservedBootId"] = strconv.FormatInt(result.PrevObservedBootNanos, 10), result.PrevObservedBootID
	fields["observedBootNanos"], fields["observedBootId"] = strconv.FormatInt(result.ObservedBootNanos, 10), result.ObservedBootID
	fields["returnedBootNanos"], fields["returnedBootId"] = strconv.FormatInt(result.ReturnedBootNanos, 10), result.ReturnedBootID
	return fields
}

func observedAfterRead(options WaitOptions, atEntry bool) finishStamps {
	stamps := finishStamps{AtEntry: atEntry}
	bootID, elapsed, err := options.BootClock()
	if err == nil {
		stamps.ObservedBootNanos, stamps.ObservedBootID = elapsed.Nanoseconds(), bootID
	}
	return stamps
}

func (s *Store) finishV2(ctx context.Context, rowPath string, row Waiter, options WaitOptions, code int, state, reason, outcome, evidence, tip, stamp string, stamps finishStamps) WaitResult {
	return s.finishV2By(ctx, rowPath, row, options, code, state, reason, outcome, evidence, tip, stamp, stamps, "")
}

func (s *Store) finishV2By(ctx context.Context, rowPath string, row Waiter, options WaitOptions, code int, state, reason, outcome, evidence, tip, stamp string, stamps finishStamps, by string) WaitResult {
	options = normalizeWaitOptions(options)
	now := options.Now().UTC()
	result := waitResult(row, code, state, reason, outcome, evidence, tip, stamp, now)
	returnedBootID, returnedBootElapsed, bootErr := options.BootClock()
	if bootErr == nil {
		result.ReturnedBootID, result.ReturnedBootNanos = returnedBootID, returnedBootElapsed.Nanoseconds()
	}
	result.AtEntry = stamps.AtEntry || row.AtEntry
	result.ObservedBootNanos, result.ObservedBootID = stamps.ObservedBootNanos, stamps.ObservedBootID
	if state != "ready" && result.ObservedBootNanos == 0 {
		result.ObservedBootNanos, result.ObservedBootID = row.LastObservedBootNanos, row.LastObservedBootID
	}
	deadline, _ := time.Parse(time.RFC3339Nano, row.Deadline)
	updated, err := s.persistV2(ctx, rowPath, row.Nonce, deadline, options, func(current *Waiter) {
		current.State = state
		current.Target = row.Target
		current.LastCheckedTip = tip
		current.LastPollAt = row.LastPollAt
		current.LastPollError = row.LastPollError
		current.SourceResultIdentity = evidence
		current.RemainingNanos = 0
		current.Result = &result
		if by != "" {
			current.InterruptedBy = by
		}
	})
	if err != nil {
		return waitResult(row, ExitWaiterIO, "failed", "wait result could not be durably recorded: "+err.Error(), "storage-failure", "", row.LastCheckedTip, "", now)
	}
	if emitErr := options.EmitEvent(s.Root, "wait-returned", "durable wait result returned to its caller", waitReturnedFields(updated, result)); emitErr != nil {
		result.ReturnEventFailed = true
		if repaired, repairErr := s.persistV2(ctx, rowPath, row.Nonce, deadline, options, func(current *Waiter) {
			current.Result = &result
		}); repairErr == nil {
			updated = repaired
		}
	}
	removeWaiterHint(rowPath, updated)
	return result
}

// InterruptWaiterRow ends one row through the same terminal transition the
// wait loop takes when its own context is cancelled, recording who ended it.
func (s *Store) InterruptWaiterRow(rowPath, by string, options WaitOptions) (Waiter, error) {
	options = normalizeWaitOptions(options)
	row, err := readV2Waiter(rowPath)
	if err != nil {
		return Waiter{}, err
	}
	for _, state := range WaiterStates {
		if state.Name == row.State && state.Class == WaiterStateEnded {
			return Waiter{}, fmt.Errorf("waiter row %s is already ended (state %q)", rowPath, row.State)
		}
	}
	result := s.finishV2By(context.Background(), rowPath, row, options, ExitInterrupted, WaiterStateInterrupted,
		"wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", finishStamps{}, by)
	if result.ExitCode != ExitInterrupted {
		return Waiter{}, fmt.Errorf("%s", result.Reason)
	}
	return readV2Waiter(rowPath)
}

// ClaimableGoalChange reports only additions and revision changes against the
// registration snapshot. Existing backlog and removals do not wake the seat.
func ClaimableGoalChange(baseline, current []ClaimableGoal) (ClaimableGoal, bool) {
	known := make(map[string]uint64, len(baseline))
	for _, item := range baseline {
		known[item.ID] = item.Revision
	}
	for _, item := range current {
		if revision, existed := known[item.ID]; !existed || revision != item.Revision {
			return item, true
		}
	}
	return ClaimableGoal{}, false
}

func adoptsPendingSetupTarget(pinned, observed WaiterTarget, outcome string) bool {
	return pinned.OperationID != "" && pinned.OperationID == observed.OperationID &&
		pinned.Round == 0 && pinned.StartedAt == "" &&
		observed.Round > 0 && observed.StartedAt != "" && outcome == "running"
}

func (s *Store) waitLoop(ctx context.Context, rowPath string, row Waiter, options WaitOptions, receiver HintReceiver) WaitResult {
	deadline, err := time.Parse(time.RFC3339Nano, row.Deadline)
	if err != nil {
		return s.finishV2(ctx, rowPath, row, options, ExitNoRecord, "failed", "waiter deadline is invalid", "invalid-source", "", "", "", finishStamps{})
	}
	var lastHintRead time.Time
	for {
		if err := ctx.Err(); err != nil {
			return s.finishV2(context.Background(), rowPath, row, options, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", finishStamps{})
		}
		now := options.Now().UTC()
		bootID, bootElapsed, bootErr := options.BootClock()
		iterationStamps := finishStamps{ObservedBootNanos: bootElapsed.Nanoseconds(), ObservedBootID: bootID}
		if bootErr != nil || bootID != row.DeadlineBootID {
			return s.finishV2(ctx, rowPath, row, options, ExitWaitDeadline, "deadline", "the boot clock changed or became unreadable", "wait-deadline", "", row.LastCheckedTip, "", iterationStamps)
		}
		if now.Before(mustParseWaitTime(row.RegisteredAt)) {
			return s.finishV2(ctx, rowPath, row, options, ExitWaiterIO, "failed", "the wall clock moved before the wait registration time", "clock-drift", "", row.LastCheckedTip, "", iterationStamps)
		}
		if !now.Before(deadline) || bootElapsed.Nanoseconds() >= row.BootDeadlineNanos {
			// Path waits have no hint receiver, so a condition that changes during
			// the final bounded sleep must be observed before the deadline is final.
			if row.Selector.Kind == "path" {
				observation, observeErr := readWaitSource(ctx, row.Selector, row.Target, row.LastCheckedTip, options)
				observationStamps := observedAfterRead(options, false)
				if observeErr == nil && !observation.Temporary && !observation.Pending {
					return s.finishV2(ctx, rowPath, row, options, observation.ExitCode, "ready", observation.Reason, observation.Outcome, observation.Evidence, observation.LedgerTip, observation.TerminalStamp, observationStamps)
				}
			}
			return s.finishV2(ctx, rowPath, row, options, ExitWaitDeadline, "deadline", "this wait reached its deadline", "wait-deadline", "", row.LastCheckedTip, "", iterationStamps)
		}
		if options.Actionable != nil {
			remaining := deadline.Sub(options.Now())
			if remaining > 10*time.Second {
				remaining = 10 * time.Second
			}
			actionCtx, cancel := context.WithTimeout(ctx, remaining)
			reason, changed, actionErr := options.Actionable(actionCtx, row)
			cancel()
			actionStamps := observedAfterRead(options, false)
			if ctx.Err() != nil {
				return s.finishV2(context.Background(), rowPath, row, options, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", iterationStamps)
			}
			if actionErr != nil {
				_, currentBoot, bootCheckErr := options.BootClock()
				if !options.Now().UTC().Before(deadline) || (bootCheckErr == nil && currentBoot.Nanoseconds() >= row.BootDeadlineNanos) {
					return s.finishV2(ctx, rowPath, row, options, ExitWaitDeadline, "deadline", "this wait reached its deadline during the actionable-work check", "wait-deadline", "", row.LastCheckedTip, "", iterationStamps)
				}
				return s.finishV2(ctx, rowPath, row, options, ExitWaiterIO, "failed", "actionable work could not be checked: "+actionErr.Error(), "storage-failure", "", row.LastCheckedTip, "", iterationStamps)
			}
			if changed {
				return s.finishV2(ctx, rowPath, row, options, 6, "ready", reason, "actionable-work", "", row.LastCheckedTip, "", actionStamps)
			}
		}
		observation, observeErr := s.observe(ctx, row, options, deadline)
		observationStamps := observedAfterRead(options, false)
		if ctx.Err() != nil {
			return s.finishV2(context.Background(), rowPath, row, options, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", iterationStamps)
		}
		if observeErr != nil || observation.Temporary {
			lastGood := time.Duration(row.LastObservedBootNanos)
			if lastGood <= 0 || bootElapsed-lastGood > 30*time.Second {
				reason := "source observation failed"
				if observeErr != nil {
					reason += ": " + observeErr.Error()
				}
				return s.finishV2(ctx, rowPath, row, options, ExitWaiterIO, "failed", reason, "transport-failure", "", row.LastCheckedTip, "", iterationStamps)
			}
		} else {
			if observation.PollAt != "" {
				row.LastPollAt = observation.PollAt
				row.LastPollError = observation.PollError
			}
			if observation.Incarnation != row.Target && adoptsPendingSetupTarget(row.Target, observation.Incarnation, observation.Outcome) {
				row.Target = observation.Incarnation
			} else if observation.Incarnation != row.Target {
				return s.finishV2(ctx, rowPath, row, options, ExitNoRecord, "failed", "the source identifier now names a different incarnation", "target-replaced", observation.Evidence, observation.LedgerTip, observation.TerminalStamp, iterationStamps)
			}
			// The wait's own event wins over a frontier change seen in the
			// same cycle: a matched act or landing is the result, and backlog
			// that became claimable is still claimable at the seat's next
			// turn end.
			if !observation.Pending {
				return s.finishV2(ctx, rowPath, row, options, observation.ExitCode, "ready", observation.Reason, observation.Outcome, observation.Evidence, observation.LedgerTip, observation.TerminalStamp, observationStamps)
			}
			if observation.ClaimableRead {
				if item, changed := ClaimableGoalChange(row.ClaimableGoals, observation.ClaimableGoals); changed {
					return s.finishV2(ctx, rowPath, row, options, 6, "ready", fmt.Sprintf("goal %s became claimable at revision %d", item.ID, item.Revision), "actionable-work", observation.Evidence, observation.LedgerTip, observation.TerminalStamp, observationStamps)
				}
			}
			updated, persistErr := s.persistV2(ctx, rowPath, row.Nonce, deadline, options, func(current *Waiter) {
				current.Target = row.Target
				current.LastObservedAt = now.Format(time.RFC3339Nano)
				current.LastObservedBootNanos = bootElapsed.Nanoseconds()
				current.LastObservedBootID = bootID
				current.LastCheckedTip = observation.LedgerTip
				current.LastPollAt = row.LastPollAt
				current.LastPollError = row.LastPollError
				current.RemainingNanos = minDuration(deadline.Sub(now), time.Duration(row.BootDeadlineNanos-bootElapsed.Nanoseconds())).Nanoseconds()
			})
			if persistErr != nil {
				return s.finishV2(ctx, rowPath, row, options, ExitWaiterIO, "failed", "successful observation could not be recorded: "+persistErr.Error(), "storage-failure", "", row.LastCheckedTip, "", iterationStamps)
			}
			row = updated
		}
		pause := 10 * time.Second
		wallLeft := deadline.Sub(options.Now())
		bootLeft := time.Duration(row.BootDeadlineNanos - bootElapsed.Nanoseconds())
		pause = minDuration(pause, minDuration(wallLeft, bootLeft))
		if pause <= 0 {
			continue
		}
		if receiver != nil {
			hintWaitStarted := options.Now()
			hinted, hintErr := receiver.Wait(ctx, pause)
			if hintErr == nil && hinted {
				if earliest := lastHintRead.Add(time.Second); !lastHintRead.IsZero() && options.Now().Before(earliest) {
					if err := options.Sleep(ctx, earliest.Sub(options.Now())); err != nil {
						return s.finishV2(context.Background(), rowPath, row, options, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", iterationStamps)
					}
				}
				lastHintRead = options.Now()
				continue
			}
			if hintErr == nil {
				continue
			}
			_ = receiver.Close()
			removeWaiterHint(rowPath, row)
			updated, persistErr := s.persistV2(ctx, rowPath, row.Nonce, deadline, options, func(current *Waiter) {
				current.HintPath = ""
				current.Accelerator = "unavailable"
			})
			if persistErr == nil {
				row = updated
			}
			receiver = nil
			pause -= options.Now().Sub(hintWaitStarted)
			if pause <= 0 {
				continue
			}
		}
		if err := options.Sleep(ctx, pause); err != nil {
			return s.finishV2(context.Background(), rowPath, row, options, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", iterationStamps)
		}
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func mustParseWaitTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

// Wait registers a version-2 row and blocks on bounded source observations.
func (s *Store) Wait(ctx context.Context, request WaitRequest, options WaitOptions) WaitResult {
	options = normalizeWaitOptions(options)
	if err := validateWaitRequest(request); err != nil {
		return invalidWaitResult(err.Error())
	}
	registeredAt := options.Now().UTC()
	deadline := registeredAt.Add(request.Timeout)
	baseRow := Waiter{
		Selector:     request.Selector,
		RegisteredAt: registeredAt.Format(time.RFC3339Nano),
		Deadline:     deadline.Format(time.RFC3339Nano),
	}
	waitID, err := mintNonce()
	if err != nil {
		return waitResult(baseRow, ExitWaiterIO, "failed", "a wait identifier could not be generated", "storage-failure", "", "", "", registeredAt)
	}
	baseRow.WaitID = waitID
	bootID, bootElapsed, err := options.BootClock()
	if err != nil || bootID == "" {
		return waitResult(baseRow, ExitWaiterIO, "failed", "the operating-system boot clock is unavailable", "clock-failure", "", "", "", registeredAt)
	}
	registrationStamps := finishStamps{ObservedBootNanos: bootElapsed.Nanoseconds(), ObservedBootID: bootID}
	initial, err := s.initialObservation(ctx, request.Selector, options, deadline)
	initialStamps := observedAfterRead(options, !initial.Pending)
	if err != nil {
		if ctx.Err() != nil {
			return waitResult(baseRow, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", "", "", options.Now())
		}
		return waitResult(baseRow, ExitWaiterIO, "failed", "the wait source could not be validated: "+err.Error(), "transport-failure", "", "", "", registeredAt)
	}
	if initial.Temporary {
		return waitResult(baseRow, ExitWaiterIO, "failed", "the wait source could not provide an initial successful observation", "transport-failure", "", initial.LedgerTip, "", registeredAt)
	}
	if waitSelectorHasIncarnation(request.Selector) && initial.Incarnation == (WaiterTarget{}) {
		code := initial.ExitCode
		if code == 0 {
			code = ExitNoRecord
		}
		reason := initial.Reason
		if reason == "" {
			reason = "the wait source did not provide an immutable incarnation"
		}
		return waitResult(baseRow, code, "failed", reason, initial.Outcome, initial.Evidence, initial.LedgerTip, initial.TerminalStamp, registeredAt)
	}
	baseRow.Target = initial.Incarnation
	nonce, err := mintNonce()
	if err != nil {
		return waitResult(baseRow, ExitWaiterIO, "failed", "a wait nonce could not be generated", "storage-failure", "", initial.LedgerTip, "", registeredAt)
	}
	exact, err := currentProcessIdentity(s.prober())
	if err != nil {
		return waitResult(baseRow, ExitWaiterUnknown, "failed", err.Error(), "uncertain-identity", "", initial.LedgerTip, "", registeredAt)
	}
	ownerDigest := OwnerDigest(request.Owner.MainId)
	rowPath := WaiterPath(s.Root, request.Selector.Kind, request.Selector.TargetID, ownerDigest)
	row := Waiter{
		SchemaVersion: 2, WaitID: waitID, Nonce: nonce, Kind: request.Selector.Kind,
		TargetID: request.Selector.TargetID, OwnerDigest: ownerDigest,
		Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), PidStartedAtMicro: processMicrosecondIdentity(exact), PidStartTicks: exact.StartTicks, BootID: exact.BootID,
		Session: request.Owner.SessionId, MainId: request.Owner.MainId, OwnerLineage: request.Owner.OwnerLineage,
		ClaimEpoch: request.Owner.ClaimEpoch, RuntimeSession: request.RuntimeSession, Runtime: options.Runtime,
		Mode: "register", AtEntry: !initial.Pending,
		Selector: request.Selector, GoalID: request.Selector.GoalID, Target: initial.Incarnation,
		OriginalCursor: request.Selector.After, LastCheckedTip: floorAfterObservation(request.Selector.After, initial),
		RegisteredAt: registeredAt.Format(time.RFC3339Nano), Deadline: deadline.Format(time.RFC3339Nano),
		BootDeadlineNanos: (bootElapsed + request.Timeout).Nanoseconds(), DeadlineBootID: bootID, RemainingNanos: request.Timeout.Nanoseconds(),
		LastObservedAt: registeredAt.Format(time.RFC3339Nano), LastObservedBootNanos: bootElapsed.Nanoseconds(), LastObservedBootID: bootID,
		RegisteredBootNanos: bootElapsed.Nanoseconds(), RegisteredBootID: bootID,
		OpenWorkSignature: request.OpenWorkSignature, State: "registering", Accelerator: "unavailable",
		ClaimableGoals: initial.ClaimableGoals, LastPollAt: initial.PollAt, LastPollError: initial.PollError,
	}
	var receiver HintReceiver
	if options.OpenHintReceiver != nil {
		row.HintPath = waiterHintPath(rowPath, nonce)
		if opened, openErr := options.OpenHintReceiver(row.HintPath, waitID, nonce); openErr == nil {
			receiver = opened
			row.Accelerator = "fifo"
		} else {
			row.HintPath = ""
		}
	}
	if receiver != nil {
		defer receiver.Close()
		defer os.Remove(waiterHintPath(rowPath, nonce))
	}
	if !initial.ClaimableRead && options.Claimable != nil && request.Selector.Kind == "goal" && request.Selector.Event == "human-act" {
		claimCtx, cancel := context.WithTimeout(ctx, minDuration(10*time.Second, deadline.Sub(options.Now())))
		row.ClaimableGoals, err = options.Claimable(claimCtx, row)
		cancel()
		if ctx.Err() != nil {
			return waitResult(row, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", initial.LedgerTip, "", options.Now())
		}
		if err != nil {
			return waitResult(row, ExitWaiterIO, "failed", "the claimable-work baseline could not be read: "+err.Error(), "storage-failure", "", initial.LedgerTip, "", options.Now())
		}
	}
	if err = withWaiterLockBounded(ctx, s.Root, deadline, options.Now, options.Sleep, func() error {
		existing, readErr := readV2Waiter(rowPath)
		if readErr == nil {
			if !supersedesStaleSession(existing, row) {
				if err := existingWaitRefuses(s.prober(), existing); err != nil {
					return err
				}
			}
			removeWaiterHint(rowPath, existing)
			removeV2Pointer(s.Root, existing)
		} else if !os.IsNotExist(readErr) {
			data, legacyErr := os.ReadFile(rowPath)
			if legacyErr != nil {
				return readErr
			}
			var legacy Waiter
			if json.Unmarshal(data, &legacy) != nil {
				return &waiterError{ExitWaiterUnknown, "existing waiter row is unreadable; refusing replacement"}
			}
			if err := existingWaitRefuses(s.prober(), legacy); err != nil {
				return err
			}
		}
		if err := writeV2Waiter(rowPath, row); err != nil {
			return err
		}
		return writeV2Pointer(s.Root, row, rowPath)
	}); err != nil {
		if ctx.Err() != nil {
			return waitResult(row, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", initial.LedgerTip, "", options.Now())
		}
		return waitResult(baseRow, WaiterExitCode(err), "failed", err.Error(), "registration-refused", "", initial.LedgerTip, "", registeredAt)
	}
	if options.Deliver == nil {
		return s.finishV2(ctx, rowPath, row, options, ExitWaiterBusy, "failed", "the runtime has no wait-delivery operation", "ineligible-registration", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	answer, declined, deliveryErr := deliverBeforeDeadline(ctx, deadline, options, waitID, nonce, request.RuntimeSession)
	if ctx.Err() != nil {
		return s.finishV2(context.Background(), rowPath, row, options, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	if declined {
		return s.finishV2(ctx, rowPath, row, options, ExitWaiterBusy, "failed", "the runtime declined blocking wait delivery", "ineligible-registration", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	if deliveryErr != nil || answer != "blocking" {
		reason := "wait delivery failed"
		if deliveryErr != nil {
			reason += ": " + deliveryErr.Error()
		}
		return s.finishV2(ctx, rowPath, row, options, ExitWaiterIO, "failed", reason, "transport-failure", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	row, err = s.persistV2(ctx, rowPath, nonce, deadline, options, func(current *Waiter) {
		current.State = "pending"
		current.Delivery = answer
	})
	if err != nil {
		return s.finishV2(ctx, rowPath, row, options, ExitWaiterIO, "failed", "pending registration could not be published: "+err.Error(), "storage-failure", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	emitWaitRegistered(s.Root, row, options)
	if !initial.Pending {
		return s.finishV2(ctx, rowPath, row, options, initial.ExitCode, "ready", initial.Reason, initial.Outcome, initial.Evidence, initial.LedgerTip, initial.TerminalStamp, initialStamps)
	}
	return s.waitLoop(ctx, rowPath, row, options, receiver)
}

func findWaiterByID(root, waitID string) (Waiter, string, bool, error) {
	if !nonceRe.MatchString(waitID) {
		return Waiter{}, "", false, fmt.Errorf("wait identifier is invalid")
	}
	pointer := WaiterPointerPath(root, waitID)
	data, err := os.ReadFile(pointer)
	if err != nil {
		if os.IsNotExist(err) {
			var matches []string
			paths, _ := filepath.Glob(filepath.Join(WaitersDir(root), "*.json"))
			for _, candidate := range paths {
				row, readErr := readV2Waiter(candidate)
				if readErr == nil && row.WaitID == waitID {
					expected := filepath.Base(WaiterPath(root, row.Kind, row.TargetID, row.OwnerDigest))
					if filepath.Base(candidate) != expected {
						return Waiter{}, "", false, fmt.Errorf("waiter row for %s is not stored under its owner key", waitID)
					}
					matches = append(matches, candidate)
				}
			}
			if len(matches) != 1 {
				return Waiter{}, "", false, fmt.Errorf("no unique waiter row exists for wait %s", waitID)
			}
			rowPath := matches[0]
			row, readErr := readV2Waiter(rowPath)
			return row, rowPath, true, readErr
		}
		return Waiter{}, "", false, err
	}
	name := strings.TrimSpace(string(data))
	if name == "" || filepath.Base(name) != name {
		return Waiter{}, "", false, fmt.Errorf("waiter pointer for %s is invalid", waitID)
	}
	rowPath := filepath.Join(WaitersDir(root), name)
	row, err := readV2Waiter(rowPath)
	if err != nil {
		return Waiter{}, "", false, err
	}
	if row.WaitID != waitID {
		return Waiter{}, "", false, fmt.Errorf("waiter pointer does not identify the requested wait")
	}
	return row, rowPath, false, nil
}

// FindWaiterByID reads a row without repairing storage. Command entrypoints
// use it before they have proved that the current holder owns or succeeds the
// row.
func FindWaiterByID(root, waitID string) (Waiter, string, error) {
	row, path, _, err := findWaiterByID(root, waitID)
	return row, path, err
}

func repairWaiterPointer(root, waitID, rowPath string) (Waiter, error) {
	var repaired Waiter
	err := withWaiterLock(root, func() error {
		current, readErr := readV2Waiter(rowPath)
		if readErr != nil || current.WaitID != waitID {
			return fmt.Errorf("waiter row changed while its pointer was repaired")
		}
		current.PointerRepaired = true
		if writeErr := writeV2Waiter(rowPath, current); writeErr != nil {
			return writeErr
		}
		if writeErr := writeV2Pointer(root, current, rowPath); writeErr != nil {
			return writeErr
		}
		repaired = current
		return nil
	})
	return repaired, err
}

// LoadWaiterByID uses the version-2 pointer and repairs a missing pointer from
// the unique owner-keyed row. Resume performs its ownership check before
// calling this repair path.
func LoadWaiterByID(root, waitID string) (Waiter, string, error) {
	row, path, missingPointer, err := findWaiterByID(root, waitID)
	if err != nil || !missingPointer {
		return row, path, err
	}
	repaired, err := repairWaiterPointer(root, waitID, path)
	return repaired, path, err
}

// floorAfterObservation is the last checked tip a new row records: the
// observation's tip while the wait is pending, but the cursor before the
// observation when it already matched, so a registration that ends before
// publishing its result never checks the event's commit off.
func floorAfterObservation(before string, observation SourceObservation) string {
	if !observation.Pending {
		return before
	}
	return observation.LedgerTip
}

// tipAfterFailure is the tip a failed or interrupted registration or renewal
// saves: the tip before the observation when the observation already
// matched, so the matched event stays above the floor a renewal reads from.
func tipAfterFailure(row Waiter, observation SourceObservation) string {
	if !observation.Pending {
		return row.LastCheckedTip
	}
	return observation.LedgerTip
}

func renewableWaitResult(code int) bool {
	switch code {
	case ExitWaitDeadline, ExitInterrupted, 6, ExitWaiterIO:
		return true
	default:
		return false
	}
}

var goalResultEvidenceRe = regexp.MustCompile(`^(?:ledger|commit):([0-9a-f]{40}|[0-9a-f]{64})(?::|$)`)

// replayObservationFloor starts a saved-result check at the newest durable
// ledger point the waiter already inspected. Published goal matches retain
// their event commit here; the goal observer can therefore prove that commit
// remains on the accepted branch without walking forward from the original
// selector cursor. Older version-2 rows fall back through the result's ledger
// tip to the original cursor and require no schema change.
func replayObservationFloor(row Waiter) string {
	if row.LastCheckedTip != "" {
		return row.LastCheckedTip
	}
	if row.Result != nil && row.Result.LedgerTip != "" {
		return row.Result.LedgerTip
	}
	return row.Selector.After
}

func goalResultAnchoredAtFloor(row Waiter, floor string) bool {
	if row.Selector.Kind != "goal" || row.Result == nil || floor == "" {
		return false
	}
	match := goalResultEvidenceRe.FindStringSubmatch(row.Result.SourceEvidence)
	return len(match) == 2 && match[1] == floor
}

func replayEvidenceInvalid(row Waiter, observation SourceObservation, floor string) bool {
	if observation.Incarnation != row.Target {
		return true
	}
	// An invalid-source observation disproves every saved result except a
	// saved exit 4, whose source-terminal fact is that exact invalid state.
	if observation.Outcome == "invalid-source" && row.Result.ExitCode != ExitNoRecord {
		return true
	}
	if row.Result.SourceEvidence == "" || row.Result.ExitCode == 6 {
		return false
	}
	// For a goal match, the evidence names an immutable commit and the floor
	// is that commit. A successful pending observation from this floor proves
	// that the accepted ledger still contains the saved act or landing.
	if goalResultAnchoredAtFloor(row, floor) {
		return false
	}
	return observation.Pending || observation.Evidence != row.Result.SourceEvidence
}

func (s *Store) replaySavedWait(ctx context.Context, row Waiter, options WaitOptions) WaitResult {
	if options.Observe == nil {
		return waitResult(row, ExitWaiterIO, "failed", "the wait source reader is unavailable", "transport-failure", "", row.LastCheckedTip, "", options.Now())
	}
	floor := replayObservationFloor(row)
	validationCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	observation, observeErr := readWaitSource(validationCtx, row.Selector, row.Target, floor, options)
	cancel()
	if observeErr != nil || observation.Temporary {
		if errors.Is(ctx.Err(), context.Canceled) {
			return waitResult(row, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", options.Now())
		}
		reason := "the saved result's source evidence could not be checked"
		if observeErr != nil {
			reason += ": " + observeErr.Error()
		}
		return waitResult(row, ExitWaiterIO, "failed", reason, "transport-failure", "", row.LastCheckedTip, "", options.Now())
	}
	if replayEvidenceInvalid(row, observation, floor) {
		return waitResult(row, ExitNoRecord, "failed", "the saved result no longer matches its source evidence", "invalid-source", "", row.LastCheckedTip, "", options.Now())
	}
	replayed := *row.Result
	replayed.PointerRepaired = row.PointerRepaired
	replayed.Mode = "replay"
	replayed.AtEntry = false
	replayed.ReturnedAt = options.Now().UTC().Format(time.RFC3339Nano)
	if bootID, bootElapsed, bootErr := options.BootClock(); bootErr == nil {
		replayed.ReturnedBootID, replayed.ReturnedBootNanos = bootID, bootElapsed.Nanoseconds()
	}
	if emitErr := options.EmitEvent(s.Root, "wait-returned", "retained wait result replayed to its caller", waitReturnedFields(row, replayed)); emitErr != nil {
		replayed.ReturnEventFailed = true
	}
	return replayed
}

func (s *Store) renewSavedWait(ctx context.Context, rowPath string, row Waiter, owner Caller, runtimeSession string, renewal time.Duration, options WaitOptions) WaitResult {
	now := options.Now().UTC()
	deadline := now.Add(renewal)
	bootID, bootElapsed, bootErr := options.BootClock()
	if bootErr != nil || bootID == "" {
		return waitResult(row, ExitWaiterIO, "failed", "the operating-system boot clock is unavailable", "clock-failure", "", row.LastCheckedTip, "", now)
	}
	registrationStamps := finishStamps{ObservedBootNanos: bootElapsed.Nanoseconds(), ObservedBootID: bootID}
	if options.Observe == nil {
		return waitResult(row, ExitWaiterIO, "failed", "the wait source reader is unavailable", "transport-failure", "", row.LastCheckedTip, "", now)
	}
	readBudget := minDuration(10*time.Second, deadline.Sub(options.Now()))
	if readBudget <= 0 {
		return waitResult(row, ExitWaitDeadline, "deadline", "this wait reached its renewal deadline", "wait-deadline", "", row.LastCheckedTip, "", now)
	}
	readCtx, cancel := context.WithTimeout(ctx, readBudget)
	// A renewal reads from the last checked tip, as a running wait does. A
	// registration or renewal that matched its event and then ended before
	// publishing never advanced that tip past the event, so the event is
	// still above the floor here.
	initial, observeErr := readWaitSource(readCtx, row.Selector, WaiterTarget{}, row.LastCheckedTip, options)
	cancel()
	initialStamps := observedAfterRead(options, !initial.Pending)
	if ctx.Err() != nil {
		return waitResult(row, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", options.Now())
	}
	if observeErr != nil || initial.Temporary {
		reason := "the wait source could not provide a renewal observation"
		if observeErr != nil {
			reason += ": " + observeErr.Error()
		}
		return waitResult(row, ExitWaiterIO, "failed", reason, "transport-failure", "", row.LastCheckedTip, "", options.Now())
	}
	if waitSelectorHasIncarnation(row.Selector) && initial.Incarnation == (WaiterTarget{}) {
		code := initial.ExitCode
		if code == 0 {
			code = ExitNoRecord
		}
		return waitResult(row, code, "failed", initial.Reason, initial.Outcome, initial.Evidence, tipAfterFailure(row, initial), initial.TerminalStamp, options.Now())
	}
	if initial.Incarnation != row.Target {
		if adoptsPendingSetupTarget(row.Target, initial.Incarnation, initial.Outcome) {
			row.Target = initial.Incarnation
		} else {
			return waitResult(row, ExitNoRecord, "failed", "the source identifier now names a different incarnation", "target-replaced", initial.Evidence, tipAfterFailure(row, initial), initial.TerminalStamp, options.Now())
		}
	}
	openWorkSignature := row.OpenWorkSignature
	if options.OpenWorkSignature != nil {
		scanBudget := minDuration(10*time.Second, deadline.Sub(options.Now()))
		if scanBudget <= 0 {
			return waitResult(row, ExitWaitDeadline, "deadline", "this wait reached its renewal deadline during the open-work scan", "wait-deadline", "", row.LastCheckedTip, "", options.Now())
		}
		scanCtx, scanCancel := context.WithTimeout(ctx, scanBudget)
		var scanErr error
		openWorkSignature, scanErr = options.OpenWorkSignature(scanCtx)
		scanCancel()
		if ctx.Err() != nil {
			return waitResult(row, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", options.Now())
		}
		if scanErr != nil {
			code, outcome := ExitWaiterIO, "storage-failure"
			if !options.Now().Before(deadline) {
				code, outcome = ExitWaitDeadline, "wait-deadline"
			}
			return waitResult(row, code, "failed", "the open-work signature could not be read during renewal: "+scanErr.Error(), outcome, "", row.LastCheckedTip, "", options.Now())
		}
	}
	exact, identityErr := currentProcessIdentity(s.prober())
	if identityErr != nil {
		return waitResult(row, ExitWaiterUnknown, "failed", identityErr.Error(), "uncertain-identity", "", row.LastCheckedTip, "", options.Now())
	}
	newNonce, nonceErr := mintNonce()
	if nonceErr != nil {
		return waitResult(row, ExitWaiterIO, "failed", "a renewal nonce could not be generated", "storage-failure", "", row.LastCheckedTip, "", options.Now())
	}
	newOwnerDigest := OwnerDigest(owner.MainId)
	newPath := WaiterPath(s.Root, row.Kind, row.TargetID, newOwnerDigest)
	var receiver HintReceiver
	newHintPath := ""
	if options.OpenHintReceiver != nil {
		newHintPath = waiterHintPath(newPath, newNonce)
		if opened, openErr := options.OpenHintReceiver(newHintPath, row.WaitID, newNonce); openErr == nil {
			receiver = opened
		} else {
			newHintPath = ""
		}
	}
	if receiver != nil {
		defer receiver.Close()
		defer os.Remove(newHintPath)
	}
	oldNonce := row.Nonce
	savedResult := *row.Result
	err := withWaiterLockBounded(ctx, s.Root, deadline, options.Now, options.Sleep, func() error {
		current, readErr := readV2Waiter(rowPath)
		if readErr != nil || current.Nonce != oldNonce || current.Result == nil || current.Result.ExitCode != savedResult.ExitCode || current.Result.ReturnedAt != savedResult.ReturnedAt {
			return fmt.Errorf("wait registration changed before renewal")
		}
		if newPath != rowPath {
			if successor, successorErr := readV2Waiter(newPath); successorErr == nil {
				if refusal := existingWaitRefuses(s.prober(), successor); refusal != nil {
					return refusal
				}
				removeWaiterHint(newPath, successor)
				removeV2Pointer(s.Root, successor)
			} else if !os.IsNotExist(successorErr) {
				return fmt.Errorf("the successor's waiter row is unreadable")
			}
		}
		current.Renewals = append(current.Renewals, WaitRenewal{Result: savedResult, RenewedAt: now.Format(time.RFC3339Nano)})
		current.Result = nil
		current.State, current.Delivery = "registering", ""
		current.Nonce = newNonce
		current.Pid, current.PidStartedAt, current.PidStartedAtMicro = exact.Pid, exact.StartedAt.Unix(), processMicrosecondIdentity(exact)
		current.PidStartTicks, current.BootID = exact.StartTicks, exact.BootID
		current.Session, current.MainId, current.OwnerDigest = owner.SessionId, owner.MainId, newOwnerDigest
		current.OwnerLineage, current.ClaimEpoch, current.RuntimeSession = owner.OwnerLineage, owner.ClaimEpoch, runtimeSession
		current.Runtime, current.Mode, current.AtEntry = options.Runtime, "renew", !initial.Pending
		current.Target = row.Target
		current.RegisteredAt, current.Deadline = now.Format(time.RFC3339Nano), deadline.Format(time.RFC3339Nano)
		current.RemainingNanos = renewal.Nanoseconds()
		current.BootDeadlineNanos, current.DeadlineBootID = (bootElapsed + renewal).Nanoseconds(), bootID
		current.LastObservedAt, current.LastObservedBootNanos, current.LastObservedBootID = now.Format(time.RFC3339Nano), bootElapsed.Nanoseconds(), bootID
		current.RegisteredBootNanos, current.RegisteredBootID = bootElapsed.Nanoseconds(), bootID
		current.OpenWorkSignature = openWorkSignature
		current.SourceResultIdentity = ""
		current.LastCheckedTip = tipAfterFailure(row, initial)
		current.ClaimableGoals = initial.ClaimableGoals
		current.LastPollAt, current.LastPollError = initial.PollAt, initial.PollError
		current.ResumedBy = &ResumeIdentity{Session: owner.SessionId, Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), PidStartedAtMicro: processMicrosecondIdentity(exact), PidStartTicks: exact.StartTicks, BootID: exact.BootID}
		removeWaiterHint(rowPath, current)
		current.HintPath, current.Accelerator = newHintPath, "unavailable"
		if receiver != nil {
			current.Accelerator = "fifo"
		}
		if writeErr := writeV2Waiter(newPath, current); writeErr != nil {
			return writeErr
		}
		if writeErr := writeV2Pointer(s.Root, current, newPath); writeErr != nil {
			if newPath != rowPath {
				_ = os.Remove(newPath)
			}
			return writeErr
		}
		if newPath != rowPath {
			if removeErr := os.Remove(rowPath); removeErr != nil {
				_ = writeV2Pointer(s.Root, current, rowPath)
				_ = os.Remove(newPath)
				return removeErr
			}
		}
		row = current
		return nil
	})
	if err != nil {
		return waitResult(row, WaiterExitCode(err), "failed", "wait could not be renewed: "+err.Error(), "storage-failure", "", row.LastCheckedTip, "", options.Now())
	}
	if options.Deliver == nil {
		return s.finishV2(ctx, newPath, row, options, ExitWaiterBusy, "failed", "the runtime has no wait-delivery operation", "ineligible-registration", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	answer, declined, deliveryErr := deliverBeforeDeadline(ctx, deadline, options, row.WaitID, row.Nonce, runtimeSession)
	if ctx.Err() != nil {
		return s.finishV2(context.Background(), newPath, row, options, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	if declined {
		return s.finishV2(ctx, newPath, row, options, ExitWaiterBusy, "failed", "the runtime declined blocking wait delivery", "ineligible-registration", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	if deliveryErr != nil || answer != "blocking" {
		reason := "wait delivery failed during renewal"
		if deliveryErr != nil {
			reason += ": " + deliveryErr.Error()
		}
		return s.finishV2(ctx, newPath, row, options, ExitWaiterIO, "failed", reason, "transport-failure", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	row, err = s.persistV2(ctx, newPath, row.Nonce, deadline, options, func(current *Waiter) {
		current.State = "pending"
		current.Delivery = answer
	})
	if err != nil {
		return s.finishV2(ctx, newPath, row, options, ExitWaiterIO, "failed", "renewed pending registration could not be published: "+err.Error(), "storage-failure", "", tipAfterFailure(row, initial), "", registrationStamps)
	}
	emitWaitRegistered(s.Root, row, options)
	if !initial.Pending {
		return s.finishV2(ctx, newPath, row, options, initial.ExitCode, "ready", initial.Reason, initial.Outcome, initial.Evidence, initial.LedgerTip, initial.TerminalStamp, initialStamps)
	}
	return s.waitLoop(ctx, newPath, row, options, receiver)
}

// ResumeWait replays a retained result or takes over a dead waiter from its
// durable row. A non-zero renewal is an explicit new bounded registration.
func (s *Store) ResumeWait(ctx context.Context, waitID string, owner Caller, runtimeSession string, renewal time.Duration, options WaitOptions) WaitResult {
	options = normalizeWaitOptions(options)
	if !ValidWaitID(waitID) {
		return invalidWaitResult("wait identifier is invalid")
	}
	row, rowPath, missingPointer, err := findWaiterByID(s.Root, waitID)
	if err != nil {
		return waitResult(Waiter{WaitID: waitID}, ExitNoRecord, "failed", err.Error(), "missing-registration", "", "", "", options.Now())
	}
	succeeded := options.SucceededMainID != "" && options.SucceededMainID == row.MainId && options.SucceededOwnerLineage != "" && options.SucceededOwnerLineage == row.OwnerLineage
	if owner.OwnerLineage == "" || (owner.OwnerLineage != row.OwnerLineage && !succeeded) {
		return waitResult(row, 6, "ready", "the wait owner lineage changed", "ownership-changed", "", row.LastCheckedTip, "", options.Now())
	}
	if succeeded && (owner.SessionId == "" || owner.SessionId != runtimeSession || row.Session != owner.SessionId || row.RuntimeSession != runtimeSession) {
		return waitResult(row, ExitWaiterBusy, "failed", "the successor must register a fresh wait for its authenticated session", "registration-refused", "", row.LastCheckedTip, "", options.Now())
	}
	if missingPointer {
		row, err = repairWaiterPointer(s.Root, waitID, rowPath)
		if err != nil {
			return waitResult(row, ExitWaiterIO, "failed", "the owned wait pointer could not be repaired: "+err.Error(), "storage-failure", "", row.LastCheckedTip, "", options.Now())
		}
	}
	if renewal < 0 || renewal > 24*time.Hour {
		return invalidWaitResult("resume timeout must be positive and no longer than 24 hours")
	}
	if row.Result != nil {
		if renewal == 0 || !renewableWaitResult(row.Result.ExitCode) {
			return s.replaySavedWait(ctx, row, options)
		}
		return s.renewSavedWait(ctx, rowPath, row, owner, runtimeSession, renewal, options)
	}
	switch identity.AliveRef(s.prober(), identity.Ref{Pid: row.Pid, StartedAtSec: row.PidStartedAt, StartedAtUnixMicro: row.PidStartedAtMicro, StartTicks: row.PidStartTicks, BootID: row.BootID}) {
	case identity.Alive:
		return waitResult(row, ExitWaiterBusy, "failed", "the recorded waiter is still live", "registration-refused", "", row.LastCheckedTip, "", options.Now())
	case identity.Unknown:
		return waitResult(row, ExitWaiterUnknown, "failed", "the recorded waiter identity is uncertain", "uncertain-identity", "", row.LastCheckedTip, "", options.Now())
	}
	now := options.Now().UTC()
	bootID, bootElapsed, bootErr := options.BootClock()
	deadline := mustParseWaitTime(row.Deadline)
	if renewal == 0 {
		if bootErr != nil || bootID != row.DeadlineBootID || !now.Before(deadline) || bootElapsed.Nanoseconds() >= row.BootDeadlineNanos {
			return s.finishV2(ctx, rowPath, row, options, ExitWaitDeadline, "deadline", "the saved wait has no remaining bounded time", "wait-deadline", "", row.LastCheckedTip, "", finishStamps{})
		}
		remaining := minDuration(time.Duration(row.RemainingNanos), minDuration(deadline.Sub(now), time.Duration(row.BootDeadlineNanos-bootElapsed.Nanoseconds())))
		deadline = now.Add(remaining)
		row.RemainingNanos = remaining.Nanoseconds()
	} else {
		if renewal <= 0 {
			return invalidWaitResult("resume timeout must be positive")
		}
		if bootErr != nil {
			return waitResult(row, ExitWaiterIO, "failed", "the operating-system boot clock is unavailable", "clock-failure", "", row.LastCheckedTip, "", now)
		}
		deadline = now.Add(renewal)
		row.RegisteredAt = now.Format(time.RFC3339Nano)
		row.RemainingNanos = renewal.Nanoseconds()
		row.BootDeadlineNanos = (bootElapsed + renewal).Nanoseconds()
		row.DeadlineBootID = bootID
	}
	exact, err := currentProcessIdentity(s.prober())
	if err != nil {
		return waitResult(row, ExitWaiterUnknown, "failed", err.Error(), "uncertain-identity", "", row.LastCheckedTip, "", now)
	}
	newNonce, err := mintNonce()
	if err != nil {
		return waitResult(row, ExitWaiterIO, "failed", "a resume nonce could not be generated", "storage-failure", "", row.LastCheckedTip, "", now)
	}
	newOwnerDigest := OwnerDigest(owner.MainId)
	newPath := WaiterPath(s.Root, row.Kind, row.TargetID, newOwnerDigest)
	var receiver HintReceiver
	newHintPath := ""
	if options.OpenHintReceiver != nil {
		newHintPath = waiterHintPath(newPath, newNonce)
		if opened, openErr := options.OpenHintReceiver(newHintPath, row.WaitID, newNonce); openErr == nil {
			receiver = opened
		} else {
			newHintPath = ""
		}
	}
	if receiver != nil {
		defer receiver.Close()
		defer os.Remove(newHintPath)
	}
	if options.Deliver == nil {
		return waitResult(row, ExitWaiterBusy, "failed", "the runtime has no wait-delivery operation", "ineligible-registration", "", row.LastCheckedTip, "", now)
	}
	answer, declined, deliveryErr := deliverBeforeDeadline(ctx, deadline, options, row.WaitID, newNonce, runtimeSession)
	if ctx.Err() != nil {
		return s.finishV2(context.Background(), rowPath, row, options, ExitInterrupted, "interrupted", "wait command was interrupted", "interrupted", "", row.LastCheckedTip, "", finishStamps{})
	}
	if declined {
		return waitResult(row, ExitWaiterBusy, "failed", "the runtime declined blocking wait delivery", "ineligible-registration", "", row.LastCheckedTip, "", now)
	}
	if deliveryErr != nil || answer != "blocking" {
		return waitResult(row, ExitWaiterIO, "failed", "wait delivery failed during resume", "transport-failure", "", row.LastCheckedTip, "", now)
	}
	oldNonce := row.Nonce
	err = withWaiterLockBounded(ctx, s.Root, deadline, options.Now, options.Sleep, func() error {
		current, err := readV2Waiter(rowPath)
		if err != nil {
			if latest, _, latestErr := FindWaiterByID(s.Root, row.WaitID); latestErr == nil {
				if refusal := existingWaitRefuses(s.prober(), latest); refusal != nil {
					return refusal
				}
			}
			return fmt.Errorf("wait registration changed before resume")
		}
		if current.Nonce != oldNonce || current.State != "pending" {
			return fmt.Errorf("wait registration changed before resume")
		}
		current.Nonce = newNonce
		current.Pid, current.PidStartedAt, current.PidStartedAtMicro = exact.Pid, exact.StartedAt.Unix(), processMicrosecondIdentity(exact)
		current.PidStartTicks, current.BootID, current.DeadlineBootID = exact.StartTicks, exact.BootID, bootID
		current.Session, current.MainId, current.OwnerDigest = owner.SessionId, owner.MainId, newOwnerDigest
		current.OwnerLineage = owner.OwnerLineage
		current.ClaimEpoch, current.RuntimeSession = owner.ClaimEpoch, runtimeSession
		current.Runtime, current.Mode, current.AtEntry = options.Runtime, "takeover", false
		current.RegisteredAt, current.Deadline, current.RemainingNanos = row.RegisteredAt, deadline.Format(time.RFC3339Nano), row.RemainingNanos
		current.BootDeadlineNanos, current.DeadlineBootID = row.BootDeadlineNanos, row.DeadlineBootID
		current.Delivery = answer
		current.ResumedBy = &ResumeIdentity{Session: owner.SessionId, Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), PidStartedAtMicro: processMicrosecondIdentity(exact), PidStartTicks: exact.StartTicks, BootID: exact.BootID}
		removeWaiterHint(rowPath, current)
		current.HintPath, current.Accelerator = newHintPath, "unavailable"
		if receiver != nil {
			current.Accelerator = "fifo"
		}
		if newPath != rowPath {
			if successor, successorErr := readV2Waiter(newPath); successorErr == nil {
				if refusal := existingWaitRefuses(s.prober(), successor); refusal != nil {
					return refusal
				}
				removeWaiterHint(newPath, successor)
				removeV2Pointer(s.Root, successor)
			} else if !os.IsNotExist(successorErr) {
				data, readErr := os.ReadFile(newPath)
				if readErr != nil {
					return fmt.Errorf("the successor's waiter row is unreadable")
				}
				var legacy Waiter
				if json.Unmarshal(data, &legacy) != nil {
					return &waiterError{ExitWaiterUnknown, "the successor's waiter row cannot be identity-checked"}
				}
				if refusal := existingWaitRefuses(s.prober(), legacy); refusal != nil {
					return refusal
				}
			}
		}
		if err := writeV2Waiter(newPath, current); err != nil {
			return err
		}
		if err := writeV2Pointer(s.Root, current, newPath); err != nil {
			if newPath != rowPath {
				_ = os.Remove(newPath)
			}
			return err
		}
		if newPath != rowPath {
			if err := os.Remove(rowPath); err != nil {
				_ = writeV2Pointer(s.Root, row, rowPath)
				_ = os.Remove(newPath)
				return err
			}
		}
		row = current
		return nil
	})
	if err != nil {
		code := WaiterExitCode(err)
		outcome := "storage-failure"
		if code == ExitWaiterBusy || code == ExitWaiterUnknown {
			outcome = "registration-refused"
		}
		return waitResult(row, code, "failed", "wait could not be resumed: "+err.Error(), outcome, "", row.LastCheckedTip, "", now)
	}
	emitWaitRegistered(s.Root, row, options)
	return s.waitLoop(ctx, newPath, row, options, receiver)
}

// PendingWaiters returns only schema-2 pending rows; legacy watch rows cannot
// become restart commands or pending-wait evidence.
func PendingWaiters(root, ownerLineage string) ([]Waiter, []error) {
	return PendingWaitersForLineages(root, []string{ownerLineage})
}

// PendingWaitersForLineages includes only the current lineage and any
// predecessor lineage already proven from this checkout's lease.
func PendingWaitersForLineages(root string, ownerLineages []string) ([]Waiter, []error) {
	allowed := make(map[string]bool, len(ownerLineages))
	for _, lineage := range ownerLineages {
		if lineage != "" {
			allowed[lineage] = true
		}
	}
	if len(allowed) == 0 {
		return nil, nil
	}
	paths, _ := filepath.Glob(filepath.Join(WaitersDir(root), "*.json"))
	var rows []Waiter
	var failures []error
	for _, path := range paths {
		row, err := readV2Waiter(path)
		if err != nil {
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				var legacy Waiter
				if json.Unmarshal(data, &legacy) == nil && legacy.SchemaVersion == 0 {
					continue
				}
			}
			failures = append(failures, fmt.Errorf("%s: %w", filepath.Base(path), err))
			continue
		}
		if row.State == "pending" && allowed[row.OwnerLineage] {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].WaitID < rows[j].WaitID })
	return rows, failures
}

// ObserveRun translates the run owner's validated record into the public wait
// outcomes while pinning generation and launch nonce.
func (s *Store) ObserveRun(ctx context.Context, selector WaitSelector, _ WaiterTarget, _ string) (SourceObservation, error) {
	select {
	case <-ctx.Done():
		return SourceObservation{}, ctx.Err()
	default:
	}
	record, err := s.Read(selector.TargetID)
	if err != nil || record == nil {
		reason := "the run record is missing"
		if err != nil {
			reason = "the run record is invalid: " + err.Error()
		}
		return SourceObservation{ExitCode: ExitNoRecord, Reason: reason, Outcome: "invalid-source"}, nil
	}
	if record.RunId != selector.TargetID {
		return SourceObservation{ExitCode: ExitNoRecord, Reason: "the run record identifies another run", Outcome: "invalid-source"}, nil
	}
	incarnation := WaiterTarget{Generation: record.Generation, LaunchNonce: record.LaunchNonce}
	evidence := fmt.Sprintf("run:%s:g%d:%s", record.RunId, record.Generation, record.LaunchNonce)
	observation := SourceObservation{Pending: true, Incarnation: incarnation, Outcome: record.Status, Evidence: evidence}
	switch record.Status {
	case StatusLaunching, StatusRunning, StatusDraining:
		return observation, nil
	case StatusGreen:
		observation.Pending, observation.ExitCode, observation.Reason = false, ExitGreen, "run completed green"
	case StatusRed:
		observation.Pending, observation.ExitCode, observation.Reason = false, ExitRed, "run completed red"
	case StatusEndedUnknown:
		observation.Pending, observation.ExitCode, observation.Reason = false, ExitEndedUnknown, "run ended with an unknown result"
	case StatusLaunchFailed:
		observation.Pending, observation.ExitCode, observation.Reason = false, ExitLaunchFailed, "run launch failed"
	default:
		observation.Pending, observation.ExitCode, observation.Reason, observation.Outcome = false, ExitNoRecord, "the run record has an unknown status", "invalid-source"
	}
	if record.EndedAt != nil {
		observation.TerminalStamp = *record.EndedAt
	}
	return observation, nil
}

// Watch blocks until the run record is terminal and returns the pinned
// exit code. It registers its waiter record on entry and removes it on
// every exit path. Watch is OPEN — it polls the record; conclusions
// belong to the watcher and the record-writer path.
// Watch blocks until the run concludes and PRINTS a typed line for
// every exit including success — a watcher that ends in silence is
// indistinguishable from one that died, and the silent-conclusion
// defect class exists because this function once returned codes
// without a word (run-watch-silent-conclusion).
func (s *Store) Watch(id string, owner Caller, poll time.Duration, out io.Writer) int {
	if out == nil {
		out = io.Discard
	}
	line := func(outcome string, rc int, record *Record) int {
		log := ""
		if record != nil {
			log = record.Log
		}
		fmt.Fprintf(out, "run %s %s rc=%d log=%s\n", id, outcome, rc, log)
		return rc
	}
	record, err := s.Read(id)
	if err != nil || record == nil {
		return line("no-record", ExitNoRecord, nil)
	}
	target := WaiterTarget{Generation: record.Generation, LaunchNonce: record.LaunchNonce}
	if err := s.RegisterWaiter("run", id, owner, target); err != nil {
		return WaiterExitCode(err)
	}
	defer s.RemoveWaiter("run", id, owner)
	if poll <= 0 {
		poll = 2 * time.Second
	}
	for {
		record, err := s.Read(id)
		if err != nil || record == nil {
			return line("record-vanished", ExitNoRecord, nil)
		}
		if record.Generation != target.Generation || record.LaunchNonce != target.LaunchNonce {
			// The public id now names a DIFFERENT lifecycle (reuse or
			// adoption): this waiter's target is gone.
			return line("target-replaced", ExitNoRecord, record)
		}
		switch record.Status {
		case StatusGreen:
			return line("green", ExitGreen, record)
		case StatusRed:
			return line("red", ExitRed, record)
		case StatusEndedUnknown:
			return line("ended-unknown", ExitEndedUnknown, record)
		case StatusLaunchFailed:
			return line("launch-failed", ExitLaunchFailed, record)
		}
		time.Sleep(poll)
	}
}

// List returns every parsed record plus the unreadable paths — readers
// surface failures, never skip them.
func (s *Store) List() (records []Record, unreadable []string) {
	for _, path := range RecordFiles(s.Root) {
		data, err := os.ReadFile(path)
		if err != nil {
			unreadable = append(unreadable, path+": "+err.Error())
			continue
		}
		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			unreadable = append(unreadable, path+": unparsable run record")
			continue
		}
		if problems := Validate(&record); len(problems) > 0 {
			unreadable = append(unreadable, path+": "+problems[0])
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt < records[j].StartedAt })
	return records, unreadable
}
