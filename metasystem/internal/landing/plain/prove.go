package plain

// landing prove: the project's own proof command (proof.full)
// over the lane checkout's HEAD, detached so it outlives the agent's
// session. The command runs in a fresh detached worktree of the lane
// repository at exactly the commit being proven, so it sees only that
// committed tree: what the lane checkout holds besides (a steward's record,
// an uncommitted file) neither reaches nor reds it. running.json names the proof while it runs; results.jsonl gets
// one line when it ends (exit 0 green, else red). A proof whose process is
// gone without a result died: it holds nothing and is shown as such.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// The results a proof ends with.
const (
	Green = "green"
	Red   = "red"
	Held  = "held"
)

// Running is the proof that runs, as running.json keeps it.
type Running struct {
	Checkpoint        bool                 `json:"checkpoint,omitempty"`
	ClassificationOf  string               `json:"classification-of,omitempty"`
	ObservedIncidents []goal.TrunkRedEntry `json:"observed-incidents,omitempty"`
	Admission         *ExecutionAdmission  `json:"admission,omitempty"`
	Executions        []ExecutionAdmission `json:"executions,omitempty"`
	Person            *ActProvenance       `json:"person,omitempty"`
	BatchID           string               `json:"batch-id,omitempty"`
	BatchMembers      []GoalSHA            `json:"batch-members,omitempty"`
	Trunk             bool                 `json:"trunk,omitempty"`
	Gate              bool                 `json:"gate,omitempty"`
	Impact            bool                 `json:"impact,omitempty"`
	Attempt           string               `json:"attempt"`
	Tree              string               `json:"tree"`
	Commit            string               `json:"commit"`
	Log               string               `json:"log"`
	Since             string               `json:"since"`
	Minutes           *float64             `json:"minutes,omitempty"`
	Pid               int64                `json:"pid"`
	// Process is the exact identity of Pid, so a reused pid is not read as
	// the proof.
	Process string `json:"process,omitempty"`
}

// Result is one line of results.jsonl.
type Result struct {
	FlakeRepeats          []Running            `json:"flake-repeats,omitempty"`
	RepeatComplete        bool                 `json:"repeat-complete,omitempty"`
	FlakePublished        bool                 `json:"flake-published,omitempty"`
	Flakes                []FlakeRecord        `json:"flakes,omitempty"`
	ClassificationOf      string               `json:"classification-of,omitempty"`
	NextFull              *Result              `json:"next-full,omitempty"`
	ClassificationPolicy  PolicyValue          `json:"classification-policy,omitempty"`
	ClassificationWarning string               `json:"classification-warning,omitempty"`
	ClassificationPending bool                 `json:"classification-pending,omitempty"`
	ClassificationPerson  *ActProvenance       `json:"classification-person,omitempty"`
	ObservedIncidents     []goal.TrunkRedEntry `json:"observed-incidents,omitempty"`
	Person                *ActProvenance       `json:"person,omitempty"`
	Executions            []ExecutionAdmission `json:"executions,omitempty"`
	executionErr          error
	BatchID               string    `json:"batch-id,omitempty"`
	BatchMembers          []GoalSHA `json:"batch-members,omitempty"`
	Trunk                 bool      `json:"trunk,omitempty"`
	// CountedFull identifies a whole execution with a complete report.
	CountedFull bool `json:"countedFull,omitempty"`
	// LoopClosed ends the batch budget without changing the proof verdict.
	LoopClosed bool      `json:"loopClosed,omitempty"`
	Cause      *Cause    `json:"cause,omitempty"`
	Goals      []GoalSHA `json:"goals"`
	Tree       string    `json:"tree"`
	Commit     string    `json:"commit"`
	Result     string    `json:"result"`
	Log        string    `json:"log"`
	At         string    `json:"at"`
	StartedAt  string    `json:"started_at,omitempty"`
	Minutes    *float64  `json:"minutes"`
	Attempt    string    `json:"attempt,omitempty"`
	// Reason is why the result is what it is, in one sentence: for red,
	// how the proving command ended ("the proving command exited 1") or why it
	// could not run; for an inherited green, the tree it inherits from. An
	// ordinary green has none.
	Reason      string       `json:"reason,omitempty"`
	Failed      []FailedUnit `json:"failed,omitempty"`
	Load        float64      `json:"load,omitempty"`
	Repeat      string       `json:"repeat,omitempty"`
	Scope       string       `json:"scope,omitempty"`
	ScopeReason string       `json:"scopeReason,omitempty"`
	Base        string       `json:"base,omitempty"`
	BaseCommit  string       `json:"baseCommit,omitempty"`
	PlanHash    string       `json:"planHash,omitempty"`
	FullTree    string       `json:"fullTree,omitempty"`
	FullAt      string       `json:"fullAt,omitempty"`
	Ran         []string     `json:"ran,omitempty"`
	Environment string       `json:"environment,omitempty"`
}

// FailedUnit names the tests that failed and the surfaces the judge read.
type FailedUnit struct {
	Unit     string   `json:"unit"`
	Tests    []string `json:"tests"`
	Surfaces []string `json:"surfaces"`
}

// UnitJudgement says whether the batch affects a unit and all its failures are registered.
type UnitJudgement struct {
	Affected, Known bool
	Surfaces        []string
}

