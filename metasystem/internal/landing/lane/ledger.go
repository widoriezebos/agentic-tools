package lane

// The lane adapter around the goal ledger's compare-and-swap (design r10
// K3): the lane's own goal transactions (returns and releases, renewal,
// next-step edits, trunk-red records, cadence status) publish through the
// same boundary as a landing, as tuples of kind ledger. A lease the lane
// loses comes back as LANE_BASE_MOVED instead of the goal transaction's
// retry loop. Seats' own goal writes never take this adapter.

import (
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// LedgerEndpoint is endpoint routed through the lane's publication
// boundary, as op for authority: the lane's agent publishes with OpPublish,
// a person's cleanup return with OpReturn.
func LedgerEndpoint(home string, endpoint goal.Endpoint, op Operation, authority Authority) goal.Endpoint {
	return endpoint.WithCASPublisher(LedgerPublisher(home, op, authority))
}

// LedgerPublisher is the lane's goal CAS publisher. A ledger that does not
// live on main (single-machine mode, another sync branch) is not a write to
// main and pushes as before.
func LedgerPublisher(home string, op Operation, authority Authority) goal.CASPublisher {
	return func(endpoint goal.Endpoint, tip, commit string) (goal.CASOutcome, error) {
		if endpoint.LocalMode() || endpoint.Branch != MainRef {
			return goal.PublishCAS(endpoint.WithCASPublisher(nil), tip, commit)
		}
		tuple, err := ledgerTuple(home, endpoint, tip, commit)
		if err != nil {
			return goal.CASRefused, &PublishError{Code: CodePublishRefused, Expected: tip,
				Message: "the lane's ledger write could not be bound to its publication, so nothing was published", Detail: err.Error()}
		}
		switch err := Publish(home, tuple, op, authority); {
		case err == nil:
			return goal.CASLanded, nil
		case isUnknown(err):
			return goal.CASUnknown, err
		default:
			return goal.CASRefused, err
		}
	}
}

func isUnknown(err error) bool {
	refused, ok := err.(*PublishError)
	return ok && refused.Code == CodePublishUnknown
}

// ledgerTuple binds one ledger commit to its tuple: the registered lane's
// checkout, the endpoint's remote URL, the commit's tree, its goal
// transaction and the lane engine's enrolled generation.
func ledgerTuple(home string, endpoint goal.Endpoint, tip, commit string) (Tuple, error) {
	record, ok, err := Read(home)
	if err != nil {
		return Tuple{}, err
	}
	if !ok {
		return Tuple{}, fmt.Errorf("no landing lane is registered on this computer")
	}
	// The write leaves from the checkout that built it; the gate refuses it
	// unless that is the lane's.
	repo, err := laneGit(endpoint.Root, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return Tuple{}, fmt.Errorf("the checkout of %s: %w", endpoint.Root, err)
	}
	url := endpoint.Remote
	if out, err := laneGit(endpoint.Root, nil, "remote", "get-url", endpoint.Remote); err == nil && out != "" {
		url = out
	}
	tree, err := laneGit(endpoint.Root, nil, "rev-parse", "--verify", "--quiet", commit+"^{tree}")
	if err != nil {
		return Tuple{}, fmt.Errorf("the tree of %s: %w", commit, err)
	}
	message, err := laneGit(endpoint.Root, nil, "log", "-1", "--format=%(trailers:key=Goal-Transaction,valueonly)", commit)
	if err != nil {
		return Tuple{}, fmt.Errorf("the goal transaction of %s: %w", commit, err)
	}
	opid := strings.TrimSpace(message)
	if opid == "" || strings.Contains(opid, "\n") {
		return Tuple{}, fmt.Errorf("commit %s names no one goal transaction", commit)
	}
	enrolled, err := steward.VerifyIdentity(steward.RepoIdentityPath(record.Install), record.Install)
	if err != nil {
		return Tuple{}, fmt.Errorf("the lane engine's enrollment: %w", err)
	}
	return Tuple{Repo: repo, RemoteURL: url, Ref: MainRef, Old: tip, New: commit, Tree: tree,
		Kind: KindLedger, Op: opid, Generation: enrolled.Generation}, nil
}
