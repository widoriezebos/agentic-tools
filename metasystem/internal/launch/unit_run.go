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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
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
	ID            string      `json:"id"`
	Unit          string      `json:"unit"`
	Goal          string      `json:"goal"`
	Worktree      string      `json:"worktree"`
	Base          string      `json:"base"`
	Plan          string      `json:"plan"`
	PlanDirectory string      `json:"planDirectory"`
	State         string      `json:"state"`
	Rounds        []UnitRound `json:"rounds"`
}

type UnitRound struct {
	Number     int        `json:"number"`
	Directory  string     `json:"directory"`
	FollowUp   string     `json:"followUp"`
	Outcome    string     `json:"outcome"`
	BuildModel string     `json:"buildModel"`
	ReadModel  string     `json:"readModel"`
	Steps      []UnitStep `json:"steps"`
}

type UnitStep struct {
	Name          string        `json:"name"`
	LaunchID      string        `json:"launchId"`
	State         UnitStepState `json:"state"`
	Reason        string        `json:"reason"`
	StartedAt     string        `json:"startedAt"`
	FinishedAt    string        `json:"finishedAt"`
	Model         string        `json:"model"`
	Mode          string        `json:"mode,omitempty"`
	Verdict       string        `json:"verdict,omitempty"`
	VerdictCounts *bool         `json:"verdictCounts,omitempty"`
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
	Manager    *Manager
	Git        GitRunner
	Root       string
	AfterWrite func(UnitRunRecord) error
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
		if err = runner.admitRound(plan, plan.Build.Brief, nil); err != nil {
			return UnitResult{}, err
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
		plan, err = readUnitPlan(record.Plan, record.PlanDirectory)
		if err != nil {
			return UnitResult{}, err
		}
	}
	lock, err := runner.lock(record.ID)
	if err != nil {
		return UnitResult{}, err
	}
	defer lock.Close()
	if request.Resume != "" {
		record, err = runner.read(record.ID)
		if err != nil {
			return UnitResult{}, err
		}
	}
	if request.FollowUp != "" {
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
	if err := runner.Manager.Admit(StartSpec{Kind: "build", Goal: plan.Goal, Tag: plan.Unit, WorkingDirectory: plan.Worktree,
		Brief: buildBrief, Inputs: buildInputs, Outputs: plan.Build.Outputs, UnitsPage: plan.Build.UnitsPage, Units: plan.Build.Units}); err != nil {
		return err
	}
	readInputs := append(append([]string{}, plan.Read.Inputs...), previous...)
	if buildBrief != plan.Build.Brief {
		readInputs = append(readInputs, buildBrief)
	}
	return runner.Manager.Admit(StartSpec{Kind: "read", Goal: plan.Goal, Tag: plan.Unit, WorkingDirectory: plan.Worktree,
		Brief: plan.Read.Brief, Inputs: readInputs, Outputs: plan.Read.Outputs, Model: plan.Read.Model})
}

func (runner *UnitRunner) newRun(plan UnitPlan) (UnitRunRecord, error) {
	id, err := newID(runner.Manager.Now())
	if err != nil {
		return UnitRunRecord{}, err
	}
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
	record := UnitRunRecord{ID: id, Unit: plan.Unit, Goal: plan.Goal, Worktree: plan.Worktree, Base: plan.Base, Plan: copyPath, PlanDirectory: filepath.Dir(plan.Path), State: "running"}
	round := UnitRound{Number: 1, Directory: filepath.Join(dir, "round-1"), BuildModel: settings.BuildModel, ReadModel: choose(plan.Read.Model, settings.ReadModel)}
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
	record.Rounds = append(record.Rounds, UnitRound{Number: number, Directory: directory, FollowUp: target,
		BuildModel: settings.BuildModel, ReadModel: choose(plan.Read.Model, settings.ReadModel), Steps: []UnitStep{{Name: "build", State: StepPending, Model: settings.BuildModel}}})
	return runner.save(*record)
}

func (runner *UnitRunner) advanceRunning(record *UnitRunRecord, plan UnitPlan, deadline time.Time) (UnitResult, error) {
	round := &record.Rounds[len(record.Rounds)-1]
	build := &round.Steps[0]
	brief, previous := plan.Build.Brief, []string(nil)
	if round.FollowUp != "" {
		brief = round.FollowUp
		previous = readOutputs(runner.Manager, record.Rounds[len(record.Rounds)-2])
	}
	buildSpec := StartSpec{Kind: "build", Goal: plan.Goal, Tag: plan.Unit, WorkingDirectory: plan.Worktree, Brief: brief,
		Inputs: append(append([]string{}, plan.Build.Inputs...), previous...), Outputs: plan.Build.Outputs, UnitsPage: plan.Build.UnitsPage, Units: plan.Build.Units}
	if capped, err := runner.advanceStep(record, round, 0, buildSpec, deadline); err != nil || capped {
		return runner.result(*record, round, build, capped), err
	}
	if build.State != StepPassed {
		if len(round.Steps) == 1 {
			for _, command := range proofCommands(plan) {
				round.Steps = append(round.Steps, UnitStep{Name: "proof:" + command.Name, State: StepSkipped, Reason: "build-failed"})
			}
			round.Steps = append(round.Steps, UnitStep{Name: "read", State: StepSkipped, Reason: "build-failed", Model: round.ReadModel})
		}
		return runner.finish(record, round, "build-failed")
	}
	if len(round.Steps) == 1 {
		for _, command := range proofCommands(plan) {
			round.Steps = append(round.Steps, UnitStep{Name: "proof:" + command.Name, State: StepPending})
		}
		if err := runner.save(*record); err != nil {
			return UnitResult{}, err
		}
	}
	if err := proofMayStart(*round); err != nil {
		return UnitResult{}, err
	}
	commands := proofCommands(plan)
	before, err := runner.savedRepositorySnapshot(round.Directory, "proof-before.json", plan.Worktree)
	if err != nil {
		return UnitResult{}, err
	}
	red := false
	for offset, command := range commands {
		index := 1 + offset
		briefPath := filepath.Join(round.Directory, "proof-"+command.Name+".json")
		if _, err := os.Stat(briefPath); os.IsNotExist(err) {
			data, _ := json.Marshal(PlainBrief{command.Argv, command.Dir, command.Env})
			if _, err := atomicfile.WriteText(briefPath, string(data)+"\n", runner.runDir(record.ID)); err != nil {
				return UnitResult{}, err
			}
		}
		spec := StartSpec{Kind: "proof", Goal: plan.Goal, Tag: plan.Unit, WorkingDirectory: command.Dir, Brief: briefPath}
		if capped, err := runner.advanceStep(record, round, index, spec, deadline); err != nil || capped {
			return runner.result(*record, round, &round.Steps[index], capped), err
		}
		red = red || round.Steps[index].State != StepPassed
	}
	after, err := runner.savedRepositorySnapshot(round.Directory, "proof-after.json", plan.Worktree)
	if err != nil {
		return UnitResult{}, err
	}
	moved := before.changed(after)
	diffPath := filepath.Join(round.Directory, "worktree.diff")
	if _, err := os.Stat(diffPath); os.IsNotExist(err) {
		if err := runner.writeDiff(plan.Worktree, plan.Base, diffPath); err != nil {
			return UnitResult{}, err
		}
	}
	readStart, err := runner.appendReadSteps(record, round, diffPath)
	if err != nil {
		return UnitResult{}, err
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
	readInputs := append(append(append([]string{}, plan.Read.Inputs...), previous...), round.FollowUp)
	if round.FollowUp == "" {
		readInputs = readInputs[:len(readInputs)-1]
	}
	for index := readStart; index < len(round.Steps); index++ {
		step := &round.Steps[index]
		if step.State == StepPending || step.State == StepStarting {
			packageName := strings.TrimPrefix(step.Name, "read:")
			if step.Name == "read" {
				packageName = ""
			}
			spec := StartSpec{Kind: "read", Goal: plan.Goal, Tag: plan.Unit, WorkingDirectory: plan.Worktree, Brief: plan.Read.Brief,
				Inputs: readInputs, Outputs: plan.Read.Outputs, Model: plan.Read.Model, DiffFile: diffPath, Package: packageName, Wide: step.Mode == "wide"}
			if _, err := runner.startStep(record, round, index, spec); err != nil {
				return UnitResult{}, err
			}
		}
	}
	for index := readStart; index < len(round.Steps); index++ {
		step := &round.Steps[index]
		if capped, err := runner.waitStep(record, round, index, deadline); err != nil || capped {
			return runner.result(*record, round, step, capped), err
		}
	}
	for index := readStart; index < len(round.Steps); index++ {
		if round.Steps[index].State != StepPassed {
			return runner.finish(record, round, "read-failed")
		}
	}
	return runner.finish(record, round, "green")
}

func (runner *UnitRunner) appendReadSteps(record *UnitRunRecord, round *UnitRound, diffPath string) (int, error) {
	readStart := len(round.Steps)
	for index := 1; index < len(round.Steps); index++ {
		if strings.HasPrefix(round.Steps[index].Name, "read") {
			return index, nil
		}
	}
	settings, err := runner.Manager.resolvedSettings()
	if err != nil {
		return 0, err
	}
	choice, err := ChooseReadMode(diffPath, settings.ReadSplitLines)
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
			round.Steps = append(round.Steps, UnitStep{Name: "read:" + directory, State: StepPending, Model: round.ReadModel, Mode: "package"})
		}
	} else {
		round.Steps = append(round.Steps, UnitStep{Name: "read", State: StepPending, Model: round.ReadModel, Mode: choice.Mode})
	}
	return readStart, runner.save(*record)
}

