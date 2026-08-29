package gaterun

// The battery weight file is one cross-process transaction domain. Every
// read-modify-publish transition holds battery-weight.flock, a stable sibling
// that survives atomic replacement of battery-weight.json. A checkpoint owns
// only the weight that existed when its runner started; later landings remain
// after a green reset.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
)

var (
	ErrCheckpointLive    = errors.New("battery checkpoint runner is still alive")
	ErrCheckpointUnknown = errors.New("battery checkpoint runner liveness is unknown")
	ErrStaleCheckpoint   = errors.New("battery checkpoint is stale or already terminal")
	ErrResetRequiresFull = errors.New("battery weight reset requires a FULL run")
)

// RunClass records whether the validation root proved the engine itself or
// imported witness proof. Descendant deduplication does not change FULL.
type RunClass string

const (
	FullRun            RunClass = "FULL"
	WitnessAssistedRun RunClass = "WITNESS-ASSISTED"
)

func ParseRunClass(value string) (RunClass, error) {
	class := RunClass(value)
	switch class {
	case FullRun, WitnessAssistedRun:
		return class, nil
	default:
		return "", fmt.Errorf("unknown battery run class %q", value)
	}
}

// RunnerIdentity is the complete process identity used for supersession.
type RunnerIdentity struct {
	PID          int64  `json:"pid"`
	StartedAtSec int64  `json:"startedAtSec"`
	StartTicks   int64  `json:"startTicks,omitempty"`
	BootID       string `json:"bootId,omitempty"`
}

func runnerIdentity(ref identity.Ref) RunnerIdentity {
	return RunnerIdentity{PID: ref.Pid, StartedAtSec: ref.StartedAtSec, StartTicks: ref.StartTicks, BootID: ref.BootID}
}

func (r RunnerIdentity) ref() identity.Ref {
	return identity.Ref{Pid: r.PID, StartedAtSec: r.StartedAtSec, StartTicks: r.StartTicks, BootID: r.BootID}
}

// WeightCheckpoint is the one open battery transaction.
type WeightCheckpoint struct {
	RunID             string         `json:"runId"`
	Subject           string         `json:"subject"`
	OpenedGeneration  uint64         `json:"openedGeneration"`
	Accumulated       int64          `json:"accumulated"`
	Landings          int64          `json:"landings"`
	OpenedAtUTC       string         `json:"openedAtUtc"`
	Runner            RunnerIdentity `json:"runner"`
	RepairDestination string         `json:"repairDestination"`
}

// ResetResult is both the reset command's report and reset.json's content.
// Subject comes from the checkpoint; LastCommit remains landing-owned.
type ResetResult struct {
	RunID                 string   `json:"runId"`
	Subject               string   `json:"subject"`
	RunClass              RunClass `json:"runClass,omitempty"`
	CheckpointGeneration  uint64   `json:"checkpointGeneration"`
	ResetGeneration       uint64   `json:"resetGeneration"`
	ResetAtUTC            string   `json:"resetAtUtc"`
	CheckpointAccumulated int64    `json:"checkpointAccumulated"`
	CheckpointLandings    int64    `json:"checkpointLandings"`
	RemainingAccumulated  int64    `json:"remainingAccumulated"`
	RemainingLandings     int64    `json:"remainingLandings"`
	RemainingSinceUTC     string   `json:"remainingSinceUtc"`
	LastCommit            string   `json:"lastCommit"`
}

// PendingReset retains everything needed to repair reset.json after the
// checkpoint has already been consumed.
type PendingReset struct {
	Destination string      `json:"destination"`
	Result      ResetResult `json:"result"`
}

// PendingAbandon retains a terminal abandonment until abandoned.json is
// durably present. Best-effort records come only from an evidence-copy
// failure; an unavailable destination may be retired after one repair attempt
// because the checkpoint is already terminal and must not block later runs.
type PendingAbandon struct {
	Destination string        `json:"destination"`
	Result      AbandonResult `json:"result"`
	BestEffort  bool          `json:"bestEffort,omitempty"`
}

