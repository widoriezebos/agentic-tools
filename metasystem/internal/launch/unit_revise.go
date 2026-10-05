package launch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// UnitRevision is one retained correction request of a run: the attempt it
// corrects, the digest of its frozen brief (and of the frozen decisions file
// when one was given) and the attempt it creates. It is written into the run
// before the new attempt is added or anything is launched, so a repeated or
// interrupted request reaches the same attempt instead of another one.
type UnitRevision struct {
	After              int    `json:"after"`
	Attempt            int    `json:"attempt"`
	BriefSHA256        string `json:"briefSha256"`
	Brief              string `json:"brief"`
	DecisionsSHA256    string `json:"decisionsSha256,omitempty"`
	Decisions          string `json:"decisions,omitempty"`
	RequestedAtUnixSec int64  `json:"requestedAt"`
}

// UnitRevisionRequest asks for one correction. After is the attempt being
// corrected; zero means the request is first matched against the run's
// retained requests and otherwise binds the current attempt. Brief and
// Decisions are the caller's bytes; they are frozen with the request.
type UnitRevisionRequest struct {
	Run       string
	After     int
	Brief     []byte
	Decisions []byte
}

// UnitRevisionResult is the attempt a request reached. Rejoined is true when
// an earlier identical request already created it; Current is the run's
// newest attempt, which differs from Attempt when later work followed.
type UnitRevisionResult struct {
	UnitResult
	Revision UnitRevision
	Rejoined bool
	Current  int
}

// ErrUnitRevisionStale is returned when After names an attempt that is no
// longer the run's newest and no identical request exists for it.
var ErrUnitRevisionStale = errors.New("a correction names the newest attempt, and this one is not")

// Revise creates, or rejoins, the one attempt a correction request asks for.
// It holds the unit's named lock (when the run is a named unit) and the run
// lock, so concurrent identical calls reach one attempt: the second is told
// the unit is busy and repeating the same command continues it.
func (runner *UnitRunner) Revise(request UnitRevisionRequest) (UnitRevisionResult, error) {
	if runner.Manager == nil {
		return UnitRevisionResult{}, errors.New("unit launch manager is unavailable")
	}
	if len(request.Brief) == 0 {
		return UnitRevisionResult{}, coded("UNIT_FOLLOW_UP_MISSING", "", errors.New("the correction brief is empty"))
	}
	if request.After < 0 {
		return UnitRevisionResult{}, coded("UNIT_REVISION_INVALID", fmt.Sprintf("after=%d", request.After), fmt.Errorf("--after %d is not an attempt number", request.After))
	}
	record, err := runner.read(request.Run)
	if err != nil {
		return UnitRevisionResult{}, err
	}
	bound := *runner
	worktree, key, identityErr := namedUnitIdentity(UnitPlan{Worktree: record.Worktree, Goal: record.Goal, Unit: record.Unit})
	if identityErr == nil {
		entry, found, err := runner.readNamed(key)
		if err != nil {
			return UnitRevisionResult{}, err
		}
		if found && entry.Run == record.ID {
			lock, err := runner.namedLock(key, UnitPlan{Unit: record.Unit, Goal: record.Goal})
			if err != nil {
				return UnitRevisionResult{}, err
			}
			defer releaseUnitLock(lock)
			if entry, found, err = runner.readNamed(key); err != nil {
				return UnitRevisionResult{}, err
			}
			if !found || entry.Run != record.ID || entry.Digest == "" {
				return UnitRevisionResult{}, coded("UNIT_NAMED_ENTRY_CORRUPT", unitFacts(record.Unit, record.Goal, "run="+record.ID), fmt.Errorf("the record of unit %s changed while this command held it; run the command again", record.Unit))
			}
			bound.named = &namedBinding{unit: record.Unit, goal: record.Goal, worktree: worktree, digest: entry.Digest, run: entry.Run,
				options: UnitOptions{BuildModel: record.BuildModel, BuildEffort: record.BuildEffort}}
		}
	}
	return bound.reviseLocked(request)
}

