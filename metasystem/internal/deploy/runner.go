package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// Git is the runner's way to the repository: origin's main as fetched, and
// the clean trees every operation runs in, so the checkout that triggered a
// run is never built in.
type Git struct {
	// FetchMain fetches origin's main and returns its tip.
	FetchMain func() (string, error)
	// AddTree checks commit out, detached, at dir.
	AddTree func(dir, commit string) error
	// RemoveTree removes the tree at dir, whether this checkout registered
	// it or another run left it behind.
	RemoveTree func(dir string) error
}

// Runner deploys origin's main for one project. Every trigger starts it and
// none carries a commit: its target is always main's tip as it fetches it,
// landed by construction and never behind what is active.
type Runner struct {
	Dir     string
	Project string
	// Installation is where the installation lies in a tree of the
	// repository ("." at its root): deploy.json is read there in the clean
	// tree of the commit an adapter call is about, never in a working tree.
	Installation string
	Git          Git
	By           string
	Now          func() time.Time
	// Starting, when set, is told each adapter call that holds the lock as
	// run.json records it, before the pause is checked and its process
	// starts; Started, when set, is told each such process as it starts.
	Starting, Started func(Active)
	// acting is set on the runner of a person's rollback: it holds the lock
	// while deploys are paused, and its calls run all the same.
	acting bool
	// trees are the clean trees of the commits this run's or act's adapter
	// calls are about, each with the contract read from it.
	trees map[string]*landedTree
}

type landedTree struct {
	source   string
	contract Contract
	release  func()
}

// The outcomes of a run.
const (
	RunBusy     = "busy"     // another run holds the lock and deploys main's newest tip before it ends
	RunPaused   = "paused"   // deploys are paused; nothing was built
	RunCurrent  = "current"  // main's tip is already active; nothing was built or written
	RunDeployed = "deployed" // main's tip is active now
	RunStopped  = "stopped"  // a pause stopped the run in progress
	RunFailed   = "failed"   // the attempt on main's tip failed; its line says why
)

// Report is what one run did.
type Report struct {
	Outcome string `json:"outcome"`
	Tip     string `json:"tip,omitempty"`
	// Lines are the attempts this run made; Recorded are the lines it wrote
	// to bring the record in line with what the adapter reports active.
	Lines    []Line  `json:"lines,omitempty"`
	Recorded []Line  `json:"recorded,omitempty"`
	Holder   *Active `json:"holder,omitempty"`
	// Orphaned is a Holder whose run died and left its adapter running: no
	// one deploys main's newest tip after it, and a pause ends it.
	Orphaned bool `json:"orphaned,omitempty"`
}

// Refusal is an act the record does not allow, with its code.
type Refusal struct {
	Code   string
	Reason string
}

func (r *Refusal) Error() string { return r.Code + ": " + r.Reason }

// Run deploys main's tip unless it is current, paused, or already tried by
// this run. A tip is tried once per run: a failed attempt ends the run, and
// the next push or deploy now tries again. After it releases the lock, and
// only when not paused, it fetches main once more and starts over when the
// tip is neither current nor tried, so a trigger that found the lock held
// just before the release is not lost.
func (r *Runner) Run() (Report, error) {
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return Report{}, err
	}
	tried := map[string]bool{}
	var report Report
	for {
		again, outcome, err := r.pass(tried, &report)
		report.Outcome = outcome
		if count := len(report.Lines); count > 0 {
			switch report.Lines[count-1].Outcome {
			case OutcomeActive:
				report.Outcome = RunDeployed
			case OutcomeStopped:
				report.Outcome = RunStopped
			default:
				report.Outcome = RunFailed
			}
		}
		if err != nil || !again {
			return report, err
		}
	}
}

