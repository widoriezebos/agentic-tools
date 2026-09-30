package humanauthority

import "strings"

// Remedy is what a person reads when this shell was refused a person's act
// ("Messages a Person Reads"): the plain reason for this situation, and the
// one command that resolves it with every value the engine knows filled in.
type Remedy struct {
	// Kind is the refusal's cause: RemedyAgent, RemedyNotEnrolled,
	// RemedyOtherTerminal, RemedyUnreadable, or empty for another cause.
	Kind string
	// Reason is line 1's cause: "this terminal isn't enrolled yet".
	Reason string
	// Argv is the command that resolves it, ready to run.
	Argv []string
	// Then is what goes with the command: "then repeat this command".
	Then string
}

// The causes a Remedy names.
const (
	RemedyAgent         = "agent"
	RemedyNotEnrolled   = "not-enrolled"
	RemedyOtherTerminal = "other-terminal"
	RemedyUnreadable    = "unreadable"
)

// RemedyFor is the remedy for err, the refusal of a person's act at root.
// person is the acting person when the caller knows them (a typed --by, the
// helm holder); otherwise the enrolled person's name is used, and NAME only
// when nobody is known. retry is the act as the person typed it. err may be a
// proof's outcome or the plain words PlainReason made of it.
func RemedyFor(root string, err error, person string, retry []string) Remedy {
	text := ""
	if err != nil {
		text = err.Error()
	}
	enrolledAs := ""
	if enrollment, readErr := ReadEnrollment(root); readErr == nil {
		enrolledAs = enrollment.Human
	}
	name := strings.TrimSpace(person)
	if name == "" {
		name = enrolledAs
	}
	if name == "" {
		name = "NAME"
	}
	enroll := []string{"metasystem", "system", "enroll", "--name", name}
	contains := func(parts ...string) bool {
		for _, part := range parts {
			if strings.Contains(text, part) {
				return true
			}
		}
		return false
	}
	switch {
	case contains(OutcomeAgent, "started by an agent"):
		reason := "an agent started this shell"
		if runtime := agentRuntimeIn(text); runtime != "" {
			reason = "an agent (" + runtime + ") started this shell"
		}
		return Remedy{Kind: RemedyAgent, Reason: reason, Argv: retry, Then: "in a terminal you opened yourself"}
	case contains(OutcomeNotEnrolled, "no terminal is enrolled"),
		enrolledAs == "" && contains(OutcomeTerminalMissing, "does not descend from the terminal enrolled", "no enrolled terminal among", "not the enrolled terminal"):
		return Remedy{Kind: RemedyNotEnrolled, Reason: "this terminal isn't enrolled yet", Argv: enroll, Then: "then repeat this command"}
	case contains("the enrolled terminal has no recorded name"):
		return Remedy{Kind: RemedyNotEnrolled, Reason: "the enrolled terminal has no recorded name", Argv: enroll,
			Then: "records your name; then repeat this command"}
	case contains(OutcomeTerminalMissing, "does not descend from the terminal enrolled", "no enrolled terminal among", "not the enrolled terminal"):
		return Remedy{Kind: RemedyOtherTerminal, Reason: "this terminal isn't enrolled (" + enrolledAs + " enrolled another one)", Argv: enroll,
			Then: "moves the enrollment here; then repeat this command"}
	case contains(OutcomeUnreadable, OutcomeChanged, OutcomeArgvUnreadable, OutcomeReused, OutcomeCycle,
		"could not be read", "changed while they were read", "was replaced while it was read"):
		return Remedy{Kind: RemedyUnreadable, Reason: "the processes behind this shell couldn't be read", Argv: retry, Then: "try again"}
	}
	// Another cause (a flag that does not combine, say): repeating the act
	// would not resolve it, so the remedy names no command.
	return Remedy{Reason: PlainReason(err)}
}

// agentRuntimeIn is the agent runtime a refusal names: "AGENT_IN_AUTHORITY_CHAIN:
// claude; later ..." or "started by an agent (codex)".
func agentRuntimeIn(text string) string {
	if _, rest, found := strings.Cut(text, OutcomeAgent+": "); found {
		runtime, _, _ := strings.Cut(rest, ";")
		return strings.TrimSpace(runtime)
	}
	if _, rest, found := strings.Cut(text, "started by an agent ("); found {
		runtime, _, _ := strings.Cut(rest, ")")
		return strings.TrimSpace(runtime)
	}
	return ""
}

// WalkRefusal is why the walk to the enrolled terminal refused an act the
// helm or a power of attorney then admitted; nil for a proof the walk made.
func (p Proof) WalkRefusal() error { return p.walkRefusal }