// WeightState is the accumulator, machine-local under artifacts/agents.
type WeightState struct {
	Generation             uint64            `json:"generation"`
	Accumulated            int64             `json:"accumulated"`
	Landings               int64             `json:"landings"`
	SinceUTC               string            `json:"sinceUtc"`
	LastCommit             string            `json:"lastCommit"`
	PostCheckpointSinceUTC string            `json:"postCheckpointSinceUtc,omitempty"`
	Checkpoint             *WeightCheckpoint `json:"checkpoint,omitempty"`
	PendingReset           *PendingReset     `json:"pendingReset,omitempty"`
	PendingAbandon         *PendingAbandon   `json:"pendingAbandon,omitempty"`
}

type CheckpointRequest struct {
	RunID             string
	Subject           string
	RunnerPID         int64
	RepairDestination string
	Now               time.Time
}

type CheckpointResult struct {
	State      WeightState       `json:"state"`
	Checkpoint WeightCheckpoint  `json:"checkpoint"`
	Superseded *WeightCheckpoint `json:"superseded,omitempty"`
}

type AbandonResult struct {
	RunID             string `json:"runId"`
	Subject           string `json:"subject"`
	Reason            string `json:"reason"`
	AbandonedAtUTC    string `json:"abandonedAtUtc"`
	Generation        uint64 `json:"generation"`
	WeightPreserved   int64  `json:"weightPreserved"`
	LandingsPreserved int64  `json:"landingsPreserved"`
	AppendixPublished bool   `json:"appendixPublished"`
	AppendixError     string `json:"appendixError,omitempty"`
}

// ResetAppendixPendingError means the subtraction is committed but the
// appendix or the state cleanup still needs read-side repair.
type ResetAppendixPendingError struct{ Cause error }

func (e *ResetAppendixPendingError) Error() string {
	return fmt.Sprintf("battery reset consumed its checkpoint but reset.json remains repairable: %v", e.Cause)
}

func weightPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "battery-weight.json")
}

// WeightLockPath is public for fixture assertions.
func WeightLockPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "battery-weight.flock")
}

type weightLock struct{ file *os.File }

func acquireWeightLock(root string) (*weightLock, error) {
	path := WeightLockPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	for {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("battery weight lock: %w", err)
	}
	return &weightLock{file: file}, nil
}

func (l *weightLock) release() {
	_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	_ = l.file.Close()
}

var weightNow = func() time.Time { return time.Now().UTC() }

var writeWeightState = func(root string, state WeightState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(weightPath(root), string(data)+"\n", root)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("battery weight state published but directory durability is unknown")
	}
	return nil
}

var publishWeightAppendix = func(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(path, string(data)+"\n", filepath.Dir(path))
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("appendix published but directory durability is unknown")
	}
	return nil
}

func initialWeight(now time.Time) WeightState {
	return WeightState{SinceUTC: now.UTC().Format(time.RFC3339)}
}

func loadWeightLocked(root string, now time.Time) (WeightState, error) {
	data, err := os.ReadFile(weightPath(root))
	if os.IsNotExist(err) {
		return initialWeight(now), nil
	}
	if err != nil {
		return WeightState{}, fmt.Errorf("read battery weight: %w", err)
	}
	var state WeightState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return WeightState{}, fmt.Errorf("malformed battery weight state: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return WeightState{}, fmt.Errorf("malformed battery weight state: trailing JSON content")
	}
	if err := validateWeightState(state); err != nil {
		return WeightState{}, fmt.Errorf("malformed battery weight state: %w", err)
	}
	return state, nil
}

func validateAccumulation(label string, accumulated, landings int64) error {
	if accumulated < 0 || landings < 0 {
		return fmt.Errorf("%s contains a negative count", label)
	}
	if accumulated > 0 && landings == 0 {
		return fmt.Errorf("%s has weight without a landing", label)
	}
	return nil
}

