package launch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func correctionBudget(policy string, record UnitRunRecord) (int, error) {
	cap := 2
	if policy != "auto" && policy != "person" {
		n, ok := new(big.Int).SetString(policy, 10)
		if !ok || n.Sign() < 0 {
			return 0, fmt.Errorf("review.stop needs auto, person or a decimal cap; run metasystem settings set review.stop auto")
		}
		ceiling := new(big.Int).SetUint64(uint64(^uint(0)>>1) - 1)
		if n.Cmp(ceiling) > 0 {
			cap = int(^uint(0)>>1) - 1
		} else {
			cap = int(n.Int64())
		}
	}
	for _, limit := range []int{record.MaxRounds, record.CountedCap} {
		if limit > 0 && limit-1 < cap {
			cap = limit - 1
		}
	}
	return cap, nil
}

func collectLaunchRead(record Record, stateDir string) (readsubject.Read, error) {
	paths, err := declaredOutputPaths(record)
	if err != nil {
		return readsubject.Read{}, err
	}
	var retained []string
	if len(paths) > 1 {
		structured := filepath.Join(stateDir, "outputs", filepath.Base(paths[1]))
		for _, output := range record.Outputs {
			if output.Path == structured {
				retained = append(retained, output.Path)
			}
		}
	}
	// Declared paths may already have another writer. Collection uses the
	// launch's own copies; legacy evidence stays inside this launch's state.
	for _, output := range record.Outputs {
		dir := filepath.Dir(output.Path)
		if (dir == stateDir || dir == filepath.Join(stateDir, "outputs")) && (filepath.Base(output.Path) == "return.json" || strings.HasSuffix(output.Path, "-return.json")) {
			retained = append(retained, output.Path)
		}
	}
	// Older launch records can declare their per-launch evidence without an
	// output list. These paths belong to this launch and cannot be shared.
	for _, path := range paths {
		if filepath.Dir(path) == stateDir && (filepath.Base(path) == "return.json" || strings.HasSuffix(path, "-return.json")) {
			retained = append(retained, path)
		}
	}
	var data []byte
	output := ""
	for _, path := range retained {
		data, err = os.ReadFile(path)
		if err == nil {
			output = path
			break
		}
	}
	if output == "" {
		return readsubject.Read{}, fmt.Errorf("structured read evidence is unavailable")
	}
	diffPath := readString(record.AdapterData, "readDiff")
	diff, err := os.ReadFile(diffPath)
	if err != nil {
		return readsubject.Read{}, fmt.Errorf("read subject diff is unavailable: %w", err)
	}
	subject := readsubject.ReadSubject{Kind: readsubject.SubjectLive, ImplementerRoot: record.Goal, ReviewedProjectTree: fmt.Sprintf("%x", sha256.Sum256(diff)), DiffDigest: fmt.Sprintf("%x", sha256.Sum256(diff))}
	return readsubject.Collect(record.ID, subject, readString(record.AdapterData, "engine"), readString(record.AdapterData, "model"), output, data, record.Measurement.Verdict)
}

func (runner *UnitRunner) collectRoundRead(record *UnitRunRecord, round *UnitRound) error {
	if len(round.Reads) > 0 {
		return runner.decideRound(record, round, "")
	}
	unknown := ""
	for _, step := range round.Steps {
		if !strings.HasPrefix(step.Name, "read") || step.LaunchID == "" || step.State == StepSkipped {
			continue
		}
		launch, err := runner.Manager.Store.Read(step.LaunchID)
		if err != nil {
			unknown = "unreadable-read"
			continue
		}
		if launch.Measurement.Compactions > 0 || launch.VerdictCounts != nil && !*launch.VerdictCounts {
			continue
		}
		read := launch.Read
		if read == nil && launch.State == Completed {
			stateDir, err := runner.Manager.Store.StateDir(launch.ID)
			if err != nil {
				return err
			}
			upgraded, err := collectLaunchRead(launch, stateDir)
			if err == nil {
				read = &upgraded
			}
		}
		if read == nil {
			unknown = "unavailable-stop-inputs"
			continue
		}
		round.Reads = append(round.Reads, *read)
	}
	if len(round.Reads) == 0 {
		unknown = "unavailable-stop-inputs"
	}
	return runner.decideRound(record, round, unknown)
}

