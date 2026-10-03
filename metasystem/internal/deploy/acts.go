package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// The outcomes of a rollback.
const (
	RollbackDone   = "rolled-back" // the deploy before the newest forward one is active again
	RollbackHolds  = "holds"       // it already was; nothing was written
	RollbackFailed = "failed"      // the attempt failed; its line says why
)

// RollbackReport is what a rollback did.
type RollbackReport struct {
	Outcome string `json:"outcome"`
	Target  Line   `json:"target"`
	Line    *Line  `json:"line,omitempty"`
	// Recorded are the lines that brought the record in line with what
	// version reported before the rollback decided.
	Recorded []Line `json:"recorded,omitempty"`
}

// Rollback activates the deploy that was current before the newest forward
// deploy, without building, and pauses: otherwise the next landing would
// deploy the bad commit again, since it is still in main. It never moves
// forward; only a person's deploy resume does. It never runs beside a run:
// it refuses while one holds the lock, and holding it, decides only once
// the record agrees with what version reports. A refusal pauses nothing and
// starts nothing.
func (r *Runner) Rollback() (report RollbackReport, err error) {
	held, err := r.lockFor("rollback")
	if err != nil {
		return report, err
	}
	defer held.Release()
	// The rollback's calls run while deploys are paused: it pauses them.
	actor := *r
	actor.acting = true
	defer actor.unland()
	tip, err := r.Git.FetchMain()
	if err != nil {
		return report, fmt.Errorf("origin's main can't be fetched: %w", err)
	}
	if err := actor.clear(); err != nil {
		return report, err
	}
	log, err := actor.logPath()
	if err != nil {
		return report, err
	}
	var reconciled Report
	_, _, ended, err := actor.reconcile(tip, log, &reconciled)
	report.Recorded = reconciled.Recorded
	if err == nil && ended != nil {
		// Its line is written; the rollback fails, since it can't decide.
		err = errors.New(ended.Detail)
	}
	if err != nil {
		return report, err
	}
	if len(report.Recorded) == 0 {
		// A rollback that writes no line keeps no log: asking what is
		// active is a read.
		defer func() {
			if report.Line == nil {
				_ = removeFile(log)
			}
		}()
	}
	lines, err := Lines(r.Dir)
	if err != nil {
		return report, err
	}
	target, holds, err := rollbackTarget(lines)
	if err != nil {
		return report, err
	}
	report.Target = target
	if _, paused, err := ReadPause(r.Dir); err != nil {
		return report, err
	} else if !paused {
		if err := writePause(r.Dir, Pause{By: r.By, At: r.Now(), Reason: "deploy rollback"}); err != nil {
			return report, err
		}
	}
	if holds {
		report.Outcome = RollbackHolds
		return report, nil
	}
	current := Current(lines)
	line := Line{Kind: KindRollback, Commit: target.Commit, Version: target.Version, Artifact: target.Artifact, Digest: target.Digest,
		Previous: CommitRef(commitOf(current)), By: r.By, StartedAt: r.Now(), Log: log}
	report.Outcome, line.Outcome = RollbackFailed, OutcomeActivateFailed
	finish := func() (RollbackReport, error) {
		line.EndedAt = r.Now()
		report.Line = &line
		if err := appendLine(r.Dir, line); err != nil {
			return report, err
		}
		return report, removeFile(pendingPath(r.Dir))
	}
	// From here what is active may change before the line is written.
	if err := writeJSON(pendingPath(r.Dir), line); err != nil {
		line.Detail = "nothing was rolled back: the rollback can't be kept while it activates: " + err.Error()
		return finish()
	}
	back := actor.call("rollback", target.Commit, "", &target, log, &Active{Kind: KindRollback, Commit: target.Commit})
	switch {
	case back.state == stateFailed || back.state == stateUnsupported:
		line.Detail = back.describe()
	default:
		verify := func() answer {
			return actor.call("version", target.Commit, "", &target, log, &Active{Kind: KindRollback, Commit: target.Commit})
		}
		ended, verified := back, verify()
		if !actor.endedByPause(back) && actor.endedByPause(verified) {
			// A pause ended the version after the rollback: it is asked once more.
			ended, verified = verified, verify()
		}
		switch {
		case actor.endedByPause(ended):
			// A call was ended, as a person's pause ends it: the rollback
			// failed, and its line is what version then reports.
			switched(&line, ended, verified, &target)
		case verified.done(OutcomeActive) && verified.Artifact == target.Artifact && verified.Digest == target.Digest:
			report.Outcome, line.Outcome = RollbackDone, OutcomeActive
		default:
			line.Outcome, line.Detail = OutcomeVerifyFailed, "version did not report "+Short(target.Commit)+" active after the rollback: "+reported(verified)
		}
	}
	return finish()
}

