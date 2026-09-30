// Package landpath is the landing path: the commit boundary every agent
// commit and landing passes (formerly scripts/agents/commit.sh), the driver
// that stages, commits, rebases, proves and pushes one landing (formerly
// scripts/agents/land.sh, including its carried transaction), and the
// pre-commit guard body (formerly scripts/agents/pre-commit-guard.sh).
//
// It is a composition package above the owners it orchestrates
// (plans/designs/verbs-object-action.md 6.3): every owner decision (lease
// holdership, the brain fence, retained proof, the provenance observation,
// held goals, drift, the carry ledger) is reached through Owners, which the
// command layer wires to the owner functions and tests replace per instance.
// Git is reached only through Owners.Git, so behavior tests run with Git
// stubbed.
package landpath

import (
	"io"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

// GitCall is one git invocation: git -C Dir Args..., with Stdin fed when set.
// Env, when non-nil, is the complete environment; nil inherits the process's.
type GitCall struct {
	Dir   string
	Args  []string
	Env   []string
	Stdin []byte
}

// GitResult is what one git invocation wrote and its exit status; Code is -1
// when git could not be started.
type GitResult struct {
	Stdout []byte
	Stderr []byte
	Code   int
}

// Git runs one git invocation.
type Git func(GitCall) GitResult

// ObserveRequest is one provenance observation of a prospective tree: the
// landing's declarations as the boundary passes them to the evaluator.
type ObserveRequest struct {
	Root, Tree                                 string
	Chain, Attested, AttestedSnapshot          string
	AttestedBase, DirectFix, RevertOf, Goal    string
	RootJob, TestReceipt, Recertification      string
	Carried, ProjectTree, LedgerTip, CarriedBy string
	Judge, LiveFailure, Actor                  string
}

// Judge is the engine that decides a landing: the live engine in this
// process, or, for a carried landing whose live engine could not decide, the
// base engine built from HEAD. Observe returns the observation and the
// evaluator's exit status (0 when it printed one); Workspace projects a
// whole-project tree; VerifyCarried returns the structured delivery result
// of a carried verification and its exit status.
type Judge struct {
	Observe       func(ObserveRequest) (landing.Observation, int)
	Workspace     func(root, tree string) (string, error)
	VerifyCarried func(root, tree, goal string) ([]byte, int)
	// Digest is the SHA-256 of the deciding engine's bytes.
	Digest string
}

// VerifyRequest is one read-only verification of retained delivery proof for
// an exact whole-project tree (the owner of `test status --tree`).
type VerifyRequest struct {
	Root, Tree, Goal string
}

// BootSample is one boot-clock sample taken before a publication.
type BootSample struct {
	ID    string
	Nanos int64
}

// Owners are the owner functions and effects the landing path reaches. The
// command layer wires production owners; every test supplies its own.
type Owners struct {
	Git Git
	// CallerPID is the process identity lease classification starts from:
	// this process, the parent the former wrapper child observed.
	CallerPID int64
	// Getpid is the landing process's own pid (the wrapper-token owner).
	Getpid func() int64
	// LiveEngine is the path of the engine running the landing; its digest
	// names the live judge.
	LiveEngine func() (string, error)
	Now        func() time.Time
	// Environ is this process's environment, the base of the environment
	// of the git commit and push the landing runs.
	Environ func() []string

	RequireHolder func(root string, caller int64, epoch *int64) (*int64, error)
	WithHeld      func(root string, caller int64, epoch *int64, fn func() error) error
	BrainFence    func(root, act string) (string, error)
	// ConfValue reads one key of root/metasystem.conf; absent is "".
	ConfValue func(root, key string) string
	// StartedAt is a live pid's kernel start second.
	StartedAt  func(pid int64) (int64, error)
	TokenNonce func() (string, error)
	WriteToken func(path string, token WrapperToken) error
	RemoveFile func(path string) error
	ReadFile   func(path string) ([]byte, error)
	// FileReadable reports a readable regular file.
	FileReadable func(path string) bool
	FileExists   func(path string) bool

	// Verify prints the verification of retained delivery proof for one tree
	// to stdout and stderr exactly as `test status --tree` does and returns
	// its exit status.
	Verify func(request VerifyRequest, stdout, stderr io.Writer) int
	// SelectLanding filters paths through the LANDING projection.
	SelectLanding func(paths []string, prefix string) ([]string, error)
	// Live is the live judge (this engine).
	Live func() Judge
	// BuildBaseJudge builds the engine at HEAD in a scratch worktree of
	// toplevel (prefix names the metasystem directory inside it) and returns
	// it as a judge with its cleanup; build output goes to stderr.
	BuildBaseJudge func(toplevel, prefix string, stderr io.Writer) (Judge, func(), error)
	Held           func(root, base, commit, remote, ref string, stdout, stderr io.Writer) int
	// LandingGate reads a goal's landing gate (g1-s70 D2) against a freshly
	// fetched ledger, immediately before each push of a landing in that
	// goal's name: nil lets the push go, an error is the refusal a person
	// reads, its register code and the human verb that carries past it. A
	// nil owner reads no gate.
	LandingGate func(root, goal string) error
	// RecordRelease, when set, records the goal's release set for the
	// commit about to be pushed (branch is the pushed branch), before each
	// push; ReleaseLanded releases it once the push succeeded
	// (disk-lifetimes Part B 3.6, the staged route). A failure to record
	// is reported and never stops the landing: nothing is then released.
	RecordRelease func(commit, branch string) error
	ReleaseLanded func(commit string)
	WeightAdd     func(root, commit, prefix, goal string, numstat []byte, stdout, stderr io.Writer) int
	SyncTransport func(root, branch string, stdout, stderr io.Writer) int

	// The land driver's owners.
	Drift                func(root string, requireEmptyIndex bool, stdout, stderr io.Writer) int
	Advance              func(root, upstream string, stdout, stderr io.Writer) int
	ReceiptLine          func(root, tree, goal, directFix string) (ReceiptDecision, error)
	TestReceipt          func(root, tree, command string, stdout, stderr io.Writer) int
	JobGateWidth         func(root, chain string) string
	RecertificationFacts func(path string) (RecertificationFacts, error)
	Park                 func(ParkRequest) (string, error)
	OutputSpill          func(root, verb, ext string, data []byte) (string, error)
	BootClock            func() (BootSample, error)
	NotifyGoal           func(root, goal, publication string, began *BootSample)
	// Goal ledger owners of the carried transaction; each returns what the
	// former verb printed and its exit status.
	GoalFetch       func(root string) (string, int)
	CarryStatus     func(root, carried, goal, ledgerTip string) (landing.CarryStatus, string, int)
	GoalCarrying    func(request CarryingRequest) (string, int)
	GoalCarried     func(request CarriedRequest) (string, int)
	ChannelAskCarry func(root, goal, wants, fact string)
	// Seam is a named point of the carried transaction a test may stop at;
	// production leaves it nil.
	Seam func(point string)
}

// ReceiptDecision is the receipt-line owner's decision for a staged landing.
type ReceiptDecision struct {
	Refused bool
	Detail  string
	// Encoded is the decision as the owner encodes it.
	Encoded string
}

// RecertificationFacts are the transport facts of a recertification record.
type RecertificationFacts struct {
	TargetCommit, SourceAnchorRef, MergedAnchorRef string
}

// ParkRequest durably records one stopped recertified landing attempt.
type ParkRequest struct {
	Root, Chain, Target, Reason, Detail, Recertification, CandidateCommit string
	RecoveryRefs                                                          []string
	CallerPID                                                             int64
}

// CarryingRequest is one call of the carry reservation owner: a reservation
// (Ref and Tree), a local pre-push intent (Carrying and Commit), or an
// abandonment (Abandon).
type CarryingRequest struct {
	Root, Goal, Ref, Tree, By                       string
	Carrying, Commit, Workspace, Past, Battery      string
	Missing, Failing, Judge, JudgeTree, JudgeDigest string
	LiveFailure, Ledger                             string
	OwnerPID                                        int64
	Abandon, Why                                    string
	Lineage                                         string
}

// CarriedRequest completes (Entry), rebuilds (RebuildFromCommit with Goal
// and Ref) or repairs the counselor line of (RepairCounselor with Ref) one
// carried landing record.
type CarriedRequest struct {
	Root, Entry, RebuildFromCommit, Goal, Ref string
	RepairCounselor                           bool
	Lineage                                   string
}
