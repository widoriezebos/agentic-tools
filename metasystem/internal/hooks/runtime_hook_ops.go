package hooks

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Ops are the owner operations the runtime hook composes. Each method is one
// engine call the former scripts/agents/supervision-hook.sh made through the
// binary; the engine now makes it as a function call. A method returns what
// that call printed and its exit status, because the hook's decisions are
// taken on exactly those (an absent delegate is status 3, a checkout holder
// that owns no wait rows is 64, a refused installation is 1). The production
// binding lives beside the verb handlers in cmd/metasystem; tests supply a
// fake per test.
type Ops interface {
	// RuntimeNames is the registered runtime list, one name per line.
	RuntimeNames() (string, int)
	// StateRoot validates one installation and returns its state root.
	StateRoot(installation string) (string, int)
	// HookDelegate proves exact delegate custody for a hook caller: the
	// custody JSON with status 0, status 3 when the caller is no delegate.
	HookDelegate(root, metasystemRoot, job string, callerPid int) (string, int)
	// FindAncestor walks up from pid to the first agent runtime ancestor.
	FindAncestor(repo string, pid int, runtime string, allHosts bool) (string, int)
	// Classify classifies a caller against the checkout lease.
	Classify(root, metasystemRoot string, callerPid int) (string, int)
	// StartContext is a runtime's session-start context declaration.
	StartContext(runtime string) (string, int)
	// StewardPending names undelivered steward incidents in one line.
	StewardPending(repo string) (string, int)
	// BrainBoot composes the read-only brain context packet as JSON.
	BrainBoot(ctx context.Context, root, repo string, bytes, deadlineMS int) (stdout, stderr string, status int)
	// BrainStartDelivered records a published SessionStart packet.
	BrainStartDelivered(root, repo, declarationSHA string, digestCursor, digestPrefix string) int
	// Up arms supervision, or retires or recovers it, as `up` does.
	Up(request UpRequest, stdout, stderr io.Writer) int
	// SessionStart prints the durable wait rows a holder session recovers.
	SessionStart(root, session string, stdout, stderr io.Writer) int
	// SessionEnd retires the session's unused Stop authorization.
	SessionEnd(root, session string) int
	// HookAttempt records a Stop attempt before turn work.
	HookAttempt(repo string, pid int, turnKey string) (string, int)
	// HookComplete records a Stop completion after payload emission.
	HookComplete(request HookCompletion) int
	// HookExpire records a Stop attempt the deadline parent expired.
	HookExpire(repo string, elapsedSec int64) int
	// HealthPreview renders the hook's health preview JSON.
	HealthPreview(repo, metasystemRoot string) (string, int)
	// DigestPending reads the narrator digest since the last check-in;
	// its diagnostic, when it fails, is the output.
	DigestPending(repo string) (string, int)
	// DigestAdvance advances the narrator digest after emission.
	DigestAdvance(repo string, cursor, prefix string) int
	// ReceiptCheck reports whether a retro is due (status 1).
	ReceiptCheck(root string) (stdout, stderr string, status int)
	// ProtocolGrowth reports protocol errors a main has not seen.
	ProtocolGrowth(root, mainID string) (string, int)
	// ProtocolAdvance merges a main's protocol-error counts into its cursor.
	ProtocolAdvance(root, mainID string, callerPid int, counts string) int
	// RenewLease bumps the holder's lease revision.
	RenewLease(root string, callerPid int) int
	// WatchdogReport reports stale census, untracked processes and dead components.
	WatchdogReport(repo string) (string, int)
	// TurnVerdict is the one structured turn-end decision.
	TurnVerdict(request TurnVerdictRequest) (stdout, stderr string, status int)
	// StopBlock renders a Stop refusal, recording it when the request names a record.
	StopBlock(request StopBlockRequest) (string, int)
	// StopInput composes the Stop presentation input from captured files.
	StopInput(request StopInputRequest, stderr io.Writer) int
	// StopPresent publishes the immutable Stop report and its presentation.
	StopPresent(root, inputFile, outputFile string, stderr io.Writer) int
	// StopOutput maps one Stop presentation to the runtime's payload.
	StopOutput(runtime, inputFile, outputFile string, stderr io.Writer) int
	// ChannelStatusPost posts the brain's status line.
	ChannelStatusPost(root string, stdout, stderr io.Writer) int
	// EvidenceGC runs the installation's hook evidence collection.
	EvidenceGC(installation string, output io.Writer) int
	// Slug is the stable slug of a session, as announcement names use it.
	Slug(value string) string
	// TokenHex is a fresh random hexadecimal token of the given bytes.
	TokenHex(bytes int) (string, error)
	// Git runs git with inherited repository steering removed.
	Git(args ...string) (string, error)
	// EngineBehind reports whether the installation's enrolled engine was
	// built from sources the checkout's landed tree has moved past.
	EngineBehind(installation, repo string) (bool, error)
	// UnmigratableRetainedPlans names retained executable plans whose stored
	// argv a rebuilt engine could not run. The rearm is held while any exist.
	UnmigratableRetainedPlans(repo string) ([]string, error)
	// RebuildEngine rebuilds the installation's engine in place.
	RebuildEngine(installation string) error
}

