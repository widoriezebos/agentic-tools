package delegation_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// fixtureLedger is a ULID-shaped ledger identity.
const fixtureLedger = "01J5X000000000000000000000"

// acceptLedger commits a goal root naming fixtureLedger and points the
// accepted goal ref at it. The brain fence reads this checkout's ledger
// identity through the goal owner's own Git (goal.ExistingLedgerIdentity),
// which no lifecycle Git stub reaches: that real read is why the declared
// brain legs run on the integration bed.
func (b *bed) acceptLedger() {
	b.t.Helper()
	b.writeFile("plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{
		Identity: fixtureLedger, FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	})))
	b.git("add", "plans/goals/backlog.md")
	b.git("commit", "-qm", "ledger")
	b.git("update-ref", "refs/metasystem/goals/accepted", "HEAD")
	b.git("config", "goal.sync-remote", "local")
	b.git("config", "metasystem.goal.machine", "node")
}

// declareBrain writes a valid brain declaration for fixtureLedger at root.
func declareBrain(t *testing.T, root string) {
	t.Helper()
	encoded, err := json.Marshal(brain.Record{Schema: brain.Schema, Ledger: fixtureLedger, Machine: "brain", DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	(&bed{t: t, root: root}).writeFile(strings.TrimPrefix(brain.Path(root), root+"/"), string(encoded)+"\n")
}

// brain-delegate-refuses, brain-cancel-close-reap-refuse and
// brain-breach-stop-exempt over a declared (not corrupt) brain: each act
// refuses with its own remedy, inside the boundary and on the legacy
// grammar, no record is written or mutated, nothing launches; the breach
// stop callback is not a fenced act.
func TestPortP1BrainIntegrationDeclaredCheckoutRefusesEveryActWithItsRemedy(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.acceptLedger()
	declareBrain(t, b.root)
	b.writeFile("artifacts/agents/jobs/pending-job.json", `{"jobId":"pending-job","status":"pending"}`+"\n")
	b.writeFile("artifacts/agents/jobs/closed-root.json", `{"jobId":"closed-root","status":"completed","chainClosed":true}`+"\n")
	brief := b.brief("brief.md", "implement", "Let a node finish it.")
	before := b.agentFiles()
	remedy := map[string][]string{
		"dispatch":  {"the brain never dispatches", "metasystem work build"},
		"follow-up": {"the brain never dispatches"},
		"cancel":    {"the brain never cancels a node's job"},
		"close":     {"the brain never closes or reaps dispatcher records"},
		"reap":      {"the brain never closes or reaps dispatcher records"},
	}
	acts := append([]struct {
		name string
		argv []string
	}{{"dispatch", []string{"dispatch", "--role", "implementer", "--brief", brief, "--destructive-reach", "MECHANICAL"}}}, fencedActs...)
	for _, act := range acts {
		for _, env := range []delegation.Env{b.dispatchEnv("fresh"), {}} {
			result := b.runEnv(env, act.argv...)
			requireExit(t, result, 2, b.stderr.String())
			line := string(result.Stdout)
			if !strings.Contains(line, `"outcome":"BRAIN_REFUSED"`) {
				t.Fatalf("%s (internal=%v): stdout %q", act.name, env.DelegateInternal, line)
			}
			for _, want := range remedy[act.name] {
				if !strings.Contains(line, want) {
					t.Fatalf("%s (internal=%v): refusal %q lacks %q", act.name, env.DelegateInternal, line, want)
				}
			}
		}
	}
	after := b.agentFiles()
	for path := range after {
		// The claim capabilities this test minted stand in for the
		// boundary's; they are not the fence's writes.
		if strings.HasPrefix(path, "capabilities/delegate-claim/") {
			delete(after, path)
		}
	}
	if drift, same := sameFiles(before, after); !same {
		t.Fatalf("the brain fence mutated agent state: %s", drift)
	}
	if len(b.launches) != 0 {
		t.Fatalf("a brain checkout launched: %v", b.launches)
	}

	result := b.run("__breach-stop-goal", "--goal", "ship-widget", "--revision", "1")
	combined := string(result.Stdout) + string(result.Outcome) + b.stderr.String()
	if strings.Contains(combined, "BRAIN_REFUSED") || strings.Contains(combined, "brain never") {
		t.Fatalf("breach-stop was caught by the brain fence: exit %d %q", result.ExitCode, combined)
	}
	if calls := b.calls("lease.Authorize"); len(calls) == 0 || !strings.Contains(calls[len(calls)-1], "mode=stop-custodian") {
		t.Fatalf("breach-stop did not reach the stop-custodian gate: %v", b.doubles.Log.Calls())
	}
}

// brain-absent-node-proceeds: a node sharing the brain's ledger, whose own
// checkout carries no declaration, is not fenced by another checkout's
// designation; its dispatch runs through to the adapter launch.
func TestPortP1BrainIntegrationUndeclaredNodeOnTheBrainsLedgerLaunches(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	b.acceptLedger()
	declareBrain(t, t.TempDir()) // the brain's checkout, elsewhere
	brief := b.brief("brief.md", "implement", "Run the plain absent-node brief.")
	result := b.runEnv(b.dispatchEnv("fresh"), "dispatch", "--role", "implementer", "--brief", brief,
		"--destructive-reach", "MECHANICAL", "--job-id", "brain-absent-node-delegate")
	requireExit(t, result, 0, b.stderr.String())
	if strings.Contains(string(result.Stdout)+string(result.Outcome), "BRAIN_REFUSED") {
		t.Fatalf("the absent node was brain-refused: %q", result.Stdout)
	}
	if len(b.launches) != 1 || b.launches[0].Job != "brain-absent-node-delegate" {
		t.Fatalf("the node's delegate did not launch: %v", b.launches)
	}
	if record := b.record("brain-absent-node-delegate"); record["status"] != "running" {
		t.Fatalf("record %v", record)
	}
}

// steward-continuation, the full path's dispatcher half: an authorized
// continuation launches exactly once with the authorization's role, job,
// brief and roster, in a worktree, under the steward's standing; the
// dispatcher consults the steward exactly once.
func TestPortP1StewardIntegrationAuthorizedContinuationLaunchesOnce(t *testing.T) {
	t.Parallel()
	b := newDispatchBed(t)
	brief := b.brief("artifacts/agents/steward/staged/brief.md", "implement", "Repair the thing.")
	b.doubles.Steward.AuthorizeFunc = func(_ delegation.Invocation, intent string) (steward.DispatchAuthorization, error) {
		return steward.DispatchAuthorization{
			Goal: "fix-it", JobId: "steward-" + intent, Runtime: "fake", Model: "fake-model",
			Role: "steward-continuation", Brief: brief,
		}, nil
	}
	result := b.runEnv(b.dispatchEnv("fresh"), "--steward-intent", "n1")
	requireExit(t, result, 0, b.stderr.String())
	if string(result.Stdout) != "steward-n1\n" {
		t.Fatalf("stdout %q stderr %q", result.Stdout, b.stderr.String())
	}
	if calls := b.calls("steward.AuthorizeDispatch"); len(calls) != 1 {
		t.Fatalf("authorization calls %v", calls)
	}
	if len(b.launches) != 1 || b.launches[0].Job != "steward-n1" || b.launches[0].Runtime != "fake" {
		t.Fatalf("launches %v", b.launches)
	}
	record := b.record("steward-n1")
	if record["role"] != "steward-continuation" || record["status"] != "running" {
		t.Fatalf("record %v", record)
	}
	if workspace, _ := record["workspaceRoot"].(string); !strings.HasPrefix(workspace, filepath.Join(b.root, "artifacts", "agents", "worktrees")) {
		t.Fatalf("the continuation did not run in a worktree: %v", record["workspaceRoot"])
	}
}
