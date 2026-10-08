package launch

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"context"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"golang.org/x/sys/unix"
)

type UnitStepState string

const (
	StepPending  UnitStepState = "pending"
	StepStarting UnitStepState = "starting"
	StepRunning  UnitStepState = "running"
	StepPassed   UnitStepState = "passed"
	StepFailed   UnitStepState = "failed"
	StepSkipped  UnitStepState = "skipped"
)

type UnitRunRecord struct {
	CorrectionBudget *int        `json:"correctionBudget,omitempty"`
	PolicyError      string      `json:"policyError,omitempty"`
	ID               string      `json:"id"`
	Unit             string      `json:"unit"`
	Goal             string      `json:"goal"`
	Worktree         string      `json:"worktree"`
	Base             string      `json:"base"`
	Plan             string      `json:"plan"`
	PlanDirectory    string      `json:"planDirectory"`
	State            string      `json:"state"`
	Rounds           []UnitRound `json:"rounds"`
	// BuildModel and BuildEffort are the unit's own choice over the
	// configured build lane; MaxRounds is its approved ceiling on rounds.
	// All three are fixed when the run starts and kept for every round.
	BuildModel  string `json:"buildModel,omitempty"`
	BuildEffort string `json:"buildEffort,omitempty"`
	MaxRounds   int    `json:"maxRounds,omitempty"`
	// CountedCap limits demonstrated own defects; zero imposes no cap.
	CountedCap int `json:"countedCap,omitempty"`
	// Subjects binds completed rounds to their committed goal-branch
	// subjects; ReviewSubject is its only writer.
	Subjects []UnitSubject `json:"subjects,omitempty"`
	Notes    []string      `json:"notes,omitempty"`
	// Revisions are the retained correction requests; Revise is their only
	// writer.
	Revisions []UnitRevision `json:"revisions,omitempty"`
	Mutation  *identity.Ref  `json:"mutation,omitempty"`
}

type UnitRound struct {
	Result           *RoundResult       `json:"result,omitempty"`
	GapMessage       string             `json:"gapMessage,omitempty"`
	BuildLines       int64              `json:"buildLines,omitempty"`
	DeclaredLines    int64              `json:"declaredLines,omitempty"`
	SizeAcceptedBy   string             `json:"sizeAcceptedBy,omitempty"`
	Reads            []readsubject.Read `json:"reads,omitempty"`
	Stop             *loopstop.Stop     `json:"stop,omitempty"`
	UnknownRetries   int                `json:"unknownRetries,omitempty"`
	Transferred      bool               `json:"transferred,omitempty"`
	BuildBriefSHA256 string             `json:"buildBriefSha256,omitempty"`
	Number           int                `json:"number"`
	Directory        string             `json:"directory"`
	FollowUp         string             `json:"followUp"`
	Outcome          string             `json:"outcome"`
	Cause            string             `json:"cause,omitempty"`
	// Material is -1 when the examination has no readable return.
	Material   int        `json:"material"`
	Repeats    int        `json:"repeats,omitempty"`
	BuildModel string     `json:"buildModel"`
	ReadModel  string     `json:"readModel"`
	Steps      []UnitStep `json:"steps"`
}

type UnitStep struct {
	Name          string        `json:"name"`
	LaunchID      string        `json:"launchId"`
	State         UnitStepState `json:"state"`
	Reason        string        `json:"reason"`
	Cause         string        `json:"cause,omitempty"`
	StartedAt     string        `json:"startedAt"`
	FinishedAt    string        `json:"finishedAt"`
	Model         string        `json:"model"`
	Mode          string        `json:"mode,omitempty"`
	Package       string        `json:"package,omitempty"`
	File          string        `json:"file,omitempty"`
	Brief         string        `json:"brief,omitempty"`
	Units         []string      `json:"units,omitempty"`
	Rerun         bool          `json:"rerun,omitempty"`
	Verdict       string        `json:"verdict,omitempty"`
	VerdictCounts *bool         `json:"verdictCounts,omitempty"`

	Moved       []string            `json:"moved,omitempty"`
	RetryBy     string              `json:"retryBy,omitempty"`
	RetryReason string              `json:"retryReason,omitempty"`
	RetryLaunch string              `json:"retryLaunch,omitempty"`
	Deadline    bool                `json:"deadline,omitempty"`
	LaunchIDs   []string            `json:"launchIds,omitempty"`
	Retained    *StartSpec          `json:"retained,omitempty"`
	Before      *repositorySnapshot `json:"before,omitempty"`
	Comparison  *unitComparison     `json:"comparison,omitempty"`
	FlakeRepeat bool                `json:"flakeRepeat,omitempty"`
}

type UnitRequest struct{ Plan, Resume, FollowUp string }
type UnitResult struct {
	Record       UnitRunRecord
	Round        int
	Step, Launch string
	Capped       bool
}

type GitRunner interface {
	Run(directory string, environment []string, args ...string) ([]byte, error)
}
type OSGitRunner struct{}

