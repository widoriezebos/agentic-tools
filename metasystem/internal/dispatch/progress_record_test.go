package dispatch

import (
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/progress"
)

func TestRecordSetupCapturesOutputAndPerRootLaunchStanding(t *testing.T) {
	root := sandbox(t)
	workspace := filepath.Join(root, "worktree")
	inside := filepath.Join(workspace, "product")
	outside := filepath.Join(root, "shared-product")
	job := "progress-record"
	reservation := writeJSON(t, filepath.Join(t.TempDir(), "reservation.json"), map[string]any{
		"jobId": job, "status": "pending-setup", "mainId": "main-1", "claimEpoch": 5,
		"goalId": "goal-a", "createdAt": "2026-08-10T00:00:00Z",
		"launchMode": LaunchModeWorktree, "productRoots": []any{inside, outside},
	})
	if err := RecordCreate(root, job, reservation); err != nil {
		t.Fatal(err)
	}
	stream := filepath.Join(root, "artifacts", "agents", job, "rounds", "1", "events.jsonl")
	setup := writeJSON(t, filepath.Join(t.TempDir(), "setup.json"), map[string]any{
		"jobId": job, "status": "pending", "mainId": "main-1", "claimEpoch": 5,
		"goalId": "goal-a", "workspaceRoot": workspace, "outputStream": stream,
	})
	if err := RecordSetup(root, job, setup); err != nil {
		t.Fatal(err)
	}
	record := readRecord(t, root, job)
	if record["outputStream"] != stream {
		t.Fatalf("outputStream = %v, want %s", record["outputStream"], stream)
	}
	roots, ok := record["productRoots"].([]any)
	if !ok || len(roots) != 2 || roots[0] != inside || roots[1] != outside {
		t.Fatalf("canonical product roots were not captured: %#v", record["productRoots"])
	}
	scopes, ok := record["productRootScopes"].([]any)
	if !ok || len(scopes) != 2 {
		t.Fatalf("launch scopes = %#v, want two roots", record["productRootScopes"])
	}
	first := scopes[0].(map[string]any)
	second := scopes[1].(map[string]any)
	if first["standing"] != progress.StandingLiveness || first["reason"] != progress.ReasonContainedAtLaunch {
		t.Fatalf("contained root scope = %#v", first)
	}
	if second["standing"] != progress.StandingAttributionOnly || second["reason"] != progress.ReasonOutsideWorktreeAtLaunch {
		t.Fatalf("outside root scope = %#v", second)
	}
}

func TestSharedCheckoutSetupRecordsEveryRootAsAttributionOnly(t *testing.T) {
	root := sandbox(t)
	workspace := filepath.Join(root, "checkout")
	product := filepath.Join(workspace, "product")
	job := "shared-progress-record"
	reservation := writeJSON(t, filepath.Join(t.TempDir(), "reservation.json"), map[string]any{
		"jobId": job, "status": "pending-setup", "mainId": "main-1", "claimEpoch": 5,
		"goalId": "goal-a", "createdAt": "2026-08-10T00:00:00Z",
		"launchMode": LaunchModeSharedCheckout, "productRoots": []any{product},
	})
	if err := RecordCreate(root, job, reservation); err != nil {
		t.Fatal(err)
	}
	setup := writeJSON(t, filepath.Join(t.TempDir(), "setup.json"), map[string]any{
		"jobId": job, "status": "pending", "mainId": "main-1", "claimEpoch": 5,
		"goalId": "goal-a", "workspaceRoot": workspace,
		"outputStream": filepath.Join(root, "events.jsonl"),
	})
	if err := RecordSetup(root, job, setup); err != nil {
		t.Fatal(err)
	}
	scopes := readRecord(t, root, job)["productRootScopes"].([]any)
	scope := scopes[0].(map[string]any)
	if scope["standing"] != progress.StandingAttributionOnly || scope["reason"] != progress.ReasonSharedCheckout {
		t.Fatalf("shared-checkout root scope = %#v", scope)
	}
}

func TestProgressLaunchFieldsAreImmutable(t *testing.T) {
	root := sandbox(t)
	createPending(t, root, "job-a")
	setupPending(t, root, "job-a")
	for _, field := range []string{"outputStream", "productRootScopes"} {
		patch := writeJSON(t, filepath.Join(t.TempDir(), field+".json"), map[string]any{field: "replacement"})
		if _, err := RecordCAS(root, "job-a", "pending", "pending", patch); err == nil {
			t.Fatalf("record CAS allowed immutable launch field %s", field)
		}
	}
}