func (r *Runner) pass(tried map[string]bool, report *Report) (again bool, outcome string, err error) {
	if paused, err := r.paused(); err != nil || paused {
		return false, RunPaused, err
	}
	held, err := lock.File(lockPath(r.Dir), 0o600, lock.TryExclusive)
	if lock.Busy(err) {
		if active, ok, _ := ReadActive(r.Dir); ok {
			report.Holder = &active
		}
		return false, RunBusy, nil
	}
	if err != nil {
		return false, "", err
	}
	defer held.Release()
	if leftover, ok, _ := ReadActive(r.Dir); ok && leftover.AdapterAlive() {
		// A run that died left its adapter running: it is the run in
		// progress until it ends, so no second one starts beside it.
		report.Holder, report.Orphaned = &leftover, true
		return false, RunBusy, nil
	}
	log, err := r.logPath()
	if err != nil {
		return false, "", err
	}
	tip, err := r.Git.FetchMain()
	if err != nil {
		return false, "", fmt.Errorf("origin's main can't be fetched: %w", err)
	}
	defer r.unland()
	if err := r.clear(); err != nil {
		return false, "", fmt.Errorf("%w, so nothing was deployed", err)
	}
	written := len(report.Lines) + len(report.Recorded)
	current, asked, ended, err := r.reconcile(tip, log, report)
	if err != nil {
		return false, "", fmt.Errorf("%w, so nothing was deployed", err)
	}
	if ended != nil {
		// version did not tell what is active: its line ends this tip's
		// attempt, and the next trigger asks again.
		report.Tip, tried[tip] = tip, true
		report.Lines = append(report.Lines, *ended)
	}
	for owed := asked.state == stateDone; ended == nil; owed = false {
		report.Tip = tip
		// Once version answered, the attempt owes its line: a pause is told
		// by the stopped line of its build, which checks the pause before it
		// starts. Before the question, or once an attempt wrote its line, a
		// pause ends the run with no line.
		if paused, err := r.paused(); err != nil || paused && !owed {
			if err != nil {
				return false, "", err
			}
			outcome = RunPaused
			break
		}
		if current != nil && current.Commit == tip {
			outcome = RunCurrent
			break
		}
		if tried[tip] {
			outcome = RunFailed
			break
		}
		tried[tip] = true
		line, err := r.attempt(tip, current, log)
		if err != nil {
			return false, "", err
		}
		report.Lines = append(report.Lines, line)
		if line.Outcome != OutcomeActive {
			outcome = RunFailed
			break
		}
		current = &line
		if tip, err = r.Git.FetchMain(); err != nil {
			return false, "", fmt.Errorf("origin's main can't be fetched: %w", err)
		}
	}
	if len(report.Lines)+len(report.Recorded) == written {
		// A pass that wrote no line keeps no log: asking what is active
		// is a read.
		_ = removeFile(log)
	}
	r.unland()
	if err := held.Release(); err != nil {
		return false, outcome, err
	}
	if paused, err := r.paused(); err != nil || paused {
		return false, outcome, err
	}
	next, err := r.Git.FetchMain()
	if err != nil {
		// The work is done; the next trigger fetches again.
		return false, outcome, nil
	}
	return (current == nil || next != current.Commit) && !tried[next], outcome, nil
}

// reconcile asks the adapter of the current deploy, or of main's tip while
// nothing is current, what is active and brings the record in line with it:
// an activation or rollback interrupted before its line was written is
// recorded now, and so is a current deploy that is active no more. It
// returns the record's current deploy and what version answered. When
// version does not tell what is active, it appends the line that ends the
// run or the act, as ended: stopped when a pause ended the question,
// version-failed otherwise. A run's question that a pause kept from
// starting ends with no line.
func (r *Runner) reconcile(tip, log string, report *Report) (*Line, answer, *Line, error) {
	lines, err := Lines(r.Dir)
	if err != nil {
		return nil, answer{}, nil, err
	}
	var pending Line
	interrupted, _ := readJSON(pendingPath(r.Dir), &pending)
	current := Current(lines)
	about := tip
	if current != nil {
		about = current.Commit
	}
	asked := r.Now()
	active := r.call("version", about, "", current, log, &Active{Kind: KindDeploy, Commit: tip})
	if err := removeFile(activePath(r.Dir)); err != nil {
		return nil, answer{}, nil, err
	}
	if active.state == statePaused {
		return nil, active, nil, nil
	}
	if !active.done(OutcomeNone) && !active.done(OutcomeActive) {
		ended := Line{Kind: KindDeploy, Commit: tip, Previous: CommitRef(commitOf(current)), By: r.By, StartedAt: asked, EndedAt: r.Now(),
			Outcome: OutcomeVersionFailed, Log: log, Detail: "what is active can't be learned: " + active.describe()}
		// A run's call is ended by a pause; a rollback's, which runs while
		// paused, by a person's pause ending its process group.
		switch pause, paused, _ := ReadPause(r.Dir); {
		case paused && !r.acting:
			ended.Outcome, ended.Detail = OutcomeStopped, "deploy pause by "+pause.By+" stopped the deploy of "+Short(tip)+": "+active.describe()
		case r.acting && active.state == stateUnknown && active.exit < 0:
			ended.Outcome = OutcomeStopped
		}
		return nil, active, &ended, appendLine(r.Dir, ended)
	}
	// pending.json goes only once the record holds what it may name, so a
	// run that dies between the two leaves it for the next.
	settled := func(current *Line) (*Line, answer, *Line, error) {
		if err := removeFile(pendingPath(r.Dir)); err != nil {
			return nil, answer{}, nil, err
		}
		return current, active, nil, nil
	}
	switch {
	case active.done(OutcomeNone) && current != nil:
		// version is the truth: nothing is current until an active line.
		now := r.Now()
		none := Line{Kind: current.Kind, Commit: current.Commit, Previous: CommitRef(current.Commit), By: r.By, StartedAt: now, EndedAt: now,
			Outcome: OutcomeNone, Log: log, Detail: "the adapter's version reports nothing active, so " + Short(current.Commit) + " is active no more"}
		if err := appendLine(r.Dir, none); err != nil {
			return nil, answer{}, nil, err
		}
		report.Recorded = append(report.Recorded, none)
		return settled(nil)
	case active.done(OutcomeNone):
		return settled(nil)
	case current != nil && current.Artifact == active.Artifact && current.Digest == active.Digest:
		return settled(current)
	}
	// What is active is not the record's current deploy: the attempt whose
	// run died between its activation and its line, or a deploy the record
	// holds (an older one active again is a return to it).
	found := Line{Previous: CommitRef(commitOf(current))}
	known := interrupted && pending.Artifact == active.Artifact && pending.Digest == active.Digest
	if known {
		found.Kind, found.Commit, found.Previous = pending.Kind, pending.Commit, pending.Previous
	}
	for index := len(lines) - 1; index >= 0 && !known; index-- {
		if lines[index].Artifact == active.Artifact && lines[index].Digest == active.Digest {
			found.Kind, found.Commit, known = lines[index].Kind, lines[index].Commit, true
			if lines[index].Outcome == OutcomeActive {
				found.Kind = KindRollback
			}
		}
	}
	if !known {
		// An artifact the record never saw: main's tip is deployed over it.
		return settled(nil)
	}
	now := r.Now()
	found.Version, found.Artifact, found.Digest, found.By = active.Version, active.Artifact, active.Digest, r.By
	found.StartedAt, found.EndedAt, found.Outcome, found.Log = now, now, OutcomeActive, log
	found.Detail = "the adapter reports this deploy active and the record did not hold it: its run ended before it wrote its line"
	if err := appendLine(r.Dir, found); err != nil {
		return nil, answer{}, nil, err
	}
	report.Recorded = append(report.Recorded, found)
	return settled(&found)
}