func (OSGitRunner) Run(directory string, environment []string, args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir, command.Env = directory, childEnvironment(os.Environ(), environment)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

type UnitRunner struct {
	Manager *Manager
	Git     GitRunner
	Root    string
	// ExaminationRoot is the caller's installation; each examination carries its own return path.
	ExaminationRoot string
	AfterWrite      func(UnitRunRecord) error
	// BeforeModelLaunch, when set, is asked before each new build, proof or read
	// launch of any run it advances (new, resumed, follow-up or waited);
	// its refusal starts nothing.
	BeforeModelLaunch func(UnitRunRecord, StartSpec) error
	// CollectLaunch settles the physical execution through its reservation owner.
	CollectLaunch func(UnitRunRecord, Record, string) error
	// PlanProof selects proof after the build. A nil result retains the
	// caller's commands; selected commands are frozen for this round's retries.
	PlanProof         func(UnitPlan) ([]ProofCommand, error)
	InheritedFindings func(string, string) ([]readsubject.Finding, error)
	ReviewPolicy      func() (string, error)
	ExaminationRead   func(string, string) (readsubject.Read, error)
	KnownFlake        func(Record) (bool, error)
	RecordMain        func(Record, string, string) error
	// named is set only on the per-call copy AdvanceNamed hands to Advance.
	named *namedBinding
	// options is set only on the per-call copy AdvancePrepared makes; a new
	// run records it.
	options   UnitOptions
	resolving bool
	tree      *treeCommand
	mutation  string
}

func (runner *UnitRunner) Advance(request UnitRequest) (UnitResult, error) {
	if runner.Manager == nil {
		return UnitResult{}, errors.New("unit launch manager is unavailable")
	}
	if (request.Plan == "") == (request.Resume == "") {
		return UnitResult{}, errors.New("unit run requires exactly one of plan or resume")
	}
	if request.Plan != "" && request.FollowUp != "" {
		return UnitResult{}, errors.New("follow-up requires resume")
	}
	var record UnitRunRecord
	var plan UnitPlan
	var err error
	if request.Plan != "" {
		plan, err = ReadUnitPlan(request.Plan)
		if err != nil {
			return UnitResult{}, err
		}
		if err = runner.requireGoalBranch(plan); err != nil {
			return UnitResult{}, err
		}
		if err = runner.admitRound(plan, plan.Build.Brief, nil); err != nil {
			return UnitResult{}, err
		}
		if runner.tree == nil {
			return treeCall(runner, plan.Worktree, func(r *UnitRunner) (UnitResult, error) { return r.Advance(request) })
		}
		record, err = runner.newRun(plan)
		if err != nil {
			return UnitResult{}, err
		}
	} else {
		record, err = runner.read(request.Resume)
		if err != nil {
			return UnitResult{}, err
		}
		if runner.tree == nil {
			return treeCall(runner, record.Worktree, func(r *UnitRunner) (UnitResult, error) { return r.Advance(request) })
		}
		plan, err = readUnitPlan(record.Plan, record.PlanDirectory)
		if err != nil {
			return UnitResult{}, err
		}
	}
	if record.State != "cancelled" {
		if err := runner.reserveTree(record); err != nil {
			return UnitResult{Record: record}, err
		}
	}
	lock, err := runner.lock(record.ID)
	if err != nil {
		return UnitResult{}, err
	}
	defer releaseUnitLock(lock)
	if request.Resume != "" {
		record, err = runner.read(record.ID)
		if err != nil {
			return UnitResult{}, err
		}
	}
	if record.State == "cancelled" {
		return UnitResult{Record: record}, coded("UNIT_CANCELLED", "run="+record.ID, errors.New("this run was cancelled; build new work under another name"))
	}
	for _, round := range record.Rounds {
		if err := runner.collectLaunches(record, round); err != nil {
			return UnitResult{Record: record}, err
		}
	}
	if runner.named != nil {
		runner.named.plan = plan
		if err := runner.named.verify(runner); err != nil {
			return UnitResult{}, err
		}
	}
	if request.Resume != "" && (request.FollowUp != "" || record.State != "awaiting-judgement") {
		if request.FollowUp == "" {
			for _, revision := range record.Revisions {
				if revision.Attempt == len(record.Rounds) && revision.Rebase != nil {
					plan, err = runner.rebasePlan(plan, revision.Rebase)
					if err != nil {
						return UnitResult{}, err
					}
					bound := *runner
					bound.resolving = true
					runner = &bound
				}
			}
		}
		if !runner.resolving {
			if err := runner.requireGoalBranch(plan); err != nil {
				return UnitResult{}, err
			}
		}
	}
	if request.FollowUp != "" {
		if err := runner.allowCorrection(record); err != nil {
			return UnitResult{Record: record}, err
		}
		if record.MaxRounds > 0 && len(record.Rounds) >= record.MaxRounds {
			return UnitResult{}, roundLimit(record, len(record.Rounds))
		}
		if err := runner.countedCap(record); err != nil {
			return UnitResult{}, err
		}
		if err := runner.roundDivergent(record); err != nil {
			return UnitResult{Record: record}, err
		}
		if err := admitFollowUp(record, request.FollowUp); err != nil {
			return UnitResult{}, err
		}
		previous := readOutputs(runner.Manager, record.Rounds[len(record.Rounds)-1])
		if err := runner.admitRound(plan, request.FollowUp, previous); err != nil {
			return UnitResult{}, err
		}
		if err := runner.addRound(&record, plan, request.FollowUp); err != nil {
			return UnitResult{}, err
		}
	} else if record.State == "awaiting-judgement" {
		return UnitResult{Record: record, Round: len(record.Rounds)}, nil
	}
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return UnitResult{}, err
	}
	deadline := runner.Manager.Now().Add(time.Duration(settings.WaitCapSeconds) * time.Second)
	return runner.advanceRunning(&record, plan, deadline)
}

func (runner *UnitRunner) admitRound(plan UnitPlan, buildBrief string, previous []string) error {
	buildInputs := append(append([]string{}, plan.Build.Inputs...), previous...)
	spec := StartSpec{Kind: "build", Goal: plan.Goal, Tag: plan.Unit, WorkingDirectory: plan.Worktree,
		Brief: buildBrief, Inputs: buildInputs, Outputs: plan.Build.Outputs, UnitsPage: plan.Build.UnitsPage, Units: plan.Build.Units}
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return err
	}
	if err := runner.Manager.admit(spec, settings); err != nil {
		if ErrorCode(err) != "LAUNCH_BUILD_OVERSIZE" {
			return err
		}
		units, _, sizeErr := buildSize(spec)
		if sizeErr != nil {
			return sizeErr
		}
		for _, group := range serialGroups(units, settings.BuildLinesCap) {
			groupSpec := spec
			groupSpec.Units = group.Units
			if group.OverCap {
				return runner.Manager.Admit(groupSpec)
			}
			if groupErr := runner.Manager.Admit(groupSpec); groupErr != nil {
				return groupErr
			}
		}
	}
	return nil
}

func (runner *UnitRunner) requireGoalBranch(plan UnitPlan) error {
	git := runner.Git
	if git == nil {
		git = OSGitRunner{}
	}
	output, branchErr := git.Run(plan.Worktree, nil, "symbolic-ref", "--short", "HEAD")
	branch := strings.TrimSpace(string(output))
	if branchErr != nil {
		branch = "unavailable"
		if _, headErr := git.Run(plan.Worktree, nil, "rev-parse", "--verify", "HEAD"); headErr == nil {
			branch = "detached HEAD"
		}
	}
	required := "goal/" + plan.Goal
	if branch == required {
		return nil
	}
	err := coded("LAUNCH_UNIT_GOAL_BRANCH_REQUIRED", fmt.Sprintf("worktree=%q branch=%q required=%q", plan.Worktree, branch, required),
		fmt.Errorf("%s is on %s, and work on goal %s is built on its branch %s", plan.Worktree, branch, plan.Goal, required))
	_ = runner.Manager.Store.AppendRefusal(Refusal{Time: runner.Manager.Now().UTC().Format(time.RFC3339Nano), Code: "LAUNCH_UNIT_GOAL_BRANCH_REQUIRED", Kind: "build", Goal: plan.Goal, Tag: plan.Unit})
	return err
}

