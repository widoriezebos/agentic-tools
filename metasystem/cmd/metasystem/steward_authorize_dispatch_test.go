package main

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

// The steward authorize-dispatch verb's surface, pinned while the delegate
// lifecycle calls its owner in-process: the usage refusal, and the one
// caller class it admits.
func TestStewardAuthorizeDispatchVerbPinsItsSurface(t *testing.T) {
	code, _, stderr := captureCommandOutput(t, true, true, func() int {
		return runStewardAuthorizeDispatch([]string{"--repo", t.TempDir()})
	})
	if code != 2 || stderr != "steward authorize-dispatch: --repo, --caller-pid, and --intent are required\n" {
		t.Fatalf("usage refusal: exit %d stderr %q", code, stderr)
	}
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runStewardAuthorizeDispatch([]string{"--repo", t.TempDir(), "--caller-pid", strconv.Itoa(os.Getpid()), "--intent", "n1"})
	})
	refusal := regexp.MustCompile(`^steward authorize-dispatch: caller is [A-Z-]+, not the steward; the continuation mode admits exactly one caller\n$`)
	if code != 1 || stdout != "" || !refusal.MatchString(stderr) {
		t.Fatalf("a non-steward caller: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}
