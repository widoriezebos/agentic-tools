package identity

// VerificationOutcome is the total result vocabulary for the
// start/argv/start identity sandwich.
type VerificationOutcome string

const (
	VerificationDead          VerificationOutcome = "DEAD"
	VerificationIndeterminate VerificationOutcome = "INDETERMINATE"
	VerificationNotOurs       VerificationOutcome = "NOT-OURS"
	VerificationVerified      VerificationOutcome = "VERIFIED"
)

// Verification carries one table outcome plus the stable observation,
// when one survived the sandwich. Presence distinguishes an unreadable
// argv on a live process from an unknown start read.
type Verification struct {
	Outcome  VerificationOutcome
	Presence Liveness
	Identity Exact
}

// VerifyProcess evaluates the identity sandwich in its binding order,
// ported onto the Prober seam (the wip original read start and argv
// through separate readers; today's probe carries argv with the exact
// identity, and the double probe preserves the sandwich's stability
// guarantee: the identity that reaches the argv decision is the one
// that held on BOTH sides of the read). A first-probe absence stops
// immediately; later absence outranks argv results; Unknown never
// authorizes anything.
func VerifyProcess(prober Prober, pid int64, tagPosition func([]string) bool) Verification {
	first, state1, err1 := prober.Probe(pid)
	if state1 == Dead && err1 == nil {
		return Verification{Outcome: VerificationDead, Presence: Dead}
	}
	if state1 != Alive || err1 != nil {
		return Verification{Outcome: VerificationIndeterminate, Presence: Unknown}
	}
	second, state2, err2 := prober.Probe(pid)
	if (state2 != Alive && state2 != Dead) || err2 != nil {
		return Verification{Outcome: VerificationIndeterminate, Presence: Unknown}
	}
	if state2 == Dead {
		return Verification{Outcome: VerificationDead, Presence: Dead}
	}
	if !sameIdentity(second, first.Ref()) {
		return Verification{Outcome: VerificationIndeterminate, Presence: Alive, Identity: second}
	}
	if !second.ArgvKnown {
		return Verification{Outcome: VerificationIndeterminate, Presence: Alive, Identity: second}
	}
	if tagPosition == nil || !tagPosition(second.Argv) {
		return Verification{Outcome: VerificationNotOurs, Presence: Alive, Identity: second}
	}
	return Verification{Outcome: VerificationVerified, Presence: Alive, Identity: second}
}
