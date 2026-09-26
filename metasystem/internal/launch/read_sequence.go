package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// stepDriver starts and waits for one round's launches. A unit run and a
// standalone read differ only in how a step's launch is named, gated before
// it starts, started and saved; the step states and their recording are the
// same for both.
type stepDriver struct {
	manager  *Manager
	round    *UnitRound
	launchID func(index int) string
	// before is asked before a step's launch is created; its refusal
	// starts nothing.
	before func(StartSpec) error
	start  func(StartSpec) (Record, error)
	save   func() error
}

func (driver stepDriver) advanceStep(index int, spec StartSpec, deadline time.Time) (bool, error) {
	if _, err := driver.startStep(index, spec); err != nil {
		return false, err
	}
	return driver.waitStep(index, deadline)
}

func (driver stepDriver) startStep(index int, spec StartSpec) (Record, error) {
	step := &driver.round.Steps[index]
	if step.State == StepPending {
		step.LaunchID = driver.launchID(index)
		step.State, step.StartedAt = StepStarting, driver.manager.Now().UTC().Format(time.RFC3339Nano)
		if err := driver.save(); err != nil {
			return Record{}, err
		}
	}
	launchRecord, err := driver.manager.Store.Read(step.LaunchID)
	if errors.Is(err, fs.ErrNotExist) {
		if driver.before != nil {
			if err := driver.before(spec); err != nil {
				return Record{}, err
			}
		}
		spec.ID = step.LaunchID
		launchRecord, err = driver.start(spec)
	}
	if err != nil && launchRecord.ID == "" {
		return Record{}, err
	}
	if launchRecord.State.Terminal() {
		driver.endStep(index, launchRecord)
		return launchRecord, driver.save()
	}
	step.State = StepRunning
	return launchRecord, driver.save()
}

func (driver stepDriver) waitStep(index int, deadline time.Time) (bool, error) {
	step := &driver.round.Steps[index]
	if step.State == StepPassed || step.State == StepFailed {
		return false, nil
	}
	remaining := deadline.Sub(driver.manager.Now())
	if remaining < 0 {
		remaining = 0
	}
	launchRecord, terminal, err := driver.manager.Wait(step.LaunchID, remaining)
	if err != nil {
		return false, err
	}
	if !terminal {
		step.State = StepRunning
		if err := driver.save(); err != nil {
			return false, err
		}
		return true, nil
	}
	driver.endStep(index, launchRecord)
	return false, driver.save()
}

func (driver stepDriver) endStep(index int, launchRecord Record) {
	step := &driver.round.Steps[index]
	step.State, step.Reason, step.FinishedAt = StepFailed, launchRecord.Reason, driver.manager.Now().UTC().Format(time.RFC3339Nano)
	if launchRecord.State == Completed {
		step.State = StepPassed
	}
	if strings.HasPrefix(step.Name, "read") {
		step.Verdict, step.VerdictCounts = launchRecord.Measurement.Verdict, launchRecord.VerdictCounts
	}
}

// readSequence is the one read partition policy builds and standalone reads
// share: a whole, wide or per-package first pass chosen from the candidate
// diff's size, then bounded reruns of every read whose verdict did not count
// because its context was compacted. Its steps live in the driver's round.
type readSequence struct {
	driver stepDriver
	// model is the read model a planned step records.
	model string
	// diff is the full candidate diff every step's share is cut from.
	diff string
	// spec is every read launch's shared request; a step adds its share.
	spec StartSpec
	// serial completes each read before the next one starts.
	serial bool
	// outputs, when set, names one step's declared outputs instead of the
	// spec's shared ones.
	outputs func(UnitStep) []string
	// notice is added to every step's read packet.
	notice string
}

// plan appends the first pass of reads once and returns where the reads
// start in the round.
func (sequence readSequence) plan() (int, error) {
	round := sequence.driver.round
	readStart := len(round.Steps)
	for index := 0; index < len(round.Steps); index++ {
		if strings.HasPrefix(round.Steps[index].Name, "read") {
			return index, nil
		}
	}
	settings, err := sequence.driver.manager.resolvedSettings()
	if err != nil {
		return 0, err
	}
	choice, err := ChooseReadMode(sequence.diff, settings.ReadSplitLines)
	if err != nil {
		return 0, err
	}
	if choice.Mode == "package" {
		var directories []string
		for directory := range choice.Directories {
			directories = append(directories, directory)
		}
		sort.Strings(directories)
		for _, directory := range directories {
			round.Steps = append(round.Steps, UnitStep{Name: "read:" + directory, State: StepPending, Model: sequence.model, Mode: "package", Package: directory})
		}
	} else {
		round.Steps = append(round.Steps, UnitStep{Name: "read", State: StepPending, Model: sequence.model, Mode: choice.Mode})
	}
	return readStart, sequence.driver.save()
}

// advance runs the planned reads, adds the compaction reruns once the first
// pass has passed, and runs those. It returns the round's outcome, or the
// index of the step that reached the deadline or failed to advance (-1 when
// no step is to blame) with its error.
func (sequence readSequence) advance(readStart int, deadline time.Time) (string, int, bool, error) {
	if stop, capped, err := sequence.run(readStart, deadline); stop >= 0 || capped || err != nil {
		return "", stop, capped, err
	}
	steps := sequence.driver.round.Steps
	for index := readStart; index < len(steps); index++ {
		if steps[index].State != StepPassed {
			return "read-failed", -1, false, nil
		}
	}
	if err := sequence.addReruns(readStart); err != nil {
		return "", -1, false, err
	}
	if stop, capped, err := sequence.run(readStart, deadline); stop >= 0 || capped || err != nil {
		return "", stop, capped, err
	}
	return readOutcome(sequence.driver.round.Steps, readStart), -1, false, nil
}

