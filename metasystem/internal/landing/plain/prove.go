package plain

// landing prove: the project's own proof command (landing.prove.command)
// over the lane checkout's HEAD, detached so it outlives the agent's
// session. The command runs in a fresh detached worktree of the lane
// repository at exactly the commit being proven, so it sees only that
// committed tree: what the lane checkout holds besides (a steward's record,
// an uncommitted file) neither reaches nor reds it. running.json names the proof while it runs; results.jsonl gets
// one line when it ends (exit 0 green, else red). A proof whose process is
// gone without a result died: it holds nothing and is shown as such.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// The results a proof ends with.
const (
	Green = "green"
	Red   = "red"
)

// Running is the proof that runs, as running.json keeps it.
type Running struct {
	Attempt string `json:"attempt"`
	Tree    string `json:"tree"`
	Commit  string `json:"commit"`
	Log     string `json:"log"`
	Since   string `json:"since"`
	Pid     int64  `json:"pid"`
	// Process is the exact identity of Pid, so a reused pid is not read as
	// the proof.
	Process string `json:"process,omitempty"`
}

// Result is one line of results.jsonl.
type Result struct {
	Tree    string `json:"tree"`
	Commit  string `json:"commit"`
	Result  string `json:"result"`
	Log     string `json:"log"`
	At      string `json:"at"`
	Attempt string `json:"attempt,omitempty"`
	// Reason is why the result is what it is, in one sentence: for red,
	// how the proving command ended ("the proving command exited 1") or why it
	// could not run. A green has none.
	Reason      string   `json:"reason,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	ScopeReason string   `json:"scopeReason,omitempty"`
	Base        string   `json:"base,omitempty"`
	FullTree    string   `json:"fullTree,omitempty"`
	FullAt      string   `json:"fullAt,omitempty"`
	Ran         []string `json:"ran,omitempty"`
	Environment string   `json:"environment,omitempty"`
}

// ProveSeams are a proof's effects.
type ProveSeams struct {
	Now        func() time.Time
	Executable func() (string, error)
	// Launch starts argv detached in dir, its output appended to log, and
	// returns its pid (the kernel's detached start).
	Launch func(argv []string, dir, log string) (int64, error)
	// Alive says whether a running proof's process runs; nil reads its
	// recorded identity.
	Alive func(Running) bool
	NewID func() string
	// Git runs git in a directory for a proof run (Run); nil is Git.
	Git func(dir string, args ...string) (string, error)
	// Closure names language units changed between the two trees; nil detects
	// the checkout's adapter. Command runs the prepared proof; nil runs it.
	Closure func(root, base, tree string) (adapter.Closure, error)
	Command func(*exec.Cmd) error
}

func (s ProveSeams) git(dir string, args ...string) (string, error) {
	if s.Git != nil {
		return s.Git(dir, args...)
	}
	return Git(dir, args...)
}

func (s ProveSeams) now() time.Time {
	if s.Now == nil {
		return time.Now().UTC()
	}
	return s.Now().UTC()
}

func (s ProveSeams) alive(running Running) bool {
	if s.Alive != nil {
		return s.Alive(running)
	}
	ref, err := identity.ParseRef(running.Process)
	if err != nil {
		return false
	}
	// A process whose liveness can't be read counts as running: the lane
	// waits rather than proving twice.
	return identity.LiveRef(identity.KernelProber{}, ref) != identity.Dead
}

func (s ProveSeams) newID() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return s.now().Format("20060102T150405.000000000Z")
}

// processRef is the exact identity of pid, encoded; "" when it can't be
// read.
func processRef(pid int64) string {
	exact, live, err := identity.KernelProber{}.Probe(pid)
	if err != nil || live != identity.Alive {
		return ""
	}
	encoded, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		return ""
	}
	return encoded
}

// Busy is a prove refused because another tree's proof runs.
type Busy struct{ Running Running }

func (b *Busy) Error() string {
	return fmt.Sprintf("tree %s is being proven (attempt %s, since %s), so nothing else was started", Short(b.Running.Tree), b.Running.Attempt, b.Running.Since)
}

// ReadRunning is the proof recorded running and whether its process runs;
// false when none is recorded.
func ReadRunning(install string, seams ProveSeams) (Running, bool, bool, error) {
	data, err := os.ReadFile(runningPath(install))
	if errors.Is(err, os.ErrNotExist) {
		return Running{}, false, false, nil
	}
	if err != nil {
		return Running{}, false, false, err
	}
	var running Running
	if err := json.Unmarshal(data, &running); err != nil {
		// A torn record is a proof that died mid-start.
		return Running{}, true, false, nil
	}
	return running, true, seams.alive(running), nil
}

// Head is the commit and tree of the checkout's HEAD.
func Head(checkout string) (commit, tree string, err error) {
	return head(Git, checkout)
}

func head(git func(string, ...string) (string, error), checkout string) (commit, tree string, err error) {
	commit, err = git(checkout, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", "", fmt.Errorf("read the lane checkout's HEAD: %w", err)
	}
	tree, err = git(checkout, "rev-parse", "--verify", "HEAD^{tree}")
	if err != nil {
		return "", "", err
	}
	return commit, tree, nil
}

// Start starts the proof of the checkout's HEAD detached: the engine runs
// landing prove --wait --attempt ID in a session of its own, which runs the
// command and records the result. A repeat while the same tree's proof runs
// starts nothing (already true); another tree's running proof is *Busy.
func Start(install, checkout string, seams ProveSeams) (Running, bool, error) {
	commit, tree, err := Head(checkout)
	if err != nil {
		return Running{}, false, err
	}
	var started Running
	already := false
	err = withLock(install, func() error {
		running, recorded, alive, err := ReadRunning(install, seams)
		if err != nil {
			return err
		}
		if recorded && alive {
			if running.Tree == tree {
				started, already = running, true
				return nil
			}
			return &Busy{Running: running}
		}
		executable, err := seams.Executable()
		if err != nil {
			return err
		}
		id := seams.newID()
		logs := filepath.Join(Dir(install), "proofs")
		if err := os.MkdirAll(logs, 0o755); err != nil {
			return err
		}
		log := filepath.Join(logs, id+".log")
		pid, err := seams.Launch([]string{executable, "landing", "prove", "--wait", "--attempt", id}, checkout, log)
		if err != nil {
			return fmt.Errorf("start proving tree %s: %w", Short(tree), err)
		}
		started = Running{Attempt: id, Tree: tree, Commit: commit, Log: log, Since: seams.now().Format(time.RFC3339), Pid: pid, Process: processRef(pid)}
		return writeRunning(install, started)
	})
	return started, already, err
}

// Settled reuses the green recorded for HEAD's exact tree. A tree whose own
// last result is red, or a running proof, settles nothing.
func Settled(install, checkout string, seams ProveSeams) (Result, bool, error) {
	_, tree, err := head(seams.git, checkout)
	if err != nil {
		return Result{}, false, err
	}
	var settled Result
	found := false
	err = withLock(install, func() error {
		_, recorded, alive, err := ReadRunning(install, seams)
		if err != nil || recorded && alive {
			return err
		}
		if result, ok, err := ResultFor(install, tree); err != nil || ok {
			settled, found = result, ok && result.Result == Green
			return err
		}
		return nil
	})
	return settled, found, err
}

func writeRunning(install string, running Running) error {
	data, err := json.Marshal(running)
	if err != nil {
		return err
	}
	path := runningPath(install)
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// Run runs the proof command over the checkout's HEAD in this process, in
// a fresh detached worktree at that commit, and appends its result. attempt names a detached start's record (Start wrote
// it); empty records this process as the running proof first, refusing
// while another tree's proof runs. The command's output goes to output.
func Run(install, checkout, command, attempt string, output io.Writer, seams ProveSeams) (Result, error) {
	commit, tree, err := head(seams.git, checkout)
	if err != nil {
		return Result{}, err
	}
	running := Running{Attempt: attempt, Tree: tree, Commit: commit}
	err = withLock(install, func() error {
		current, recorded, alive, err := ReadRunning(install, seams)
		if err != nil {
			return err
		}
		if attempt != "" && recorded && current.Attempt == attempt {
			running = current
			return nil
		}
		if recorded && alive {
			return &Busy{Running: current}
		}
		if running.Attempt == "" {
			running.Attempt = seams.newID()
		}
		pid := int64(os.Getpid())
		running.Since, running.Pid, running.Process = seams.now().Format(time.RFC3339), pid, processRef(pid)
		if file, ok := output.(*os.File); ok {
			running.Log = file.Name()
		}
		return writeRunning(install, running)
	})
	if err != nil {
		return Result{}, err
	}
	decision := decideScope(install, checkout, running, seams)
	observed := &proofOutput{output: io.Discard}
	runErr := proveInWorktree(seams, install, checkout, command, running, decision, output, observed)
	observed.finish()
	if runErr == nil && decision.Scope == "scoped" && (observed.environment == "" || decision.base.Environment == "" || observed.environment != decision.base.Environment) {
		decision.Scope, decision.ScopeReason = "full", fmt.Sprintf("the proof environment changed from %q to %q", decision.base.Environment, observed.environment)
		decision.Base = ""
		observed = &proofOutput{output: io.Discard}
		runErr = proveInWorktree(seams, install, checkout, command, running, decision, output, observed)
		observed.finish()
	}
	outcome, reason := Green, ""
	if runErr != nil {
		// The proof command ran and failed, or could not run: its own
		// exit is the reason, read from nothing but runErr.
		outcome, reason = Red, runErr.Error()
		fmt.Fprintf(output, "\nlanding prove: %v\n", runErr)
	}
	result := Result{Tree: running.Tree, Commit: running.Commit, Result: outcome, Log: running.Log, At: seams.now().Format(time.RFC3339), Attempt: running.Attempt, Reason: reason}
	result.Scope, result.ScopeReason, result.Base = decision.Scope, decision.ScopeReason, decision.Base
	result.Ran, result.Environment = observed.ran, observed.environment
	if decision.Scope == "scoped" {
		result.FullTree, result.FullAt = decision.base.FullTree, decision.base.FullAt
	} else if outcome == Green {
		result.FullTree, result.FullAt = result.Tree, result.At
	}
	err = withLock(install, func() error {
		if err := decision.writeRecord(install, result, observed); err != nil {
			return err
		}
		if err := appendLine(resultsPath(install), result); err != nil {
			return err
		}
		if current, recorded, _, _ := ReadRunning(install, seams); recorded && current.Attempt == running.Attempt {
			return os.Remove(runningPath(install))
		}
		return nil
	})
	return result, err
}

// Results are every recorded result, oldest first.
func Results(install string) ([]Result, error) {
	return readLines[Result](resultsPath(install))
}

// LastResult is the newest result.
func LastResult(install string) (Result, bool, error) {
	results, err := Results(install)
	if err != nil || len(results) == 0 {
		return Result{}, false, err
	}
	return results[len(results)-1], true, nil
}

// ResultFor is the newest result for tree.
func ResultFor(install, tree string) (Result, bool, error) {
	results, err := Results(install)
	if err != nil {
		return Result{}, false, err
	}
	for index := len(results) - 1; index >= 0; index-- {
		if results[index].Tree == tree {
			return results[index], true, nil
		}
	}
	return Result{}, false, nil
}

// Short is the first twelve characters of an id.
func Short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// Git runs git in dir and returns its trimmed output.
func Git(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	out, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exit.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// proveInWorktree runs command in a fresh detached worktree of the lane
// repository checked out at running.Commit, from the worktree's folder that
// matches the installation's place in the checkout, and removes the
// worktree after. Worktrees a crashed proof left are removed first. Only
// this process proves (it holds running.json), so every worktree under
// proofTrees is a leftover.
func proveInWorktree(seams ProveSeams, install, checkout, command string, running Running, decision scopeDecision, output io.Writer, observed *proofOutput) error {
	git := seams.git
	trees := proofTrees(install)
	removeProofTrees(git, checkout, trees, output)
	if err := os.MkdirAll(trees, 0o755); err != nil {
		return err
	}
	tree := filepath.Join(trees, running.Attempt)
	if _, err := git(checkout, "worktree", "add", "--detach", tree, running.Commit); err != nil {
		return fmt.Errorf("the worktree of commit %s could not be made: %w", Short(running.Commit), err)
	}
	defer func() {
		if _, err := git(checkout, "worktree", "remove", "--force", tree); err != nil {
			fmt.Fprintf(output, "\nlanding prove: the proof's worktree stays until the next prove: %v\n", err)
		}
	}()
	dir := tree
	if rel, err := filepath.Rel(checkout, install); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		dir = filepath.Join(tree, rel)
	}
	shell := exec.Command("/bin/sh", "-c", command)
	shell.Dir = dir
	shell.Env = append(os.Environ(), "LANDING_TREE="+running.Tree, "LANDING_COMMIT="+running.Commit,
		"LANDING_PROOF_SCOPE="+decision.Scope, "LANDING_PROOF_BASE="+decision.Base, "LANDING_PROOF_GROUPS="+strings.Join(decision.groupIDs(), " "))
	shell.Stdin = nil
	shell.Stdout = output
	shell.Stderr = output
	run := seams.Command
	if run == nil {
		run = (*exec.Cmd).Run
	}
	offset := int64(-1)
	if file, ok := output.(*os.File); ok {
		if info, err := file.Stat(); err == nil && info.Mode().IsRegular() {
			if position, err := file.Seek(0, io.SeekCurrent); err == nil {
				offset = position
			}
		}
	}
	err := run(shell)
	observed.readLog(output, offset)
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.Exited() {
			return fmt.Errorf("the proving command exited %d", exit.ExitCode())
		}
		return fmt.Errorf("the proving command ended: %w", err)
	}
	return nil
}

// removeProofTrees removes the lane repository's worktrees under trees,
// each by the path git lists for it, and prunes what is gone.
func removeProofTrees(git func(string, ...string) (string, error), checkout, trees string, output io.Writer) {
	list, err := git(checkout, "worktree", "list", "--porcelain")
	if err != nil {
		return
	}
	resolved := trees
	if real, err := filepath.EvalSymlinks(trees); err == nil {
		resolved = real
	}
	for _, line := range strings.Split(list, "\n") {
		path, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		for _, parent := range []string{trees, resolved} {
			if strings.HasPrefix(path, parent+string(filepath.Separator)) {
				if _, err := git(checkout, "worktree", "remove", "--force", path); err != nil {
					fmt.Fprintf(output, "landing prove: a crashed proof's worktree %s stays: %v\n", path, err)
				}
				break
			}
		}
	}
	_, _ = git(checkout, "worktree", "prune")
}

// proofTrees holds the proofs' detached worktrees of the lane repository.
func proofTrees(install string) string { return filepath.Join(Dir(install), "proof-trees") }

// ErrNoProofLog marks a proof log ProofLog does not serve.
var ErrNoProofLog = errors.New("no landing log is served")

// ProofLog is the log a lane record names for attempt: its result in
// results.jsonl, else the proof running.json names. It answers only a log
// that lies directly inside the lane's proofs folder, where Start and a
// person's landing prove --wait write every log; an attempt no record names,
// or a record whose log lies anywhere else (a result an older engine wrote),
// is ErrNoProofLog in words. A record it cannot read is said as such, never
// as no record.
func ProofLog(install, attempt string) (string, error) {
	if attempt == "" {
		return "", fmt.Errorf("%w: no attempt was named", ErrNoProofLog)
	}
	log, err := recordedLog(install, attempt)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(log)
	if log == "" || filepath.Dir(clean) != filepath.Join(Dir(install), "proofs") {
		return "", fmt.Errorf("%w: the log of attempt %s is not in the lane's own log folder", ErrNoProofLog, attempt)
	}
	// A link in the folder is refused, never followed: it could point
	// anywhere. A log that is gone is the caller's to say.
	if info, err := os.Lstat(clean); err == nil && !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: the log of attempt %s is not a file in the lane's own log folder", ErrNoProofLog, attempt)
	}
	return clean, nil
}

// recordedLog is the log the newest result of attempt names, else the
// running proof's when it is that attempt.
func recordedLog(install, attempt string) (string, error) {
	results, damaged, err := countedLines[Result](resultsPath(install))
	if err != nil {
		return "", fmt.Errorf("the landing results can't be read: %w", err)
	}
	for index := len(results) - 1; index >= 0; index-- {
		if results[index].Attempt == attempt {
			return results[index].Log, nil
		}
	}
	data, err := os.ReadFile(runningPath(install))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("the running landing check can't be read: %w", err)
	}
	// A torn running.json is a proof that died mid-start: it names nothing.
	var running Running
	if err == nil && json.Unmarshal(data, &running) == nil && running.Attempt == attempt {
		return running.Log, nil
	}
	if damaged > 0 {
		return "", fmt.Errorf("the landing results have %d line(s) that can't be read, so whether one names attempt %s is not known", damaged, attempt)
	}
	return "", fmt.Errorf("%w: no lane record names attempt %s", ErrNoProofLog, attempt)
}