func validateGenerationLandingHistory(label string, generation, baseGeneration uint64, landings, baseLandings int64) error {
	if baseGeneration > generation || baseLandings > landings {
		return fmt.Errorf("%s is outside state history", label)
	}
	if generation-baseGeneration != uint64(landings-baseLandings) {
		return fmt.Errorf("%s generation and landing counts disagree", label)
	}
	return nil
}

func validateWeightState(state WeightState) error {
	if err := validateAccumulation("accumulator", state.Accumulated, state.Landings); err != nil {
		return err
	}
	since, err := time.Parse(time.RFC3339, state.SinceUTC)
	if err != nil {
		return fmt.Errorf("sinceUtc: %w", err)
	}
	var postCheckpoint time.Time
	if state.PostCheckpointSinceUTC != "" {
		postCheckpoint, err = time.Parse(time.RFC3339, state.PostCheckpointSinceUTC)
		if err != nil {
			return fmt.Errorf("postCheckpointSinceUtc: %w", err)
		}
		if state.Checkpoint == nil {
			return fmt.Errorf("postCheckpointSinceUtc has no open checkpoint")
		}
	}
	if checkpoint := state.Checkpoint; checkpoint != nil {
		if checkpoint.RunID == "" || checkpoint.Subject == "" || !filepath.IsAbs(checkpoint.RepairDestination) {
			return fmt.Errorf("open checkpoint identity is incomplete")
		}
		if checkpoint.OpenedGeneration == 0 || checkpoint.OpenedGeneration > state.Generation {
			return fmt.Errorf("open checkpoint generation is outside state history")
		}
		if checkpoint.Runner.PID <= 0 || checkpoint.Runner.StartedAtSec <= 0 {
			return fmt.Errorf("open checkpoint runner identity is incomplete")
		}
		if checkpoint.Runner.StartTicks < 0 || (checkpoint.Runner.StartTicks == 0) != (checkpoint.Runner.BootID == "") {
			return fmt.Errorf("open checkpoint runner pair identity is incomplete")
		}
		if err := validateAccumulation("open checkpoint", checkpoint.Accumulated, checkpoint.Landings); err != nil {
			return err
		}
		if checkpoint.Accumulated > state.Accumulated || checkpoint.Landings > state.Landings {
			return fmt.Errorf("open checkpoint exceeds accumulator")
		}
		if err := validateGenerationLandingHistory("open checkpoint provenance", state.Generation, checkpoint.OpenedGeneration, state.Landings, checkpoint.Landings); err != nil {
			return err
		}
		if !postCheckpoint.IsZero() && checkpoint.Landings == state.Landings {
			return fmt.Errorf("postCheckpointSinceUtc has no post-checkpoint landing")
		}
		opened, err := time.Parse(time.RFC3339, checkpoint.OpenedAtUTC)
		if err != nil {
			return fmt.Errorf("checkpoint openedAtUtc: %w", err)
		}
		if opened.Before(since) || (!postCheckpoint.IsZero() && postCheckpoint.Before(opened)) {
			return fmt.Errorf("checkpoint timestamps are outside the accumulator window")
		}
	}
	terminalRecords := 0
	if state.PendingReset != nil {
		terminalRecords++
		if err := validatePendingReset(state, *state.PendingReset); err != nil {
			return err
		}
	}
	if state.PendingAbandon != nil {
		terminalRecords++
		if err := validatePendingAbandon(state, *state.PendingAbandon); err != nil {
			return err
		}
	}
	if terminalRecords > 1 || (terminalRecords == 1 && state.Checkpoint != nil) {
		return fmt.Errorf("terminal repair record and open checkpoint cannot coexist")
	}
	return nil
}

