package batchowner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(script.String()), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The landing owner is machinery, not a session, so the rebuilt engine's
// ordinary up it runs ends with a non-zero exit at its session step. The
// owner judges that run by the state it must leave, never by up's words: the
// enrolled engine is current and the lane's supervision is armed. A run that
// leaves the supervision down fails the re-arm, whatever step up stopped at.
// Here the stub up stops exactly where the live one does, and the lane has
// neither an enrolled engine nor a supervision owner: the re-arm must not
// read as done (on 982fcdc00 it did, from up's stop point alone).
func TestLandingOwnerRearmFailsWhenSupervisionIsNotArmedAfterwards(t *testing.T) {
	t.Parallel()
	if reflect.ValueOf(BatchBaseRearm.Up).Pointer() != reflect.ValueOf(OwnerUpLandedEngine).Pointer() {
		t.Fatal("the owner's re-arm does not run up through OwnerUpLandedEngine")
	}
	root := t.TempDir()
	writeUpStub(t, root, 1,
		`component=host-preflight outcome=verified`,
		`component=accepted-engine outcome=re-armed detail="generation=4 previous=3 engine=7db15ce landed=7db15ce ref=refs/remotes/origin/main"`,
		`component=session-identity outcome=failed detail="runtime-signature ancestry proof failed: no agent ancestor"`,
		`up outcome=failed re-armed="generation=4 previous=3 engine=7db15ce landed=7db15ce" component=session-identity remedy="pass --pid <session-pid> and --start-time <epoch-seconds>, or configure a runtime signature and invoke up from that session"`)
	_, err := OwnerUpLandedEngine(context.Background(), root, root)
	if err == nil || !strings.Contains(err.Error(), "the lane's engine is not re-armed") {
		t.Fatalf("owner re-arm that left nothing armed = %v; want it refused", err)
	}
}
