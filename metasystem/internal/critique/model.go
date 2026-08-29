// Package critique owns the rigor classification of material critic findings.
package critique

import "strings"

// RigorClass determines how a material finding constrains later critique.
type RigorClass string

const (
	Severe   RigorClass = "severe"
	Bounded  RigorClass = "bounded"
	Unproven RigorClass = "unproven"
)

// EvidenceFacts state the boundaries relevant to a rigor classification.
type EvidenceFacts struct {
	Local                             bool `json:"local"`
	Recoverable                       bool `json:"recoverable"`
	ProofBoundaryCrossed              bool `json:"proofBoundaryCrossed"`
	AuthorityBoundaryCrossed          bool `json:"authorityBoundaryCrossed"`
	SecretsBoundaryCrossed            bool `json:"secretsBoundaryCrossed"`
	IrreversibleDataBoundaryCrossed   bool `json:"irreversibleDataBoundaryCrossed"`
	ExternalSideEffectBoundaryCrossed bool `json:"externalSideEffectBoundaryCrossed"`
}

// RigorRow is the version-three wire declaration joined to one material
// finding.
type RigorRow struct {
	FindingID        string        `json:"findingId"`
	RigorClass       RigorClass    `json:"rigorClass"`
	Facts            EvidenceFacts `json:"facts"`
	ReopeningTrigger string        `json:"reopeningTrigger"`
}

// Valid reports whether the class is one of the wire protocol's declared
// classes.
func (c RigorClass) Valid() bool {
	switch c {
	case Severe, Bounded, Unproven:
		return true
	default:
		return false
	}
}

// FailsClosed reports whether the class keeps the severe critique discipline.
// An invalid class also fails closed when this predicate is used before
// normalization.
func (c RigorClass) FailsClosed() bool {
	return c != Bounded
}

// Dangerous reports whether the facts prove that a finding is not confined
// to a local, recoverable change or crosses a protected boundary.
func (f EvidenceFacts) Dangerous() bool {
	return !f.Local || !f.Recoverable || f.ProofBoundaryCrossed ||
		f.AuthorityBoundaryCrossed || f.SecretsBoundaryCrossed ||
		f.IrreversibleDataBoundaryCrossed || f.ExternalSideEffectBoundaryCrossed
}

// Normalize returns the effective class. A malformed or unknown class has the
// same effect as an unproven declaration before facts are considered. For a
// known class, facts that establish a dangerous blast radius are severe, while
// recurrence invalidates only the bounded claim.
func Normalize(declared RigorClass, facts EvidenceFacts, wellFormed, recurrent bool) RigorClass {
	if !wellFormed || !declared.Valid() {
		return Unproven
	}
	if facts.Dangerous() {
		return Severe
	}
	if declared == Bounded && recurrent {
		return Unproven
	}
	return declared
}

// ParseEvidenceFacts converts the strict JSON object carried by a rigor row
// into typed facts. Missing, extra, or non-boolean members are malformed.
func ParseEvidenceFacts(value any) (EvidenceFacts, bool) {
	object, ok := value.(map[string]any)
	if !ok || len(object) != 7 {
		return EvidenceFacts{}, false
	}
	read := func(name string) (bool, bool) {
		value, present := object[name]
		if !present {
			return false, false
		}
		boolean, typed := value.(bool)
		return boolean, typed
	}
	local, localOK := read("local")
	recoverable, recoverableOK := read("recoverable")
	proof, proofOK := read("proofBoundaryCrossed")
	authority, authorityOK := read("authorityBoundaryCrossed")
	secrets, secretsOK := read("secretsBoundaryCrossed")
	irreversible, irreversibleOK := read("irreversibleDataBoundaryCrossed")
	external, externalOK := read("externalSideEffectBoundaryCrossed")
	facts := EvidenceFacts{
		Local:                             local,
		Recoverable:                       recoverable,
		ProofBoundaryCrossed:              proof,
		AuthorityBoundaryCrossed:          authority,
		SecretsBoundaryCrossed:            secrets,
		IrreversibleDataBoundaryCrossed:   irreversible,
		ExternalSideEffectBoundaryCrossed: external,
	}
	return facts, localOK && recoverableOK && proofOK && authorityOK && secretsOK && irreversibleOK && externalOK
}

// NormalizeWire normalizes an unchecked wire declaration. A classification
// is well formed only when its class and facts parse and its reopening trigger
// contains non-whitespace text.
func NormalizeWire(class, facts, reopeningTrigger any, recurrent bool) RigorClass {
	declared, classOK := class.(string)
	typedFacts, factsOK := ParseEvidenceFacts(facts)
	trigger, triggerOK := reopeningTrigger.(string)
	wellFormed := classOK && factsOK && triggerOK && strings.TrimSpace(trigger) != ""
	return Normalize(RigorClass(declared), typedFacts, wellFormed, recurrent)
}
