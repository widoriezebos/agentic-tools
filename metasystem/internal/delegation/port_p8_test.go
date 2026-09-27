package delegation_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
)

// The mission-runner scenario (dispatch-fixtures.sh, mission-runner) ends
// every mission with the runner's chain-close sweep: for each fully
// terminal, unclosed chain the runner reaps and then closes it with
// --runner-closed, and a mission-stamped terminal orphan whose parent walk
// breaks (the patience fixture's pat-lost) is deliberately not closeable,
// so the sweep never touches it. The runner now reaches the lifecycle in
// process through missionrunner.Engine.Delegate; this drives the same
// sweep — missionrunner.CloseableChains, then reap and close per root, in
// the runner's argv — against the real lifecycle.
func TestP8MissionEndSweepClosesTerminalChainsAndLeavesTheOrphan(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	// Mirroring refuses an evidence root inside the repository; the runner
	// closes against a real installation whose evidence lives outside it.
	b.writeFile("metasystem.conf", "evidence.root="+t.TempDir()+"\n")
	const mission = "runner-patience"
	b.writeRecord("pat-lost", map[string]any{"mission": mission, "parentJob": "pat-gone",
		"status": "completed", "role": "design-critic", "startedAt": "2026-09-20T00:00:00Z", "endedAt": "2026-09-20T00:01:00Z"})
	b.writeRecord("chain", map[string]any{"mission": mission, "status": "completed", "phase": "reaped", "round": 1,
		"capabilitySnapshot": "artifacts/agents/capabilities/fake-001.json"})
	b.writeRecord("chain-r2", map[string]any{"mission": mission, "status": "failed", "phase": "reaped", "round": 2, "parentJob": "chain",
		"capabilitySnapshot": "artifacts/agents/capabilities/fake-001.json"})
	b.writeRecord("busy", map[string]any{"mission": mission, "status": "running", "round": 1})
	b.writeRecord("foreign", map[string]any{"mission": "other", "status": "completed", "round": 1})
	orphanBefore := b.record("pat-lost")

	roots := missionrunner.CloseableChains(b.root, mission)
	if !reflect.DeepEqual(roots, []string{"chain"}) {
		t.Fatalf("closeable chains %v, want [chain]: the orphan, the live chain and the foreign mission stay out", roots)
	}
	// The runner's own argv, in its order (loop.go closeTerminalChains).
	var argv [][]string
	engine := missionrunner.Engine{Root: b.root, Mission: mission, Delegate: func(args ...string) (string, string, int) {
		argv = append(argv, append([]string(nil), args...))
		result := b.run(args...)
		return string(result.Stdout), b.stderr.String(), result.ExitCode
	}}
	for _, root := range roots {
		if _, stderr, code := engine.Delegate("reap", "--job", root); code != 0 {
			t.Fatalf("reap of a terminal chain root exited %d: %s", code, stderr)
		}
		if _, stderr, code := engine.Delegate("close", "--job", root, "--runner-closed"); code != 0 {
			t.Fatalf("runner close exited %d: %s", code, stderr)
		}
	}
	want := [][]string{{"reap", "--job", "chain"}, {"close", "--job", "chain", "--runner-closed"}}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("delegate argv %v, want %v", argv, want)
	}
	if record := b.record("chain"); record["chainClosed"] != true || record["runnerClosed"] != true {
		t.Fatalf("the runner close did not mark the root: %v", record)
	}
	if !reflect.DeepEqual(b.record("pat-lost"), orphanBefore) {
		t.Fatalf("the sweep touched the orphan: %v", b.record("pat-lost"))
	}
	if record := b.record("busy"); record["status"] != "running" || record["chainClosed"] != nil {
		t.Fatalf("the sweep touched a live chain: %v", record)
	}
	if after := missionrunner.CloseableChains(b.root, mission); len(after) != 0 {
		t.Fatalf("a closed chain is still closeable: %v", after)
	}
}

// The orphan stays unclosed even if something names it directly: the
// lifecycle's close resolves the chain root first, and a broken parent walk
// refuses without marking the record.
func TestP8RunnerCloseOfAnOrphanRootIsRefusedByTheLifecycle(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRecord("pat-lost", map[string]any{"mission": "runner-patience", "parentJob": "pat-gone", "status": "completed"})
	result := b.run("close", "--job", "pat-lost", "--runner-closed")
	if result.ExitCode == 0 {
		t.Fatalf("a close of an orphan with a broken parent walk succeeded: %v", b.record("pat-lost"))
	}
	if !strings.Contains(b.stderr.String(), "cannot resolve job chain") && !strings.Contains(b.stderr.String(), "root job id") {
		t.Fatalf("stderr %q", b.stderr.String())
	}
	if b.record("pat-lost")["chainClosed"] != nil {
		t.Fatal("a refused close marked the orphan")
	}
}