func (runner *UnitRunner) reviseLocked(request UnitRevisionRequest) (UnitRevisionResult, error) {
	lock, err := runner.lock(request.Run)
	if err != nil {
		return UnitRevisionResult{}, err
	}
	defer releaseUnitLock(lock)
	record, err := runner.read(request.Run)
	if err != nil {
		return UnitRevisionResult{}, err
	}
	plan, err := readUnitPlan(record.Plan, record.PlanDirectory)
	if err != nil {
		return UnitRevisionResult{}, err
	}
	if runner.named != nil {
		runner.named.plan = plan
		if err := runner.named.verify(runner); err != nil {
			return UnitRevisionResult{}, err
		}
	}
	briefDigest := digestHex(request.Brief)
	decisionsDigest := ""
	if len(request.Decisions) > 0 {
		decisionsDigest = digestHex(request.Decisions)
	}
	current := len(record.Rounds)
	same := func(revision UnitRevision) bool {
		return revision.BriefSHA256 == briefDigest && revision.DecisionsSHA256 == decisionsDigest
	}
	var retained *UnitRevision
	for index := range record.Revisions {
		revision := record.Revisions[index]
		if request.After != 0 && revision.After != request.After {
			continue
		}
		if same(revision) {
			retained = &revision
		} else if request.After != 0 {
			return UnitRevisionResult{}, coded("UNIT_REVISION_CONFLICT", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d attempt=%d", revision.After, revision.Attempt)),
				fmt.Errorf("attempt %d was already corrected by another brief, which started attempt %d", revision.After, revision.Attempt))
		}
	}
	if retained == nil {
		after := request.After
		if after == 0 {
			after = current
		}
		if after != current || current == 0 {
			return UnitRevisionResult{Current: current}, coded("UNIT_REVISION_STALE", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d current=%d", after, current)),
				fmt.Errorf("%w (you named %d, the newest is %d)", ErrUnitRevisionStale, after, current))
		}
		if record.State != "awaiting-judgement" {
			return UnitRevisionResult{Current: current}, coded("UNIT_RUN_NOT_AWAITING", unitFacts(record.Unit, record.Goal, "state="+string(record.State)), fmt.Errorf("attempt %d is still running", current))
		}
		if record.MaxRounds > 0 && current >= record.MaxRounds {
			return UnitRevisionResult{Current: current}, roundLimit(record, current)
		}
		if err := runner.countedCap(record); err != nil {
			return UnitRevisionResult{Current: current}, err
		}
		revision := UnitRevision{After: after, Attempt: after + 1, BriefSHA256: briefDigest, DecisionsSHA256: decisionsDigest,
			RequestedAtUnixSec: runner.Manager.Now().Unix()}
		directory := filepath.Join(runner.runDir(record.ID), "revisions")
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return UnitRevisionResult{}, err
		}
		revision.Brief = filepath.Join(directory, fmt.Sprintf("after-%d-brief.md", after))
		if _, err := atomicfile.WriteText(revision.Brief, string(request.Brief), runner.root()); err != nil {
			return UnitRevisionResult{}, err
		}
		if decisionsDigest != "" {
			revision.Decisions = filepath.Join(directory, fmt.Sprintf("after-%d-decisions.md", after))
			if _, err := atomicfile.WriteText(revision.Decisions, string(request.Decisions), runner.root()); err != nil {
				return UnitRevisionResult{}, err
			}
		}
		record.Revisions = append(record.Revisions, revision)
		if err := runner.save(record); err != nil {
			return UnitRevisionResult{}, err
		}
		retained = &revision
	} else if retained.Attempt <= current {
		// The attempt exists: the request is answered by it, continuing it
		// only while it still runs.
		result := UnitRevisionResult{Revision: *retained, Rejoined: true, Current: current}
		if retained.Attempt < current || record.State == "awaiting-judgement" {
			result.UnitResult = UnitResult{Record: record, Round: retained.Attempt}
			return result, nil
		}
		if err := runner.requireGoalBranch(plan); err != nil {
			return result, err
		}
		unit, err := runner.continueRunning(&record, plan)
		result.UnitResult = unit
		return result, err
	}
	// The request is retained and its attempt not yet added: add it with
	// the frozen brief. The previous inputs are the corrected attempt's read
	// outputs and the frozen decisions, which carry the reviewed findings
	// even when the corrected attempt itself produced no read.
	if data, err := os.ReadFile(retained.Brief); err != nil || digestHex(data) != retained.BriefSHA256 {
		return UnitRevisionResult{}, coded("UNIT_REVISION_CORRUPT", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d", retained.After)), fmt.Errorf("the kept brief of the correction after attempt %d is missing or changed", retained.After))
	}
	if err := runner.requireGoalBranch(plan); err != nil {
		return UnitRevisionResult{}, err
	}
	previous := readOutputs(runner.Manager, record.Rounds[retained.After-1])
	if retained.Decisions != "" {
		if data, err := os.ReadFile(retained.Decisions); err != nil || digestHex(data) != retained.DecisionsSHA256 {
			return UnitRevisionResult{}, coded("UNIT_REVISION_CORRUPT", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d", retained.After)), fmt.Errorf("the kept decisions of the correction after attempt %d are missing or changed", retained.After))
		}
		previous = append(previous, retained.Decisions)
	}
	if retained.After != current {
		return UnitRevisionResult{}, coded("UNIT_REVISION_CORRUPT", unitFacts(record.Unit, record.Goal, fmt.Sprintf("after=%d current=%d", retained.After, current)),
			fmt.Errorf("the kept correction follows attempt %d, but the newest attempt is %d", retained.After, current))
	}
	if err := runner.admitRound(plan, retained.Brief, previous); err != nil {
		return UnitRevisionResult{}, err
	}
	if err := runner.addRound(&record, plan, retained.Brief); err != nil {
		return UnitRevisionResult{}, err
	}
	unit, err := runner.continueRunning(&record, plan)
	return UnitRevisionResult{UnitResult: unit, Revision: *retained, Current: len(record.Rounds)}, err
}

func (runner *UnitRunner) continueRunning(record *UnitRunRecord, plan UnitPlan) (UnitResult, error) {
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return UnitResult{}, err
	}
	deadline := runner.Manager.Now().Add(time.Duration(settings.WaitCapSeconds) * time.Second)
	return runner.advanceRunning(record, plan, deadline)
}
