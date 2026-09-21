// Package snapshot answers what this checkout's accepted ledger says, right
// now, without moving anything. A request reads the accepted ref as it
// stands; the ref is carried forward by this package's own loop, on a
// schedule, never by a request. The validated tree of one commit is kept
// because it cannot change while the tip stands still; everything that can
// change under a standing tip — the clock, the horizon, the git
// configuration, the claim gate's answer — is read again every time.
package snapshot

import (
	"errors"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// State is what could be read of the accepted ledger.
type State string

const (
	// StateRead is a tip whose tree every at-rest rule accepts.
	StateRead State = "read"
	// StateAbsent is a clone with no accepted ref. Neither git clone nor git
	// fetch brings it, so this is where a new clone starts.
	StateAbsent State = "absent"
	// StateNoLedger is a ref pointing at a commit whose tree carries no root
	// record. The engine never creates such a ref.
	StateNoLedger State = "no-ledger"
	// StateBroken is a ref that cannot be read, or a tip whose tree cannot be
	// read from git.
	StateBroken State = "broken"
	// StateUnreadable is a tip whose tree the at-rest rules refuse. The
	// problems are kept, one per rule, and the tree is refused whole.
	StateUnreadable State = "unreadable"
	// StateRefused is a configuration the engine will not project from: an
	// unqualified sync branch, or a tip whose root record contradicts this
	// clone's configured sync mode.
	StateRefused State = "refused"
)

// Observation is one answer about the accepted ledger, complete in itself.
type Observation struct {
	ObservedAt  time.Time
	StateRoot   string
	State       State
	Tip         string
	CommittedAt time.Time

	// Message and Problems are the engine's own words. Problems are the typed
	// at-rest problems; Message is the whole refusal.
	Message  string
	Problems []goal.Problem

	Tree      *goal.TreeGoals
	Horizon   goal.ApprovalHorizon
	SyncMode  string
	Admission backlog.Admission

	// LiveFiles and ArchivedFiles count the checkout's goal files where no
	// accepted tip can be read. Nil is "not known", never zero.
	LiveFiles, ArchivedFiles *int

	// Fetch is the freshness loop's state as it stands. It is present in
	// every ledger state, because what the loop last found is most worth
	// saying exactly when the ledger cannot be read.
	Fetch FetchState
}

// Holder answers observations and owns the freshness loop's state.
type Holder struct {
	root string
	now  func() time.Time

	mu sync.Mutex
	// held is the one commit whose read is kept: its validated tree, or its
	// refusal, both immutable facts of that tip.
	held held
	// The loop's shared state. wake carries at most one pending signal: a
	// request tells the loop its cadence changed and waits for nothing.
	fetch       FetchState
	lastObserve time.Time
	next        time.Time
	stopped     bool
	wake        chan struct{}
}

type held struct {
	tip     string
	read    goal.ValidatedTree
	refusal *goal.TreeReadError
}

// New builds a holder over one state root: the root every ledger read takes.
func New(stateRoot string, now func() time.Time) *Holder {
	if now == nil {
		now = time.Now
	}
	return &Holder{
		root:  stateRoot,
		now:   now,
		wake:  make(chan struct{}, 1),
		fetch: FetchState{Outcome: OutcomeNever},
	}
}

// Observe answers from the accepted ref as it stands. It starts no fetch,
// waits for none, and moves nothing; its only effect on the loop is to say a
// browser is connected, which shortens the loop's cadence.
func (h *Holder) Observe() Observation {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := h.now().UTC()
	h.markConnectedLocked(now)

	observation := Observation{ObservedAt: now, StateRoot: h.root, Fetch: h.fetch}

	tip, exists, err := goal.AcceptedLedgerTip(h.root)
	switch {
	case err != nil:
		observation.State, observation.Message = StateBroken, err.Error()
		return observation
	case !exists && tip == "":
		observation.State = StateAbsent
		observation.LiveFiles, observation.ArchivedFiles = workingTreeCounts(h.root)
		return observation
	case !exists:
		observation.State, observation.Tip = StateNoLedger, tip
		observation.LiveFiles, observation.ArchivedFiles = workingTreeCounts(h.root)
		return observation
	}
	observation.Tip = tip

	read, refusal, err := h.readTreeLocked(tip)
	switch {
	case refusal != nil:
		observation.State = StateUnreadable
		observation.Message = refusal.Error()
		observation.Problems = refusal.Problems
		return observation
	case err != nil:
		observation.State, observation.Message = StateBroken, err.Error()
		return observation
	}

	// Configuration is read every observation: git config and the tier-box
	// files change independently of the tip, and a verdict cached with the
	// tree would keep answering for a world that has moved.
	endpoint, err := goal.ResolveEndpoint(h.root)
	if err != nil {
		observation.State, observation.Message = StateRefused, err.Error()
		return observation
	}
	if err := goal.SyncModeGate(endpoint, tip); err != nil {
		observation.State, observation.Message = StateRefused, err.Error()
		return observation
	}

	observation.State = StateRead
	observation.Tree = read.Tree
	observation.CommittedAt = read.CommittedAt
	if read.Tree.Root != nil {
		observation.SyncMode = read.Tree.Root.SyncMode
	}
	observation.Horizon = goal.NewApprovalHorizon(read.Tree, now)
	observation.Admission = backlog.Admit(goal.Projection{
		Root: h.root, Tip: tip, Tree: read.Tree, Horizon: observation.Horizon,
	})
	return observation
}

// readTreeLocked answers for one tip from the kept read when it is that tip's,
// and reads it through the engine otherwise. A commit's refusal is kept beside
// its tree: both are permanent facts of that commit, and re-reading a torn
// tree on every request would neither change the answer nor help anyone.
func (h *Holder) readTreeLocked(tip string) (goal.ValidatedTree, *goal.TreeReadError, error) {
	if h.held.tip == tip {
		return h.held.read, h.held.refusal, nil
	}
	read, err := goal.ReadValidatedTree(h.root, tip)
	var refusal *goal.TreeReadError
	if errors.As(err, &refusal) {
		h.held = held{tip: tip, refusal: refusal}
		return goal.ValidatedTree{}, refusal, nil
	}
	if err != nil {
		return goal.ValidatedTree{}, nil, err
	}
	h.held = held{tip: tip, read: read}
	return read, nil, nil
}
