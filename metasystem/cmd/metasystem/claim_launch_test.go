package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func seedClaimLaunchGoal(t *testing.T, root string) {
	t.Helper()
	goalSyncMutationGit(t, root, "init", "-q", "-b", "main")
	goalSyncMutationGit(t, root, "config", "user.name", "fixture")
	goalSyncMutationGit(t, root, "config", "user.email", "fixture@example.invalid")
	goalSyncMutationGit(t, root, "config", "goal.sync-remote", "local")
	goalSyncMutationGit(t, root, "config", "goal.sync-branch", goal.LocalLedgerBranch)
	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	stamp := "2026-08-20T00:00:00Z"
	rootRecord := &goal.RootRecord{Identity: "01J5X00000000000000000CA00", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	file := &goal.GoalFile{
		Id: "goal-a", State: goal.StateClaimed, Tier: 2, Intent: "Exercise claim launch.", Origin: goal.OriginMain,
		NextStep: "Launch it.", OpenedAt: stamp, Revision: 3,
		Budget:         &goal.Budget{ElapsedLimit: "8h", AttemptLimit: 2, ReservedJobMinutesLimit: 120, ActiveJobLimit: 1},
		Claimed:        &goal.ClaimRecord{Machine: "m-test", Lineage: "lin-claim", At: stamp, Revision: 3},
		StopCapability: &goal.StopCapability{Generation: 3, Revision: 3, Machine: "m-test", ClaimEpoch: 5},
		History: []goal.HistoryLine{
			{At: stamp, Opid: goal.Opid("01J5X00000000000000000CA01", "m-test", "lin-claim"), Verb: "open", Actor: "m-test+lin-claim", Keep: -1},
			{At: stamp, Opid: goal.Opid("01J5X00000000000000000CA02", "m-test", "lin-claim"), Verb: "set-budget", Actor: "m-test+lin-claim", Keep: -1},
			{At: stamp, Opid: goal.Opid("01J5X00000000000000000CA03", "m-test", "lin-claim"), Verb: "claim", Actor: "m-test+lin-claim", Keep: -1},
		},
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "backlog.md"), goal.RenderRoot(rootRecord), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "goal-a.md"), goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "plans/goals")
	goalSyncMutationGit(t, root, "commit", "-qm", "seed goal ledger")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
}

func setClaimLaunchCapability(t *testing.T, root string, mode dispatchcore.DispatchMode) {
	t.Helper()
	raw, err := dispatchcore.MintDelegateClaimCapability(root, mode)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dispatchcore.RemoveDelegateClaimCapability(root, raw) })
	t.Setenv("METASYSTEM_DELEGATE_INTERNAL", "1")
	t.Setenv(delegateClaimCapabilityEnv, raw)
}

