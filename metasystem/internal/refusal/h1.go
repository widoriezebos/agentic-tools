package refusal

// Rule H1 (verbs-object-action 3.6): an authenticated person is never denied
// a verb. A refusal a person can meet is allowed only when the act would
// damage the system, and then it states what would go wrong and names the
// runnable public command that achieves the intent the right way.
//
// Standing classifies one register row against that rule. The witness R14
// (cmd/metasystem) walks the register with it: every row a person can meet
// either asks for a corrected word or guides with a public command.

// Standing is a refusal's standing under rule H1.
type Standing string

const (
	// StandingInput: the person's word is malformed, stale or names nothing
	// (an unknown id, a moved revision, a missing input); they re-read and
	// re-issue it. Input validation is not a denial.
	StandingInput Standing = "input"
	// StandingAgent: the refusal binds only a caller that is not the
	// authenticated person (an agent, a delegate, the steward).
	StandingAgent Standing = "agent"
	// StandingIdentity: the refusal establishes that the caller is the
	// person; it is the proof H1 presumes, not a denial of it.
	StandingIdentity Standing = "identity"
	// StandingGuide: the act as asked would damage the system. The message
	// states what would go wrong and names Forward, a runnable public
	// command that achieves the intent the right way.
	StandingGuide Standing = "guide"
)

// Standing returns the row's standing: the explicit H1 value when the row
// declares one, else the one its shape implies for Identity and Agent rows.
// A Question row has no implied standing: the person meets it, so the row
// must say whether it asks for a corrected word or guides.
func (row Row) Standing() Standing {
	if row.H1 != "" {
		return row.H1
	}
	switch row.Shape {
	case Identity:
		return StandingIdentity
	case Agent:
		return StandingAgent
	}
	return ""
}