func (runner *UnitRunner) newRun(plan UnitPlan) (UnitRunRecord, error) {
	id, err := newID(runner.Manager.Now())
	if err != nil {
		return UnitRunRecord{}, err
	}
	return runner.newRunWithID(plan, id, filepath.Dir(plan.Path))
}

// newRunWithID writes round one of a run whose id was chosen before the
// record exists, so a named unit can reserve the id first and a restart
// after an interruption writes the same run again.
func (runner *UnitRunner) newRunWithID(plan UnitPlan, id, planDirectory string) (UnitRunRecord, error) {
	dir := runner.runDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return UnitRunRecord{}, err
	}
	copyPath := filepath.Join(dir, "plan.json")
	if _, err := atomicfile.CopyFile(plan.Path, copyPath, filepath.Dir(dir)); err != nil {
		return UnitRunRecord{}, err
	}
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return UnitRunRecord{}, err
	}
	record := UnitRunRecord{ID: id, Unit: plan.Unit, Goal: plan.Goal, Worktree: plan.Worktree, Base: plan.Base, Plan: copyPath, PlanDirectory: planDirectory, State: "running",
		BuildModel: runner.options.BuildModel, BuildEffort: runner.options.BuildEffort, MaxRounds: runner.options.MaxRounds, CountedCap: int(settings.UnitCountedRounds)}
	policy := "auto"
	if runner.ReviewPolicy != nil {
		value, readErr := runner.ReviewPolicy()
		if readErr != nil {
			return UnitRunRecord{}, readErr
		}
		policy = value
	}
	budget, err := correctionBudget(policy, record)
	if err != nil {
		return UnitRunRecord{}, err
	}
	record.CorrectionBudget = &budget
	round := UnitRound{Number: 1, Directory: filepath.Join(dir, "round-1"), BuildModel: choose(record.BuildModel, settings.BuildModel)}
	if plan.HasRead() {
		round.ReadModel = choose(plan.Read.Model, settings.ReadModel)
	}
	if err := os.MkdirAll(round.Directory, 0o700); err != nil {
		return UnitRunRecord{}, err
	}
	round.Steps = []UnitStep{{Name: "build", State: StepPending, Model: round.BuildModel}}
	record.Rounds = append(record.Rounds, round)
	return record, runner.save(record)
}

func (runner *UnitRunner) addRound(record *UnitRunRecord, plan UnitPlan, followUp string) error {
	number := len(record.Rounds) + 1
	directory := filepath.Join(runner.runDir(record.ID), "round-"+strconv.Itoa(number))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	target := filepath.Join(directory, "follow-up.md")
	if _, err := atomicfile.CopyFile(followUp, target, runner.runDir(record.ID)); err != nil {
		return err
	}
	settings, _ := runner.Manager.resolvedSettings()
	record.State = "running"
	round := UnitRound{Number: number, Directory: directory, FollowUp: target,
		BuildModel: choose(record.BuildModel, settings.BuildModel), Steps: []UnitStep{{Name: "build", State: StepPending, Model: choose(record.BuildModel, settings.BuildModel)}}}
	if plan.HasRead() {
		round.ReadModel = choose(plan.Read.Model, settings.ReadModel)
	}
	record.Rounds = append(record.Rounds, round)
	return runner.save(*record)
}