func (runner *UnitRunner) advanceStep(record *UnitRunRecord, round *UnitRound, index int, spec StartSpec, deadline time.Time) (bool, error) {
	if _, err := runner.startStep(record, round, index, spec); err != nil {
		return false, err
	}
	return runner.waitStep(record, round, index, deadline)
}

func (runner *UnitRunner) startStep(record *UnitRunRecord, round *UnitRound, index int, spec StartSpec) (Record, error) {
	step := &round.Steps[index]
	if step.State == StepPending {
		step.LaunchID = fmt.Sprintf("%s-r%d-s%d", record.ID, round.Number, index+1)
		step.State, step.StartedAt = StepStarting, runner.Manager.Now().UTC().Format(time.RFC3339Nano)
		if err := runner.save(*record); err != nil {
			return Record{}, err
		}
	}
	launchRecord, err := runner.Manager.Store.Read(step.LaunchID)
	if errors.Is(err, fs.ErrNotExist) {
		spec.ID = step.LaunchID
		launchRecord, err = runner.Manager.Start(spec)
	}
	if err != nil && launchRecord.ID == "" {
		return Record{}, err
	}
	if launchRecord.State.Terminal() {
		runner.endStep(record, round, index, launchRecord)
		return launchRecord, runner.save(*record)
	}
	step.State = StepRunning
	return launchRecord, runner.save(*record)
}