func (runner *UnitRunner) decideRound(record *UnitRunRecord, round *UnitRound, unknown string) error {
	readUnknown := unknown != "" || len(round.Reads) == 0
	policy := "auto"
	if runner.ReviewPolicy != nil {
		value, err := runner.ReviewPolicy()
		if err != nil {
			unknown = "unreadable-policy"
			record.PolicyError = err.Error()
		} else {
			policy = value
		}
	}
	budget := 2
	if record.CorrectionBudget != nil {
		budget = *record.CorrectionBudget
	}
	var prior []readsubject.Read
	attempt := 1
	for _, p := range record.Rounds {
		if p.Number >= round.Number {
			continue
		}
		if len(p.Reads) > 0 && p.Material >= 0 {
			attempt++
			prior = append(prior, p.Reads...)
		}
	}
	combined := readsubject.Read{ID: record.ID + ":" + strconv.Itoa(round.Number)}
	for _, read := range round.Reads {
		combined.Material += read.Material
		combined.Findings = append(combined.Findings, read.Findings...)
	}
	// Partitioned reads are one completed examination for progress accounting.
	if len(prior) > 0 {
		var aggregate []readsubject.Read
		for _, p := range record.Rounds {
			if p.Number >= round.Number {
				break
			}
			r := readsubject.Read{}
			for _, v := range p.Reads {
				r.Material += v.Material
				r.Findings = append(r.Findings, v.Findings...)
			}
			if len(p.Reads) > 0 && p.Material >= 0 {
				aggregate = append(aggregate, r)
			}
		}
		prior = aggregate
	}
	var inherited []readsubject.Finding
	if runner.InheritedFindings != nil {
		rows, err := runner.InheritedFindings(record.Goal, record.Unit)
		if err != nil {
			unknown = "unreadable-inherited-findings"
		} else {
			inherited = rows
		}
	}
	subject := record.Goal + "/" + record.Unit + "/" + record.ID
	stopID := fmt.Sprintf("%x", sha256.Sum256([]byte(subject+":"+strconv.Itoa(attempt))))
	destination := record.Unit + "-stop-" + strconv.Itoa(attempt) + "-" + stopID[:12]
	s := loopstop.Stop{Loop: "unit-round", Subject: subject, Attempt: attempt, Budget: budget + 1, Handoff: "split " + destination, Evidence: filepath.Join(round.Directory, "read-decision.json"), At: runner.Manager.Now().UTC().Format(time.RFC3339Nano)}
	if unknown != "" {
		s.Handoff = "stopped " + unknown
		round.Cause = "environment"
	}
	if !readUnknown && combined.Material > 0 {
		round.Cause = "own"
	}
	round.Material = combined.Material
	if readUnknown {
		round.Material = -1
	}
	s = loopstop.Decide(loopstop.Input{Stop: s, Prior: prior, Inherited: inherited, Read: &combined, Policy: policy, Unknown: unknown})
	round.Stop = &s
	return runner.saveDecision(record, round)
}

// saveDecision leaves the stop register beside its subject. Replay joins the
// same operation file instead of creating another stop or handoff.
func (runner *UnitRunner) saveDecision(record *UnitRunRecord, round *UnitRound) error {
	if err := runner.save(*record); err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		Reads []readsubject.Read `json:"reads"`
		Stop  *loopstop.Stop     `json:"stop"`
	}{round.Reads, round.Stop}, "", "  ")
	if err != nil {
		return err
	}
	if _, err = atomicfile.WriteFile(filepath.Join(round.Directory, "read-decision.json"), data, 0o600, runner.root()); err != nil {
		return err
	}
	if round.Number > 1 || round.Stop != nil && (round.Stop.Decision == "close" || round.Transferred) {
		for _, previous := range record.Rounds {
			if (round.Stop == nil || round.Stop.Decision != "close" && !round.Transferred) && (previous.Stop != nil && previous.Stop.Loop != "unit-build" || previous.Number >= round.Number) {
				continue
			}
			if previous.Stop == nil {
				continue
			}
			path := filepath.Join(previous.Directory, "stop-register.json")
			if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
				continue
			}
			entry := struct {
				Kind   string         `json:"kind"`
				Status string         `json:"status"`
				Stop   *loopstop.Stop `json:"stop"`
			}{"stop", "cleared", previous.Stop}
			bytes, writeErr := json.Marshal(entry)
			if writeErr != nil {
				return writeErr
			}
			if _, writeErr = atomicfile.WriteFile(path, bytes, 0o600, runner.root()); writeErr != nil {
				return writeErr
			}
		}
	}
	if round.Stop != nil && round.Stop.Decision == "stop" && !round.Transferred {
		entry := struct {
			Kind   string         `json:"kind"`
			Status string         `json:"status"`
			Stop   *loopstop.Stop `json:"stop"`
		}{"stop", "open", round.Stop}
		data, err = json.Marshal(entry)
		if err != nil {
			return err
		}
		_, err = atomicfile.WriteFile(filepath.Join(round.Directory, "stop-register.json"), data, 0o600, runner.root())
	}
	return err
}

