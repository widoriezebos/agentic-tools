package main

import (
	"strings"
	"testing"
)

// work land's help speaks of the host's landing lane (landing status), not of
// the retired landing.batch-root setting or landing "by hand".
func TestWorkLandHelpNamesTheLandingLane(t *testing.T) {
	t.Parallel()
	command := mustIntentCommand(t, "work land")
	details := strings.Join(command.details, " ")
	if strings.Contains(details, "batch-root") || strings.Contains(details, "by hand") || !strings.Contains(details, "metasystem landing status") {
		t.Fatalf("work land help is not in the lane model:\n%s", details)
	}
}