func validatePendingReset(state WeightState, pending PendingReset) error {
	result := pending.Result
	if !filepath.IsAbs(pending.Destination) || result.RunID == "" || result.Subject == "" {
		return fmt.Errorf("pending reset repair identity is incomplete")
	}
	if result.RunClass != "" && result.RunClass != FullRun {
		return fmt.Errorf("pending reset carries a non-FULL run class")
	}
	// Landings lawfully advance the state's generation past the reset
	// while its appendix is pending, so the reset must sit in the
	// state's PAST — never its future, never at or before its own
	// checkpoint.
	if result.CheckpointGeneration == 0 || result.ResetGeneration <= result.CheckpointGeneration || result.ResetGeneration > state.Generation {
		return fmt.Errorf("pending reset generations are outside state history")
	}
	resetAt, err := time.Parse(time.RFC3339, result.ResetAtUTC)
	if err != nil {
		return fmt.Errorf("pending reset resetAtUtc: %w", err)
	}
	remainingSince, err := time.Parse(time.RFC3339, result.RemainingSinceUTC)
	if err != nil {
		return fmt.Errorf("pending reset remainingSinceUtc: %w", err)
	}
	if remainingSince.After(resetAt) {
		return fmt.Errorf("pending reset remaining window starts after reset")
	}
	if err := validateAccumulation("pending reset checkpoint", result.CheckpointAccumulated, result.CheckpointLandings); err != nil {
		return err
	}
	if err := validateAccumulation("pending reset remainder", result.RemainingAccumulated, result.RemainingLandings); err != nil {
		return err
	}
	if result.ResetGeneration-result.CheckpointGeneration != uint64(result.RemainingLandings)+1 {
		return fmt.Errorf("pending reset checkpoint provenance generation and landing counts disagree")
	}
	// Landings may lawfully fold in while the appendix is pending, so
	// the recorded remainder is a FLOOR of the current accumulator,
	// never an equality — adds only ever increase both counters, and
	// they may also advance LastCommit past the reset's record.
	if result.RemainingAccumulated > state.Accumulated || result.RemainingLandings > state.Landings || result.RemainingSinceUTC != state.SinceUTC {
		return fmt.Errorf("pending reset result conflicts with current accumulator")
	}
	if err := validateGenerationLandingHistory("pending reset provenance", state.Generation, result.ResetGeneration, state.Landings, result.RemainingLandings); err != nil {
		return err
	}
	sameGeneration := result.ResetGeneration == state.Generation
	if sameGeneration && result.LastCommit != state.LastCommit {
		return fmt.Errorf("pending reset last commit conflicts with unchanged accumulator")
	}
	return nil
}

func validatePendingAbandon(state WeightState, pending PendingAbandon) error {
	result := pending.Result
	if !filepath.IsAbs(pending.Destination) || result.RunID == "" || result.Subject == "" || result.Reason == "" {
		return fmt.Errorf("pending abandonment repair identity is incomplete")
	}
	if result.Generation == 0 || result.Generation > state.Generation {
		return fmt.Errorf("pending abandonment generation is outside state history")
	}
	abandonedAt, err := time.Parse(time.RFC3339, result.AbandonedAtUTC)
	if err != nil {
		return fmt.Errorf("pending abandonment abandonedAtUtc: %w", err)
	}
	since, _ := time.Parse(time.RFC3339, state.SinceUTC)
	if abandonedAt.Before(since) {
		return fmt.Errorf("pending abandonment predates the accumulator window")
	}
	if err := validateAccumulation("pending abandonment", result.WeightPreserved, result.LandingsPreserved); err != nil {
		return err
	}
	if result.WeightPreserved > state.Accumulated || result.LandingsPreserved > state.Landings {
		return fmt.Errorf("pending abandonment counts conflict with current accumulator")
	}
	if err := validateGenerationLandingHistory("pending abandonment provenance", state.Generation, result.Generation, state.Landings, result.LandingsPreserved); err != nil {
		return err
	}
	if !result.AppendixPublished || result.AppendixError != "" {
		return fmt.Errorf("pending abandonment does not contain its intended terminal appendix")
	}
	return nil
}