func (runner *UnitRunner) advanceRunning(record *UnitRunRecord, plan UnitPlan, deadline time.Time) (UnitResult, error) {
	round := &record.Rounds[len(record.Rounds)-1]
	brief, previous := plan.Build.Brief, []string(nil)
	if round.FollowUp != "" {
		brief = round.FollowUp
		previous = readOutputs(runner.Manager, record.Rounds[len(record.Rounds)-2])
		prior := record.Rounds[len(record.Rounds)-2]
		if prior.Outcome == "build-gap" {
			previous = append(previous, record.Plan, filepath.Join(prior.Directory, "worktree.diff"))
			if _, err := os.Stat(prior.GapMessage); err == nil {
				previous = append(previous, prior.GapMessage)
			}
		}
		for _, revision := range record.Revisions {
			if revision.Attempt == round.Number && revision.Decisions != "" {
				previous = append(previous, revision.Decisions)
			}
		}
	}
	buildCount, err := runner.ensureBuildSteps(record, round, plan, brief)
	if err != nil {
		return UnitResult{}, err
	}
	if round.BuildBriefSHA256 == "" && round.Steps[0].LaunchID == "" {
		data, err := os.ReadFile(brief)
		if err != nil {
			return UnitResult{}, err
		}
		round.BuildBriefSHA256 = digestHex(data)
		if err := runner.save(*record); err != nil {
			return UnitResult{}, err
		}
	}
	buildInputs := append(append([]string{}, plan.Build.Inputs...), previous...)
	if round.Steps[0].LaunchID == "" {
		if err := runner.writeDiff(plan.Worktree, plan.Base, filepath.Join(round.Directory, "build-before", "worktree.diff")); err != nil {
			return UnitResult{}, err
		}
	}
	for index := 0; index < buildCount; index++ {
		step := &round.Steps[index]
		buildSpec := StartSpec{Kind: "build", Goal: plan.Goal, Tag: plan.Unit, WorkingDirectory: plan.Worktree, Brief: step.Brief, Model: record.BuildModel, Effort: record.BuildEffort,
			Inputs: buildInputs, Outputs: plan.Build.Outputs, UnitsPage: plan.Build.UnitsPage, Units: step.Units,
			Round: round.Number, MaxRounds: record.MaxRounds}
		if capped, stepErr := runner.advanceStep(record, round, index, buildSpec, deadline); stepErr != nil || capped {
			return runner.result(*record, round, step, capped), stepErr
		}
		if step.State != StepPassed {
			for pending := index + 1; pending < buildCount; pending++ {
				if round.Steps[pending].State == StepPending {
					round.Steps[pending].State, round.Steps[pending].Reason = StepSkipped, "build-failed"
				}
			}
			for _, command := range proofCommands(plan) {
				round.Steps = append(round.Steps, UnitStep{Name: "proof:" + command.Name, State: StepSkipped, Reason: "build-failed"})
			}
			if plan.HasRead() {
				round.Steps = append(round.Steps, UnitStep{Name: "read", State: StepSkipped, Reason: "build-failed", Model: round.ReadModel})
			}
			return runner.finish(record, round, "build-failed")
		}
	}
	if held, err := runner.freezeBuildOutcome(record, round, plan, buildCount); err != nil || held {
		return UnitResult{Record: *record, Round: round.Number}, err
	}
	planned, err := runner.roundProofPlan(plan, round.Directory, len(round.Steps) > buildCount)
	if err != nil {
		round.Steps = append(round.Steps, UnitStep{Name: "proof:plan", State: StepFailed, Reason: err.Error()})
		for _, command := range proofCommands(plan) {
			round.Steps = append(round.Steps, UnitStep{Name: "proof:" + command.Name, State: StepSkipped, Reason: "proof-red"})
		}
		if plan.HasRead() {
			round.Steps = append(round.Steps, UnitStep{Name: "read", State: StepSkipped, Reason: "proof-red", Model: round.ReadModel})
		}
		return runner.finish(record, round, "proof-red")
	}
	plan = planned
	if round.Result != nil {
		commands, err := json.Marshal(proofCommands(plan))
		if err != nil {
			return UnitResult{}, err
		}
		round.Result.ProofIdentity = digestHex(commands)
	}
	if len(round.Steps) == buildCount {
		for _, command := range proofCommands(plan) {
			round.Steps = append(round.Steps, UnitStep{Name: "proof:" + command.Name, State: StepPending})
		}
		if err := runner.save(*record); err != nil {
			return UnitResult{}, err
		}
	}
	if err := proofMayStart(*round, buildCount); err != nil {
		return UnitResult{}, err
	}
	commands := proofCommands(plan)
	beforeProof, err := runner.savedRepositorySnapshot(round.Directory, "proof-before.json", plan.Worktree)
	if IsCode(err, "UNIT_TREE_CHANGED") {
		return runner.treeMoved(record, round)
	}
	if err != nil {
		return UnitResult{}, err
	}
	red := false
	for offset, command := range commands {
		index := buildCount + offset
		briefPath := filepath.Join(round.Directory, "proof-"+command.Name+".json")
		if _, err := os.Stat(briefPath); os.IsNotExist(err) {
			data, _ := json.Marshal(PlainBrief{command.Argv, command.Dir, command.Env})
			if _, err := atomicfile.WriteText(briefPath, string(data)+"\n", runner.runDir(record.ID)); err != nil {
				return UnitResult{}, err
			}
		}
		spec := StartSpec{Kind: "proof", Goal: plan.Goal, Tag: plan.Unit, WorkingDirectory: command.Dir, Brief: briefPath, Round: round.Number, MaxRounds: record.MaxRounds}
		if capped, err := runner.advanceStep(record, round, index, spec, deadline); err != nil || capped {
			return runner.result(*record, round, &round.Steps[index], capped), err
		}
		if capped, err := runner.attributeProof(record, round, index, plan.Base, deadline); err != nil || capped {
			return runner.result(*record, round, &round.Steps[index], capped), err
		}
		red = red || round.Steps[index].State != StepPassed
		if red {
			if err := runner.skipAfter(record, round, index+1, "proof-red"); err != nil {
				return UnitResult{}, err
			}
			break
		}
	}
	afterProof, err := runner.savedRepositorySnapshot(round.Directory, "proof-after.json", plan.Worktree)
	if IsCode(err, "UNIT_TREE_CHANGED") {
		return runner.treeMoved(record, round)
	}
	if err != nil {
		return UnitResult{}, err
	}
	var moved []string
	if beforeProof != afterProof {
		moved = append(moved, "worktree")
	}
	for _, step := range round.Steps[buildCount:] {
		moved = append(moved, step.Moved...)
	}
	if len(moved) > 0 {
		round.Cause = "environment"
	}
	diffPath := filepath.Join(round.Directory, "worktree.diff")
	if _, err := os.Stat(diffPath); os.IsNotExist(err) {
		if err := runner.writeDiff(plan.Worktree, plan.Base, diffPath); err != nil {
			return UnitResult{}, err
		}
	}
	readInputs := append(append(append([]string{}, plan.Read.Inputs...), previous...), round.FollowUp)
	if runner.resolving {
		if len(moved) != 0 {
			return runner.finish(record, round, "proof-wrote")
		}
		if red {
			return runner.finish(record, round, "proof-red")
		}
		return runner.finish(record, round, "green")
	}
	if round.FollowUp == "" {
		readInputs = readInputs[:len(readInputs)-1]
	}
	sequence := runner.readSequence(record, round, plan, readInputs, diffPath)
	readStart := len(round.Steps)
	if plan.HasRead() {
		readStart, err = sequence.plan()
		if err != nil {
			return UnitResult{}, err
		}
	}
	if len(moved) != 0 {
		if err := runner.skipAfter(record, round, readStart, "proof-wrote:"+strings.Join(moved, ",")); err != nil {
			return UnitResult{}, err
		}
		return runner.finish(record, round, "proof-wrote")
	}
	if red {
		if err := runner.skipAfter(record, round, readStart, "proof-red"); err != nil {
			return UnitResult{}, err
		}
		return runner.finish(record, round, "proof-red")
	}
	packet, err := runner.warmRead(*record, *round)
	if err != nil {
		return UnitResult{}, err
	}
	if packet != "" && plan.HasRead() {
		brief, err := os.ReadFile(plan.Read.Brief)
		if err != nil {
			return UnitResult{}, err
		}
		brief = []byte(strings.Replace(string(brief), "Not this read. A follow-up round of this unit is read again by the unit runner with this brief.", "Check the follow-up packet below before reading new changes.", 1))
		path := filepath.Join(round.Directory, "read-brief.md")
		if _, err := atomicfile.WriteText(path, string(brief)+packet, runner.root()); err != nil {
			return UnitResult{}, err
		}
		sequence.spec.Brief = path
	}
	if !plan.HasRead() {
		return runner.finish(record, round, "green")
	}
	outcome, stop, capped, err := sequence.advance(readStart, deadline)
	if err != nil || capped {
		if stop < 0 {
			return UnitResult{}, err
		}
		return runner.result(*record, round, &round.Steps[stop], capped), err
	}
	afterRead, err := runner.snapshotRepository(plan.Worktree)
	if err != nil {
		return UnitResult{}, err
	}
	if afterRead != afterProof {
		return runner.treeMoved(record, round)
	}
	return runner.finish(record, round, outcome)
}

