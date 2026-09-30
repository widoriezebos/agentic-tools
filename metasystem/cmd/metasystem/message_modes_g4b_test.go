package main

// Group G4b of the traced "Messages a Person Reads" rewrite (round 2): the
// mission runner and the delegate engine. Every traced path of both packages
// is rewritten and enforced, except the sources below, which no person reads
// as a message and which are therefore excluded, never enforced.
var _ = enforceTracedMessages(
	"internal/delegation",
	"internal/missionrunner",
)

// messageTracedExcludedG4b are the G4b sources the traced reading may not
// hold to the two-line rule, each with why.
var messageTracedExcludedG4b = map[string]string{
	// Prompt text for an agent: the Declared Outputs section appended to a
	// design critic's brief.
	"internal/delegation/dispatch_phase.go#session.appendDeclaredOutputs": "prompt text sent to an agent",
	// A JSON outcome record a parent reads by its outcome field.
	"internal/delegation/router.go#session.route": "JSON outcome record a parent reads",
	// The driver-facing status record: the mission verb reads its words
	// (runIntentMission matches " status=unreadable reason=missing-state"),
	// so it changes only when that verb renders from a typed status.
	"internal/missionrunner/status.go#Engine.Status": "status record the mission verb reads by its words",
}

var _ = func() bool {
	for key := range messageTracedExcludedG4b {
		messageTracedModes[key] = "excluded"
	}
	return true
}()