// UpRequest is one `up` invocation of the hook.
type UpRequest struct {
	Runtime          string
	MetasystemRoot   string
	Repo             string
	Session          string
	Pid              string
	StartTime        string
	Tag              string
	RuntimeSession   string
	NoRuntimeSession bool
	StartSource      string
	RecoverOnly      bool
	IfDown           bool
	Retire           bool
	// CallerPid is the process `up` proves descends from the session pid:
	// the hook itself, as when `up` ran as the hook's child.
	CallerPid int
}

// HookCompletion is one completion record of a Stop attempt.
type HookCompletion struct {
	Repo         string
	Generation   string
	Attempt      string
	Result       string
	Outcome      string
	HealthLine   string
	PayloadFile  string
	Installation string
	ReportID     string
	ReportAlias  string
	ReportPath   string
	ReportSHA256 string
	ElapsedSec   int64
	HasElapsed   bool
}

// TurnVerdictRequest is one turn verdict of a Stop.
type TurnVerdictRequest struct {
	Root           string
	Session        string
	SessionAbsent  bool
	Watchdog       string
	MainID         string
	StopHookActive bool
	Transcript     string
	Runtime        string
	FactsFile      string
	CompletionFile string
}

// StopBlockRequest is one Stop refusal rendering. Without RefusalRecord it
// renders an unrecorded block; with it, the recorded infrastructure refusal.
type StopBlockRequest struct {
	Detail        string
	SystemMessage string
	BoundedIdle   bool
	Class         string
	RefusalRecord string
	Session       string
	Cause         string
	Remedy        string
	ArmingResult  string
	OpenWorkRoot  string
}

// StopInputRequest names the captured files one Stop presentation composes.
type StopInputRequest struct {
	Root, Runtime, Session, Attempt, MainID, Machine, Lineage string
	ClaimEpoch                                                string
	Advisor                                                   bool
	VerdictFile, FactsFile, CompletionFile                    string
	HealthFile, DigestFile, DigestCursorPrefix                string
	ReceiptFile, ReceiptStderrFile                            string
	ReceiptExit                                               int
	ArmingFile, ArmingStderrFile                              string
	ArmingExit                                                int
	NoticeFile, FailureFile, OutputFile                       string
}

// Worker is the Stop worker the deadline parent launched. The launcher
// reaps it; Done closes once it has exited and Status is then its exit
// status.
type Worker interface {
	Pid() int
	Done() <-chan struct{}
	Status() int
}

// Invocation is one runtime lifecycle hook call and the process facts it
// runs with. Production fills it from the process; tests supply each field.
type Invocation struct {
	Runtime, Event string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	// Lookup reads the process environment.
	Lookup func(string) (string, bool)
	// Pid and Ppid are this process and its parent: the parent is the
	// runtime (or the Stop deadline parent for a worker).
	Pid, Ppid int
	// Script is the stub's path as it was invoked; relative paths resolve
	// against the working directory, exactly as the stub resolved them.
	Script string
	// Now is the wall clock.
	Now func() time.Time
	// Monotonic is elapsed time since an arbitrary origin, for budgets.
	Monotonic func() time.Duration
	// After fires once after the duration.
	After func(time.Duration) <-chan time.Time
	// Sleep pauses between polls.
	Sleep func(time.Duration)
	// Signals delivers HUP, INT and TERM to SessionStart; nil means none.
	Signals <-chan os.Signal
	// Exec replaces this process (the Claude tool gate, the bootstrap
	// restart); it returns only on failure.
	Exec func(path string, argv []string, env []string) error
	// Environ is the environment an exec or a worker inherits.
	Environ func() []string
	// StartWorker launches the Stop worker: the stub again, for this
	// runtime, with the deadline parent's markers in its environment.
	StartWorker func(script, runtime string, env []string, stdin, stdout, stderr *os.File) (Worker, error)
	// IsExecutable reports whether a path is an executable file.
	IsExecutable func(path string) bool
	// TempDir is where the hook stages files.
	TempDir string
	// Deadline carries the Stop deadline owner's process dependencies.
	Deadline DeadlineDeps
}

// DeadlineDeps are the process-identity dependencies of the Stop deadline
// parent: the boot clock, the kernel prober, the signal sender and the
// polling event source the wait and cleanup owners take.
type DeadlineDeps struct {
	BootClock func() (string, time.Duration, error)
	Prober    identity.Prober
	Signal    identity.SignalFunc
	ParentPid func(int64) (int64, bool)
	// EventInterval is the process-state polling interval.
	EventInterval time.Duration
	// FixtureDeadline, when set, may replace the deadline timer with a
	// fixture event for this installation (nil channel: no fixture). An
	// error refuses the typed wait, as an unauthorized fixture event did.
	FixtureDeadline func(ctx context.Context, installation string) (<-chan time.Time, error)
}