// LandingWeight measures NUL-delimited `git --numstat -z --no-renames`
// records. A newline-only legacy stream is accepted for direct callers, but
// landing wrappers use NUL so path bytes are never split by shell grammar.
func LandingWeight(numstat []byte, prefix string) (int64, error) {
	policy, err := behaviorsurface.Load()
	if err != nil {
		return 0, err
	}
	separator := byte(0)
	if !bytes.ContainsRune(numstat, 0) {
		separator = '\n'
	}
	var weight, lines int64
	for len(numstat) > 0 {
		index := bytes.IndexByte(numstat, separator)
		var row []byte
		if index < 0 {
			row, numstat = numstat, nil
		} else {
			row, numstat = numstat[:index], numstat[index+1:]
		}
		first := bytes.IndexByte(row, '\t')
		if first < 0 {
			continue
		}
		secondRelative := bytes.IndexByte(row[first+1:], '\t')
		if secondRelative < 0 {
			continue
		}
		second := first + 1 + secondRelative
		path := string(row[second+1:])
		included, err := policy.Includes(behaviorsurface.Landing, path, prefix)
		if err != nil {
			return 0, err
		}
		if !included {
			continue
		}
		normalized, err := behaviorsurface.NormalizePath(path, prefix)
		if err != nil {
			return 0, err
		}
		if normalized == "" {
			normalized, err = behaviorsurface.NormalizePath(path, "")
			if err != nil {
				return 0, err
			}
		}
		perFile := int64(1)
		if strings.HasSuffix(normalized, ".go") && !strings.HasSuffix(normalized, "_test.go") {
			perFile = 3
		}
		weight += perFile
		for _, field := range [][]byte{row[:first], row[first+1 : second]} {
			if string(field) == "-" {
				continue
			}
			value, err := strconv.ParseInt(string(field), 10, 64)
			if err != nil || value < 0 {
				return 0, fmt.Errorf("invalid numstat count %q for %q", field, path)
			}
			lines += value
		}
	}
	return weight + lines/100, nil
}

func repairResetAppendixLocked(root string, state *WeightState) error {
	pending := state.PendingReset
	if pending == nil {
		return nil
	}
	path := filepath.Join(pending.Destination, "reset.json")
	want, err := json.MarshalIndent(pending.Result, "", "  ")
	if err != nil {
		return err
	}
	want = append(want, '\n')
	if existing, readErr := os.ReadFile(path); readErr == nil {
		if !bytes.Equal(existing, want) {
			return fmt.Errorf("reset appendix at %s conflicts with pending repair", path)
		}
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("read reset appendix: %w", readErr)
	} else if err := publishWeightAppendix(path, pending.Result); err != nil {
		return err
	}
	resetAt, err := time.Parse(time.RFC3339, pending.Result.ResetAtUTC)
	if err != nil {
		return err
	}
	if _, err := retrodebt.Raise(root, retrodebt.KindBattery, pending.Result.RunID, resetAt); err != nil {
		return fmt.Errorf("raise battery retro debt: %w", err)
	}
	if err := narratordigest.Append(root, []narratordigest.Entry{{
		Kind: "highlight", Text: "The milestone battery discharged green.",
		SourceType: "episode", SourceID: pending.Result.RunID,
	}}, resetAt); err != nil {
		return fmt.Errorf("record green battery digest: %w", err)
	}
	next := *state
	next.PendingReset = nil
	next.Generation++
	if err := writeWeightState(root, next); err != nil {
		return err
	}
	*state = next
	return nil
}