// FlakeRecord carries both checks of a unit to its register and fix goal.
type FlakeRecord struct {
	FailedUnit
	Commit, Tree, Attempt, Log, Where string
	Load                              float64
	Repeat, RepeatAttempt, RepeatLog  string
	Outputs, RepeatOutputs            map[string]repoproof.TestOutput
}

// FlakeRecorded is the confirmed record's fix goal and sighting count.
type FlakeRecorded struct {
	Goal string
	Seen int
}

// ProveSeams are a proof's effects.
type ProveSeams struct {
	// Machine reads the lane computer's existing nickname.
	Machine          func(string) (string, error)
	ClassificationOf string
	// Person admits one direct proof; Start binds its provenance to the attempt.
	Person *ActProvenance
	// FenceCheck rechecks execution admission under the owning queue lock.
	FenceCheck func() error
	// Pause observes the standing maintenance fence at selection.
	Pause func() (lane.Pause, bool)
	// TimerHeld suppresses periodic trunk checks while a fence stands.
	TimerHeld func() bool
	// Policy resolves only the policy requested at its effect boundary.
	Policy func(string) (PolicyValue, error)
	// Lane reads the host registration again before a selection or admission.
	Lane         func() (lane.Record, error)
	AgentRunning func() (bool, error)
	// FetchCommand prepares the bounded fetch process; nil runs Git unchanged.
	FetchCommand func(*exec.Cmd)
	// FetchTimeout overrides the fetch deadline for isolated tests.
	FetchTimeout time.Duration
	// FetchDeadline supplies the fetch expiry; nil uses the configured timeout.
	FetchDeadline func(time.Duration) <-chan time.Time
	// Trunk selects origin/main for a fresh full check.
	Trunk bool
	// Gate selects the cheap merge check and its separate result register.
	Gate         bool
	Impact       bool
	gateBaseline bool
	// Incidents reads main's register for queue holds; nil uses Git.
	Incidents  func(install, checkout, main string) ([]goal.TrunkRedEntry, error)
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
	// Judge reads the batch's effect and registered failures; nil cannot tell.
	Judge func(checkout, commit string, failed []FailedUnit) (map[string]UnitJudgement, error)
	// RecordFlake confirms a sighting and its fix goal; nil cannot record.
	RecordFlake func(FlakeRecord) (FlakeRecorded, error)
	// RecordMain publishes failed checks of main in one transaction. Each
	// result carries its own log and main's commit/tree.
	RecordMain func([]Result) error
	// Closure names language units changed between the two trees; nil detects
	// the checkout's adapter. Command runs the prepared proof; nil runs it.
	Closure func(root, base, tree string) (adapter.Closure, error)
	Command func(*exec.Cmd) error
	// CommandForCommit resolves the proof declaration for the exact commit
	// selected under the lane lock, including a detached attempt's commit.
	CommandForCommit     func(commit string) (string, error)
	GateCommandForCommit func(commit string) (string, error)
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
	if running.Checkpoint {
		return false
	}
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

// OwnProcess reports whether this process is the recorded proof child.
func (r Running) OwnProcess() bool {
	return r.Pid == int64(os.Getpid()) && r.Process != "" && r.Process == processRef(int64(os.Getpid()))
}

// Busy is a prove refused because another tree's proof runs.
type Busy struct{ Running Running }

func (b *Busy) Error() string {
	return fmt.Sprintf("tree %s is being proven (attempt %s, since %s), so nothing else was started", Short(b.Running.Tree), b.Running.Attempt, b.Running.Since)
}

// NoRepeat refuses another check of a tree whose allowance has been spent.
type NoRepeat struct{}

func (*NoRepeat) Error() string {
	return "this code failed its check and gets no other; the waiting goals hold"
}

// checkState clears completed records and records dead checks under the lane lock.
// The attempt running in this process must not be marked dead.
func checkState(install, checkout, attempt string, seams ProveSeams) (Running, bool, bool, error) {
	running, recorded, alive, err := ReadRunning(install, seams)
	if err != nil || !recorded || alive {
		return running, recorded, alive, err
	}
	if running.Checkpoint {
		return running, true, false, nil
	}
	if running.Admission != nil {
		if running.Admission.State == "pending" {
			running.setAdmissionState("failed")
			if err := writeRunning(install, running); err != nil {
				return running, recorded, alive, err
			}
		}
		if running.Admission.State == "failed" {
			return Running{}, false, false, nil
		}
	}

	mode := seams
	mode.Gate = running.Gate
	results, err := readLines[Result](mode.resultsPath(install))
	if err != nil {
		return running, recorded, alive, err
	}
	exists := false
	for _, result := range results {
		if running.Attempt != "" && result.Attempt == running.Attempt {
			return Running{}, false, false, os.Remove(runningPath(install))
		}
		exists = exists || result.Tree == running.Tree
	}
	if attempt != "" && running.Attempt == attempt && running.OwnProcess() {
		return running, recorded, alive, nil
	}
	red := Result{Trunk: running.Trunk, Tree: running.Tree, Commit: running.Commit, Attempt: running.Attempt, Log: running.Log,
		At: seams.now().Format(time.RFC3339), Result: Red, Reason: "the lane's check stopped before it ended", Cause: &Cause{Kind: "environment", Name: "lost-process", Evidence: running.Log}}
	red.BatchID, red.BatchMembers = running.BatchID, running.BatchMembers
	red.Person, red.Executions, red.ClassificationOf = running.Person, running.Executions, running.ClassificationOf
	if running.Gate {
		red.Scope = "gate"
	}
	red.Goals, err = goalsInCommit(install, checkout, running.Commit, seams.git)
	if err != nil {
		return running, recorded, alive, err
	}
	if !exists {
		red.Repeat = "allowed"
	}
	if err := recordProofStop(install, red); err != nil {
		return running, recorded, alive, err
	}
	if err := appendLine(mode.resultsPath(install), red); err != nil {
		return running, recorded, alive, err
	}
	return Running{}, false, false, os.Remove(runningPath(install))
}

// checkBound reads the newest line under the lane lock before any check starts.
func (s ProveSeams) checkBound(install, tree string) (Result, bool, error) {
	result, found, err := resultFor(s.resultsPath(install), tree)
	if err == nil && found && result.Result == Red && result.RepeatComplete && len(result.FlakeRepeats) > 0 {
		green := result
		green.Result, green.Failed, green.Cause = Green, nil, nil
		result = recordFlakes(s, result, green, "alone", result.FlakeRepeats)
		err = appendLine(s.resultsPath(install), result)
	}
	if err == nil && found && result.Result == Red && s.Person == nil && (result.ClassificationPending || result.ClassificationPerson == nil && result.ClassificationOf == "") {
		err = redContinuationLocked(install, result, s)
	}
	if err == nil && found && result.Result != Green && result.Result != Held && result.Repeat != "allowed" && s.Person == nil {
		err = &NoRepeat{}
	}
	if err != nil && s.Person != nil {
		return Result{}, false, nil
	}
	return result, found, err
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
		return Running{}, true, false, fmt.Errorf("the running check cannot be decoded: %w", err)
	}
	return running, true, (running.Admission == nil || running.Admission.State != "failed") && seams.alive(running), nil
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

func (s ProveSeams) subject(checkout string) (string, string, error) {
	if !s.Trunk {
		return head(s.git, checkout)
	}
	commit, err := s.git(checkout, "rev-parse", "--verify", "origin/main^{commit}")
	if err != nil {
		return "", "", err
	}
	tree, err := s.git(checkout, "rev-parse", "--verify", commit+"^{tree}")
	return commit, tree, err
}

// Start starts the proof of HEAD, or origin/main for a trunk check, detached: the engine runs
// landing prove --wait --attempt ID in a session of its own, which runs the
// command and records the result. A repeat while the same tree's proof runs
// starts nothing (already true); another tree's running proof is *Busy.
func Start(install, checkout string, seams ProveSeams) (Running, bool, error) {
	if err := proofFixReady(install, seams); err != nil {
		return Running{}, false, err
	}
	commit, tree, err := seams.subject(checkout)
	if err != nil {
		return Running{}, false, err
	}
	var started Running
	already := false
	err = withLock(install, func() error {
		currentCommit, currentTree, subjectErr := seams.subject(checkout)
		if subjectErr != nil {
			return subjectErr
		}
		if currentCommit != commit || currentTree != tree {
			return &Refusal{Code: "LANE_PROOF_SUBJECT", Reason: "the check target changed before admission", Next: proofCommand(seams.Gate, seams.Trunk)}
		}

		running, recorded, alive, err := checkState(install, checkout, "", seams)
		if err != nil {
			return err
		}
		if recorded && alive {
			if running.Gate == seams.Gate && running.Trunk == seams.Trunk && running.Impact == seams.Impact && (running.Tree == tree || !seams.Gate && !seams.Trunk && ledgerOnlyBetween(seams.git, checkout, running.Tree, tree)) {
				started, already = running, true
				return nil
			}
			return &Busy{Running: running}
		}
		var batch *Batch
		if !seams.Trunk {
			batch, err = checkBatchLocked(install, checkout, commit, "", seams.Gate, true, seams)
			if err != nil {
				return err
			}
		}
		if !seams.Trunk {
			result, found, err := seams.checkBound(install, tree)
			if err != nil {
				return err
			}
			if found && seams.acceptsGreen(install, checkout, result) {
				started, already = Running{Gate: seams.Gate, Trunk: seams.Trunk, Attempt: result.Attempt, Tree: result.Tree, Commit: result.Commit, Log: result.Log, Since: result.At}, true
				return nil
			}
		}
		if seams.FenceCheck != nil {
			if err := seams.FenceCheck(); err != nil {
				return err
			}
		}
		if seams.Trunk {
			prior, found, err := resultFor(seams.resultsPath(install), tree)
			if err != nil && seams.Person == nil {
				return err
			}
			if found && prior.Cause != nil && prior.Cause.Kind == "environment" && prior.Repeat != "allowed" && seams.Person == nil {
				return &NoRepeat{}
			}
		}
		if err := proofAdmission(install, checkout, commit, seams); err != nil {
			return err
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
		argv := []string{executable, "landing", "prove", "--wait", "--attempt", id}
		if seams.Gate {
			argv = append(argv, "--gate")
		}
		if seams.Impact {
			argv = append(argv, "--impact")
		}
		if seams.Trunk {
			argv = append(argv, "--trunk")
		}
		started = Running{Person: seams.Person, Impact: seams.Impact, Gate: seams.Gate, Trunk: seams.Trunk, Attempt: id, Tree: tree, Commit: commit, Log: log, Since: seams.now().Format(time.RFC3339), Pid: int64(os.Getpid()), Process: processRef(int64(os.Getpid()))}
		if batch != nil {
			started.BatchID, started.BatchMembers = batch.ID, batch.Members
		}
		command := ""
		if seams.CommandForCommit != nil {
			command, err = seams.CommandForCommit(commit)
			if err != nil {
				return err
			}
		}
		if err := admitExecutionLocked(install, checkout, command, &started, proofScope(install, checkout, started, seams), seams); err != nil {
			return err
		}
		started.setAdmissionState("pending")
		if err := advanceFix(install, checkout, started, seams); err != nil {
			return err
		}
		if err := writeRunning(install, started); err != nil {
			return err
		}
		pid, err := seams.Launch(argv, checkout, log)
		if err != nil {
			started.setAdmissionState("failed")
			if writeErr := writeRunning(install, started); writeErr != nil {
				return errors.Join(err, writeErr)
			}
			return fmt.Errorf("start proving tree %s: %w", Short(tree), err)
		}
		process := processRef(pid)
		if pid <= 0 || process == "" {
			started.setAdmissionState("failed")
			if err := writeRunning(install, started); err != nil {
				return err
			}
			reason := "the check process has no readable identity (environment cause); its admission failed"
			if pid > 0 {
				if err := syscall.Kill(int(pid), syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
					reason += "; stopping the child failed: " + err.Error()
				}
			}
			retry := "metasystem landing prove"
			if seams.Gate {
				retry += " --gate"
			}
			if seams.Trunk {
				retry += " --trunk"
			}
			return fmt.Errorf("start proving tree %s: %s; retry: %s", Short(tree), reason, retry)
		}
		started.Pid, started.Process = pid, process
		started.setAdmissionState("launched")
		if err := writeRunning(install, started); err != nil {
			return err
		}
		return closeProofAdmissionStopsLocked(install, started, seams.now())
	})
	return started, already, err
}

// Settled is HEAD's tree already proven green: the green recorded for that
// exact tree, or the green it inherits from a tree that differs from it only
// in goal ledger files, recorded here as Run would record it. Neither needs a
// proof in the background, whose instant result would end inside the
// caller's own turn and leave nobody to push it. A tree whose own last
// result is red, or a running proof, settles nothing: Start reports or
// starts the proof. Inherited and scoped greens require a full proof no
// more than an hour old.
func Settled(install, checkout string, seams ProveSeams) (Result, bool, error) {
	if err := proofFixReady(install, seams); err != nil {
		return Result{}, false, err
	}
	if seams.Trunk {
		return Result{}, false, nil
	}
	commit, tree, err := seams.subject(checkout)
	if err != nil {
		return Result{}, false, err
	}
	var settled Result
	found := false
	err = withLock(install, func() error {
		_, recorded, alive, err := checkState(install, checkout, "", seams)
		if err != nil {
			return err
		}
		if result, ok, err := seams.checkBound(install, tree); err != nil || ok {
			settled, found = result, ok && seams.acceptsGreen(install, checkout, result)
			if found {
				_, err = checkBatchLocked(install, checkout, commit, "", seams.Gate, true, seams)
			}
			return err
		}
		if recorded && alive || seams.Gate || seams.Impact {
			return nil
		}
		from, ok := ledgerOnlySinceGreen(seams.git, install, checkout, tree)
		if !ok || !from.fullCurrent(seams.now()) {
			return nil
		}
		settled = Result{Tree: tree, Commit: commit, Result: Green, At: seams.now().Format(time.RFC3339), Attempt: seams.newID(),
			Reason: inheritedReason(from)}
		batch, err := checkBatchLocked(install, checkout, commit, "", false, true, seams)
		if err != nil {
			return err
		}
		if batch != nil {
			settled.BatchID, settled.BatchMembers = batch.ID, batch.Members
		}
		settled, err = inheritScope(install, settled, from)
		if err != nil {
			return err
		}
		settled.Goals, err = goalsInCommit(install, checkout, commit, seams.git)
		if err != nil {
			return err
		}
		if err := advanceFix(install, checkout, Running{Attempt: settled.Attempt, Commit: settled.Commit, BatchID: settled.BatchID, BatchMembers: settled.BatchMembers}, seams); err != nil {
			return err
		}
		found = true
		return appendLine(resultsPath(install), settled)
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

// Run proves HEAD, or origin/main for a trunk check, in this process, in
// a fresh detached worktree at that commit, and appends its result. attempt names a detached start's record (Start wrote
// it); empty records this process as the running proof first, refusing
// while another tree's proof runs. The command's output goes to output.
func Run(install, checkout, command, attempt string, output io.Writer, seams ProveSeams) (Result, error) {
	if err := proofFixReady(install, seams); err != nil {
		return Result{}, err
	}
	var commit, tree string
	var err error
	if attempt == "" {
		commit, tree, err = seams.subject(checkout)
		if err != nil {
			return Result{}, err
		}
	}
	running := Running{Person: seams.Person, Impact: seams.Impact, Gate: seams.Gate, Trunk: seams.Trunk, Attempt: attempt, Tree: tree, Commit: commit}
	var previous, result Result
	already := false
	err = withLock(install, func() error {
		if attempt == "" {
			currentCommit, currentTree, subjectErr := seams.subject(checkout)
			if subjectErr != nil {
				return subjectErr
			}
			if currentCommit != commit || currentTree != tree {
				return &Refusal{Code: "LANE_PROOF_SUBJECT", Reason: "the check target changed before admission", Next: proofCommand(seams.Gate, seams.Trunk)}
			}
		}

		var current Running
		var recorded, alive bool
		var err error
		if attempt != "" {
			current, recorded, alive, err = ReadRunning(install, seams)
			if err == nil && recorded && (current.Gate != seams.Gate || current.Trunk != seams.Trunk || current.Impact != seams.Impact) {
				return &Busy{Running: current}
			}

			if err == nil && (!recorded || current.Attempt != attempt || current.Admission == nil || current.Admission.State != "launched" || !current.OwnProcess() || len(current.Executions) == 0 || !current.Admission.matches(current)) {
				err = &Refusal{Code: "LANE_PROOF_ADMISSION", Reason: "this background check has no matching unclaimed admission", Next: proofCommand(seams.Gate, seams.Trunk)}
			}
		} else {
			current, recorded, alive, err = checkState(install, checkout, attempt, seams)
		}
		if err != nil {
			return err
		}

		if attempt != "" && recorded && current.Attempt == attempt {
			if current.Admission.Lane != nil && seams.Lane != nil {
				registered, err := seams.Lane()
				if err != nil {
					return err
				}
				if registered != *current.Admission.Lane {
					return fmt.Errorf("the admitted landing lane registration changed")
				}
			}
			running = current
		}
		if recorded && alive && (current.Attempt != attempt || current.Gate != seams.Gate || current.Trunk != seams.Trunk || current.Impact != seams.Impact) {
			return &Busy{Running: current}
		}
		if !running.Trunk {
			batch, err := checkBatchLocked(install, checkout, running.Commit, "", seams.Gate, true, seams)
			if err != nil {
				return err
			}
			if batch != nil {
				running.BatchID, running.BatchMembers = batch.ID, batch.Members
			}
		}
		if seams.CommandForCommit != nil {
			command, err = seams.CommandForCommit(running.Commit)
			if err != nil {
				return err
			}
		}
		var found bool
		if !running.Trunk && attempt == "" {
			previous, found, err = seams.checkBound(install, running.Tree)
		} else {
			previous, found, err = resultFor(seams.resultsPath(install), running.Tree)
			if err != nil && running.Person != nil {
				previous, found, err = Result{}, false, nil
			}
			if running.Trunk && (previous.Cause == nil || previous.Cause.Kind != "environment") {
				previous, found = Result{}, false
			}
			if running.Trunk && found && previous.Repeat != "allowed" && seams.Person == nil && attempt == "" {
				return &NoRepeat{}
			}
		}
		if err != nil {
			return err
		}
		if !running.Trunk && found && seams.acceptsGreen(install, checkout, previous) {
			result, already = previous, true
			return nil
		}
		if seams.FenceCheck != nil {
			if err := seams.FenceCheck(); err != nil {
				return err
			}
		}
		if attempt == "" {
			if err := proofAdmission(install, checkout, running.Commit, seams); err != nil {
				return err
			}
		}
		if previous.Result == Held {
			previous, found = Result{}, false
		} else if previous.Result == Green {
			previous = Result{}
		}
		if attempt != "" {
			if running.Admission.Command != "" && running.Admission.Command != command {
				return fmt.Errorf("the admitted check declaration changed")
			}
			running.setAdmissionState("claimed")
			running.Pid, running.Process = int64(os.Getpid()), processRef(int64(os.Getpid()))
			if err := writeRunning(install, running); err != nil {
				return err
			}
			if found && previous.Result != Green {
				if previous.Cause == nil {
					previous.Cause = &Cause{Kind: "unclassified", Tests: failingTests(previous.Failed), Evidence: previous.Log}
				}
				if previous.Goals == nil {
					previous.Goals, err = goalsInCommit(install, checkout, previous.Commit, seams.git)
					if err != nil {
						return err
					}
				}
				previous.Repeat, previous.LoopClosed = "started", false
				if err := appendLine(seams.resultsPath(install), previous); err != nil {
					return err
				}
			}
			return nil
		}
		if running.Attempt == "" {
			running.Attempt = seams.newID()
		}
		pid := int64(os.Getpid())
		running.Since, running.Pid, running.Process = seams.now().Format(time.RFC3339), pid, processRef(pid)
		if file, ok := output.(*os.File); ok {
			running.Log = file.Name()
		}
		if err := admitExecutionLocked(install, checkout, command, &running, proofScope(install, checkout, running, seams), seams); err != nil {
			return err
		}
		running.setAdmissionState("claimed")
		if err := advanceFix(install, checkout, running, seams); err != nil {
			return err
		}
		if err := writeRunning(install, running); err != nil {
			return err
		}
		if found && previous.Result != Green {
			if previous.Cause == nil {
				previous.Cause = &Cause{Kind: "unclassified", Tests: failingTests(previous.Failed), Evidence: previous.Log}
			}
			if previous.Goals == nil {
				previous.Goals, err = goalsInCommit(install, checkout, previous.Commit, seams.git)
				if err != nil {
					return err
				}
			}
			previous.Repeat, previous.LoopClosed = "started", false
			if err := appendLine(seams.resultsPath(install), previous); err != nil {
				return err
			}
		}
		return closeProofAdmissionStopsLocked(install, running, seams.now())
	})
	if err != nil || already {
		return result, err
	}
	result = Result{Trunk: running.Trunk, Tree: running.Tree, Commit: running.Commit, Result: Green, Log: running.Log, At: seams.now().Format(time.RFC3339), Attempt: running.Attempt}
	result.BatchID, result.BatchMembers = running.BatchID, running.BatchMembers
	result.Goals, err = goalsInCommit(install, checkout, running.Commit, seams.git)
	if err != nil {
		return result, err
	}
	var decision scopeDecision
	observed := &proofOutput{output: io.Discard}
	inherited := false
	from, ok := Result{}, false
	if !seams.Gate && !running.Trunk && !seams.Impact {
		from, ok = ledgerOnlySinceGreen(seams.git, install, checkout, running.Tree)
	}
	if !seams.Gate && !running.Trunk && !seams.Impact && previous.Result == "" && ok && from.fullCurrent(seams.now()) {
		result.Reason = inheritedReason(from)
		result, err = inheritScope(install, result, from)
		if err != nil {
			return result, err
		}
		inherited = true
	} else {
		if seams.Gate {
			decision = scopeDecision{scopeRecord: scopeRecord{Scope: "gate"}}
		} else if running.Trunk {
			decision = scopeDecision{scopeRecord: scopeRecord{Scope: "full", ScopeReason: "fresh full check of main"}}
		} else {
			decision = proofScope(install, checkout, running, seams)
		}
		if running.Admission != nil && (running.Admission.Scope != decision.Scope || running.Admission.Base != decision.Base || running.Admission.BaseSHA != decision.BaseCommit || !slices.Equal(running.Admission.Groups, decision.groupIDs())) {
			return result, fmt.Errorf("the admitted check scope changed; request a fresh check")
		}
		result = proveInWorktree(seams, install, checkout, command, running, &decision, output, observed, result, previous)
		result = timedResult(result, running.Since, seams.now())
		result = decision.describe(result, observed)
	}
	if result.Reason != "" {
		fmt.Fprintf(output, "\nlanding prove: %s\n", result.Reason)
	}
	if result.Trunk && result.Result == Red && len(result.Failed) > 0 && (result.Cause == nil || result.Cause.Kind != "environment") {
		result = recordMainFailures(seams, result, []Result{result})
	}
	if !result.Trunk && result.Result == Red && len(result.Failed) > 0 && (result.Cause == nil || result.Cause.Kind != "environment") && seams.RecordMain != nil {
		main, mainErr := checkoutGit(checkout, seams).main()
		if mainErr == nil {
			mainTree, treeErr := seams.git(checkout, "rev-parse", "--verify", main+"^{tree}")
			if treeErr == nil && mainTree == result.Tree {
				check := result
				check.Commit = main
				result = recordMainFailures(seams, result, []Result{check})
			}
		}
	}
	result.ClassificationOf = running.ClassificationOf
	result.Person, result.Executions, result.ObservedIncidents = running.Person, running.Executions, running.ObservedIncidents
	if current, recorded, _, readErr := ReadRunning(install, seams); readErr == nil && recorded && current.Attempt == running.Attempt {
		result.Executions = current.Executions
	}
	err = withLock(install, func() error {
		if !inherited && !seams.Gate {
			if err := decision.writeRecord(install, result, observed); err != nil {
				return err
			}
		}
		if err := recordProofStop(install, result); err != nil {
			return err
		}
		if err := appendLine(seams.resultsPath(install), result); err != nil {
			return err
		}
		if !seams.Gate && !running.Trunk && !seams.Impact && result.Result == Green {
			commit, tree, err := head(seams.git, checkout)
			if err == nil && tree != result.Tree && ledgerOnlyBetween(seams.git, checkout, result.Tree, tree) {
				carried := result
				carried.CountedFull = false
				carried.Commit, carried.Tree, carried.Reason = commit, tree, inheritedReason(result)
				carried.Base, carried.Scope, carried.ScopeReason = result.Tree, "scoped", carried.Reason
				if err := appendLine(seams.resultsPath(install), carried); err != nil {
					return err
				}
			}
		}
		if current, recorded, _, _ := ReadRunning(install, seams); recorded && current.Attempt == running.Attempt {
			if result.Result == Red {
				current.Checkpoint = true
				return writeRunning(install, current)
			}
			return os.Remove(runningPath(install))
		}
		return nil
	})
	return result, errors.Join(err, result.executionErr)
}

// ledgerPaths are the goal ledger paths goal verbs rewrite, which
// docs/project-rules.md excludes from delivery content: a tree that
// differs from a green tree only in them needs no new proof. With seats
// publishing goal acts every few minutes, re-proving each such tree means
// a proof never finishes before main moves again, and nothing lands.
var ledgerPaths = []string{
	"metasystem/plans/goals/", "metasystem/plans/goals.md", "metasystem/plans/goals-accepted.json",
	"metasystem/records/goals/", "metasystem/records/counselor/",
	"metasystem/memory/receipts.log", "metasystem/records/narrator-digest.log",
}

func ledgerPath(path string) bool {
	for _, ledger := range ledgerPaths {
		if path == ledger || strings.HasSuffix(ledger, "/") && strings.HasPrefix(path, ledger) {
			return true
		}
	}
	return false
}

// ledgerOnlyBetween compares the proof's tree with a candidate tree.
func ledgerOnlyBetween(git func(string, ...string) (string, error), checkout, from, to string) bool {
	changed, err := git(checkout, "diff", "--name-only", "--no-renames", from, to)
	if err != nil {
		return false
	}
	for _, path := range strings.Split(strings.TrimSpace(changed), "\n") {
		if path != "" && !ledgerPath(path) {
			return false
		}
	}
	return true
}

// ledgerOnlySinceGreen names a recent green tree from which tree differs
// only in goal ledger files.
func ledgerOnlySinceGreen(git func(string, ...string) (string, error), install, checkout, tree string) (Result, bool) {
	results, err := Results(install)
	if err != nil {
		return Result{}, false
	}
	checked := 0
	for index := len(results) - 1; index >= 0 && checked < 10; index-- {
		green := results[index]
		if green.Result != Green || green.Tree == tree {
			continue
		}
		checked++
		changed, err := git(checkout, "diff", "--name-only", "--no-renames", green.Tree, tree)
		if err != nil || changed == "" {
			continue
		}
		ledgerOnly := true
		for _, path := range strings.Split(changed, "\n") {
			if !ledgerPath(path) {
				ledgerOnly = false
				break
			}
		}
		if ledgerOnly {
			return green, true
		}
	}
	return Result{}, false
}

func inheritedReason(from Result) string {
	reason := "inherits green from tree " + Short(from.Tree) + ": only goal ledger files changed since"
	if from.Reason != "" {
		reason += "; " + from.Reason
	}
	return reason
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
	return resultFor(resultsPath(install), tree)
}

func resultFor(path, tree string) (Result, bool, error) {
	results, err := readLines[Result](path)
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
	if len(args) > 0 && args[0] == "fetch" {
		return boundedFetch(dir, 60*time.Second, nil, nil, args...)
	}
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	out, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exit.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// boundedFetch keeps a stalled remote or credential prompt inside one keeper tick.
func boundedFetch(dir string, timeout time.Duration, prepare func(*exec.Cmd), deadline func(time.Duration) <-chan time.Time, args ...string) (string, error) {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = append(gittree.ScrubbedEnviron(), "GIT_TERMINAL_PROMPT=0")
	command.WaitDelay = time.Second
	var output, stderr bytes.Buffer
	command.Stdout, command.Stderr = &output, &stderr
	if prepare != nil {
		prepare(command)
	}
	if deadline == nil {
		deadline = time.After
	}
	err := boundedexec.RunWithDeadline(command, boundedexec.FixedBound(timeout, ""), "the landing fetch", deadline)
	if err != nil {
		return "", fmt.Errorf("git fetch: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(output.String()), nil
}

// runCheck gives the shell files, so a descendant holding its output open
// cannot delay its exit. Both protocols are read from this run's log bytes.
func runCheck(seams ProveSeams, dir, command string, running Running, only string, decision scopeDecision, output io.Writer, observed *proofOutput) (checkReport, error) {
	if only == "" && (decision.Scope == "full" || decision.Scope == "scoped" || decision.Scope == "impact") {
		if running.Admission == nil || running.Admission.State != "claimed" || !running.Admission.matches(running) || running.Admission.Scope != decision.Scope || (running.Admission.Command != "" && running.Admission.Command != command) {
			return checkReport{}, fmt.Errorf("this execution has no matching claimed check admission")
		}
	}

	log, ok := output.(*os.File)
	offset := int64(-1)
	if ok {
		if info, err := log.Stat(); err == nil && info.Mode().IsRegular() {
			if position, err := log.Seek(0, io.SeekCurrent); err == nil {
				offset = position
			}
		}
	}
	temporary := offset < 0
	if temporary {
		var err error
		log, err = os.CreateTemp(dir, "landing-check-*.log")
		if err != nil {
			return checkReport{}, fmt.Errorf("the proving command's log could not be made: %w", err)
		}
		defer os.Remove(log.Name())
		defer log.Close()
		offset = 0
	}
	base := decision.BaseCommit
	if base == "" {
		base = decision.Base
	}
	shell := exec.Command("/bin/sh", "-c", command)
	shell.Dir = dir
	// The proof's reporter asks the serving engine (`metasystem landing status`,
	// `settings show`): the engine running this lane leads the proof's PATH, so the
	// proof never depends on the environment the lane's agent happened to inherit.
	enginePath := os.Getenv("PATH")
	if executable, err := os.Executable(); err == nil {
		enginePath = filepath.Dir(executable) + string(os.PathListSeparator) + enginePath
	}
	shell.Env = append(os.Environ(), "PATH="+enginePath, "LANDING_TREE="+running.Tree, "LANDING_COMMIT="+running.Commit, "LANDING_ONLY="+only,
		"LANDING_PROOF_SCOPE="+decision.Scope, "LANDING_PROOF_BASE="+base, "LANDING_PROOF_GROUPS="+strings.Join(decision.groupIDs(), " "))
	shell.Stdout, shell.Stderr = log, log
	run := seams.Command
	if run == nil {
		run = (*exec.Cmd).Run
	}
	err := run(shell)
	observed.readLog(log, offset)
	observed.finish()
	report := observed.report
	if temporary {
		if info, statErr := log.Stat(); statErr == nil {
			_, copyErr := io.Copy(output, io.NewSectionReader(log, offset, info.Size()-offset))
			err = errors.Join(err, copyErr)
		}
		// Without the caller's seekable log the scope environment stays unknown.
		*observed = proofOutput{output: io.Discard}
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.Exited() {
			return report, fmt.Errorf("the proving command exited %d", exit.ExitCode())
		}
		return report, fmt.Errorf("the proving command ended: %w", err)
	}
	return report, nil
}

// proveInWorktree runs command in a fresh detached worktree of the lane
// repository checked out at running.Commit, from the worktree's folder that
// matches the installation's place in the checkout, and removes the
// worktree after. Worktrees a crashed proof left are removed first. Only
// this process proves (it holds running.json), so every worktree under
// proofTrees is a leftover.
func proveInWorktree(seams ProveSeams, install, checkout, command string, running Running, decision *scopeDecision, output io.Writer, observed *proofOutput, result, previous Result) Result {
	if seams.Gate && !seams.gateBaseline {
		var baseline Result
		var ok bool
		baseline, ok, *decision = gateBaseline(seams, install, checkout, command, running, output)
		if !ok {
			return baseline
		}
	}
	git := seams.git
	trees := proofTrees(install)
	removeProofTrees(git, checkout, trees, output)
	if err := os.MkdirAll(trees, 0o755); err != nil {
		result.Result, result.Reason, result.Cause = Red, err.Error(), &Cause{Kind: "environment", Name: "lost-process", Evidence: running.Log}
		if previous.Result == "" {
			result.Repeat = "allowed"
		}
		return result
	}
	tree := filepath.Join(trees, running.Attempt)
	if _, err := git(checkout, "worktree", "add", "--detach", tree, running.Commit); err != nil {
		result.Cause = &Cause{Kind: "environment", Name: "lost-process", Evidence: running.Log}
		if previous.Result == "" {
			result.Repeat = "allowed"
		}
		result.Result, result.Reason = Red, fmt.Sprintf("the worktree of commit %s could not be made: %v", Short(running.Commit), err)
		return result
	}
	defer func() {
		if _, err := git(checkout, "worktree", "remove", "--force", tree); err != nil {
			fmt.Fprintf(output, "\nlanding prove: the check's worktree stays until the next check: %v\n", err)
		}
	}()
	dir := tree
	if rel, err := filepath.Rel(checkout, install); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		dir = filepath.Join(tree, rel)
	}
	if decision.Scope == "impact" {
		if err := decision.impactPlan(seams, dir); err != nil {
			decision.ScopeReason = "impact proof error: " + err.Error()
			result.Result, result.Reason = Red, decision.ScopeReason
			result.Cause = &Cause{Kind: "environment", Evidence: running.Log}
			result.allowEnvironmentRepeat(previous)
			return result
		}
	}
	report, runErr := runCheck(seams, dir, command, running, "", *decision, output, observed)
	if decision.Scope == "impact" && strings.TrimSpace(observed.environment) == "" {
		decision.ScopeReason = "impact proof error: the environment fingerprint is empty"
		result.Result, result.Reason = Red, decision.ScopeReason
		result.Cause = &Cause{Kind: "environment", Evidence: running.Log}
		result.allowEnvironmentRepeat(previous)
		return result
	}
	if seams.Gate && report.kind == "not-run" && runErr == nil {
		runErr = fmt.Errorf("the cheap check reported that it did not run")
	}
	if (runErr == nil || report.kind == "complete") && decision.Scope == "scoped" && (observed.environment == "" || decision.base.Environment == "" || observed.environment != decision.base.Environment) {
		if runErr != nil {
			result = observeRed(seams, install, running, *decision, observed, result, previous, report, runErr)
			if result.executionErr != nil {
				return result
			}
		}
		full := *decision
		full.Scope, full.Base = "full", ""
		next := seams
		next.Person = nil
		err := withLock(install, func() error {
			if err := admitExecutionLocked(install, checkout, command, &running, full, next); err != nil {
				return err
			}
			running.setAdmissionState("claimed")
			return writeRunning(install, running)
		})
		if err != nil {
			decision.ScopeReason = "full check pending: the proof environment changed"
			result.Result, result.Reason, result.executionErr = Held, err.Error(), err
			return result
		}
		decision.Scope, decision.ScopeReason = "full", fmt.Sprintf("the proof environment changed from %q to %q", decision.base.Environment, observed.environment)
		decision.Base = ""
		*observed = proofOutput{output: io.Discard}
		report, runErr = runCheck(seams, dir, command, running, "", *decision, output, observed)
	}
	result.CountedFull = report.kind == "complete" && decision.Scope == "full"
	if previous.Result != "" {
		result.Repeat = "started"
	}
	if runErr == nil {
		if previous.Result == Red && len(previous.FlakeRepeats) == 0 && len(previous.Failed) > 0 && (previous.Cause == nil || previous.Cause.Kind != "environment") {
			return recordFlakes(seams, previous, result, "whole", []Running{running})
		}
		return result
	}
	return classifyRed(seams, install, checkout, command, dir, running, *decision, output, observed, result, previous, report, runErr,
		func(red, prior Result) Result {
			if seams.Gate {
				return replayGate(seams, install, checkout, command, running, red, prior)
			}
			return replayBatch(seams, install, checkout, command, running, red, prior)
		})
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
	gates, gateDamaged, gateErr := countedLines[Result](gatesPath(install))
	results, damaged, err = append(results, gates...), damaged+gateDamaged, errors.Join(err, gateErr)
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