// rollbackTarget is the deploy before the newest forward one: the previous
// commit of the newest active line of kind deploy, as its own active line
// recorded it. holds is true when the current deploy already is the
// rollback to it, never while nothing is active.
func rollbackTarget(lines []Line) (Line, bool, error) {
	noPrevious := func(reason string) (Line, bool, error) {
		return Line{}, false, &Refusal{Code: CodeNoPrevious, Reason: reason}
	}
	var forward *Line
	for index := len(lines) - 1; index >= 0 && forward == nil; index-- {
		if lines[index].Kind == KindDeploy && lines[index].Outcome == OutcomeActive {
			forward = &lines[index]
		}
	}
	if forward == nil || forward.Previous == "" {
		return noPrevious("nothing was deployed before the newest deploy, so there is nothing to roll back to")
	}
	var target *Line
	for index := len(lines) - 1; index >= 0 && target == nil; index-- {
		if lines[index].Commit == string(forward.Previous) && lines[index].Outcome == OutcomeActive && lines[index].Artifact != "" {
			target = &lines[index]
		}
	}
	if target == nil {
		return noPrevious("the deploy of " + Short(string(forward.Previous)) + " before the newest one is not in the record")
	}
	if current := Current(lines); current != nil && current.Kind == KindRollback && current.Commit == target.Commit {
		return *target, true, nil
	}
	if problem := artifactProblem(*target); problem != "" {
		return noPrevious("the deploy of " + Short(target.Commit) + " can't be returned to: " + problem)
	}
	return *target, false, nil
}

// artifactProblem says why a recorded artifact that is a file on this
// computer can no longer be activated: it is gone, or its sha256 checksum
// differs from the record's. Any other artifact is the adapter's to judge.
func artifactProblem(line Line) string {
	if !filepath.IsAbs(line.Artifact) {
		return ""
	}
	info, err := os.Stat(line.Artifact)
	if err != nil {
		return "its artifact " + line.Artifact + " is gone"
	}
	want := strings.TrimPrefix(line.Digest, "sha256:")
	if !info.Mode().IsRegular() || len(want) != sha256.Size*2 {
		return ""
	}
	file, err := os.Open(line.Artifact)
	if err != nil {
		return "its artifact " + line.Artifact + " can't be read: " + err.Error()
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "its artifact " + line.Artifact + " can't be read: " + err.Error()
	}
	if hex.EncodeToString(sum.Sum(nil)) != strings.ToLower(want) {
		return "its artifact " + line.Artifact + " no longer has the recorded checksum"
	}
	return ""
}

// PauseReport is what a pause did. It says nothing of what is active: a run
// it stopped may still be ending, and deploy status reads what is.
type PauseReport struct {
	// Already is true when deploys were paused before.
	Already bool `json:"already"`
	// Stopped is the adapter call in progress the pause ended.
	Stopped *Active `json:"stopped,omitempty"`
}

// Pause holds deploys until a person resumes them, and ends the adapter
// call run.json names, whoever made it, a rollback's included: its process
// group ends, and the run or rollback that made it appends its line as it
// ends, from what version then reports. It takes no lock and waits for no
// run, so a stalled one never keeps a person from stopping it.
func (r *Runner) Pause(reason string) (report PauseReport, err error) {
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return report, err
	}
	if _, report.Already, err = ReadPause(r.Dir); err != nil {
		return report, err
	}
	if !report.Already {
		if err := writePause(r.Dir, Pause{By: r.By, At: r.Now(), Reason: reason}); err != nil {
			return report, err
		}
	}
	report.Stopped, err = r.stopRun()
	return report, err
}

// lockFor takes the deploy lock for a person's rollback or resume without
// waiting. While a run or another rollback holds it, or an adapter a dead
// run left running holds the deploy, it changes nothing and refuses, naming
// deploy pause, which stops what is in progress.
func (r *Runner) lockFor(act string) (*lock.FileLock, error) {
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return nil, err
	}
	held, err := lock.File(lockPath(r.Dir), 0o600, lock.TryExclusive)
	if err != nil && !lock.Busy(err) {
		return nil, err
	}
	holder, named, _ := ReadActive(r.Dir)
	alive := named && holder.AdapterAlive()
	if err == nil && !alive {
		return held, nil
	}
	if err == nil {
		_ = held.Release()
	}
	running := "a deploy is in progress"
	if alive {
		running = fmt.Sprintf("the %s of %s is in progress (process %d)", holder.Operation, Short(holder.Commit), holder.PID)
	}
	return nil, &Refusal{Code: CodeRunning, Reason: running + "; deploy pause stops it, and deploy " + act + " can be repeated once it has stopped"}
}