func (runner *UnitRunner) waitStep(record *UnitRunRecord, round *UnitRound, index int, deadline time.Time) (bool, error) {
	step := &round.Steps[index]
	if step.State == StepPassed || step.State == StepFailed {
		return false, nil
	}
	remaining := deadline.Sub(runner.Manager.Now())
	if remaining < 0 {
		remaining = 0
	}
	launchRecord, terminal, err := runner.Manager.Wait(step.LaunchID, remaining)
	if err != nil {
		return false, err
	}
	if !terminal {
		step.State = StepRunning
		if err := runner.save(*record); err != nil {
			return false, err
		}
		return true, nil
	}
	runner.endStep(record, round, index, launchRecord)
	return false, runner.save(*record)
}

func (runner *UnitRunner) endStep(record *UnitRunRecord, round *UnitRound, index int, launchRecord Record) {
	step := &round.Steps[index]
	step.State, step.Reason, step.FinishedAt = StepFailed, launchRecord.Reason, runner.Manager.Now().UTC().Format(time.RFC3339Nano)
	if launchRecord.State == Completed {
		step.State = StepPassed
	}
	if strings.HasPrefix(step.Name, "read") {
		step.Verdict, step.VerdictCounts = launchRecord.Measurement.Verdict, launchRecord.VerdictCounts
	}
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
	if err := runner.save(*record); err != nil {
		return UnitResult{}, err
	}
	return UnitResult{Record: *record, Round: round.Number}, nil
}

func (runner *UnitRunner) result(record UnitRunRecord, round *UnitRound, step *UnitStep, capped bool) UnitResult {
	return UnitResult{Record: record, Round: round.Number, Step: step.Name, Launch: step.LaunchID, Capped: capped}
}