func (runner *UnitRunner) roundProofPlan(plan UnitPlan, directory string, proofPlanned bool) (UnitPlan, error) {
	path := filepath.Join(directory, "plan.json")
	if _, err := os.Stat(path); err == nil {
		return ReadUnitPlan(path)
	} else if !os.IsNotExist(err) {
		return plan, err
	}
	if runner.PlanProof != nil && !proofPlanned {
		commands, err := runner.PlanProof(plan)
		if err != nil {
			return plan, err
		}
		if commands != nil {
			plan.Proof = commands
		}
	}
	if err := plan.resolveAndValidate(directory); err != nil {
		return plan, err
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return plan, err
	}
	_, err = atomicfile.WriteText(path, string(data)+"\n", directory)
	return plan, err
}

// readSequence is the round's reads under the shared read partition policy.
func (runner *UnitRunner) readSequence(record *UnitRunRecord, round *UnitRound, plan UnitPlan, inputs []string, diffPath string) readSequence {
	return readSequence{driver: runner.driver(record, round), model: round.ReadModel, diff: diffPath, serial: len(plan.Read.Outputs) > 0,
		spec: StartSpec{StopInputs: true, Kind: "read", Goal: plan.Goal, Tag: plan.Unit, WorkingDirectory: plan.Worktree, Brief: plan.Read.Brief,
			Inputs: inputs, Outputs: plan.Read.Outputs, Model: plan.Read.Model, Round: round.Number, MaxRounds: record.MaxRounds}}
}

// driver starts a unit round's launches under the run's own launch ids,
// its launch gate and its named reservation.
func (runner *UnitRunner) driver(record *UnitRunRecord, round *UnitRound) stepDriver {
	return stepDriver{manager: runner.Manager, round: round, launchID: unitLaunchID(record, round), start: func(spec StartSpec) (Record, error) {
		spec.wait = runner.CommandWait
		return runner.Manager.Start(spec)
	},
		save: func() error {
			if err := runner.collectLaunches(*record, *round); err != nil {
				return err
			}
			return runner.save(*record)
		}, unit: true, snapshot: runner.snapshotRepository, wait: runner.waitLaunch,
		before: func(spec StartSpec) error {
			if runner.BeforeModelLaunch != nil {
				if err := runner.BeforeModelLaunch(*record, spec); err != nil {
					if IsCode(err, "BUDGET_REFUSED") || IsCode(err, "BUDGET_UNKNOWN") {
						return err
					}
					return coded("UNIT_LAUNCH_UNAUTHORIZED", unitFacts(record.Unit, record.Goal, "run="+record.ID), fmt.Errorf("unit %s may not start a launch: %w", record.Unit, err))
				}
			}
			if runner.named != nil {
				return runner.named.verify(runner)
			}
			return nil
		}}
}

func (runner *UnitRunner) ensureBuildSteps(record *UnitRunRecord, round *UnitRound, plan UnitPlan, brief string) (int, error) {
	count := 0
	for count < len(round.Steps) && strings.HasPrefix(round.Steps[count].Name, "build") {
		count++
	}
	if count > 1 || count == 1 && len(round.Steps[0].Units) > 0 {
		return count, nil
	}
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return 0, err
	}
	units, size, err := buildSize(StartSpec{Brief: brief, UnitsPage: plan.Build.UnitsPage, Units: plan.Build.Units})
	if err != nil {
		return 0, err
	}
	groups := []buildGroup{{Units: append([]string(nil), plan.Build.Units...), Size: size}}
	if size > settings.BuildLinesCap {
		groups = serialGroups(units, settings.BuildLinesCap)
	}
	steps := make([]UnitStep, 0, len(groups))
	for index, group := range groups {
		if group.OverCap {
			return 0, coded("LAUNCH_BUILD_OVERSIZE", fmt.Sprintf("unit=%s size=%d cap=%d", strings.Join(group.Units, ","), group.Size, settings.BuildLinesCap),
				fmt.Errorf("unit %s is %d lines, over the %d-line limit of one build; split it into smaller units", strings.Join(group.Units, ","), group.Size, settings.BuildLinesCap))
		}
		name, stepBrief := "build", brief
		if len(groups) > 1 {
			name = fmt.Sprintf("build:%d", index+1)
			stepBrief = filepath.Join(round.Directory, fmt.Sprintf("build-%d.md", index+1))
			if _, statErr := os.Stat(stepBrief); os.IsNotExist(statErr) {
				data, readErr := os.ReadFile(brief)
				if readErr != nil {
					return 0, readErr
				}
				content := strings.TrimRight(string(data), "\n") + "\nBuild units: " + strings.Join(group.Units, ",") + "\n"
				if _, writeErr := atomicfile.WriteText(stepBrief, content, runner.runDir(record.ID)); writeErr != nil {
					return 0, writeErr
				}
			}
		}
		steps = append(steps, UnitStep{Name: name, State: StepPending, Model: round.BuildModel, Brief: stepBrief, Units: append([]string(nil), group.Units...)})
	}
	round.Steps = append(steps, round.Steps[count:]...)
	return len(steps), runner.save(*record)
}

func unitReadPacket(diffPath string, step UnitStep) (string, error) {
	data, err := os.ReadFile(diffPath)
	if err != nil {
		return "", err
	}
	choice, err := ChooseReadMode(diffPath, 1<<62)
	if err != nil {
		return "", err
	}
	var packages []string
	for name := range choice.Directories {
		if name != step.Package {
			packages = append(packages, name)
		}
	}
	sort.Strings(packages)
	selected := step.Package
	coverage := strings.Join(packages, ", ") + " (separate unit read steps)"
	if selected == "" {
		selected = "all packages (whole or wide read)"
		coverage = strings.Join(packages, ", ") + " (included in this read)"
	}
	if len(packages) == 0 {
		coverage = "none"
	}
	packet := fmt.Sprintf("Full candidate diff: %s\nFull candidate SHA-256: %x\nSelected package: %s\nOther package coverage: %s\n",
		diffPath, sha256.Sum256(data), selected, coverage)
	if step.File != "" {
		packet += "Selected file: " + step.File + "\n"
	}
	return packet, nil
}

