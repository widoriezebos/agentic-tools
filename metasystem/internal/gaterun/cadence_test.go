package gaterun

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

var cadenceTestStart = time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)

type cadenceTestLedger struct {
	mu        sync.Mutex
	claim     *goal.CadenceClaim
	status    *goal.CadenceStatus
	published []goal.TrunkRedRecordGroup
	claims    int
}

func (ledger *cadenceTestLedger) Claim(now time.Time, key goal.CadenceClaimKey, lease time.Duration) (goal.CadenceClaimResult, error) {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.status != nil && ledger.status.Key() == key {
		copy := *ledger.status
		return goal.CadenceClaimResult{Outcome: goal.CadenceClaimComplete, Status: &copy}, nil
	}
	if ledger.claim != nil {
		until, _ := time.Parse(time.RFC3339, ledger.claim.LeaseUntil)
		if now.Before(until) {
			outcome := goal.CadenceClaimOccupied
			if ledger.claim.Key == key {
				outcome = goal.CadenceClaimJoined
			}
			copy := *ledger.claim
			return goal.CadenceClaimResult{Outcome: outcome, Claim: &copy}, nil
		}
	}
	ledger.claims++
	ledger.claim = &goal.CadenceClaim{Key: key, OwnerMachine: "landing", Opid: "claim", ClaimedAt: now.Format(time.RFC3339), LeaseUntil: now.Add(lease).Format(time.RFC3339)}
	copy := *ledger.claim
	return goal.CadenceClaimResult{Outcome: goal.CadenceClaimAcquired, Claim: &copy}, nil
}

func (ledger *cadenceTestLedger) Publish(_ time.Time, claim string, status goal.CadenceStatus, groups []goal.TrunkRedRecordGroup) error {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.claim == nil || ledger.claim.Opid != claim {
		return errors.New("claim lost")
	}
	ledger.status, ledger.claim = &status, nil
	ledger.published = append([]goal.TrunkRedRecordGroup(nil), groups...)
	return nil
}

func cadenceProbe(id, identity, status string) proofrun.GroupResult {
	group := proofrun.GroupResult{ID: id, Kind: "unit", InputManifest: []string{"source"}, ExecutionIdentity: identity,
		Status: status, CollectionComplete: status == "passed" || status == "reused"}
	if status == "reused" {
		group.ReuseAttempt = "retained-attempt"
	}
	return group
}

func cadenceLatest(now time.Time, trunk CadenceTrunk, identity, status string) *goal.CadenceStatus {
	return &goal.CadenceStatus{TrunkCommit: trunk.Commit, TrunkTree: trunk.Tree, Trigger: goal.CadenceTriggerForcedWindow,
		RunID: "run-old", AttemptID: "attempt-old", StartedAt: now.Format(time.RFC3339), EndedAt: now.Format(time.RFC3339),
		ForcedWindowStart: now.Format(time.RFC3339), Groups: []goal.CadenceGroupStatus{{Group: "section/deep", ExecutionIdentity: identity,
			Status: status, EvidenceDigest: strings.Repeat("e", 64)}}}
}

func cadenceRunResult(probe proofrun.GroupResult, status string) CadenceRunResult {
	group := probe
	group.Status, group.CollectionComplete, group.ReuseAttempt = status, true, ""
	if status == "failed" {
		group.Observed = []proofrun.NativeTestIdentity{{Report: "report.xml", Classname: "fixture", Name: "deep", Status: "failed", Reason: "planted"}}
	}
	return CadenceRunResult{RunID: "run-new", Result: proofrun.TestResult{AttemptID: "attempt-new", Groups: []proofrun.GroupResult{group}}}
}

