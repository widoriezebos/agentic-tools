package launch

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"golang.org/x/sys/unix"
)

// A named unit is the same unit run reached again by its name. The entry
// lives in the run store beside the runs it names: one file per worktree,
// goal and unit, holding the digest of what the run was asked to do and the
// run id reserved for it.
type namedUnitEntry struct {
	Worktree string `json:"worktree"`
	Goal     string `json:"goal"`
	Unit     string `json:"unit"`
	Digest   string `json:"digest"`
	Run      string `json:"run"`
	State    string `json:"state"`
}

const (
	namedReserved = "reserved"
	namedRecorded = "recorded"
)

// AdvanceNamed runs the unit a plan names, or continues the run that plan
// already started. The same worktree, goal and unit with the same inputs
// reaches the same run; changed inputs are refused with the earlier run id,
// because a changed unit is a follow-up or a new unit name. The run id is
// reserved before the run record or any launch exists, so an interrupted
// first call is finished by the next one instead of starting a second run.
// Everything after the reservation is Advance resuming that run, so a run
// awaiting judgement stays there until an explicit follow-up.
//
// The plan is read once, under the unit's lock, into the named store; the
// digest and the run's retained plan both come from that one copy. Before
// the run is continued and before every launch it starts, the retained
// plan, the files it names and the launch settings are checked against the
// reserved digest, so a changed plan, brief, input or setting is refused
// before anything is launched with it.
func (runner *UnitRunner) AdvanceNamed(planPath string) (UnitResult, error) {
	if runner.Manager == nil {
		return UnitResult{}, errors.New("unit launch manager is unavailable")
	}
	named, err := ReadUnitPlan(planPath)
	if err != nil {
		return UnitResult{}, err
	}
	worktree, key, err := namedUnitIdentity(named)
	if err != nil {
		return UnitResult{}, err
	}
	lock, err := runner.namedLock(key, named)
	if err != nil {
		return UnitResult{}, err
	}
	defer releaseUnitLock(lock)
	return runner.advanceNamedLocked(named, worktree, key)
}

// Continue advances a recorded run by its id, with an optional follow-up
// brief. A run a named unit reserved is continued as that unit: under its
// named lock, then the run lock, with the reserved digest of the retained
// plan, its files and the launch settings verified before the run goes on and
// before every launch it starts. The named entry is found from the run's own
// worktree, goal and unit, the same key that reserved it. A run no named
// entry claims (a legacy plan) continues exactly as Advance. A follow-up
// brief is the caller's new input for the next round, not part of the
// reserved plan, and the run's round limit still applies.
func (runner *UnitRunner) Continue(request UnitRequest) (UnitResult, error) {
	if runner.Manager == nil {
		return UnitResult{}, errors.New("unit launch manager is unavailable")
	}
	if request.Resume == "" || request.Plan != "" {
		return UnitResult{}, errors.New("unit continuation requires a run id and no plan")
	}
	record, err := runner.read(request.Resume)
	if err != nil {
		return UnitResult{}, err
	}
	worktree, key, err := namedUnitIdentity(UnitPlan{Worktree: record.Worktree, Goal: record.Goal, Unit: record.Unit})
	if err != nil {
		// Without its worktree no step of the run can start; only the
		// recorded judgement can be read back.
		if record.State == "awaiting-judgement" && request.FollowUp == "" {
			return UnitResult{Record: record, Round: len(record.Rounds)}, nil
		}
		return UnitResult{}, err
	}
	entry, found, err := runner.readNamed(key)
	if err != nil {
		return UnitResult{}, err
	}
	if !found || entry.Run != record.ID {
		return runner.Advance(request)
	}
	lock, err := runner.namedLock(key, UnitPlan{Unit: record.Unit, Goal: record.Goal})
	if err != nil {
		return UnitResult{}, err
	}
	defer releaseUnitLock(lock)
	if entry, found, err = runner.readNamed(key); err != nil {
		return UnitResult{}, err
	}
	if !found || entry.Run != record.ID || entry.Digest == "" {
		return UnitResult{}, fmt.Errorf("UNIT_NAMED_ENTRY_CORRUPT unit=%s goal=%s run=%s: the unit's entry changed while it was locked", record.Unit, record.Goal, record.ID)
	}
	bound := *runner
	bound.named = &namedBinding{unit: record.Unit, goal: record.Goal, worktree: worktree, digest: entry.Digest, run: entry.Run,
		options: UnitOptions{BuildModel: record.BuildModel, BuildEffort: record.BuildEffort}}
	return bound.Advance(request)
}

