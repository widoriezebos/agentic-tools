package lane

// The lane adapter around the goal ledger's compare-and-swap (design r10
// K3): the lane's own goal transactions (returns and releases, renewal,
// next-step edits, trunk-red records, cadence status) publish through the
// same boundary as a landing, as tuples of kind ledger. A lease the lane
// loses comes back as LANE_BASE_MOVED instead of the goal transaction's
// retry loop. Seats' own goal writes never take this adapter.

import (
	"errors"
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
// live on main (single-machine mode, another sync branch), or that an
// in-process repository owns instead of git, is not a push to main and
// publishes as before.
func LedgerPublisher(home string, op Operation, authority Authority) goal.CASPublisher {
	return func(endpoint goal.Endpoint, tip, commit string) (goal.CASOutcome, error) {
		if endpoint.Repository != nil || endpoint.LocalMode() || endpoint.Branch != MainRef {
			return goal.PublishCAS(endpoint.WithCASPublisher(nil), tip, commit)
		}
		// With no lane registered nothing is lane-scoped (no-lane mode is
		// first-class): the write publishes as the checkout's own. A lane
		// record that can't be read publishes nothing.
		_, registered, err := Read(home)
		var refusal *Refusal
		if err != nil && errors.As(err, &refusal) && refusal.Code == CodeRecordIncomplete && op == OpReturn && authority == AuthorityPerson {
			// A person's cleanup of a lane an older engine registered: that
			// lane has no hook and no layout to bind a tuple to, so its
			// returns publish as before and the unset can end.
			return goal.PublishCAS(endpoint.WithCASPublisher(nil), tip, commit)
		}
		if err != nil {
			return goal.CASRefused, &PublishError{Code: CodePublishRefused, Expected: tip,
				Message: "the landing lane's record can't be read, so the lane's ledger write was not published", Detail: err.Error()}
		} else if !registered {
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
			// Every refusal is final for this attempt, a gate's (paused,
			// fenced, another checkout) too: it stops the retry loop and
			// carries its reason.
			var refused *PublishError
			if errors.As(err, &refused) {
				return goal.CASRefused, err
			}
			wrapped := &PublishError{Code: CodePublishRefused, Expected: tip, Message: err.Error()}
			var gate *Refusal
			if errors.As(err, &gate) {
				wrapped.Code, wrapped.Message, wrapped.Detail = gate.Code, gate.Message, gate.Fix
			}
			return goal.CASRefused, wrapped
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
	// The lane's own ledger writes leave from its installation, in its
	// checkout; any other endpoint is not the lane's.
	layout, err := record.Layout()
	if err != nil {
		return Tuple{}, err
	}
	if at := resolved(endpoint.Root); at != string(layout.Install) && at != string(layout.Checkout) {
		return Tuple{}, fmt.Errorf("the ledger write at %s is not the landing lane's (%s)", endpoint.Root, layout.Install)
	}
	repo := string(layout.Checkout)
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