func cadenceFixture(now *time.Time, probe proofrun.GroupResult) (CadenceTickInput, CadenceDependencies, *cadenceTestLedger, *int) {
	trunk := CadenceTrunk{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40)}
	latest := cadenceLatest(cadenceTestStart, trunk, probe.ExecutionIdentity, "passed")
	ledger, executions := &cadenceTestLedger{}, new(int)
	input := CadenceTickInput{Latest: latest, Weight: WeightState{Generation: 4}, DeepOnlyGroups: []string{probe.ID}, Lease: time.Hour}
	deps := CadenceDependencies{Clock: func() time.Time { return *now }, Fetch: func() (CadenceTrunk, error) { return trunk, nil },
		Revalidate: func(CadenceTrunk) (CadenceRevalidation, error) {
			return CadenceRevalidation{Groups: []proofrun.GroupResult{probe}}, nil
		}, Ledger: ledger,
		ClaimAuthority: func(time.Time) (CadenceAuthority, error) {
			return CadenceAuthority{GoalID: "standing-validation", ObligationRevision: 3}, nil
		},
		ReleaseAuthority: func(CadenceAuthority, time.Time) error { return nil },
		Run: func(CadenceRunRequest) (CadenceRunResult, error) {
			*executions++
			return cadenceRunResult(probe, "passed"), nil
		},
		DischargeWeight: func(CadenceAuthority, string, uint64, time.Time) error { return nil }}
	return input, deps, ledger, executions
}

func TestCadenceTriggerFiresOnChangedMissingOrNonGreenIdentity(t *testing.T) {
	identity := strings.Repeat("1", 64)
	for _, test := range []struct {
		name, status string
		trigger      goal.CadenceTrigger
		mutate       func(*CadenceTickInput)
	}{
		{name: "changed", status: "reused", trigger: goal.CadenceTriggerIdentityChanged, mutate: func(input *CadenceTickInput) { input.Latest.Groups[0].ExecutionIdentity = strings.Repeat("2", 64) }},
		{name: "missing", status: "not-run", trigger: goal.CadenceTriggerIdentityChanged},
		{name: "newest non-green", status: "failed", trigger: goal.CadenceTriggerIdentityChanged},
		{name: "weight due", status: "reused", trigger: goal.CadenceTriggerWeightDue, mutate: func(input *CadenceTickInput) { input.WeightDue = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			now, probe := cadenceTestStart.Add(time.Hour), cadenceProbe("section/deep", identity, test.status)
			input, deps, _, executions := cadenceFixture(&now, probe)
			discharged := false
			deps.DischargeWeight = func(CadenceAuthority, string, uint64, time.Time) error { discharged = true; return nil }
			if test.mutate != nil {
				test.mutate(&input)
			}
			result, err := RunCadenceTick(input, deps)
			if err != nil || *executions != 1 || result.Status == nil || result.Status.Trigger != test.trigger || result.ForceGroups || discharged != input.WeightDue {
				t.Fatalf("tick=%+v executions=%d err=%v", result, *executions, err)
			}
		})
	}
}

func TestCadenceTriggerForcesEverySixHours(t *testing.T) {
	now, probe := cadenceTestStart.Add(CadenceForcedInterval-time.Second), cadenceProbe("section/deep", strings.Repeat("1", 64), "reused")
	input, deps, _, executions := cadenceFixture(&now, probe)
	if result, err := RunCadenceTick(input, deps); err != nil || result.Executed || *executions != 0 {
		t.Fatalf("early tick=%+v executions=%d err=%v", result, *executions, err)
	}
	now = cadenceTestStart.Add(CadenceForcedInterval)
	var request CadenceRunRequest
	deps.Run = func(got CadenceRunRequest) (CadenceRunResult, error) {
		request = got
		*executions++
		return cadenceRunResult(probe, "passed"), nil
	}
	result, err := RunCadenceTick(input, deps)
	if err != nil || *executions != 1 || !result.ForceGroups || !request.ForceAttempt || !request.ForceGroups || result.Status.Trigger != goal.CadenceTriggerForcedWindow {
		t.Fatalf("forced tick=%+v request=%+v executions=%d err=%v", result, request, *executions, err)
	}
}