func repairAbandonAppendixLocked(root string, state *WeightState) error {
	pending := state.PendingAbandon
	if pending == nil {
		return nil
	}
	abandonedAt, err := time.Parse(time.RFC3339, pending.Result.AbandonedAtUTC)
	if err != nil {
		return err
	}
	if err := narratordigest.Append(root, []narratordigest.Entry{{
		Kind: "lowlight", Text: "The milestone battery ended red: " + pending.Result.Reason + ".",
		SourceType: "episode", SourceID: pending.Result.RunID,
	}}, abandonedAt); err != nil {
		return fmt.Errorf("record red battery digest: %w", err)
	}
	path := filepath.Join(pending.Destination, "abandoned.json")
	want, err := json.MarshalIndent(pending.Result, "", "  ")
	if err != nil {
		return err
	}
	want = append(want, '\n')
	if existing, readErr := os.ReadFile(path); readErr == nil {
		if !bytes.Equal(existing, want) {
			return fmt.Errorf("abandonment appendix at %s conflicts with pending repair", path)
		}
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("read abandonment appendix: %w", readErr)
	} else if err := publishWeightAppendix(path, pending.Result); err != nil {
		if !pending.BestEffort {
			return err
		}
		next := *state
		next.PendingAbandon = nil
		next.Generation++
		if writeErr := writeWeightState(root, next); writeErr != nil {
			return fmt.Errorf("retire unavailable best-effort abandonment appendix: %w", writeErr)
		}
		*state = next
		return nil
	}
	next := *state
	next.PendingAbandon = nil
	next.Generation++
	if err := writeWeightState(root, next); err != nil {
		return err
	}
	*state = next
	return nil
}

func repairTerminalAppendicesLocked(root string, state *WeightState) error {
	if err := repairResetAppendixLocked(root, state); err != nil {
		return err
	}
	return repairAbandonAppendixLocked(root, state)
}

// WeightAdd folds one landing into the accumulator under the sibling lock.
func WeightAdd(root, commit string, numstat []byte, prefix string, threshold int64) (WeightState, bool, error) {
	weight, err := LandingWeight(numstat, prefix)
	if err != nil {
		return WeightState{}, false, err
	}
	lock, err := acquireWeightLock(root)
	if err != nil {
		return WeightState{}, false, err
	}
	defer lock.release()
	now := weightNow()
	state, err := loadWeightLocked(root, now)
	if err != nil {
		return WeightState{}, false, err
	}
	// A landing PRESERVES pending terminal-repair data and never
	// executes the repair itself: repair belongs to the read and
	// checkpoint paths, and an add arriving between a failed appendix
	// publish and the next read must fold its weight around the
	// replay data, not through it.
	next := state
	next.Accumulated += weight
	next.Landings++
	next.LastCommit = commit
	if next.Checkpoint != nil && next.PostCheckpointSinceUTC == "" {
		next.PostCheckpointSinceUTC = now.Format(time.RFC3339)
	}
	next.Generation++
	if err := validateWeightState(next); err != nil {
		return WeightState{}, false, fmt.Errorf("landing would persist invalid battery weight state: %w", err)
	}
	if err := writeWeightState(root, next); err != nil {
		return WeightState{}, false, err
	}
	return next, threshold > 0 && next.Accumulated >= threshold, nil
}

// WeightCheck reads strictly and repairs a consumed checkpoint's missing
// reset appendix before returning.
func WeightCheck(root string, threshold int64) (WeightState, bool, error) {
	lock, err := acquireWeightLock(root)
	if err != nil {
		return WeightState{}, false, err
	}
	defer lock.release()
	state, err := loadWeightLocked(root, weightNow())
	if err != nil {
		return WeightState{}, false, err
	}
	if err := repairTerminalAppendicesLocked(root, &state); err != nil {
		return WeightState{}, false, fmt.Errorf("battery terminal read-side repair: %w", err)
	}
	return state, threshold > 0 && state.Accumulated >= threshold, nil
}