func (runner *UnitRunner) writeDiff(worktree, base, target string) error {
	git := runner.Git
	if git == nil {
		git = OSGitRunner{}
	}
	temporary, err := os.MkdirTemp("", "metasystem-unit-diff.")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	indexPathData, err := git.Run(worktree, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return err
	}
	objectsData, err := git.Run(worktree, nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return err
	}
	index := filepath.Join(temporary, "index")
	if data, readErr := os.ReadFile(strings.TrimSpace(string(indexPathData))); readErr == nil {
		if err := os.WriteFile(index, data, 0o600); err != nil {
			return err
		}
	} else {
		return readErr
	}
	objectDir := filepath.Join(temporary, "objects")
	if err := os.MkdirAll(objectDir, 0o700); err != nil {
		return err
	}
	environment := []string{"GIT_INDEX_FILE=" + index, "GIT_OBJECT_DIRECTORY=" + objectDir, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + strings.TrimSpace(string(objectsData))}
	if _, err := git.Run(worktree, environment, "add", "-A", "--", "."); err != nil {
		return err
	}
	diff, err := git.Run(worktree, environment, "diff", "--cached", "--binary", base, "--", ".")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(target, string(diff), filepath.Dir(filepath.Dir(target)))
	return err
}

type repositorySnapshot struct {
	Head  string `json:"head"`
	Refs  string `json:"refs"`
	Index string `json:"index"`
	Tree  string `json:"tree"`
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
	if data, err := os.ReadFile(path); err == nil {
		var snapshot repositorySnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return repositorySnapshot{}, err
		}
		return snapshot, nil
	} else if !os.IsNotExist(err) {
		return repositorySnapshot{}, err
	}
	snapshot, err := runner.snapshotRepository(worktree)
	if err != nil {
		return repositorySnapshot{}, err
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
	git := runner.Git
	if git == nil {
		git = OSGitRunner{}
	}
	head, err := git.Run(worktree, nil, "rev-parse", "HEAD")
	if err != nil {
		return repositorySnapshot{}, err
	}
	refs, err := git.Run(worktree, nil, "for-each-ref", "--format=%(refname) %(objectname)")
	if err != nil {
		return repositorySnapshot{}, err
	}
	indexPath, err := git.Run(worktree, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return repositorySnapshot{}, err
	}
	indexData, err := os.ReadFile(strings.TrimSpace(string(indexPath)))
	if err != nil {
		return repositorySnapshot{}, err
	}
	objectsPath, err := git.Run(worktree, nil, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return repositorySnapshot{}, err
	}
	temporary, err := os.MkdirTemp("", "metasystem-unit-snapshot.")
	if err != nil {
		return repositorySnapshot{}, err
	}
	defer os.RemoveAll(temporary)
	index := filepath.Join(temporary, "index")
	if err := os.WriteFile(index, indexData, 0o600); err != nil {
		return repositorySnapshot{}, err
	}
	objectDir := filepath.Join(temporary, "objects")
	if err := os.MkdirAll(objectDir, 0o700); err != nil {
		return repositorySnapshot{}, err
	}
	environment := []string{"GIT_INDEX_FILE=" + index, "GIT_OBJECT_DIRECTORY=" + objectDir, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + strings.TrimSpace(string(objectsPath))}
	if _, err := git.Run(worktree, environment, "add", "-A", "--", "."); err != nil {
		return repositorySnapshot{}, err
	}
	tree, err := git.Run(worktree, environment, "diff", "--cached", "--raw", "HEAD", "--", ".")
	if err != nil {
		return repositorySnapshot{}, err
	}
	indexDigest := sha256.Sum256(indexData)
	return repositorySnapshot{Head: string(head), Refs: string(refs), Index: fmt.Sprintf("%x", indexDigest), Tree: string(tree)}, nil
}

func proofMayStart(round UnitRound) error {
	if len(round.Steps) == 0 || round.Steps[0].Name != "build" || round.Steps[0].State != StepPassed {
		return fmt.Errorf("proof requires the build launch to be completed")
	}
	return nil
}

func admitFollowUp(run UnitRunRecord, path string) error {
	if run.State != "awaiting-judgement" {
		return fmt.Errorf("UNIT_RUN_NOT_AWAITING state=%s", run.State)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("UNIT_FOLLOW_UP_MISSING")
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
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if _, err = atomicfile.WriteText(filepath.Join(runner.runDir(record.ID), "run.json"), string(data)+"\n", runner.root()); err != nil {
		return err
	}
	if runner.AfterWrite != nil {
		return runner.AfterWrite(record)
	}
	return nil
}
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
	file, err := os.OpenFile(filepath.Join(runner.runDir(id), ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("UNIT_RUN_BUSY")
	}
	return file, nil
}
func choose(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
