package missionrunner

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// resolveArmingIdentity's unattended branch (Phase 6): with no live holder
// in the checkout, the runner IS the main and announces itself under the
// mission lineage.
func TestResolveArmingIdentityUnattended(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-arm"}
	identity, err := engine.resolveArmingIdentity()
	if err != nil {
		t.Fatalf("unattended resolution: %v", err)
	}
	if identity.pid != os.Getpid() {
		t.Fatalf("identity pid: %d", identity.pid)
	}
	if identity.started <= 0 {
		t.Fatalf("identity start time: %d", identity.started)
	}
	if !strings.Contains(identity.session, "mission-runner-mr-arm-"+strconv.Itoa(os.Getpid())) {
		t.Fatalf("session name: %q", identity.session)
	}
	if identity.tag != "mission-runner.sh" {
		t.Fatalf("tag: %q", identity.tag)
	}
	if identity.lineage != MissionLineage("mr-arm") {
		t.Fatalf("lineage: %q", identity.lineage)
	}
}