func TestClaimLaunchVerbEmitsMachineReadableOutcome(t *testing.T) {
	root := t.TempDir()
	seedClaimLaunchGoal(t, root)
	product := filepath.Join(root, "artifacts", "agents", "worktrees", "claim-cli")
	if err := os.MkdirAll(product, 0o755); err != nil {
		t.Fatal(err)
	}
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	preparation := filepath.Join(root, "claim-occupancy.json")
	if code := runDispatchClaimOccupancyPrepare([]string{
		"--root", root,
		"--session", "codex:claim-cli",
		"--output", preparation,
	}); code != 0 {
		t.Fatalf("claim-occupancy-prepare exit=%d", code)
	}
	args := []string{
		"--root", root,
		"--opid", "claim-cli",
		"--session", "codex:claim-cli",
		"--dispatch-mode", "fresh",
		"--runtime", "codex",
		"--model", "gpt-5.6-sol",
		"--role", "implementer",
		"--destructive-reach", "MECHANICAL",
		"--adapter-verb", "dispatch",
		"--launch-mode", "worktree",
		"--permission-envelope-digest", digestA,
		"--product-root", product,
		"--cap-min", "120",
		"--input-hash", digestB,
		"--main-id", "main-1",
		"--claim-epoch", "5",
		"--goal", "goal-a",
		"--goal-revision", "3",
		"--goal-tier", "2",
		"--machine-id", "m-test",
		"--creator-pid", strconv.Itoa(os.Getpid()),
		"--occupancy-preparation", preparation,
	}
	setClaimLaunchCapability(t, root, dispatchcore.DispatchModeFresh)
	out, code := captureStdout(t, func() int { return runDispatchClaimLaunch(args) })
	if code != 0 {
		t.Fatalf("claim-launch exit=%d output=%q", code, out)
	}
	var result struct {
		Outcome  string         `json:"outcome"`
		Evidence map[string]any `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("claim-launch output is not JSON: %q: %v", out, err)
	}
	if result.Outcome != "WON" || result.Evidence["fingerprint"] == "" || result.Evidence["launchCapability"] == "" {
		t.Fatalf("claim-launch result = %+v", result)
	}
	created, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", "claim-cli.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(created, &record); err != nil {
		t.Fatal(err)
	}
	creator := record["creatorLiveness"].(map[string]any)
	if creator["pid"] != float64(os.Getpid()) {
		t.Fatalf("creator pid = %v, want %d", creator["pid"], os.Getpid())
	}
	if record["mainId"] != "main-1" || record["claimEpoch"] != float64(5) || record["goalId"] != "goal-a" {
		t.Fatalf("reservation provenance = mainId:%v claimEpoch:%v goalId:%v", record["mainId"], record["claimEpoch"], record["goalId"])
	}
	if record["operationId"] != "claim-cli" || record["goalRevision"] != float64(3) || record["goalTier"] != float64(2) || record["machineId"] != "m-test" {
		t.Fatalf("setup-comparable provenance = operationId:%v goalRevision:%v machineId:%v", record["operationId"], record["goalRevision"], record["machineId"])
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, false, time.Now().UTC())
	if err != nil || projection.Tree.Live["goal-a"].Sliced == nil {
		t.Fatalf("claim launch did not publish slice-start before its reservation: %+v %v", projection.Tree.Live["goal-a"], err)
	}
	roots, ok := record["productRoots"].([]any)
	canonicalProduct, err := filepath.EvalSymlinks(product)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(roots) != 1 || roots[0] != canonicalProduct {
		t.Fatalf("reservation product roots = %#v, want [%s]", record["productRoots"], canonicalProduct)
	}

	mismatch := append([]string(nil), args...)
	for index, value := range mismatch {
		if value == "--input-hash" {
			mismatch[index+1] = strings.Repeat("c", 64)
			break
		}
	}
	setClaimLaunchCapability(t, root, dispatchcore.DispatchModeFresh)
	out, code = captureStdout(t, func() int { return runDispatchClaimLaunch(mismatch) })
	if code != 1 {
		t.Fatalf("mismatch exit=%d output=%q", code, out)
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil || result.Outcome != "REFUSED-OPID-MISMATCH" {
		t.Fatalf("mismatch output=%q result=%+v err=%v", out, result, err)
	}
}

func TestClaimLaunchInternalSurfaceRequiresMarkerAndBearerCapability(t *testing.T) {
	root := t.TempDir()
	binding := dispatchcore.DelegateClaimCapabilityBinding{
		JobID: "claim-auth", OperationID: "claim-auth", DispatchMode: dispatchcore.DispatchModeFresh, AdapterVerb: "dispatch",
	}
	raw, err := dispatchcore.MintDelegateClaimCapability(root, dispatchcore.DispatchModeFresh)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dispatchcore.RemoveDelegateClaimCapability(root, raw) })
	t.Setenv(delegateClaimCapabilityEnv, raw)
	t.Setenv("METASYSTEM_DELEGATE_INTERNAL", "")
	if claimLaunchInternalAuthorized(root, binding, true) {
		t.Fatal("bearer capability without the internal route marker was authorized")
	}
	t.Setenv("METASYSTEM_DELEGATE_INTERNAL", "1")
	t.Setenv(delegateClaimCapabilityEnv, "")
	if claimLaunchInternalAuthorized(root, binding, true) {
		t.Fatal("internal route marker without a bearer capability was authorized")
	}
	t.Setenv(delegateClaimCapabilityEnv, raw)
	if !claimLaunchInternalAuthorized(root, binding, true) {
		t.Fatal("delegate capability did not authorize claim preflight")
	}
	if !claimLaunchInternalAuthorized(root, binding, false) {
		t.Fatal("delegate capability did not authorize its bound claim")
	}
	if claimLaunchInternalAuthorized(root, binding, false) {
		t.Fatal("spent delegate capability authorized a replay")
	}
}

func TestClaimLaunchVerbRequiresTheFingerprintTuple(t *testing.T) {
	if code := runDispatchClaimLaunch([]string{"--root", t.TempDir(), "--opid", "incomplete"}); code != 2 {
		t.Fatalf("incomplete claim-launch exit=%d, want 2", code)
	}
}
