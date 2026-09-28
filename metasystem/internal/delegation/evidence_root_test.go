package delegation_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// newEvidenceBed is a census bed whose conf is the given body and whose
// lifecycle's environment lookup answers only HOME, so the evidence root
// resolves through the owner's default without touching the process's HOME.
func newEvidenceBed(t *testing.T, conf, home string) *bed {
	t.Helper()
	b := newCensusBed(t)
	b.writeFile("metasystem.conf", conf)
	life, err := delegation.New(delegation.Config{Root: b.root, RepoScope: b.root, Engine: "/engine/metasystem",
		LookupEnv: func(name string) (string, bool) {
			if name == "HOME" {
				return home, true
			}
			return "", false
		}}, b.doubles.Ports())
	if err != nil {
		t.Fatal(err)
	}
	b.life = life
	return b
}

func writeTerminalJob(b *bed, job string) {
	b.writeRecord(job, map[string]any{"status": "completed", "round": 1, "role": "design-critic", "error": nil,
		"capabilitySnapshot": "artifacts/agents/capabilities/fake-current.json"})
	b.writeFile("artifacts/agents/capabilities/fake-current.json", `{"runtime":"fake"}`)
	b.writeFile("artifacts/agents/"+job+"/rounds/1/return.json", `{"ok":true}`)
}

// A conf carrying no evidence root mirrors a terminal job under the
// compiled-in default, <HOME>/metasystem-evidence/<checkout basename>.
func TestReapMirrorsUnderTheDefaultEvidenceRoot(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	b := newEvidenceBed(t, "metasystem.runtimes=fake\n", home)
	writeTerminalJob(b, "default-root")
	requireExit(t, b.run("reap", "--job", "default-root"), 0, b.stderr.String())
	manifest := filepath.Join(home, "metasystem-evidence", filepath.Base(b.root), "agents", dispatch.CheckoutSegment(b.root), "default-root", "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("no mirror at the default root: %v (stderr %q, record %v)", err, b.stderr.String(), b.record("default-root"))
	}
	if b.exists("artifacts/agents/mirror-failures.log") {
		t.Fatal("a default-root mirror logged a failure")
	}
}

// A relative evidence root is refused by the owner's sentence into one
// mirror-failures.log line, and nothing is mirrored.
func TestReapLogsARelativeEvidenceRootAndMirrorsNothing(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	b := newEvidenceBed(t, "metasystem.runtimes=fake\nevidence.root=relative\n", home)
	writeTerminalJob(b, "relative-root")
	b.run("reap", "--job", "relative-root")
	log, err := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "mirror-failures.log"))
	if err != nil {
		t.Fatalf("no mirror failure logged: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(log)), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `relative-root evidence.root must be absolute (metasystem.conf reads "relative")`) {
		t.Fatalf("mirror-failures.log = %q", log)
	}
	if _, has := b.record("relative-root")["mirror"]; has {
		t.Fatalf("a refused root stamped a mirror: %v", b.record("relative-root"))
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Fatalf("a refused root wrote under HOME: %v", entries)
	}
}