// UnitOptions are a unit request's typed choices outside the plan: the
// build model and effort chosen for this unit, and its approved ceiling on
// rounds. A new run records them; later rounds and resumes use the record.
type UnitOptions struct {
	BuildModel, BuildEffort string
	MaxRounds               int
}

// AdvancePrepared is AdvanceNamed for a request whose plan and briefs are
// generated from caller inputs. Under the unit's named lock it compares the
// caller's request bytes with the ones retained beside the unit's inputs.
// A unit that already has a run reuses its retained plan and briefs, which
// keep the base they were made from, and a different request is refused
// with that run's id before anything is written. Only a unit with no run
// yet calls prepare, which writes the plan into the unit's input directory
// and returns its path.
func (runner *UnitRunner) AdvancePrepared(worktree, goal, unit string, request []byte, options UnitOptions, prepare func(directory string) (string, error)) (UnitResult, error) {
	if runner.Manager == nil {
		return UnitResult{}, errors.New("unit launch manager is unavailable")
	}
	real, key, err := namedUnitIdentity(UnitPlan{Worktree: worktree, Goal: goal, Unit: unit})
	if err != nil {
		return UnitResult{}, err
	}
	lock, err := runner.namedLock(key, UnitPlan{Unit: unit, Goal: goal})
	if err != nil {
		return UnitResult{}, err
	}
	defer releaseUnitLock(lock)
	directory := filepath.Join(runner.root(), ".inputs", key)
	requestPath := filepath.Join(directory, "request.json")
	planPath := filepath.Join(directory, "plan.json")
	entry, found, err := runner.readNamed(key)
	if err != nil {
		return UnitResult{}, err
	}
	if found {
		retained, readErr := os.ReadFile(requestPath)
		if readErr != nil || string(retained) != string(request) {
			return UnitResult{}, fmt.Errorf("UNIT_NAMED_INPUT_CHANGED unit=%s goal=%s run=%s: this request differs from the one the run started with, whose inputs are kept unchanged; send a follow-up to that run or use another unit name", unit, goal, entry.Run)
		}
	} else {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return UnitResult{}, err
		}
		if planPath, err = prepare(directory); err != nil {
			return UnitResult{}, err
		}
		if _, err := atomicfile.WriteText(requestPath, string(request), runner.root()); err != nil {
			return UnitResult{}, err
		}
	}
	named, err := ReadUnitPlan(planPath)
	if err != nil {
		return UnitResult{}, err
	}
	if named.Unit != unit || named.Goal != goal {
		return UnitResult{}, planInvalid("unit", nil)
	}
	bound := *runner
	bound.options = options
	return bound.advanceNamedLocked(named, real, key)
}