// WeightCheckpointOpen opens the one battery transaction, superseding an
// earlier one only when full process identity proves its runner dead.
func WeightCheckpointOpen(root string, request CheckpointRequest, prober identity.Prober) (CheckpointResult, error) {
	if request.RunID == "" || request.Subject == "" || request.RunnerPID <= 0 || !filepath.IsAbs(request.RepairDestination) {
		return CheckpointResult{}, fmt.Errorf("checkpoint requires run id, subject, positive runner pid, and absolute repair destination")
	}
	now := request.Now.UTC()
	if now.IsZero() {
		now = weightNow()
	}
	exact, live, err := prober.Probe(request.RunnerPID)
	if err != nil || live != identity.Alive {
		return CheckpointResult{}, fmt.Errorf("checkpoint runner identity is not readable and alive: %v", err)
	}
	lock, err := acquireWeightLock(root)
	if err != nil {
		return CheckpointResult{}, err
	}
	defer lock.release()
	state, err := loadWeightLocked(root, now)
	if err != nil {
		return CheckpointResult{}, err
	}
	if err := repairTerminalAppendicesLocked(root, &state); err != nil {
		return CheckpointResult{}, fmt.Errorf("repair pending terminal appendix before checkpoint: %w", err)
	}
	var superseded *WeightCheckpoint
	if state.Checkpoint != nil {
		switch identity.AliveRef(prober, state.Checkpoint.Runner.ref()) {
		case identity.Alive:
			return CheckpointResult{}, fmt.Errorf("%w: run %s", ErrCheckpointLive, state.Checkpoint.RunID)
		case identity.Unknown:
			return CheckpointResult{}, fmt.Errorf("%w: run %s", ErrCheckpointUnknown, state.Checkpoint.RunID)
		case identity.Dead:
			copy := *state.Checkpoint
			superseded = &copy
		}
	}
	next := state
	next.PostCheckpointSinceUTC = ""
	next.Generation++
	checkpoint := WeightCheckpoint{
		RunID: request.RunID, Subject: request.Subject, OpenedGeneration: next.Generation,
		Accumulated: state.Accumulated, Landings: state.Landings,
		OpenedAtUTC: now.Format(time.RFC3339), Runner: runnerIdentity(exact.Ref()),
		RepairDestination: request.RepairDestination,
	}
	next.Checkpoint = &checkpoint
	if err := writeWeightState(root, next); err != nil {
		return CheckpointResult{}, err
	}
	return CheckpointResult{State: next, Checkpoint: checkpoint, Superseded: superseded}, nil
}

// WeightAbandon terminalizes a non-green run without subtracting anything.
// bestEffortAppendix is reserved for evidence-copy failure, where an
// unwritable durable destination must not leave a live runner blocking all
// future batteries.
func WeightAbandon(root, runID, reason string, bestEffortAppendix bool) (AbandonResult, error) {
	if runID == "" || reason == "" {
		return AbandonResult{}, fmt.Errorf("abandon requires run id and reason")
	}
	lock, err := acquireWeightLock(root)
	if err != nil {
		return AbandonResult{}, err
	}
	defer lock.release()
	now := weightNow()
	state, err := loadWeightLocked(root, now)
	if err != nil {
		return AbandonResult{}, err
	}
	if state.Checkpoint == nil || state.Checkpoint.RunID != runID {
		return AbandonResult{}, ErrStaleCheckpoint
	}
	result := AbandonResult{
		RunID: runID, Subject: state.Checkpoint.Subject, Reason: reason,
		AbandonedAtUTC: now.Format(time.RFC3339), Generation: state.Generation + 1,
		WeightPreserved: state.Accumulated, LandingsPreserved: state.Landings,
		AppendixPublished: true,
	}
	next := state
	next.Checkpoint = nil
	next.PostCheckpointSinceUTC = ""
	next.Generation++
	next.PendingAbandon = &PendingAbandon{
		Destination: state.Checkpoint.RepairDestination,
		Result:      result,
		BestEffort:  bestEffortAppendix,
	}
	if err := writeWeightState(root, next); err != nil {
		return result, err
	}
	if err := narratordigest.Append(root, []narratordigest.Entry{{
		Kind: "lowlight", Text: "The milestone battery ended red: " + reason + ".",
		SourceType: "episode", SourceID: runID,
	}}, now); err != nil {
		return result, fmt.Errorf("battery abandonment landed but its narrator digest did not: %w", err)
	}
	appendixPath := filepath.Join(state.Checkpoint.RepairDestination, "abandoned.json")
	if err := publishWeightAppendix(appendixPath, result); err != nil {
		failed := result
		failed.AppendixPublished = false
		failed.AppendixError = err.Error()
		if !bestEffortAppendix {
			return failed, err
		}
		cleared := next
		cleared.PendingAbandon = nil
		cleared.Generation++
		if writeErr := writeWeightState(root, cleared); writeErr != nil {
			return failed, fmt.Errorf("abandonment terminalized but best-effort repair retirement failed: %w", writeErr)
		}
		return failed, nil
	}
	cleared := next
	cleared.PendingAbandon = nil
	cleared.Generation++
	if err := writeWeightState(root, cleared); err != nil {
		return result, fmt.Errorf("abandonment appendix published but repair cleanup remains pending: %w", err)
	}
	return result, nil
}

