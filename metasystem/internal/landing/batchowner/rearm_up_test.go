package batchowner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeUpStub installs a bin/metasystem under root that prints lines as the
// rebuilt engine's up would and exits with status.
func writeUpStub(t *testing.T, root string, status int, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	for _, line := range lines {
		script.WriteString("printf '%s\\n' '" + line + "'\n")
	}
	script.WriteString("exit " + string(rune('0'+status)) + "\n")
	if err := os.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(script.String()), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The landing owner is machinery, not a session: the rebuilt engine's
// ordinary up re-arms a landed build (its accepted-engine step, whose
// authority is the landed bytes, never the caller) and then ends at
// component=session-identity because no session is there to announce. That
// ending is the owner's re-arm done, not a failed tick; any other failed
// component still fails it. The stub is the rebuilt engine's up, run through
// the production re-arm edge.
func TestLandingOwnerRearmSucceedsWhenUpEndsOnlyAtSessionIdentity(t *testing.T) {
	previous := BatchBaseRearm
	t.Cleanup(func() { BatchBaseRearm = previous })
	BatchBaseRearm.FastForward = func(context.Context, string, string) error { return nil }
	BatchBaseRearm.Rebuild = func(context.Context, string) error { return nil }

	root := t.TempDir()
	writeUpStub(t, root, 1,
		`component=host-preflight outcome=verified`,
		`component=accepted-engine outcome=re-armed detail="generation=4 previous=3 engine=7db15ce landed=7db15ce ref=refs/remotes/origin/main"`,
		`component=session-identity outcome=failed detail="runtime-signature ancestry proof failed: no agent ancestor"`,
		`up outcome=failed re-armed="generation=4 previous=3 engine=7db15ce landed=7db15ce" component=session-identity remedy="pass --pid <session-pid> and --start-time <epoch-seconds>, or configure a runtime signature and invoke up from that session"`)
	if err := RearmBatchTip(root, "7db15cec3fa6fdfbfbf969c534599f14a8f718ae"); err != nil {
		t.Fatalf("owner re-arm ending only at session-identity = %v, want the re-arm accepted", err)
	}

	refused := t.TempDir()
	writeUpStub(t, refused, 1,
		`component=host-preflight outcome=verified`,
		`component=accepted-engine outcome=ENROLLMENT_DRIFT detail="rebuilt engine is not landed"`,
		`up outcome=ENROLLMENT_DRIFT component=accepted-engine remedy="run metasystem system start"`)
	if err := RearmBatchTip(refused, "7db15cec3fa6fdfbfbf969c534599f14a8f718ae"); err == nil || !strings.Contains(err.Error(), "ENROLLMENT_DRIFT") {
		t.Fatalf("owner re-arm refused at the accepted engine = %v, want the refusal", err)
	}

	later := t.TempDir()
	writeUpStub(t, later, 1,
		`component=accepted-engine outcome=verified detail="generation=4"`,
		`component=session-identity outcome=verified`,
		`component=supervision outcome=failed`,
		`up outcome=failed component=supervision remedy="inspect supervision"`)
	if err := RearmBatchTip(later, "7db15cec3fa6fdfbfbf969c534599f14a8f718ae"); err == nil {
		t.Fatal("owner re-arm that failed after session identity was accepted")
	}
}
