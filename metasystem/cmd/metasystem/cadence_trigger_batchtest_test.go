//go:build batchtest

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestCadenceTickSurfacesPlantedDeepOnlyRedWithoutDeepLanding(t *testing.T) {
	start := time.Date(2031, 2, 3, 4, 5, 0, 0, time.UTC)
	clockNow := start
	clock := func() time.Time { return clockNow }
	landingRoot := syncedClaimedGoalFixture(t)
	origin := filepath.Join(t.TempDir(), "origin.git")
	// -b main because the seats below are CLONED from this origin. Without it
	// the initial branch comes from init.defaultBranch, which an isolated git
	// configuration does not supply, so the origin's HEAD names master while
	// the push below creates main. git clone then checks out NOTHING and still
	// exits zero, and the seat has no ledger to read.
	goalSyncMutationGit(t, landingRoot, "init", "--bare", "-b", "main", origin)
	goalSyncMutationGit(t, landingRoot, "remote", "add", "origin", origin)

	contract := testpolicy.Contract{SchemaVersion: testpolicy.SchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "fallback", Cadence: []string{"section/deep"}, Unknown: []string{"ordinary"},
		Surfaces: []testpolicy.Surface{
			{ID: "app", Paths: []string{"app/**"}, Standard: []string{"ordinary"}, Deep: []string{"section/deep"}},
			{ID: "fallback", Standard: []string{"ordinary"}},
		},
		Groups: []testpolicy.Group{
			{ID: "ordinary", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"app/**"}, Outputs: []string{"reports"},
				Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{"bash", "-c", "test -f app/deep.flag"}, Reports: []string{"reports"},
				Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/ordinary.xml", Classname: "Fixture", Name: "ordinary"}}},
			{ID: "section/deep", Kind: "integration", Adapter: "section", CWD: ".", Inputs: []string{"app/**", "scripts/agents/validate-section-selector.sh"}, Platforms: []string{"any"},
				TargetMS: 1000, Section: "deep-only"},
		},
		Always: testpolicy.Always{Canary: []string{"ordinary"}},
	}
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	deepOnly, err := testpolicy.DeepOnlySectionGroupIDs(contract)
	if err != nil || strings.Join(deepOnly, ",") != "section/deep" {
		t.Fatalf("deep-only inventory=%v err=%v", deepOnly, err)
	}
	if err := os.MkdirAll(filepath.Join(landingRoot, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(landingRoot, "app", "deep.flag"), []byte("green\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	selector := filepath.Join(landingRoot, "scripts", "agents", "validate-section-selector.sh")
	if err := testexec.WriteFile(selector, []byte(`#!/usr/bin/env bash
set -eu
if grep -qx green app/deep.flag; then
  printf 'section\tdeep-only\tpass\t0\n' >"$METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT"
  exit 0
fi
printf 'section\tdeep-only\tfail\t1\tplanted deep-only red\n' >"$METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT"
printf '%s\n' '=== bed failed scenarios ===' '- deep-only (rc=1)' '=== end bed failed scenarios ==='
exit 1
`), 0o755); err != nil {
		t.Fatal(err)
	}
	contractBytes, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(landingRoot, "testing.json"), append(contractBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(landingRoot, "metasystem.conf")
	confBytes, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, append(confBytes, []byte("testing.contract=testing.json\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, landingRoot, "add", "app/deep.flag", "testing.json", "metasystem.conf", "scripts/agents/validate-section-selector.sh")
	goalSyncMutationGit(t, landingRoot, "commit", "-q", "-m", "green deep-only fixture")
	goalSyncMutationGit(t, landingRoot, "push", "-q", "-u", "origin", "main")

	seatRoots := []string{filepath.Join(t.TempDir(), "seat-a"), filepath.Join(t.TempDir(), "seat-b")}
	accepted := goalSyncMutationGit(t, landingRoot, "rev-parse", goal.AcceptedRef)
	for index, seat := range seatRoots {
		goalSyncMutationGit(t, landingRoot, "clone", "-q", origin, seat)
		goalSyncMutationGit(t, seat, "config", "metasystem.goal.machine", "seat-"+string(rune('a'+index)))
		goalSyncMutationGit(t, seat, "config", "goal.sync-remote", "local")
		goalSyncMutationGit(t, seat, "fetch", "-q", landingRoot, accepted)
		goalSyncMutationGit(t, seat, "update-ref", goal.AcceptedRef, "FETCH_HEAD")
	}

	endpoint, err := goal.ResolveEndpoint(landingRoot)
	if err != nil {
		t.Fatal(err)
	}
	ledger := gaterun.GoalCadenceLedger{Endpoint: endpoint, Actor: goal.Actor{Machine: "mac-cli", Lineage: "cadence"}}
	oldTrunk := gaterun.CadenceTrunk{Commit: goalSyncMutationGit(t, landingRoot, "rev-parse", "origin/main"), Tree: goalSyncMutationGit(t, landingRoot, "rev-parse", "origin/main^{tree}")}
	greenProbe := cadenceBatchProbe("passed", strings.Repeat("1", 64))
	greenDeps := cadenceBatchDependencies(landingRoot, contract, clock, ledger, oldTrunk, greenProbe, nil, nil)
	green, err := gaterun.RunCadenceTick(gaterun.CadenceTickInput{DeepOnlyGroups: deepOnly, Lease: time.Hour}, greenDeps)
	if err != nil || green.Status == nil || !green.Status.Green() {
		t.Fatalf("record initial green=%+v err=%v", green, err)
	}

	if err := os.WriteFile(filepath.Join(landingRoot, "app", "deep.flag"), []byte("planted-red\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, landingRoot, "add", "app/deep.flag")
	goalSyncMutationGit(t, landingRoot, "commit", "-q", "-m", "plant deep-only red")
	goalSyncMutationGit(t, landingRoot, "push", "-q", "origin", "main")
	newTrunk := gaterun.CadenceTrunk{Commit: goalSyncMutationGit(t, landingRoot, "rev-parse", "origin/main"), Tree: goalSyncMutationGit(t, landingRoot, "rev-parse", "origin/main^{tree}")}
	clockNow = start.Add(time.Minute)
	redProbe := cadenceBatchProbe("failed", strings.Repeat("2", 64))
	entered, release := make(chan struct{}), make(chan struct{})
	var native atomic.Int32
	redDeps := cadenceBatchDependencies(landingRoot, contract, clock, ledger, newTrunk, redProbe, &native, func() { close(entered); <-release })
	input := gaterun.CadenceTickInput{Latest: green.Status, DeepOnlyGroups: deepOnly, Lease: time.Hour}
	type tickAnswer struct {
		result gaterun.CadenceTickResult
		err    error
	}
	firstDone := make(chan tickAnswer, 1)
	go func() {
		result, tickErr := gaterun.RunCadenceTick(input, redDeps)
		firstDone <- tickAnswer{result: result, err: tickErr}
	}()
	select {
	case <-entered:
	case answer := <-firstDone:
		t.Fatalf("first cadence tick returned before entering its run: result=%+v err=%v", answer.result, answer.err)
	}
	joined, joinedErr := gaterun.RunCadenceTick(input, redDeps)
	close(release)
	first := <-firstDone
	if first.err != nil || joinedErr != nil || native.Load() != 1 || !first.result.Executed || joined.ClaimOutcome != goal.CadenceClaimJoined {
		t.Fatalf("concurrent ticks first=%+v joined=%+v native=%d errors=%v/%v", first.result, joined, native.Load(), first.err, joinedErr)
	}
	projection, err := goal.Project(endpoint, false, clockNow)
	if err != nil || projection.Tree.Cadence == nil || projection.Tree.Cadence.Green() || projection.Tree.Cadence.TrunkCommit != newTrunk.Commit ||
		projection.Tree.Cadence.TrunkTree != newTrunk.Tree || len(projection.Tree.TrunkRed) != 1 || projection.Tree.TrunkRed[0].Owner.Machine != "" {
		t.Fatalf("published cadence=%+v red=%+v err=%v", projection.Tree.Cadence, projection.Tree.TrunkRed, err)
	}
	// The fixture's standing-validation claim belongs to mac-cli, which never
	// publishes presence into these clones, so goal next flags that holder on
	// standard error. The flag is the only standard-error line a seat may
	// print, and it names no clock: an unpublished holder has no since.
	wantStderr := "goal standing-validation is held by mac-cli, which has published no presence; a human reassigns it with goal steal\n"
	for _, seat := range seatRoots {
		goalSyncMutationGit(t, seat, "fetch", "-q", landingRoot, projection.Tip)
		goalSyncMutationGit(t, seat, "update-ref", goal.AcceptedRef, "FETCH_HEAD")
		code, stdout, stderr := captureCommandOutput(t, true, true, func() int { return runGoalNext([]string{"--root", seat}) })
		if code != 0 || stderr != wantStderr || !strings.Contains(stdout, "owned by nobody") {
			t.Fatalf("seat %s next code=%d stdout=%q stderr=%q", filepath.Base(seat), code, stdout, stderr)
		}
	}

	store := batch.NewStore(landingRoot, nil)
	untouchedID := "01j5x00000000000000000ca01"
	if err := store.Create(batch.Record{Schema: 1, BatchID: untouchedID, State: batch.StateOpen, BaseTree: newTrunk.Tree, TipTree: newTrunk.Tree}); err != nil {
		t.Fatal(err)
	}
	if _, err := classifyBatchCadenceStatus(nil, clockNow); err != nil {
		t.Fatal(err)
	}
	overdue := *green.Status
	overdue.ForcedWindowStart = clockNow.Add(-gaterun.CadenceForcedInterval - time.Minute).Format(time.RFC3339)
	if _, err := classifyBatchCadenceStatus(&overdue, clockNow); err != nil {
		t.Fatal(err)
	}
	if record, err := store.Load(untouchedID); err != nil || record.State != batch.StateOpen {
		t.Fatalf("missing/overdue status held a batch: state=%s err=%v", record.State, err)
	}

	heldID := "01j5x00000000000000000ca02"
	claim := batch.Claim{Machine: "mac-cli", Lineage: landingOwnerLineage, Epoch: 1, Revision: 2, AccountingRevision: 2}
	if err := store.Create(batch.Record{Schema: 1, BatchID: heldID, State: batch.StateLanding, BaseTree: newTrunk.Tree, TipTree: newTrunk.Tree,
		Proof: &batch.Proof{Status: "green", AttemptID: "standard-green", BaseCommit: newTrunk.Commit, Passed: []string{"ordinary"}},
		Units: []batch.Unit{{GoalID: cadenceAuthorityGoal, Chain: "chain", State: batch.UnitJoined,
			Claim: claim}}}); err != nil {
		t.Fatal(err)
	}
	ledgerOwner, err := newLedgerTrunkRedOwner(landingRoot, "mac-cli", landingOwnerLineage)
	if err != nil {
		t.Fatal(err)
	}
	ledgerOwner.(*ledgerTrunkRedOwner).now = clock
	settings, err := config.NewBatchLanding(landingRoot, time.Minute, clock)
	if err != nil {
		t.Fatal(err)
	}
	landings := 0
	owner, err := batch.NewOwner(batch.OwnerOptions{Store: store.WithLedgerOwner(ledgerOwner), Settings: settings, Actor: landingOwnerLineage, PID: 7, Now: clock,
		FetchTree: func() (string, error) { return newTrunk.Tree, nil }, ReadClaim: func(string, string, string, string) (batch.Claim, error) { return claim, nil },
		Rebind: func(string, string) error { return nil }, Mint: func() (string, error) { return "cadence-existing-red-hold", nil },
		LogRed: func(string, batch.TrunkRedRecordOutcome) {}, BaseCommit: func(string) (string, error) { return newTrunk.Commit, nil },
		RunDiagnostic: func(string, batch.DiagnosticRequest, batch.Claim) (batch.DiagnosticResult, error) {
			return batch.DiagnosticResult{}, nil
		},
		DescendsFrom: func(string, string) (bool, error) { return true, nil }, Sample: func() proofrun.LoadSample { return proofrun.LoadSample{OverlapKnown: true} },
		Admission: func(proofrun.LoadSample) proofrun.AdmissionCap { return proofrun.AdmissionCap{Max: 1} },
		Launch:    func(string, proofrun.LoadSample, string) error { landings++; return nil }, After: func(time.Duration) <-chan time.Time { return make(chan time.Time) },
		Report: func(string, error) {}})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Tick(heldID); err != nil {
		t.Fatal(err)
	}
	held, err := store.Load(heldID)
	if err != nil || held.State != batch.StateHeldTrunkRed || held.TrunkRed == nil || len(held.TrunkRed.Entries) != 1 || landings != 0 {
		t.Fatalf("next batch=%+v err=%v", held, err)
	}
}

func cadenceBatchProbe(status, identity string) proofrun.GroupResult {
	group := proofrun.GroupResult{ID: "section/deep", Kind: "section", InputManifest: []string{"app/deep.flag"},
		ExecutionIdentity: identity, Status: status, NativeLaunched: true, CollectionComplete: true}
	if status == "failed" {
		group.Observed = []proofrun.NativeTestIdentity{{Report: "deep.xml", Classname: "Cadence", Name: "TestDeepOnly", Status: "failed", Reason: "planted"}}
	}
	return group
}

func cadenceBatchDependencies(root string, contract testpolicy.Contract, clock func() time.Time, ledger gaterun.CadenceLedger, trunk gaterun.CadenceTrunk, probe proofrun.GroupResult, executions *atomic.Int32, beforeReturn func()) gaterun.CadenceDependencies {
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeCadence, RequestedMode: testpolicy.ModeDeep, RequiredMode: testpolicy.ModeDeep,
		ExecutedMode: testpolicy.ModeDeep, RequiredGroups: []string{probe.ID}, SelectedGroups: []string{probe.ID},
		Stages: []testpolicy.Stage{{ID: "deep", Groups: []string{probe.ID}}}}
	request := proofrun.TestRunRequest{ControlRoot: root, ProjectRoot: root, CandidateTree: trunk.Tree, BaseCommit: trunk.Commit,
		PolicyBaseCommit: trunk.Commit, Contract: contract, Plan: plan}
	return gaterun.CadenceDependencies{Clock: clock, Fetch: func() (gaterun.CadenceTrunk, error) { return trunk, nil }, Ledger: ledger,
		Revalidate: func(gaterun.CadenceTrunk) (gaterun.CadenceRevalidation, error) {
			identities, err := proofrun.GroupExecutionIdentities(context.Background(), request)
			if err != nil {
				return gaterun.CadenceRevalidation{}, err
			}
			current := probe
			current.ExecutionIdentity = identities[probe.ID]
			return gaterun.CadenceRevalidation{Groups: []proofrun.GroupResult{current}}, nil
		}, ClaimAuthority: func(time.Time) (gaterun.CadenceAuthority, error) {
			return gaterun.CadenceAuthority{GoalID: cadenceAuthorityGoal, ObligationRevision: 1}, nil
		}, ReleaseAuthority: func(gaterun.CadenceAuthority, time.Time) error { return nil },
		Run: func(gaterun.CadenceRunRequest) (gaterun.CadenceRunResult, error) {
			attemptID := "attempt-native-" + trunk.Tree[:8]
			runRequest := request
			runRequest.AttemptID, runRequest.LogRoot = attemptID, filepath.Join(root, "artifacts", "cadence-native", attemptID)
			result, _, err := proofrun.RunTestPlan(context.Background(), runRequest)
			if err != nil || len(result.Groups) != 1 || result.Groups[0].Status != probe.Status || !result.Groups[0].NativeLaunched {
				return gaterun.CadenceRunResult{}, errors.Join(err, errors.New("native deep-only fixture produced an unexpected result"))
			}
			if executions != nil {
				executions.Add(1)
			}
			if beforeReturn != nil {
				beforeReturn()
			}
			return gaterun.CadenceRunResult{RunID: "cadence-native", Result: result}, nil
		}}
}