func appendReadPacket(data []byte, record Record) []byte {
	if packet := readString(record.AdapterData, "unitReadPacket"); packet != "" {
		data = append(data, []byte("\n"+packet)...)
	}
	if diff := readString(record.AdapterData, "readDiff"); diff != "" {
		data = append(data, []byte("\nDiff: "+diff+"\n")...)
	}
	// A read that declares its report file names it: the reader cannot
	// otherwise know where the report it must leave belongs.
	var outputs []string
	if record.Kind == "read" && json.Unmarshal(record.AdapterData["declaredOutputs"], &outputs) == nil && len(outputs) >= 1 {
		data = append(data, []byte("\nWrite your complete report, ending with its VERDICT line, to exactly this file: "+outputs[0]+"\nThe read is not complete until that file exists.\n")...)
	}
	if record.Kind == "read" && len(outputs) == 2 {
		data = append(data, []byte("\nAlso write structured findings to "+outputs[1]+". The JSON object requires findings (an array, including empty) and verdictMaterialCount. Each finding requires severity, boolean material, class (regression, weakened-test, incomplete-item, false-premise, faked-seam, missing-reader, scope, other), claim, evidence, where (repository-relative path without line suffix), change, and optional resolves (prior read-qualified finding id) or relation. Class other requires relation naming its rule. The prose VERDICT must agree with the structured material count.\n")...)
	}
	return data
}

func unitStepVerdictCounts(step UnitStep) bool {
	return step.VerdictCounts != nil && *step.VerdictCounts
}

func (runner *UnitRunner) advanceStep(record *UnitRunRecord, round *UnitRound, index int, spec StartSpec, deadline time.Time) (bool, error) {
	return runner.driver(record, round).advanceStep(index, spec, deadline)
}

func (runner *UnitRunner) skipAfter(record *UnitRunRecord, round *UnitRound, from int, reason string) error {
	for index := from; index < len(round.Steps); index++ {
		if round.Steps[index].State == StepPending {
			round.Steps[index].State, round.Steps[index].Reason = StepSkipped, reason
		}
	}
	return runner.save(*record)
}

func (runner *UnitRunner) finish(record *UnitRunRecord, round *UnitRound, outcome string) (UnitResult, error) {
	round.Outcome, record.State = outcome, "awaiting-judgement"
	if round.Cause == "" {
		round.Cause = roundCause(round.Steps)
	}
	if strings.HasPrefix(outcome, "build-") || strings.HasPrefix(outcome, "proof-") || !slices.ContainsFunc(round.Steps, func(step UnitStep) bool { return strings.HasPrefix(step.Name, "read") }) {
		round.Material = -1
	} else {
		if err := runner.collectRoundRead(record, round); err != nil {
			return UnitResult{}, err
		}
	}
	if round.Cause != "" && round.Stop == nil {
		if err := runner.holdFailedStep(record, round); err != nil {
			return UnitResult{}, err
		}
	}
	if err := runner.save(*record); err != nil {
		return UnitResult{}, err
	}
	runner.publishJudgement(*record, round.Number)
	return UnitResult{Record: *record, Round: round.Number}, nil
}

// Attribution supplies own only when evidence demonstrates a defect.
func roundCause(steps []UnitStep) string {
	for _, step := range steps {
		if step.State == StepFailed {
			if (loopstop.Cause{Kind: step.Cause}).Valid() {
				return step.Cause
			}
			return "unclassified"
		}
	}
	return ""
}

// publishJudgement puts the finished round on the board as judgement: the
// read is in, a person or the coordinator decides, and no process is waited
// on (D14, R24). The run's step states write no card: the launch manager's
// state writes cover every step.
func (runner *UnitRunner) publishJudgement(record UnitRunRecord, number int) {
	if runner.Manager == nil || runner.Manager.Seat.Machine == "" || record.Goal == "" {
		return
	}
	card := board.Card{Seat: runner.Manager.Seat, Goal: record.Goal, Stage: board.StageJudgement,
		Round: judgementRound(record, number), Job: &board.Job{ID: record.ID, Kind: "unit-run", Phase: record.State}, Writer: board.Writer{Component: "unit-run"}}
	if len(record.Rounds) > 0 {
		stop := record.Rounds[len(record.Rounds)-1].Stop
		if stop != nil {
			card.Stop = &board.ReviewStop{Decision: stop.Decision, Handoff: stop.Handoff, Class: stop.Class, Attempt: stop.Attempt, Budget: stop.Budget}
		}
	}
	if runner.Manager.Now != nil {
		card.Writer.At = runner.Manager.Now()
	}
	if err := board.Write(card); err != nil {
		fmt.Fprintf(os.Stderr, "unit run %s: the board card was not written: %v\n", record.ID, err)
	}
}

func judgementRound(record UnitRunRecord, number int) *board.Round {
	round := &board.Round{N: number}
	if record.CountedCap > 0 {
		round.N, _ = countedRounds(record)
		limit := record.CountedCap
		round.Max = &limit
	} else if record.MaxRounds > 0 {
		limit := record.MaxRounds
		round.Max = &limit
	}
	return round
}

func countedRounds(record UnitRunRecord) (counted, machinery int) {
	for _, round := range record.Rounds {
		if round.Cause == "own" {
			counted++
		} else {
			machinery++
		}
	}
	return
}

func (runner *UnitRunner) roundMaterial(record UnitRunRecord, round UnitRound) (int, error) {
	material, _, err := runner.roundFindings(record, round)
	return material, err
}

func (runner *UnitRunner) roundFindings(record UnitRunRecord, round UnitRound) (int, int, error) {
	if len(round.Reads) == 0 || round.Stop != nil && (round.Stop.Handoff == "stopped unreadable-read" || round.Stop.Handoff == "stopped unavailable-stop-inputs") {
		return -1, 0, nil
	}
	material := 0
	for _, read := range round.Reads {
		material += read.Material
	}
	return material, round.Repeats, nil
}

func (runner *UnitRunner) result(record UnitRunRecord, round *UnitRound, step *UnitStep, capped bool) UnitResult {
	return UnitResult{Record: record, Round: round.Number, Step: step.Name, Launch: step.LaunchID, Capped: capped}
}

func (runner *UnitRunner) writeDiff(worktree, base, target string) error {
	diff, err := runner.WorktreeDiff(worktree, base)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(target, string(diff), filepath.Dir(filepath.Dir(target)))
	return err
}

// WorktreeDiff is the worktree's cumulative binary diff against base,
// computed through a disposable index exactly as a round's worktree.diff.
func (runner *UnitRunner) WorktreeDiff(worktree, base string) ([]byte, error) {
	return runner.worktreeDiff(worktree, base)
}

