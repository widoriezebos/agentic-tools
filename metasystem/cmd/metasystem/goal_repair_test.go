package main

import (
	"strings"
	"testing"
)

func TestGoalRepairRequiresAcceptRemoteAndNamesItInUsage(t *testing.T) {
	stderr, code := captureStderr(t, func() int { return runGoalRepair(nil) })
	if code != 2 || !strings.Contains(stderr,
		"usage: metasystem goal repair --accept-remote --by <human> --root <checkout>") {
		t.Fatalf("bare goal repair did not print usage naming --accept-remote: code=%d stderr=%q", code, stderr)
	}
}

func TestGoalRepairReachesAcceptRemoteAndRequiresBy(t *testing.T) {
	root := syncedClaimedGoalFixture(t)
	stderr, code := captureStderr(t, func() int {
		return runGoalRepair([]string{"--root", root, "--accept-remote"})
	})
	if code != 1 || !strings.Contains(stderr, "repair --accept-remote is a human-reserved act") ||
		!strings.Contains(stderr, "--by") {
		t.Fatalf("goal repair did not relay RepairAcceptRemote's human attribution refusal: code=%d stderr=%q", code, stderr)
	}

	stdout, stderr, code := captureRelay(t, func() int {
		return runGoalRepair([]string{"--root", root, "--accept-remote", "--by", "Wido"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "advanced=true tip=") ||
		!strings.Contains(stdout, "repair by Wido accepted") {
		t.Fatalf("goal repair did not print the accepted AdvanceResult: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