func (runner *UnitRunner) allowCorrection(record UnitRunRecord) error {
	if len(record.Rounds) == 0 {
		return fmt.Errorf("there is no completed attempt")
	}
	round := record.Rounds[len(record.Rounds)-1]
	if runner.ReviewPolicy != nil {
		policy, err := runner.ReviewPolicy()
		if err != nil {
			return coded("UNIT_STOPPED", "", fmt.Errorf("the review policy cannot be read; nothing was started: %w", err))
		}
		if policy == "person" {
			return coded("UNIT_STOPPED", "", fmt.Errorf("a person holds review decisions; request a reasoned revision"))
		}
	}
	if round.Outcome == "build-gap" {
		return nil
	}
	if round.Stop == nil && (strings.HasPrefix(round.Outcome, "proof-") || strings.HasPrefix(round.Outcome, "build-")) {
		return nil
	}
	if round.Stop == nil {
		return coded("UNIT_STOPPED", "", fmt.Errorf("the completed read has no recorded decision; review this unit before correcting it"))
	}
	if round.Stop.Decision != "continue" {
		return coded("UNIT_STOPPED", "", fmt.Errorf("unit %s's review says %s (%s); nothing was started", record.Unit, round.Stop.Decision, round.Stop.Handoff))
	}
	return nil
}

// CollectExamination runs under ReviewSubject's owner lock, retaining the
// exact committed examination and its decision together with the subject.
func (runner *UnitRunner) CollectExamination(record *UnitRunRecord, round *UnitRound, subject UnitSubject) error {
	round.Reads = nil
	data, err := os.ReadFile(subject.ExaminationReturnPath)
	unknown := ""
	if err == nil {
		var read readsubject.Read
		var readErr error
		if runner.ExaminationRead != nil {
			read, readErr = runner.ExaminationRead(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(subject.ExaminationReturnPath)))))), choose(subject.ExaminationJob, subject.Examination))
		} else {
			read, readErr = readsubject.Collect(choose(subject.ExaminationJob, subject.Examination), readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: subject.Commit, Tree: subject.StagedTree, DiffDigest: subject.DiffDigest}, "retained-examination", round.ReadModel, subject.ExaminationReturnPath, data, "")
		}
		if readErr == nil {
			round.Reads = []readsubject.Read{read}
		} else {
			unknown = "unavailable-stop-inputs"
		}
	} else {
		unknown = "unreadable-read"
	}
	return runner.decideRound(record, round, unknown)
}

// RetryUnknownRead permits one new transport execution in the same attempt.
func (runner *UnitRunner) RetryUnknownRead(id string) (UnitResult, error) {
	lock, err := runner.lock(id)
	if err != nil {
		return UnitResult{}, err
	}
	defer releaseUnitLock(lock)
	record, err := runner.read(id)
	if err != nil {
		return UnitResult{}, err
	}
	if len(record.Rounds) == 0 {
		return UnitResult{}, fmt.Errorf("no read to retry")
	}
	round := &record.Rounds[len(record.Rounds)-1]
	if round.Stop == nil || !strings.HasPrefix(round.Stop.Handoff, "stopped ") || round.UnknownRetries >= 1 {
		return UnitResult{Record: record}, coded("UNIT_STOPPED", "", fmt.Errorf("the fresh examination allowance is spent; the unit stays stopped"))
	}
	plan, err := readUnitPlan(record.Plan, record.PlanDirectory)
	if err != nil {
		return UnitResult{}, err
	}
	round.UnknownRetries++
	round.Reads = nil
	for index := range round.Steps {
		if strings.HasPrefix(round.Steps[index].Name, "read") {
			round.Steps[index].Name = "unknown:" + round.Steps[index].Name
		}
	}
	round.Steps = append(round.Steps, UnitStep{Name: "read:fresh", State: StepPending, Model: round.ReadModel, Mode: "whole"})
	original := ""
	for _, step := range round.Steps {
		if strings.HasPrefix(step.Name, "unknown:read") {
			original = step.LaunchID
			break
		}
	}
	retry := loopstop.Stop{Loop: "retry", Subject: original, Attempt: 1, Budget: 1, Decision: "continue", At: runner.Manager.Now().UTC().Format(time.RFC3339Nano)}
	retryBytes, _ := json.Marshal(retry)
	if _, err := atomicfile.WriteFile(filepath.Join(round.Directory, "unknown-retry.json"), retryBytes, 0o600, runner.root()); err != nil {
		return UnitResult{}, err
	}
	if err := runner.saveDecision(&record, round); err != nil {
		return UnitResult{}, err
	}
	sequence := runner.readSequence(&record, round, plan, nil, filepath.Join(round.Directory, "worktree.diff"))
	spec, err := sequence.stepSpec(round.Steps[len(round.Steps)-1])
	if err != nil {
		return UnitResult{Record: record}, err
	}
	capped, err := sequence.driver.advanceStep(len(round.Steps)-1, spec, runner.Manager.Now().Add(DefaultWaitTimeout))
	if err != nil || capped {
		return UnitResult{Record: record, Capped: capped}, err
	}
	result, err := runner.finish(&record, round, "green")
	if err == nil && round.Stop != nil && strings.HasPrefix(round.Stop.Handoff, "stopped ") {
		retry.Decision = "stop"
		retry.Handoff = round.Stop.Handoff
		retryBytes, _ = json.Marshal(retry)
		_, err = atomicfile.WriteFile(filepath.Join(round.Directory, "unknown-retry.json"), retryBytes, 0o600, runner.root())
	}
	return result, err
}
