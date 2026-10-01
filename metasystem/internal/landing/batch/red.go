package batch

import (
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// DiagnosticRequest is one fresh, charged classification run. Tree is always
// derived from the durable batch record and NeverReuse must remain true.
type DiagnosticRequest struct {
	Tree, GoalID string
	Groups       []string
	Claim        Claim
	NeverReuse   bool
	// Fresh is the adapters' fresh-execution argv for a run whose purpose is
	// to execute, such as the second base run.
	Fresh []string
}

// DiagnosticResult is the evidence needed to classify one diagnostic tree:
// its red groups and every group's execution evidence.
type DiagnosticResult struct {
	AttemptID string
	Groups    []RedGroup
	Evidence  []GroupEvidence
	Sample    proofrun.LoadSample
}

// GroupEvidence is how one group of a diagnostic run ended.
type GroupEvidence struct {
	ID, Status, ExecutionIdentity, LogPath, LogDigest string
	NativeLaunched, CollectionComplete                bool
}

// DiagnosticRefusal identifies a refusal before a diagnostic runner started.
type DiagnosticRefusal struct{ Status string }

// RedSeams names the effects outside classification and tree derivation.
type RedSeams struct {
	Run func(DiagnosticRequest) (DiagnosticResult, error)
	// ConfirmFenced re-reads the live accepted ledger and binds a stop fence to
	// this exact joined, handed-over claim. A runner refusal alone is not proof.
	ConfirmFenced func(Unit) (string, bool, error)
	MintOpid      func() (string, error)
	Ledger        LedgerOwner
	UpdateNext    func(goalID, status string) error
	BaseCommit    string
	// Adapter names a red group's language adapter and whether the group is a
	// package-selection expansion, whose manifest is the whole module; a nil
	// adapter means the group's language facts are unknown.
	Adapter func(RedGroup) (language adapter.Adapter, expansion bool)
	// Sources is the retained verifier on the tip tree: per selected group,
	// the attempt that holds its pass.
	Sources func(Record) (map[string]string, error)
	// Location is where a known flake's allowance is counted and shown.
	Location *time.Location
	// LaneOwner names who registered the lane and whether that is a person:
	// the owner of a flake found in a batch of changes alone (U11b).
	LaneOwner func() (name string, person bool)
}

func joinedUnits(units []Unit) []Unit {
	return slices.DeleteFunc(slices.Clone(units), func(unit Unit) bool { return unit.State != UnitJoined })
}

// MaxComposedVerifierAttempts bounds how often the composed path runs the
// retained verifier for one tip proof before every member returns: each
// failed try is counted on the record, so an owner restart keeps the count.
const MaxComposedVerifierAttempts = 3