// worktreeDiff is WorktreeDiff leaving out the exact worktree-relative
// paths excluded, selected literally by Git pathspec.
func (runner *UnitRunner) worktreeDiff(worktree, base string, excluded ...string) ([]byte, error) {
	git := runner.Git
	if git == nil {
		git = OSGitRunner{}
	}
	temporary, done, err := diskstore.ScratchDir("metasystem-unit-diff.")
	if err != nil {
		return nil, err
	}
	defer done()
	indexPathData, err := git.Run(worktree, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return nil, err
	}
	objectsData, err := git.Run(worktree, nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return nil, err
	}
	index := filepath.Join(temporary, "index")
	sourceIndex := strings.TrimSpace(string(indexPathData))
	if data, readErr := os.ReadFile(sourceIndex); readErr == nil {
		if err := os.WriteFile(index, data, 0o600); err != nil {
			return nil, err
		}
	} else {
		return nil, readErr
	}
	// The copy keeps the source index's time: Git trusts an entry's cached
	// file times only when the file is older than the index, so a later copy
	// time would hide an edit made in the index's own second.
	if info, statErr := os.Stat(sourceIndex); statErr == nil {
		if err := os.Chtimes(index, info.ModTime(), info.ModTime()); err != nil {
			return nil, err
		}
	}
	objectDir := filepath.Join(temporary, "objects")
	if err := os.MkdirAll(objectDir, 0o700); err != nil {
		return nil, err
	}
	environment := []string{"GIT_INDEX_FILE=" + index, "GIT_OBJECT_DIRECTORY=" + objectDir, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + strings.TrimSpace(string(objectsData))}
	if _, err := git.Run(worktree, environment, "add", "-A", "--sparse", "--", "."); err != nil {
		return nil, err
	}
	args := []string{"diff", "--cached", "--binary", base, "--", "."}
	for _, path := range excluded {
		args = append(args, ":(exclude,literal)"+path)
	}
	return git.Run(worktree, environment, args...)
}

type repositorySnapshot struct {
	TreeID string `json:"treeId,omitempty"`
	Head   string `json:"head"`
	Refs   string `json:"refs"`
	Index  string `json:"index"`
	Tree   string `json:"tree"`
}

func (snapshot repositorySnapshot) changed(after repositorySnapshot) []string {
	var changed []string
	for _, value := range []struct {
		name          string
		before, after string
	}{
		{"head", snapshot.Head, after.Head},
		{"refs", snapshot.Refs, after.Refs},
		{"index", snapshot.Index, after.Index},
		{"tree", snapshot.Tree, after.Tree},
	} {
		if value.before != value.after {
			changed = append(changed, value.name)
		}
	}
	return changed
}

func (runner *UnitRunner) savedRepositorySnapshot(directory, name, worktree string) (repositorySnapshot, error) {
	path := filepath.Join(directory, name)
	snapshot, err := runner.snapshotRepository(worktree)
	if err != nil {
		return repositorySnapshot{}, err
	}
	if data, readErr := os.ReadFile(path); readErr == nil {
		var prior repositorySnapshot
		if err := json.Unmarshal(data, &prior); err != nil {
			return repositorySnapshot{}, err
		}
		if prior != snapshot {
			return repositorySnapshot{}, coded("UNIT_TREE_CHANGED", "", errors.New("the worktree changed while this command was waiting"))
		}
	} else if !os.IsNotExist(readErr) {
		return repositorySnapshot{}, readErr
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return repositorySnapshot{}, err
	}
	if _, err := atomicfile.WriteText(path, string(data)+"\n", runner.root()); err != nil {
		return repositorySnapshot{}, err
	}
	return snapshot, nil
}

func (runner *UnitRunner) snapshotRepository(worktree string) (repositorySnapshot, error) {
	return runner.snapshotWorktree(worktree, true)
}

