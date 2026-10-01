package hooks

import (
	"strings"
	"testing"
)

// TestStopNeverBlocksLandingAgent: the landing agent is no seat. Its keeper
// is its supervisor and relaunches it when work is queued, so its Stop is
// always allowed, while the same idle-with-backlog verdict blocks a seat. The
// first real landing session was held with seat demands (policy decisions,
// session status, the seat's goals) until it ended confused.
func TestStopNeverBlocksLandingAgent(t *testing.T) {
	t.Parallel()
	idle := `{"schemaVersion":1,"class":"idle-with-backlog","shouldBlock":true,"countSpent":true,"display":"IDLE WITH BACKLOG","surfaceWatchdog":false,"idleRefusal":true,"brainStatusDue":false}`
	for _, lineage := range []string{"", "landing-agent"} {
		installation := newHookInstallation(t)
		ops := newFakeOps(t, installation)
		ops.verdict = idle
		env := map[string]string{}
		if lineage != "" {
			env["METASYSTEM_OWNER_LINEAGE"] = lineage
		}
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"landing-fixture","hook_event_name":"Stop"}`, env: env})
		blocked := strings.Contains(run.stdout, `"decision":"block"`)
		if lineage == "" {
			if !blocked {
				t.Fatalf("a seat's idle-with-backlog stop was allowed: status %d stdout %q", run.status, run.stdout)
			}
			continue
		}
		line := strings.TrimSuffix(run.stdout, "\n")
		if run.status != 0 || blocked || !validStopOutput(line, true) || len(ops.turnVerdicts) != 0 {
			t.Fatalf("landing-agent stop: status %d stdout %q stderr %q verdicts %d", run.status, run.stdout, run.stderr, len(ops.turnVerdicts))
		}
	}
}