// WeightReset consumes exactly the named checkpoint. LastCommit is not
// touched: it always names the newest landing, including one added during the
// battery.
func WeightReset(root, runID string, runClass RunClass) (WeightState, ResetResult, error) {
	if runID == "" {
		return WeightState{}, ResetResult{}, fmt.Errorf("reset requires run id")
	}
	if runClass != FullRun {
		return WeightState{}, ResetResult{}, ErrResetRequiresFull
	}
	lock, err := acquireWeightLock(root)
	if err != nil {
		return WeightState{}, ResetResult{}, err
	}
	defer lock.release()
	now := weightNow()
	state, err := loadWeightLocked(root, now)
	if err != nil {
		return WeightState{}, ResetResult{}, err
	}
	checkpoint := state.Checkpoint
	if checkpoint == nil || checkpoint.RunID != runID {
		return state, ResetResult{}, ErrStaleCheckpoint
	}
	if state.Accumulated < checkpoint.Accumulated || state.Landings < checkpoint.Landings {
		return state, ResetResult{}, fmt.Errorf("checkpoint exceeds current accumulator")
	}
	next := state
	next.Accumulated -= checkpoint.Accumulated
	next.Landings -= checkpoint.Landings
	resetAt := now.Format(time.RFC3339)
	if next.Accumulated == 0 && next.Landings == 0 {
		next.SinceUTC = resetAt
	} else if state.PostCheckpointSinceUTC != "" {
		next.SinceUTC = state.PostCheckpointSinceUTC
	} else {
		next.SinceUTC = resetAt
	}
	next.Checkpoint = nil
	next.PostCheckpointSinceUTC = ""
	next.Generation++
	result := ResetResult{
		RunID: runID, Subject: checkpoint.Subject, RunClass: runClass,
		CheckpointGeneration: checkpoint.OpenedGeneration, ResetGeneration: next.Generation,
		ResetAtUTC: resetAt, CheckpointAccumulated: checkpoint.Accumulated,
		CheckpointLandings: checkpoint.Landings, RemainingAccumulated: next.Accumulated,
		RemainingLandings: next.Landings, RemainingSinceUTC: next.SinceUTC,
		LastCommit: next.LastCommit,
	}
	next.PendingReset = &PendingReset{Destination: checkpoint.RepairDestination, Result: result}
	if err := writeWeightState(root, next); err != nil {
		return state, ResetResult{}, err
	}
	appendixPath := filepath.Join(checkpoint.RepairDestination, "reset.json")
	if err := publishWeightAppendix(appendixPath, result); err != nil {
		return next, result, &ResetAppendixPendingError{Cause: err}
	}
	if err := repairResetAppendixLocked(root, &next); err != nil {
		return next, result, &ResetAppendixPendingError{Cause: err}
	}
	return next, result, nil
}