// stopRun ends the process group of the adapter run.json names, when that
// very process still runs, and returns it. A call run.json names before its
// process starts is waited for until it names the process, or ends without
// one. Its run, when alive, then ends its step and appends the stopped line.
func (r *Runner) stopRun() (*Active, error) {
	active, ok, err := ReadActive(r.Dir)
	for ; err == nil && ok && active.PID == 0 && sameProcess(active.Runner, active.RunnerBorn); active, ok, err = ReadActive(r.Dir) {
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || !ok || !active.AdapterAlive() {
		return nil, err
	}
	if err := syscall.Kill(-active.PID, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return nil, err
	}
	return &active, nil
}

// Resume lifts the pause; was is the pause it lifted. It never lifts a
// pause beside a run or a rollback in progress: it refuses while one holds
// the lock. deploy resume then runs the runner as a trigger does.
func (r *Runner) Resume() (was Pause, paused bool, err error) {
	held, err := r.lockFor("resume")
	if err != nil {
		return was, false, err
	}
	defer held.Release()
	was, paused, err = ReadPause(r.Dir)
	if err != nil || !paused {
		return was, paused, err
	}
	return was, true, removeFile(pausePath(r.Dir))
}

// Verify asks the adapter of the current deploy, or of main's tip while
// nothing is current, what is active now, in a clean tree of that commit.
// It only reads, so it takes no lock and names no process in run.json: a
// trigger meanwhile still finds the lock as a run left it, and its tree
// lies outside work/, which a run clears.
func (r *Runner) Verify() (Response, error) {
	lines, err := Lines(r.Dir)
	if err != nil {
		return Response{}, err
	}
	tip, err := r.Git.FetchMain()
	if err != nil {
		return Response{}, err
	}
	current := Current(lines)
	about := tip
	if current != nil {
		about = current.Commit
	}
	log, err := r.logPath()
	if err != nil {
		return Response{}, err
	}
	tree, err := r.checkout(filepath.Join(r.Dir, "verify", strconv.Itoa(os.Getpid())), about)
	if err != nil {
		return Response{}, err
	}
	r.trees = map[string]*landedTree{about: tree}
	defer r.unland()
	asked := r.call("version", about, "", current, log, nil)
	if asked.state != stateDone {
		return Response{}, errors.New(asked.describe() + " (log " + log + ")")
	}
	return asked.Response, removeFile(log)
}

// Status is the record read for a person: the current deploy and the one
// before it, the pause, the run in progress and the last failure.
type Status struct {
	Current  *Line  `json:"current"`
	Previous *Line  `json:"previous,omitempty"`
	Paused   *Pause `json:"paused,omitempty"`
	// Inactive is the line that recorded that nothing is active, while
	// that is so.
	Inactive *Line `json:"inactive,omitempty"`
	// Adapter is the process a live run has in flight, and LogGrewAt when
	// that process's log last grew.
	Adapter     *Active    `json:"adapter,omitempty"`
	LogGrewAt   *time.Time `json:"logGrewAt,omitempty"`
	LastFailure *Line      `json:"lastFailure,omitempty"`
	History     []Line     `json:"history,omitempty"`
}

// ReadStatus reads the project's deploy state without changing it; history
// keeps the newest lines, newest first.
func ReadStatus(dir string, history int) (Status, error) {
	var status Status
	lines, err := Lines(dir)
	if err != nil {
		return status, err
	}
	status.Current = Current(lines)
	if status.Current != nil && status.Current.Previous != "" {
		for index := len(lines) - 1; index >= 0; index-- {
			if lines[index].Commit == string(status.Current.Previous) && lines[index].Outcome == OutcomeActive {
				line := lines[index]
				status.Previous = &line
				break
			}
		}
	}
	if count := len(lines); count > 0 && lines[count-1].Failed() {
		status.LastFailure = &lines[count-1]
	}
	if state := newestState(lines); state != nil && state.Outcome == OutcomeNone {
		status.Inactive = state
	}
	for index := len(lines) - 1; index >= 0 && len(status.History) < history; index-- {
		status.History = append(status.History, lines[index])
	}
	if pause, paused, err := ReadPause(dir); err != nil {
		return status, err
	} else if paused {
		status.Paused = &pause
	}
	// Status takes no lock: a trigger that found one held would leave its
	// tip to a holder that never deploys. run.json names the adapter in
	// flight, also one whose run has ended.
	if active, ok, err := ReadActive(dir); err != nil {
		return status, err
	} else if ok && active.AdapterAlive() {
		status.Adapter = &active
		if info, err := os.Stat(active.Log); err == nil {
			grew := info.ModTime().UTC()
			status.LogGrewAt = &grew
		}
	}
	return status, nil
}
