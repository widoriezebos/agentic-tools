package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func seedClaimLaunchGoalFiles(t *testing.T, root string) {
	t.Helper()
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
}

// The claim-launch owner publishes the goal's first-slice fact on the shared
// ledger before it writes the reservation, and refuses a changed launch
// identity under the same operation (moved off the deleted job claim-launch
// verb, U6b).
func TestClaimLaunchPublishesSliceStartBeforeReservation(t *testing.T) {
	repository := newProofAdmissionRepositoryFixture(t, time.Now().UTC(), false)
	root := repository.root
	seedClaimLaunchGoalFiles(t, root)
	backlog, err := os.ReadFile(filepath.Join(root, "plans", "goals", "backlog.md"))
	if err != nil {
		t.Fatal(err)
	}
	goalFile, err := os.ReadFile(filepath.Join(root, "plans", "goals", "goal-a.md"))
	if err != nil {
		t.Fatal(err)
	}
	repository.seed(map[string][]byte{
		"metasystem/plans/goals/backlog.md": backlog,
		"metasystem/plans/goals/goal-a.md":  goalFile,
	})
	reads := repository.reads()
	jobPath := filepath.Join(root, "artifacts", "agents", "jobs", "claim-cli.json")
	if repository.goalFile(t, "goal-a").Sliced != nil {
		t.Fatal("goal-a was sliced before claim launch")
	}
	if _, err := os.Stat(jobPath); !os.IsNotExist(err) {
		t.Fatalf("reservation exists before slice publication: %v", err)
	}
	product := filepath.Join(root, "artifacts", "agents", "worktrees", "claim-cli")
	if err := os.MkdirAll(product, 0o755); err != nil {
		t.Fatal(err)
	}
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	preparation := filepath.Join(root, "claim-occupancy.json")
	if err := dispatchcore.WriteClaimOccupancyPreparation(root, "codex:claim-cli", preparation); err != nil {
		t.Fatalf("claim occupancy preparation: %v", err)
	}
	prepared, err := dispatchcore.ReadClaimOccupancyPreparation(preparation, "codex:claim-cli")
	if err != nil {
		t.Fatal(err)
	}
	resolvedCap, _, _, err := dispatchcore.ResolveCap(filepath.Join(root, "metasystem.conf"), "implementer", "codex", config.CanonicalModel("gpt-5.6-sol"), "", "120")
	if err != nil {
		t.Fatal(err)
	}
	startReader, err := dispatchproc.StartReader(root)
	if err != nil {
		t.Fatal(err)
	}
	resumed := ""
	params := dispatchcore.ClaimLaunchParams{
		Root: root, OpID: "claim-cli", MainID: "main-1", ClaimEpoch: "5", GoalID: "goal-a",
		GoalRevision: 3, GoalTier: 2, MachineID: "m-test", AdapterVerb: "dispatch",
		Request: dispatchcore.LaunchFingerprintRequest{
			SessionKey: "codex:claim-cli", DispatchMode: dispatchcore.DispatchModeFresh,
			ResumedSessionID: &resumed, Runtime: "codex", Model: "gpt-5.6-sol", Role: "implementer",
			LaunchMode: dispatchcore.LaunchMode("worktree"), PermissionEnvelopeDigest: digestA,
			ProductRoots: []string{product}, CapMinutes: resolvedCap, InputHash: digestB,
			GoalID: "goal-a", GoalRevision: 3, DestructiveReach: dispatchcore.HazardClass("MECHANICAL"),
		},
		DefaultCapMinutes:    resolvedCap,
		OccupancyPreparation: &prepared,
	}
	dependencies := dispatchcore.ClaimLaunchDependencies{
		CreatorPID: int64(os.Getpid()), IdentityReader: startReader, ProcessVerifier: dispatchproc.ClaimProcessVerifier{},
		Reconcile: func(root, job string) (dispatchcore.ReconciliationResult, error) {
			return dispatchcore.ReconcileReservation(root, job, dispatchcore.ReconciliationDependencies{
				Scanner: dispatchproc.TaggedProcessScanner{Root: root}, Creator: startReader,
				Emit: func(line string) { t.Log(line) },
			})
		},
		MarkFirstSlice: func(params dispatchcore.ClaimLaunchParams, now time.Time) error {
			return dispatchcore.MarkFirstSliceWithReads(params, now, reads)
		},
	}
	result, err := dispatchcore.ClaimLaunch(params, dependencies)
	if err != nil {
		t.Fatalf("claim launch: %v", err)
	}
	if result.Outcome != "WON" || dispatchcore.ClaimOutcomeExitCode(result.Outcome) != 0 || result.Evidence["fingerprint"] == "" || result.Evidence["launchCapability"] == "" {
		t.Fatalf("claim launch result = %+v", result)
	}
	if repository.goalFile(t, "goal-a").Sliced == nil {
		t.Fatal("claim launch did not publish slice-start before its reservation")
	}
	created, err := os.ReadFile(jobPath)
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
	roots, ok := record["productRoots"].([]any)
	canonicalProduct, err := filepath.EvalSymlinks(product)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(roots) != 1 || roots[0] != canonicalProduct {
		t.Fatalf("reservation product roots = %#v, want [%s]", record["productRoots"], canonicalProduct)
	}

	mismatch := params
	mismatch.OccupancyPreparation = nil
	mismatch.Request.InputHash = strings.Repeat("c", 64)
	result, err = dispatchcore.ClaimLaunch(mismatch, dependencies)
	if err != nil || result.Outcome != "REFUSED-OPID-MISMATCH" || dispatchcore.ClaimOutcomeExitCode(result.Outcome) != 1 {
		t.Fatalf("mismatch result=%+v err=%v", result, err)
	}
}