func (runner *UnitRunner) snapshotWorktree(worktree string, sharedRefs bool) (repositorySnapshot, error) {
	git := runner.Git
	if git == nil {
		git = OSGitRunner{}
	}
	repositoryRoot, err := git.Run(worktree, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return repositorySnapshot{}, err
	}
	root := strings.TrimSpace(string(repositoryRoot))
	head, err := git.Run(root, nil, "rev-parse", "HEAD")
	if err != nil {
		return repositorySnapshot{}, err
	}
	// Only the refs a proof could write are compared: local branches, tags,
	// notes and the stash. Remote-tracking refs and the engine's own
	// refs/metasystem/ namespace move while a proof runs (the presence
	// publisher and the goal ledger's fetch), and a proof writes neither.
	var refs []byte
	if sharedRefs {
		refs, err = git.Run(root, nil, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads", "refs/tags", "refs/notes", "refs/stash")
		if err != nil {
			return repositorySnapshot{}, err
		}
	}
	indexPath, err := git.Run(root, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return repositorySnapshot{}, err
	}
	indexData, err := os.ReadFile(strings.TrimSpace(string(indexPath)))
	if err != nil {
		return repositorySnapshot{}, err
	}
	objectsPath, err := git.Run(root, nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return repositorySnapshot{}, err
	}
	temporary, done, err := diskstore.ScratchDir("metasystem-unit-snapshot.")
	if err != nil {
		return repositorySnapshot{}, err
	}
	defer done()
	index := filepath.Join(temporary, "index")
	if err := os.WriteFile(index, indexData, 0o600); err != nil {
		return repositorySnapshot{}, err
	}
	objectDir := filepath.Join(temporary, "objects")
	if err := os.MkdirAll(objectDir, 0o700); err != nil {
		return repositorySnapshot{}, err
	}
	environment := []string{"GIT_INDEX_FILE=" + index, "GIT_OBJECT_DIRECTORY=" + objectDir, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + strings.TrimSpace(string(objectsPath))}
	environment = append(environment, "GIT_OPTIONAL_LOCKS=0")
	indexDigest := sha256.New()
	for _, stream := range []struct {
		label string
		args  []string
	}{
		{"ls-files-v-stage", []string{"ls-files", "-v", "--stage", "-z", "--full-name", "--", "."}},
		{"ls-files-t-stage", []string{"ls-files", "-t", "--stage", "-z", "--full-name", "--", "."}},
		{"diff-index-ita-invisible", []string{"diff-index", "--cached", "--raw", "-z", "--ita-invisible-in-index", "HEAD", "--", "."}},
		{"ls-files-resolve-undo", []string{"ls-files", "--resolve-undo", "-z", "--full-name", "--", "."}},
	} {
		output, err := git.Run(root, environment, stream.args...)
		if err != nil {
			return repositorySnapshot{}, err
		}
		_, _ = fmt.Fprintf(indexDigest, "%d:%s:%d:", len(stream.label), stream.label, len(output))
		_, _ = indexDigest.Write(output)
	}
	if _, err := git.Run(root, environment, "add", "-A", "--sparse", "--", "."); err != nil {
		return repositorySnapshot{}, err
	}
	// Full object ids and NUL-delimited exact path bytes: the retained
	// result names content exactly, whatever the path's characters.
	tree, err := git.Run(root, environment, "diff", "--cached", "--raw", "-z", "--no-abbrev", "HEAD", "--", ".")
	if err != nil {
		return repositorySnapshot{}, err
	}
	treeID, err := git.Run(root, environment, "write-tree")
	if err != nil {
		return repositorySnapshot{}, err
	}
	return repositorySnapshot{TreeID: strings.TrimSpace(string(treeID)), Head: string(head), Refs: string(refs), Index: fmt.Sprintf("%x", indexDigest.Sum(nil)), Tree: string(tree)}, nil
}

func proofMayStart(round UnitRound, buildCount int) error {
	if buildCount == 0 || len(round.Steps) < buildCount {
		return errors.New("the checks start only after every build step has passed")
	}
	for index := 0; index < buildCount; index++ {
		if !strings.HasPrefix(round.Steps[index].Name, "build") || round.Steps[index].State != StepPassed {
			return errors.New("the checks start only after every build step has passed")
		}
	}
	return nil
}

func admitFollowUp(run UnitRunRecord, path string) error {
	if run.State != "awaiting-judgement" {
		return coded("UNIT_RUN_NOT_AWAITING", "state="+string(run.State), fmt.Errorf("the work is %s, not waiting for judgement, so it takes no follow-up", run.State))
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return coded("UNIT_FOLLOW_UP_MISSING", "", fmt.Errorf("the follow-up brief %s is missing or empty", path))
	}
	return nil
}

func readOutputs(manager *Manager, round UnitRound) []string {
	var paths []string
	for _, step := range round.Steps {
		if strings.HasPrefix(step.Name, "read") && step.LaunchID != "" {
			if record, err := manager.Store.Read(step.LaunchID); err == nil {
				for _, output := range record.Outputs {
					paths = append(paths, output.Path)
				}
			}
		}
	}
	return paths
}

func (runner *UnitRunner) root() string {
	if runner.Root != "" {
		return runner.Root
	}
	launchRoot, _ := runner.Manager.Store.root()
	return filepath.Join(filepath.Dir(launchRoot), "unit")
}
func (runner *UnitRunner) runDir(id string) string { return filepath.Join(runner.root(), id) }
func (runner *UnitRunner) save(record UnitRunRecord) error {
	if err := runner.reserveTree(record); err != nil {
		return err
	}
	if err := writeUnitJSON(filepath.Join(runner.runDir(record.ID), "run.json"), record, runner.root()); err != nil {
		return err
	}
	if runner.AfterWrite != nil {
		return runner.AfterWrite(record)
	}
	return nil
}

// Status reads one unit run's record without advancing it.
func (runner *UnitRunner) Status(id string) (UnitRunRecord, error) { return runner.read(id) }

func (runner *UnitRunner) read(id string) (UnitRunRecord, error) {
	if !idPattern.MatchString(id) {
		return UnitRunRecord{}, fmt.Errorf("invalid unit run id %q", id)
	}
	data, err := os.ReadFile(filepath.Join(runner.runDir(id), "run.json"))
	if err != nil {
		return UnitRunRecord{}, err
	}
	var record UnitRunRecord
	err = json.Unmarshal(data, &record)
	return record, err
}
func (runner *UnitRunner) lock(id string) (*os.File, error) {
	if err := os.MkdirAll(runner.runDir(id), 0o700); err != nil {
		return nil, err
	}
	held, err := lock.File(filepath.Join(runner.runDir(id), ".lock"), 0o600, lock.TryExclusive)
	if err != nil {
		if !isLockFailure(err) {
			return nil, err
		}
		if !lock.Busy(err) {
			return nil, coded("UNIT_LOCK_FAILED", "run="+id, fmt.Errorf("run %s cannot be locked for this command: %w", id, err))
		}
		return nil, coded("UNIT_RUN_BUSY", "run="+id, errors.New("another command is advancing this work; run the same command again to follow it"))
	}
	if runner.tree != nil {
		runner.tree.files = append(runner.tree.files, held.File())
	}
	return held.File(), nil
}

// isLockFailure tells a flock refusal from a failure to open the lock file.
func isLockFailure(err error) bool {
	var lockErr *lock.LockError
	return errors.As(err, &lockErr)
}

// releaseUnitLock ends a unit's named or run lock explicitly, then closes
// its descriptor. Closing alone ends the flock only when no duplicate of the
// descriptor remains, and a child forked concurrently holds one until it
// execs, so the next caller would be refused as busy after this one returned.
func releaseUnitLock(file *os.File) {
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
	_ = file.Close()
}
func choose(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// prepareReadOutputDirectories recreates a declared read output's missing
// directory before a read starts, as a private directory of this user. A
// temporary directory may be cleaned between a run's rounds; an existing
// directory is left as it is.
func prepareReadOutputDirectories(outputs []string) error {
	for _, output := range outputs {
		directory := filepath.Dir(output)
		// A unit read's findings directory is a registered temporary store
		// (Part B R1): emptied in place, or made again through the registry
		// when it is gone, never removed or recreated by hand, so its
		// recorded identity always names it (Round D3).
		if owner, ok := diskstore.UnitReadFindingsOwner(directory); ok {
			if err := diskstore.PrepareTempStore(context.Background(), directory, diskstore.UnitReadFindingsClass, owner); err != nil {
				return coded("UNIT_READ_OUTPUT_UNSAFE", "directory="+directory, fmt.Errorf("the read's output directory %s cannot be made private: %w", directory, err))
			}
			continue
		}
		if _, err := os.Lstat(directory); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return coded("UNIT_READ_OUTPUT_UNSAFE", "directory="+directory, fmt.Errorf("the read's output directory %s cannot be made private: %w", directory, err))
		}
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return coded("UNIT_READ_OUTPUT_UNSAFE", "directory="+directory, fmt.Errorf("the read's output directory %s cannot be made private: %w", directory, err))
		}
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return coded("UNIT_READ_OUTPUT_UNSAFE", "directory="+directory, fmt.Errorf("the read's output directory %s is not private to this user", directory))
		}
		if owner, ok := info.Sys().(*syscall.Stat_t); !ok || int(owner.Uid) != os.Getuid() || info.Mode().Perm()&0o077 != 0 {
			return coded("UNIT_READ_OUTPUT_UNSAFE", "directory="+directory, fmt.Errorf("the read's output directory %s belongs to another user", directory))
		}
	}
	return nil
}