// attempt builds, activates and verifies tip in its clean tree, and
// appends its one line.
func (r *Runner) attempt(tip string, current *Line, log string) (Line, error) {
	line := Line{Kind: KindDeploy, Commit: tip, Previous: CommitRef(commitOf(current)), By: r.By, StartedAt: r.Now(), Log: log}
	finish := func(outcome, detail string) (Line, error) {
		line.Outcome, line.Detail, line.EndedAt = outcome, detail, r.Now()
		if err := appendLine(r.Dir, line); err != nil {
			return line, err
		}
		return line, removeFile(pendingPath(r.Dir))
	}
	stoppedOr := func(outcome, detail string) (Line, error) {
		if pause, paused, _ := ReadPause(r.Dir); paused {
			return finish(OutcomeStopped, "deploy pause by "+pause.By+" stopped the deploy of "+Short(tip)+": "+detail)
		}
		return finish(outcome, detail)
	}
	if _, err := r.treeOf(tip); err != nil {
		return finish(OutcomeBuildFailed, err.Error())
	}
	active := Active{Kind: KindDeploy, Commit: tip}
	built := r.call("build", tip, "", current, log, &active)
	if !built.done("built") {
		return stoppedOr(OutcomeBuildFailed, built.describe())
	}
	line.Version, line.Artifact, line.Digest = built.Version, built.Artifact, built.Digest
	// From here what is active may change before the line is written.
	if err := writeJSON(pendingPath(r.Dir), line); err != nil {
		return finish(OutcomeActivateFailed, "nothing was activated: the attempt can't be kept while it activates: "+err.Error())
	}
	activated := r.call("activate", tip, built.Artifact, current, log, &active)
	if activated.state == stateFailed || activated.state == stateUnsupported || activated.state == statePaused {
		return stoppedOr(OutcomeActivateFailed, activated.describe())
	}
	verified := r.call("version", tip, "", current, log, &Active{Kind: KindDeploy, Commit: tip})
	if verified.done(OutcomeActive) && verified.Artifact == line.Artifact && verified.Digest == line.Digest {
		detail := ""
		if !activated.done(OutcomeActive) {
			detail = activated.describe() + "; version reports it active"
		}
		return finish(OutcomeActive, detail)
	}
	if paused, _ := r.paused(); paused {
		return stoppedOr(OutcomeStopped, "the pause asks version what is active")
	}
	if !activated.done(OutcomeActive) && verified.state == stateDone {
		return finish(OutcomeActivateFailed, activated.describe()+"; version reports "+reported(verified))
	}
	// Activated, but version does not report it: roll back, and pause so the
	// next landing does not deploy the same commit again.
	detail := "version did not report " + Short(tip) + " active after its activation: " + reported(verified)
	if current != nil && current.Artifact != "" {
		back := r.call("rollback", current.Commit, "", current, log, &Active{Kind: KindRollback, Commit: current.Commit})
		switch {
		case back.done(OutcomeActive):
			detail += "; rolled back to " + Short(current.Commit)
		default:
			detail += "; the rollback to " + Short(current.Commit) + " failed too: " + back.describe()
		}
	} else {
		detail += "; nothing was active before, so nothing was rolled back"
	}
	if err := writePause(r.Dir, Pause{By: "the deploy runner", At: r.Now(), Reason: detail}); err != nil {
		detail += "; the pause could not be written: " + err.Error()
	}
	return finish(OutcomeVerifyFailed, detail)
}

