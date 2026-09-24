package dispatch

import (
	"fmt"

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
			return fmt.Errorf("proof admission reads missing %s", field.name)
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
}

func concreteGoalAdmissionReads() goalAdmissionReads {
	return goalAdmissionReads{
		NewWorld: goal.NewWorld, ResolveEndpoint: goal.ResolveEndpoint, ResolveMachine: goal.ResolveMachine,
		Receipt: concreteReceiptAdmissionReads(),
	}
}
