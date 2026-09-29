package act

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// helmRoots are the roots whose helm admits every caller in this test
// binary; every other root's helm declines, so no other test changes.
var helmRoots sync.Map

func init() {
	humanauthority.AtHelm = func(root string, _ int64) (humanauthority.HelmGrant, bool) {
		if _, held := helmRoots.Load(root); held {
			return humanauthority.HelmGrant{By: "wido", Class: "UNTRUSTED"}, true
		}
		return humanauthority.HelmGrant{}, false
	}
}

// TestBootProofIgnoresTheHelm: the interface's boot proof outlives the helm,
// so a helm proof never proves the interface's person.
func TestBootProofIgnoresTheHelm(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	enrollment, err := json.Marshal(humanauthority.Enrollment{Schema: 1, EnrolledAt: time.Unix(1000, 0).UTC(), Generation: 1, Human: "wido",
		TerminalID: "fixture-terminal-nobody-has", TerminalRef: humanauthority.ProcessRef{PID: 20, PIDStartedAt: 200},
		SessionLeader: humanauthority.ProcessRef{PID: 10, PIDStartedAt: 100}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "authority", "human-terminal.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, enrollment, 0o644); err != nil {
		t.Fatal(err)
	}
	helmRoots.Store(root, true)
	proof, err := humanauthority.Prove(root, int64(os.Getpid()), nil, time.Now())
	if err != nil || proof.Helm == nil {
		t.Fatalf("the fixture's helm did not admit the test process: %+v %v", proof, err)
	}
	authority := Prove(root, root, int64(os.Getpid()), time.Now())
	if authority.Proven() || !strings.Contains(authority.Reason(), "helm") {
		t.Fatalf("a helm proof proved the interface: proven=%t reason=%q", authority.Proven(), authority.Reason())
	}
}