// readOutcome judges a round's completed reads. A failed read or a rerun
// that did not count leaves the round incomplete, and so does a first-pass
// read that did not count unless its own reruns cover it: the per-file
// reruns of its package, or the per-package reruns of a whole or wide read.
// Another read's reruns never cover it.
func readOutcome(steps []UnitStep, readStart int) string {
	for index := readStart; index < len(steps); index++ {
		step := steps[index]
		if step.State != StepPassed {
			return "read-failed"
		}
		if step.Rerun && !unitStepVerdictCounts(step) {
			return "read-compacted"
		}
	}
	for index := readStart; index < len(steps); index++ {
		original := steps[index]
		if original.Rerun || unitStepVerdictCounts(original) {
			continue
		}
		covered := false
		for _, rerun := range steps[readStart:] {
			if rerun.Rerun && (original.Package != "" && rerun.Mode == "file" && rerun.Package == original.Package || original.Package == "" && rerun.Mode == "package") {
				covered = true
			}
		}
		if !covered {
			return "read-compacted"
		}
	}
	return "green"
}

func (sequence readSequence) stepSpec(step UnitStep) (StartSpec, error) {
	packet, err := unitReadPacket(sequence.diff, step)
	if err != nil {
		return StartSpec{}, err
	}
	adapterData := map[string]json.RawMessage{}
	setString(adapterData, "unitReadPacket", packet+sequence.notice)
	spec := sequence.spec
	spec.Kind, spec.DiffFile, spec.Package, spec.File, spec.Wide, spec.AdapterData = "read", sequence.diff, step.Package, step.File, step.Mode == "wide", adapterData
	if sequence.outputs != nil {
		spec.Outputs = sequence.outputs(step)
	}
	if step.Rerun {
		spec.readMode = step.Mode
	}
	return spec, nil
}

func (sequence readSequence) run(readStart int, deadline time.Time) (int, bool, error) {
	round := sequence.driver.round
	if sequence.outputs == nil {
		if err := prepareReadOutputDirectories(sequence.spec.Outputs); err != nil {
			return -1, false, err
		}
	}
	for index := readStart; index < len(round.Steps); index++ {
		step := &round.Steps[index]
		if !strings.HasPrefix(step.Name, "read") || step.State != StepPending && step.State != StepStarting {
			continue
		}
		spec, err := sequence.stepSpec(*step)
		if err != nil {
			return -1, false, err
		}
		if sequence.outputs != nil {
			if err := prepareReadOutputDirectories(spec.Outputs); err != nil {
				return -1, false, err
			}
		}
		if _, err := sequence.driver.startStep(index, spec); err != nil {
			return -1, false, err
		}
		// Split reads that share declared output paths complete each
		// writer before starting the next one, so every report belongs to
		// its own launch and a queued start does not wait behind a long
		// read.
		if sequence.serial {
			if capped, err := sequence.driver.waitStep(index, deadline); err != nil || capped {
				return index, capped, err
			}
		}
	}
	for index := readStart; index < len(round.Steps); index++ {
		if !strings.HasPrefix(round.Steps[index].Name, "read") {
			continue
		}
		if capped, err := sequence.driver.waitStep(index, deadline); err != nil || capped {
			return index, capped, err
		}
	}
	return -1, false, nil
}

func (sequence readSequence) addReruns(readStart int) error {
	round := sequence.driver.round
	for index := readStart; index < len(round.Steps); index++ {
		if round.Steps[index].Rerun {
			return nil
		}
	}
	choice, err := ChooseReadMode(sequence.diff, 1<<62)
	if err != nil {
		return err
	}
	var reruns []UnitStep
	for index := readStart; index < len(round.Steps); index++ {
		step := round.Steps[index]
		if step.Rerun || !strings.HasPrefix(step.Name, "read") || unitStepVerdictCounts(step) {
			continue
		}
		if step.Package != "" {
			var files []string
			for name := range choice.Files {
				if filepath.Dir(name) == step.Package {
					files = append(files, name)
				}
			}
			sort.Strings(files)
			if len(files) <= 1 {
				continue
			}
			for _, name := range files {
				reruns = append(reruns, UnitStep{Name: "read:" + name + ":rerun", State: StepPending, Model: sequence.model, Mode: "file", Package: step.Package, File: name, Rerun: true})
			}
			continue
		}
		var packages []string
		for name := range choice.Directories {
			packages = append(packages, name)
		}
		sort.Strings(packages)
		for _, name := range packages {
			reruns = append(reruns, UnitStep{Name: "read:" + name + ":rerun", State: StepPending, Model: sequence.model, Mode: "package", Package: name, Rerun: true})
		}
	}
	if len(reruns) == 0 {
		return nil
	}
	round.Steps = append(round.Steps, reruns...)
	return sequence.driver.save()
}

// unitLaunchID names a unit run's step launch.
func unitLaunchID(record *UnitRunRecord, round *UnitRound) func(int) string {
	return func(index int) string { return fmt.Sprintf("%s-r%d-s%d", record.ID, round.Number, index+1) }
}