// clear removes the clean trees a dead run left behind; the caller holds
// the lock.
func (r *Runner) clear() error {
	r.unland()
	if entries, err := os.ReadDir(workDir(r.Dir)); err == nil {
		for _, entry := range entries {
			if err := r.Git.RemoveTree(filepath.Join(workDir(r.Dir), entry.Name())); err != nil {
				return fmt.Errorf("the leftover clean tree %s can't be removed: %w", entry.Name(), err)
			}
		}
	}
	return nil
}

// treeOf is the clean tree of commit, which every adapter call about commit
// runs in by the deploy.json read from it, so a landing's adapter never
// answers for a deploy an older commit made. Each commit's tree is made
// once, under the lock, and unland removes it.
func (r *Runner) treeOf(commit string) (*landedTree, error) {
	if tree := r.trees[commit]; tree != nil {
		return tree, nil
	}
	tree, err := r.checkout(filepath.Join(workDir(r.Dir), commit), commit)
	if err != nil {
		return nil, err
	}
	if r.trees == nil {
		r.trees = map[string]*landedTree{}
	}
	r.trees[commit] = tree
	return tree, nil
}

// checkout makes the clean tree of commit at dir and reads the deploy
// contract from it.
func (r *Runner) checkout(dir, commit string) (*landedTree, error) {
	source, release, err := r.tree(dir, commit)
	if err != nil {
		return nil, fmt.Errorf("the clean tree of %s can't be made: %w", Short(commit), err)
	}
	contract, _, err := LoadContract(filepath.Join(source, filepath.FromSlash(r.Installation)))
	if err != nil {
		release()
		return nil, fmt.Errorf("the deploy contract of %s can't be read: %w", Short(commit), err)
	}
	return &landedTree{source: source, contract: contract, release: release}, nil
}

// unland removes the clean trees this run or act made.
func (r *Runner) unland() {
	for commit, tree := range r.trees {
		tree.release()
		delete(r.trees, commit)
	}
}

func (r *Runner) request(operation, commit, source, artifact string, previous *Line) Request {
	request := Request{Schema: 1, Operation: operation, Project: r.Project, Commit: commit, Source: source, Artifact: artifact}
	if previous != nil {
		request.Previous = &Deployed{Commit: previous.Commit, Version: previous.Version, Artifact: previous.Artifact, Digest: previous.Digest}
	}
	return request
}

// tree makes the detached clean tree of commit at dir and returns its
// removal.
func (r *Runner) tree(dir, commit string) (string, func(), error) {
	if _, err := os.Lstat(dir); err == nil {
		if err := r.Git.RemoveTree(dir); err != nil {
			return "", nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return "", nil, err
	}
	if err := r.Git.AddTree(dir, commit); err != nil {
		_ = r.Git.RemoveTree(dir)
		return "", nil, err
	}
	return dir, func() { _ = r.Git.RemoveTree(dir) }, nil
}

func (r *Runner) logPath() (string, error) {
	if err := os.MkdirAll(logDir(r.Dir), 0o700); err != nil {
		return "", err
	}
	name := r.Now().UTC().Format("20060102T150405.000000000Z") + "-" + strconv.Itoa(os.Getpid()) + ".log"
	return filepath.Join(logDir(r.Dir), name), nil
}

func (r *Runner) paused() (bool, error) {
	_, paused, err := ReadPause(r.Dir)
	return paused, err
}

func commitOf(line *Line) string {
	if line == nil {
		return ""
	}
	return line.Commit
}

func reported(a answer) string {
	if a.done(OutcomeActive) {
		return "version " + a.Version + " (" + a.Artifact + ")"
	}
	return a.describe()
}
