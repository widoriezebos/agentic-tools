package identity

// VerificationOutcome is the total result vocabulary for a start/argv/start
// identity sandwich.
type VerificationOutcome string

const (
	VerificationDead          VerificationOutcome = "DEAD"
	VerificationIndeterminate VerificationOutcome = "INDETERMINATE"
	VerificationNotOurs       VerificationOutcome = "NOT-OURS"
	VerificationVerified      VerificationOutcome = "VERIFIED"
)

// VerificationReader keeps the three kernel observations behind one seam.
// An unreadable argv is represented by known=false, not by an error.
type VerificationReader interface {
	StartReader
	ReadArgv(pid int64) (argv []string, known bool)
}

// Verification carries one table outcome plus the stable observation, when
// one survived the sandwich. Presence distinguishes an unreadable argv on a
// live process from an unknown start read.
type Verification struct {
	Outcome  VerificationOutcome
	Presence Liveness
	Identity Exact
}

// VerifyProcess evaluates the identity sandwich in its binding order. A
// first-read absence stops immediately; later absence outranks argv results;
// a stable live identity reaches the argv-position decision last.
func VerifyProcess(reader VerificationReader, pid int64, tagPosition func([]string) bool) Verification {
	start1, state1, err1 := reader.ReadStart(pid)
	if state1 == Dead && err1 == nil {
		return Verification{Outcome: VerificationDead, Presence: Dead}
	}
	if state1 != Alive || err1 != nil {
		return Verification{Outcome: VerificationIndeterminate, Presence: Unknown}
	}

	argv, argvKnown := reader.ReadArgv(pid)
	start2, state2, err2 := reader.ReadStart(pid)
	if state2 != Alive && state2 != Dead || err2 != nil {
		return Verification{Outcome: VerificationIndeterminate, Presence: Unknown}
	}
	if state2 == Dead {
		return Verification{Outcome: VerificationDead, Presence: Dead}
	}
	identity := start2
	identity.Argv = argv
	identity.ArgvKnown = argvKnown
	comparison := Compare(start2, start1.Ref())
	if comparison.Mode == CompareInvalid || !comparison.Matches {
		return Verification{Outcome: VerificationIndeterminate, Presence: Alive, Identity: identity}
	}
	if !argvKnown {
		return Verification{Outcome: VerificationIndeterminate, Presence: Alive, Identity: identity}
	}
	if tagPosition == nil || !tagPosition(argv) {
		return Verification{Outcome: VerificationNotOurs, Presence: Alive, Identity: identity}
	}
	return Verification{Outcome: VerificationVerified, Presence: Alive, Identity: identity}
}