func TestCadenceUnrelatedTipPublishesRevalidationOnly(t *testing.T) {
	now, probe := cadenceTestStart.Add(time.Hour), cadenceProbe("section/deep", strings.Repeat("1", 64), "reused")
	input, deps, _, executions := cadenceFixture(&now, probe)
	input.Latest.TrunkCommit, input.Latest.TrunkTree = strings.Repeat("c", 40), strings.Repeat("d", 40)
	root, _ := governedWeightBed(t, cadenceTestStart)
	endpoint, endpointErr := goal.ResolveEndpoint(root)
	if endpointErr != nil {
		t.Fatal(endpointErr)
	}
	ulids := []string{"01J5X00000000000000000CR01", "01J5X00000000000000000CR02"}
	deps.Ledger = GoalCadenceLedger{Endpoint: endpoint, Actor: goal.Actor{Machine: "bed-m1", Lineage: "cadence"}, MintULID: func() (string, error) {
		ulid := ulids[0]
		ulids = ulids[1:]
		return ulid, nil
	}}
	result, err := RunCadenceTick(input, deps)
	projection, projectErr := goal.Project(endpoint, false, now)
	if err != nil || projectErr != nil || *executions != 0 || !result.Published || result.Status.Trigger != goal.CadenceTriggerRevalidation || result.Status.RunID != "" || result.Status.AttemptID != "" || len(projection.Tree.TrunkRed) != 0 || result.Status.Groups[0].Status != "reused" {
		t.Fatalf("revalidation tick=%+v executions=%d red=%+v err=%v project=%v", result, *executions, projection.Tree.TrunkRed, err, projectErr)
	}
}

func TestCadenceTickJoinsLiveClaimAndReadsTerminalResult(t *testing.T) {
	now, probe := cadenceTestStart.Add(time.Hour), cadenceProbe("section/deep", strings.Repeat("1", 64), "failed")
	input, deps, _, executions := cadenceFixture(&now, probe)
	entered, release, firstDone := make(chan struct{}), make(chan struct{}), make(chan CadenceTickResult)
	releases := 0
	deps.ReleaseAuthority = func(CadenceAuthority, time.Time) error { releases++; return nil }
	deps.Run = func(CadenceRunRequest) (CadenceRunResult, error) {
		*executions++
		close(entered)
		<-release
		return cadenceRunResult(probe, "passed"), nil
	}
	go func() { result, _ := RunCadenceTick(input, deps); firstDone <- result }()
	<-entered
	joined, err := RunCadenceTick(input, deps)
	if err != nil || joined.ClaimOutcome != goal.CadenceClaimJoined || joined.Executed {
		t.Fatalf("joined tick=%+v err=%v", joined, err)
	}
	close(release)
	first := <-firstDone
	complete, err := RunCadenceTick(input, deps)
	if err != nil || !first.Published || complete.ClaimOutcome != goal.CadenceClaimComplete || complete.Status == nil || *executions != 1 || releases != 1 {
		t.Fatalf("first=%+v complete=%+v executions=%d releases=%d err=%v", first, complete, *executions, releases, err)
	}
}

