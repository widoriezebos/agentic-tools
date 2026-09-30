package main

// The traced sources no person reads as a message, outside the groups'
// own exclusions (G4b keeps its own in message_modes_g4b_test.go). Each is
// excluded with why; everything else the traced reading finds is enforced.
var messageTracedExcludedRemedies = map[string]string{
	// The adapter self-test's brief to the agent under test: the
	// PERMITTED_READ token and the attempts it names are the protocol the
	// self-test checks the agent's evidence against.
	"internal/adapter/selftestrun.go#SelftestRun": "prompt text sent to the agent under test",
	// The tails earlier settings files carried: setup matches them byte for
	// byte to recognise its own launchers and never prints them.
	"internal/hooks/setup_legacy.go": "earlier launchers' bytes, matched and never printed",
}

var _ = func() bool {
	for key := range messageTracedExcludedRemedies {
		messageTracedModes[key] = "excluded"
	}
	return true
}()
