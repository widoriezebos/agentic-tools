package dispatch

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ReceiptAdmissionSource names the repository coordinates used by receipt policy.
type ReceiptAdmissionSource = receiptAdmissionReads

// ProofAdmissionReads binds one proof admission to its repository facts.
type ProofAdmissionReads struct {
	ResolveEndpoint func(string) (goal.Endpoint, error)
	ResolveMachine  func(string) (string, error)
	Receipt         ReceiptAdmissionSource
}

func ConcreteProofAdmissionReads() ProofAdmissionReads {
	concrete := concreteGoalAdmissionReads()
	return ProofAdmissionReads{ResolveEndpoint: concrete.ResolveEndpoint, ResolveMachine: concrete.ResolveMachine, Receipt: concrete.Receipt}
}

func (r ProofAdmissionReads) Validate() error {
	for _, field := range []struct {
		name    string
		missing bool
	}{
		{"ResolveEndpoint", r.ResolveEndpoint == nil},
		{"ResolveMachine", r.ResolveMachine == nil},
		{"Receipt.AcceptedLedgerTip", r.Receipt.AcceptedLedgerTip == nil},
		{"Receipt.TopLevel", r.Receipt.TopLevel == nil},
		{"Receipt.FileAt", r.Receipt.FileAt == nil},
	} {
		if field.missing {
			return fmt.Errorf("the check run cannot be admitted: %s is missing", field.name)
		}
	}
	return nil
}

func (r ProofAdmissionReads) private() goalAdmissionReads {
	return goalAdmissionReads{ResolveEndpoint: r.ResolveEndpoint, ResolveMachine: r.ResolveMachine, Receipt: r.Receipt}
}

// receiptAdmissionReads supplies immutable receipt bytes and the repository
// coordinates used to locate them. Policy still computes the receipt path.
type receiptAdmissionReads struct {
	AcceptedLedgerTip func(string) (string, bool, error)
	TopLevel          func(string) (string, error)
	FileAt            func(string, string, string) ([]byte, bool, error)
}

func concreteReceiptAdmissionReads() receiptAdmissionReads {
	return receiptAdmissionReads{
		AcceptedLedgerTip: goal.AcceptedLedgerTip,
		TopLevel:          func(root string) (string, error) { return (gittree.Workspace{Dir: root}).TopLevel() },
		FileAt: func(root, tip, path string) ([]byte, bool, error) {
			return (gittree.Workspace{Dir: root}).FileAt(tip, path)
		},
	}
}

// goalAdmissionReads supplies the three repository facts used before policy reads.
// Each call carries its own value so concurrent admissions cannot share a fixture.
type goalAdmissionReads struct {
	NewWorld        func(string) bool
	ResolveEndpoint func(string) (goal.Endpoint, error)
	ResolveMachine  func(string) (string, error)
	Receipt         receiptAdmissionReads
	// LandingBatched says whether the goal's change waits in a landing
	// batch; nil reads no batch.
	LandingBatched func(root, goalID string, now time.Time) bool
}

// GoalInLandingBatch is the seam the landing lane wires once: whether a
// goal's change is a live member of a landing batch of root's lane. Nil in
// the library: no batch holds any goal.
var GoalInLandingBatch func(root, goalID string, now time.Time) bool

func concreteGoalAdmissionReads() goalAdmissionReads {
	return goalAdmissionReads{
		NewWorld: goal.NewWorld, ResolveEndpoint: goal.ResolveEndpoint, ResolveMachine: goal.ResolveMachine,
		Receipt: concreteReceiptAdmissionReads(), LandingBatched: GoalInLandingBatch,
	}
}