func TestCadenceTickRecoversDeadClaimAfterLease(t *testing.T) {
	now, probe := cadenceTestStart.Add(2*time.Hour), cadenceProbe("section/deep", strings.Repeat("1", 64), "failed")
	input, deps, _, executions := cadenceFixture(&now, probe)
	root, _ := governedWeightBed(t, cadenceTestStart)
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		t.Fatal(err)
	}
	ulids := []string{"01J5X00000000000000000CD01", "01J5X00000000000000000CD02", "01J5X00000000000000000CD03"}
	ledger := GoalCadenceLedger{Endpoint: endpoint, Actor: goal.Actor{Machine: "bed-m1", Lineage: "cadence"}, MintULID: func() (string, error) {
		ulid := ulids[0]
		ulids = ulids[1:]
		return ulid, nil
	}}
	deps.Ledger = ledger
	key := goal.CadenceClaimKey{TrunkTree: strings.Repeat("b", 40), WeightGeneration: 4, ForcedWindowStart: cadenceTestStart.Format(time.RFC3339)}
	if first, claimErr := ledger.Claim(cadenceTestStart, key, time.Minute); claimErr != nil || first.Outcome != goal.CadenceClaimAcquired {
		t.Fatalf("seed dead claim=%+v err=%v", first, claimErr)
	}
	result, err := RunCadenceTick(input, deps)
	if err != nil || result.ClaimOutcome != goal.CadenceClaimAcquired || *executions != 1 || !result.Published {
		t.Fatalf("recovery=%+v executions=%d err=%v", result, *executions, err)
	}
}

func TestCadenceAbsentStandingAuthorityPublishesNonGreen(t *testing.T) {
	now, probe := cadenceTestStart.Add(time.Hour), cadenceProbe("section/deep", strings.Repeat("1", 64), "failed")
	input, deps, ledger, executions := cadenceFixture(&now, probe)
	deps.ClaimAuthority = func(time.Time) (CadenceAuthority, error) { return CadenceAuthority{}, errors.New("absent") }
	result, err := RunCadenceTick(input, deps)
	if err != nil || *executions != 0 || !result.Published || result.Status == nil || result.Status.Green() || len(ledger.published) != 1 || ledger.published[0].Status != "unavailable" {
		t.Fatalf("authority refusal=%+v executions=%d red=%+v err=%v", result, *executions, ledger.published, err)
	}
}

func TestCadenceRedPublishesThroughTheBatchMapping(t *testing.T) {
	now, probe := cadenceTestStart.Add(time.Hour), cadenceProbe("section/deep", strings.Repeat("1", 64), "failed")
	input, deps, _, _ := cadenceFixture(&now, probe)
	root, _ := governedWeightBed(t, cadenceTestStart)
	endpoint, endpointErr := goal.ResolveEndpoint(root)
	if endpointErr != nil {
		t.Fatal(endpointErr)
	}
	ulids := []string{"01J5X00000000000000000RD01", "01J5X00000000000000000RD02"}
	deps.Ledger = GoalCadenceLedger{Endpoint: endpoint, Actor: goal.Actor{Machine: "bed-m1", Lineage: "cadence"}, MintULID: func() (string, error) {
		ulid := ulids[0]
		ulids = ulids[1:]
		return ulid, nil
	}}
	deps.Run = func(CadenceRunRequest) (CadenceRunResult, error) { return cadenceRunResult(probe, "failed"), nil }
	result, err := RunCadenceTick(input, deps)
	projection, projectErr := goal.Project(endpoint, false, now)
	expected := batch.RedGroup{ID: probe.ID, Status: "failed", InputManifest: []string{"source"}, Failures: []batch.Failure{{Report: "report.xml", Classname: "fixture", Name: "deep", Status: "failed", Reason: "planted"}}}
	records := projection.Tree.TrunkRed
	if err != nil || projectErr != nil || result.Status.Green() || len(records) != 1 || records[0].Identity != batch.TrunkRedID(expected) ||
		len(records[0].Failures) != 1 || records[0].Failures[0].Reason != "planted" || records[0].Owner.Machine != "" || records[0].FixGoal != "" {
		t.Fatalf("cadence red=%+v record=%+v err=%v project=%v", result.Status, records, err, projectErr)
	}
}

func TestCadencePackagesUseNoWallClock(t *testing.T) {
	paths, err := filepath.Glob("cadence*.go")
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, filepath.Join("..", "proofrun", "reuse_policy.go"))
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"time.Now(", "time.Sleep(", "time.After("} {
			if strings.Contains(string(data), forbidden) {
				t.Errorf("%s uses %s", path, forbidden)
			}
		}
	}
}