// advanceNamedLocked reserves or continues the named run; the caller holds
// the unit's named lock.
func (runner *UnitRunner) advanceNamedLocked(named UnitPlan, worktree, key string) (UnitResult, error) {
	data, err := os.ReadFile(named.Path)
	if err != nil {
		return UnitResult{}, planInvalid("plan", err)
	}
	staged := runner.namedPath(key, ".plan")
	if _, err := atomicfile.WriteText(staged, string(data), runner.root()); err != nil {
		return UnitResult{}, err
	}
	planDirectory := filepath.Dir(named.Path)
	plan, err := readUnitPlan(staged, planDirectory)
	if err != nil {
		return UnitResult{}, err
	}
	if _, stagedKey, err := namedUnitIdentity(plan); err != nil || stagedKey != key {
		return UnitResult{}, fmt.Errorf("UNIT_NAMED_INPUT_CHANGED unit=%s goal=%s: the plan changed while it was read", named.Unit, named.Goal)
	}
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return UnitResult{}, err
	}
	digest, err := namedUnitDigest(plan, worktree, runner.options.apply(settings))
	if err != nil {
		return UnitResult{}, err
	}
	entry, found, err := runner.readNamed(key)
	if err != nil {
		return UnitResult{}, err
	}
	if found {
		if entry.Worktree != worktree || entry.Goal != plan.Goal || entry.Unit != plan.Unit || !idPattern.MatchString(entry.Run) ||
			entry.State != namedReserved && entry.State != namedRecorded {
			return UnitResult{}, fmt.Errorf("UNIT_NAMED_ENTRY_CORRUPT unit=%s goal=%s entry=%s", plan.Unit, plan.Goal, runner.namedPath(key, ".json"))
		}
		if entry.Digest != digest {
			return UnitResult{}, fmt.Errorf("UNIT_NAMED_INPUT_CHANGED unit=%s goal=%s run=%s: the unit's inputs differ from its run; send a follow-up to that run or use another unit name", plan.Unit, plan.Goal, entry.Run)
		}
	} else {
		if err := runner.requireGoalBranch(plan); err != nil {
			return UnitResult{}, err
		}
		if err := runner.admitRound(plan, plan.Build.Brief, nil); err != nil {
			return UnitResult{}, err
		}
		id, err := newID(runner.Manager.Now())
		if err != nil {
			return UnitResult{}, err
		}
		entry = namedUnitEntry{Worktree: worktree, Goal: plan.Goal, Unit: plan.Unit, Digest: digest, Run: id, State: namedReserved}
		if err := runner.writeNamed(key, entry); err != nil {
			return UnitResult{}, err
		}
	}
	record, err := runner.read(entry.Run)
	switch {
	case errors.Is(err, fs.ErrNotExist) && entry.State == namedReserved:
		// Reserved but never recorded: no launch can exist yet, so write
		// round one under the reserved id. A first call already admitted it.
		if found {
			if err := runner.requireGoalBranch(plan); err != nil {
				return UnitResult{}, err
			}
			if err := runner.admitRound(plan, plan.Build.Brief, nil); err != nil {
				return UnitResult{}, err
			}
		}
		if record, err = runner.newRunWithID(plan, entry.Run, planDirectory); err != nil {
			return UnitResult{}, err
		}
	case errors.Is(err, fs.ErrNotExist):
		return UnitResult{}, fmt.Errorf("UNIT_NAMED_RUN_MISSING unit=%s goal=%s run=%s: the recorded run is gone", plan.Unit, plan.Goal, entry.Run)
	case err != nil:
		return UnitResult{}, fmt.Errorf("UNIT_NAMED_RUN_UNREADABLE unit=%s goal=%s run=%s: %w", plan.Unit, plan.Goal, entry.Run, err)
	}
	if recorded, pathErr := filepath.EvalSymlinks(record.Worktree); pathErr != nil || recorded != worktree || record.Goal != plan.Goal || record.Unit != plan.Unit || record.ID != entry.Run {
		return UnitResult{}, fmt.Errorf("UNIT_NAMED_ENTRY_CORRUPT unit=%s goal=%s run=%s: the run belongs to another unit", plan.Unit, plan.Goal, entry.Run)
	}
	if entry.State != namedRecorded {
		entry.State = namedRecorded
		if err := runner.writeNamed(key, entry); err != nil {
			return UnitResult{}, err
		}
	}
	bound := *runner
	bound.named = &namedBinding{unit: plan.Unit, goal: plan.Goal, worktree: worktree, digest: entry.Digest, run: entry.Run,
		options: UnitOptions{BuildModel: record.BuildModel, BuildEffort: record.BuildEffort}}
	return bound.Advance(UnitRequest{Resume: entry.Run})
}

// apply puts the unit's own build model and effort over the configured
// ones, so the digest names what the build actually runs with.
func (options UnitOptions) apply(settings Settings) Settings {
	settings.BuildModel = choose(options.BuildModel, settings.BuildModel)
	settings.BuildEffort = choose(options.BuildEffort, settings.BuildEffort)
	return settings
}

// namedBinding ties one AdvanceNamed call to its reserved digest. Advance
// fills in the plan it actually runs, which is the run's retained copy.
type namedBinding struct {
	unit, goal, worktree, digest, run string
	plan                              UnitPlan
	options                           UnitOptions
}

func (binding *namedBinding) verify(runner *UnitRunner) error {
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return err
	}
	digest, err := namedUnitDigest(binding.plan, binding.worktree, binding.options.apply(settings))
	if err != nil || digest != binding.digest {
		return fmt.Errorf("UNIT_NAMED_INPUT_CHANGED unit=%s goal=%s run=%s: the run's plan, inputs or launch settings no longer match its reservation, so nothing more was launched; restore them, send a follow-up to that run or use another unit name", binding.unit, binding.goal, binding.run)
	}
	return nil
}

// namedUnitIdentity returns the worktree's real path and the entry key.
// The worktree is part of the key, so the same goal and unit names in two
// repositories stay two units.
func namedUnitIdentity(plan UnitPlan) (string, string, error) {
	worktree, err := filepath.EvalSymlinks(plan.Worktree)
	if err != nil {
		return "", "", planInvalid("worktree", err)
	}
	sum := sha256.Sum256([]byte(worktree + "\x00" + plan.Goal + "\x00" + plan.Unit))
	return worktree, fmt.Sprintf("%x", sum[:16]), nil
}

type namedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// namedUnitDigest covers what the run is asked to do: the resolved plan,
// the bytes of every brief, units page and input it hands a model, the
// proof commands, and the launch settings that change what a build or read
// asks for or how the unit is split into launches: runtime, model, effort,
// window, the build line cap and the read split threshold. The plan's own
// file name and the wait cap are left out; moving an unchanged plan or
// waiting longer is the same unit.
func namedUnitDigest(plan UnitPlan, worktree string, settings Settings) (string, error) {
	file := func(path string) (namedFile, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return namedFile{}, err
		}
		return namedFile{path, fmt.Sprintf("%x", sha256.Sum256(data))}, nil
	}
	files := func(paths []string) ([]namedFile, error) {
		result := []namedFile{}
		for _, path := range paths {
			entry, err := file(path)
			if err != nil {
				return nil, err
			}
			result = append(result, entry)
		}
		return result, nil
	}
	var content struct {
		Unit, Goal, Worktree, Base         string
		BuildBrief, UnitsPage, ReadBrief   namedFile
		BuildInputs, ReadInputs            []namedFile
		BuildOutputs, ReadOutputs, Units   []string
		BuildModel, BuildEffort, ReadModel string
		BuildRuntime, ReadRuntime          string
		BuildWindow, ReadWindow            int64
		BuildLinesCap, ReadSplitLines      int64
		Proof                              []ProofCommand
	}
	content.Unit, content.Goal, content.Worktree, content.Base = plan.Unit, plan.Goal, worktree, plan.Base
	var err error
	if content.BuildBrief, err = file(plan.Build.Brief); err != nil {
		return "", err
	}
	if content.UnitsPage, err = file(plan.Build.UnitsPage); err != nil {
		return "", err
	}
	if content.ReadBrief, err = file(plan.Read.Brief); err != nil {
		return "", err
	}
	if content.BuildInputs, err = files(plan.Build.Inputs); err != nil {
		return "", err
	}
	if content.ReadInputs, err = files(plan.Read.Inputs); err != nil {
		return "", err
	}
	content.BuildOutputs, content.ReadOutputs, content.Units = plan.Build.Outputs, plan.Read.Outputs, plan.Build.Units
	content.BuildModel, content.BuildEffort, content.ReadModel = settings.BuildModel, settings.BuildEffort, choose(plan.Read.Model, settings.ReadModel)
	content.BuildRuntime, content.ReadRuntime = settings.BuildRuntime, settings.ReadRuntime
	content.BuildWindow, content.ReadWindow = settings.BuildWindow, settings.ReadWindow
	content.BuildLinesCap, content.ReadSplitLines = settings.BuildLinesCap, settings.ReadSplitLines
	content.Proof = plan.Proof
	data, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func (runner *UnitRunner) namedPath(key, suffix string) string {
	return filepath.Join(runner.root(), ".named", key+suffix)
}

// namedLock holds the unit's name while one caller reserves and advances
// it. A second caller is told which run is in progress rather than waiting
// or starting another.
func (runner *UnitRunner) namedLock(key string, plan UnitPlan) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(runner.namedPath(key, "")), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(runner.namedPath(key, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		if !lockWouldBlock(err) {
			return nil, fmt.Errorf("UNIT_LOCK_FAILED unit=%s goal=%s lock=%s: %w", plan.Unit, plan.Goal, runner.namedPath(key, ".lock"), err)
		}
		run := "reserving"
		if entry, found, readErr := runner.readNamed(key); readErr == nil && found {
			run = entry.Run
		}
		return nil, fmt.Errorf("UNIT_RUN_BUSY unit=%s goal=%s run=%s: another caller is advancing this unit; repeat the same command to continue", plan.Unit, plan.Goal, run)
	}
	return file, nil
}

func (runner *UnitRunner) readNamed(key string) (namedUnitEntry, bool, error) {
	path := runner.namedPath(key, ".json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return namedUnitEntry{}, false, nil
	}
	if err != nil {
		return namedUnitEntry{}, false, err
	}
	var entry namedUnitEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return namedUnitEntry{}, false, fmt.Errorf("UNIT_NAMED_ENTRY_CORRUPT entry=%s: %w", path, err)
	}
	return entry, true, nil
}

func (runner *UnitRunner) writeNamed(key string, entry namedUnitEntry) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(runner.namedPath(key, ".json"), string(data)+"\n", runner.root())
	return err
}

// NamedInputDirectory is where a generated plan and its briefs for one
// named unit are kept: inside the run store, under the same worktree, goal
// and unit key as the unit's entry, so regenerating them for a repeat writes
// the same paths and the unit's digest does not change.
func (runner *UnitRunner) NamedInputDirectory(worktree, goal, unit string) (string, error) {
	_, key, err := namedUnitIdentity(UnitPlan{Worktree: worktree, Goal: goal, Unit: unit})
	if err != nil {
		return "", err
	}
	return filepath.Join(runner.root(), ".inputs", key), nil
}
