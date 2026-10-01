package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

func TestBatchCostLandingReadyElapsedAuthorityMatchesDispatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	claimed := now.Add(-2 * time.Hour).Format(time.RFC3339)
	claim := batch.Claim{Machine: "source", Lineage: "lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}
	unit := batch.Unit{GoalID: "goal-ready", State: batch.UnitJoined, Claim: claim}
	record := batch.Record{BatchID: "01j5x00000000000000000ba98", Seal: map[string]batch.Claim{"goal-ready": claim}}
	// Tier 1: below the landing gate's default threshold, so the gate
	// proceeds and the elapsed authority is what this bed reads.
	file := &goal.GoalFile{Id: "goal-ready", State: goal.StateClaimed, Revision: 2, Tier: 1,
		Budget: &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1},
		Claimed: &goal.ClaimRecord{At: claimed, Revision: 2, AccountingRevision: 2,
			HandedOver: goal.HandedOver{FromMachine: claim.Machine, FromLineage: claim.Lineage, FromEpoch: claim.Epoch, Batch: record.BatchID}},
		History: []goal.HistoryLine{{At: now.Add(-3 * time.Hour).Format(time.RFC3339), Verb: "open"}, {At: claimed, Verb: "claim"}},
		Landing: &goal.LandingRecord{At: now.Add(-time.Minute).Format(time.RFC3339)},
	}
	projection := goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{unit.GoalID: file}}}
	if err := batchowner.AuthorizeBatchMemberInProjection(root, now, record, unit, projection); err != nil {
		t.Fatalf("landing-ready claim lost elapsed suspension: %v", err)
	}
	file.Landing = nil
	var budget *batch.PrefixBudgetRefusal
	if err := batchowner.AuthorizeBatchMemberInProjection(root, now, record, unit, projection); !errors.As(err, &budget) {
		t.Fatalf("ordinary elapsed claim was not refused: %v", err)
	}
	file.Landing = &goal.LandingRecord{At: now.Add(-time.Minute).Format(time.RFC3339)}
	file.StopFence = &goal.StopFence{StopID: "stop-goal-ready-r2"}
	var fenced *batch.PrefixFencedRefusal
	if err := batchowner.AuthorizeBatchMemberInProjection(root, now, record, unit, projection); !errors.As(err, &fenced) {
		t.Fatalf("landing-ready elapsed suspension escaped a live fence: %v", err)
	}
}

func TestBatchCostFreshForecastMatchesRetainedVerificationAtExpiry(t *testing.T) {
	t.Parallel()
	fixture := newPortableFileProof(t)
	fixture.contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	maxAge := int64(time.Hour / time.Millisecond)
	fixture.contract.Groups[0].Phase = "acceptance"
	fixture.contract.Groups[0].EnvironmentMode = "inherit"
	fixture.contract.Groups[0].Freshness = "episode"
	fixture.contract.Groups[0].FreshnessMaxAgeMS = &maxAge
	fixture.writeContract()
	tree, files := fixture.snapshot()
	plan := fixture.plan(fixture.loadedContract(files), "app/a.txt")
	run := fixture.request(tree, files, plan)
	run.CandidateEngineBuildIdentity = fixture.engineIdentity(tree, run.Environment)
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	run.FreshnessEpisode, run.FreshnessExpiresAt = strings.Repeat("6", 64), expiresAt.Format(time.RFC3339Nano)
	run.FreshGroups = map[string]bool{"app-a": true}
	fixture.bindFreshness(&run)
	result, _ := fixture.execute(run, true)
	if group := portableGroups(result)["app-a"]; group.Status != "passed" || !group.NativeLaunched {
		t.Fatalf("fresh command/JUnit result is incomplete: %+v", group)
	}
	requirePortableCounts(t, fixture.counts(), map[string]int{"a": 1})
	selection := testrun.CostSelection{ID: "tip:fresh", Kind: "tip", Tree: tree, GoalID: "portable", Requirements: []string{"app-a"},
		FreshEpisode: run.FreshnessEpisode, FreshExpiresAt: run.FreshnessExpiresAt}
	selected := testrun.SelectionRequest{Root: fixture.root, GoalID: "portable", Tree: tree, Mode: testpolicy.ModeAuto,
		Purpose: testpolicy.PurposeDelivery, BatchPrefixReceipt: true, BatchRequirements: []string{"app-a"},
		FreshEpisode: run.FreshnessEpisode, FreshExpiresAt: run.FreshnessExpiresAt}
	prepared := fixture.prepared(run, result)
	for _, boundary := range []struct {
		name     string
		at       time.Time
		reusable bool
	}{
		{"expiry-minus-one-nanosecond", expiresAt.Add(-time.Nanosecond), true},
		{"exact-expiry", expiresAt, false},
		{"expiry-plus-one-nanosecond", expiresAt.Add(time.Nanosecond), false},
	} {
		t.Run(boundary.name, func(t *testing.T) {
			engine, checkEngine := fixture.engineIO(tree)
			projection, checkProjection := fixture.projection(tree)
			forecast, forecastErr := testrun.ForecastPrepared(selection, 2, selected, prepared, testrun.Forecasting{
				Workspace: projection, CandidateIO: engine, OpenCandidate: fixture.open(tree, files),
				Now: func() time.Time { return boundary.at }, WorkerPolicy: testingWorkerPolicy,
			})
			checkEngine()
			checkProjection()
			verifyEngine, checkVerifyEngine := fixture.engineIO(tree)
			verifyProjection := gittree.Workspace{Dir: fixture.root}
			checkVerifyProjection := func() {}
			if boundary.reusable {
				verifyProjection, checkVerifyProjection = fixture.projection(tree)
			}
			verified, verifyErr := testrun.VerifyPrepared(selected, prepared, testrun.Verification{
				Clock: func() time.Time { return boundary.at }, Revalidate: proofrun.RevalidateRetainedGroupExecutionIdentities,
				Workspace: verifyProjection, CandidateIO: verifyEngine, OpenCandidate: fixture.open(tree, files), WorkerPolicy: testingWorkerPolicy,
			})
			if boundary.reusable {
				checkVerifyEngine()
				checkVerifyProjection()
				if forecastErr != nil || len(forecast.Groups) != 1 || forecast.Groups[0].GroupID != "app-a" ||
					forecast.Groups[0].Status != "reusable" || forecast.Groups[0].Reason != "" ||
					verifyErr != nil || !verified.Delivery.Sufficient {
					t.Fatalf("freshness boundary %s forecast=%+v forecastErr=%v verified=%+v verifyErr=%v",
						boundary.at, forecast, forecastErr, verified.Delivery, verifyErr)
				}
				return
			}
			if forecastErr != nil || len(forecast.Groups) != 1 || forecast.Groups[0].GroupID != "app-a" ||
				forecast.Groups[0].Status != "missing" || forecast.Groups[0].Reason != "missing-proof" {
				t.Fatalf("expired forecast at %s: %+v err=%v", boundary.at, forecast, forecastErr)
			}
			if !errors.Is(verifyErr, proofrun.ErrFreshnessExpired) {
				t.Fatalf("retained verification at %s: %v", boundary.at, verifyErr)
			}
		})
	}
}

func TestBatchCostTipFirstSharesKnownIdentityButNotUnknownOrFresh(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	units := []batch.Unit{{GoalID: "goal-a", Chain: "a", State: batch.UnitJoined},
		{GoalID: "goal-b", Chain: "b", State: batch.UnitJoined}, {GoalID: "goal-c", Chain: "c", State: batch.UnitJoined}}
	record := batch.Record{TipTree: "tree-c", Units: units}
	// Newer prefixes introduce B and C. A and B native identities are the
	// same across later trees, while unknown metadata and fresh episodes stay
	// distinct requirements.
	record.BaseTree, record.PrefixTrees = "base", []string{"tree-a", "tree-b", "tree-c"}
	decision := func(_ string, units []batch.Unit, _ string) (batch.PrefixDecision, error) {
		ids := []string{"a"}
		if len(units) >= 2 {
			ids = append(ids, "b", "u", "fresh")
		}
		if len(units) == 3 {
			ids = append(ids, "c")
		}
		return batch.PrefixDecision{Groups: ids}, nil
	}
	run := func(_ string, selection testrun.CostSelection, cap uint64) (testrun.CostEvidence, error) {
		out := testrun.CostEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID, SelectedGroups: slices.Clone(selection.Requirements)}}
		for _, id := range selection.Requirements {
			row := batch.CostForecastGroup{RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
				GroupID: id, ExecutionIdentity: "same-" + id, IdentityKnown: true, Status: "missing",
				DeclaredAllowanceMS: int64(cap) * 60000}
			if id == "u" {
				row.IdentityKnown, row.ExecutionIdentity, row.Status = false, "", "unknown"
			}
			if id == "fresh" {
				row.Reason = "fresh-episode-not-created"
			}
			if id == "b" {
				row.ExclusiveResources = []string{"shared-device"}
			}
			if id == "c" {
				row.ExclusiveResources = []string{"shared-device"}
			}
			out.Groups = append(out.Groups, row)
		}
		return out, nil
	}
	budget := func(_ string, _ batch.Unit, _ *batch.Unit, _ time.Time) (batchowner.BatchCostBudgetProjection, error) {
		return batchowner.BatchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown, Limits: goal.Budget{
			ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}}}, nil
	}
	forecast, err := batchowner.ForecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if forecast.Requests[0].ChargeGoal != "goal-c" || !forecast.Requests[0].NeedsAttempt ||
		forecast.Requests[1].ChargeGoal != "goal-a" || forecast.Requests[1].NeedsAttempt ||
		forecast.Requests[2].ChargeGoal != "goal-b" || !forecast.Requests[2].NeedsAttempt {
		t.Fatalf("tip-first per-owner demand: %+v", forecast.Requests)
	}
	if forecast.KnownMissing != 5 || forecast.UnknownMissing != 2 || forecast.DeclaredReservationMin != 4 ||
		!slices.Equal(forecast.ExclusiveResourceConflicts, []string{"shared-device"}) {
		t.Fatalf("identity/unknown/fresh/conflict accounting: %+v", forecast)
	}
	if forecast.Budgets[0].AttemptDemand != 0 || forecast.Budgets[1].AttemptDemand != 1 || forecast.Budgets[2].AttemptDemand != 1 {
		t.Fatalf("per-owner attempt demand: %+v", forecast.Budgets)
	}
}

func TestBatchCostReplacementFreshEpisodeChangesBindingAndNativeDemand(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	units := []batch.Unit{{GoalID: "goal-a", Chain: "a", State: batch.UnitJoined},
		{GoalID: "goal-b", Chain: "b", State: batch.UnitJoined}, {GoalID: "goal-c", Chain: "c", State: batch.UnitJoined}}
	record := batch.Record{BaseTree: "base", TipTree: "tree-c", PrefixTrees: []string{"tree-a", "tree-b", "tree-c"}, Units: units,
		Seal: map[string]batch.Claim{"goal-a": {}, "goal-b": {}, "goal-c": {}}, PrefixEpisodes: map[string]batch.PrefixEpisode{}}
	decision := func(_ string, _ []batch.Unit, _ string) (batch.PrefixDecision, error) {
		return batch.PrefixDecision{Groups: []string{"shared"}, FreshRequired: true, FreshMaxAgeMS: 60000}, nil
	}
	for index, unit := range units {
		planned, err := decision(root, units[:index+1], record.PrefixTrees[index])
		if err != nil {
			t.Fatal(err)
		}
		id, err := batch.PrefixDecisionID(record.BaseTree, record.PrefixTrees[index], units[:index+1], record.Seal, planned)
		if err != nil {
			t.Fatal(err)
		}
		record.PrefixEpisodes[unit.GoalID] = batch.PrefixEpisode{DecisionID: id, Token: strings.Repeat(string(rune('a'+index)), 64),
			ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)}
	}
	run := func(_ string, selection testrun.CostSelection, cap uint64) (testrun.CostEvidence, error) {
		return testrun.CostEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{{
			RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
			GroupID: "shared", ExecutionIdentity: "same-identity", IdentityKnown: true, Status: "missing",
			FreshEpisode: selection.FreshEpisode, DeclaredAllowanceMS: int64(cap) * 60000,
		}}}, nil
	}
	budget := func(_ string, _ batch.Unit, _ *batch.Unit, _ time.Time) (batchowner.BatchCostBudgetProjection, error) {
		return batchowner.BatchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
			Limits: goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}}}, nil
	}
	first, err := batchowner.ForecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Requests) != 3 || first.KnownMissing != 3 {
		t.Fatalf("different episodes coalesced native demand: %+v", first)
	}
	for _, row := range first.Groups {
		if row.CoveredBy != "" || row.FreshEpisode == "" {
			t.Fatalf("fresh work reused a different episode: %+v", row)
		}
	}
	replaced := record.PrefixEpisodes["goal-b"]
	replaced.Token = strings.Repeat("d", 64)
	record.PrefixEpisodes["goal-b"] = replaced
	second, err := batchowner.ForecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(first.Binding, second.Binding) || second.Binding.PrefixEpisodes[1].Token != replaced.Token ||
		second.KnownMissing != 3 || second.Requests[2].NeedsAttempt != true {
		t.Fatalf("replacement episode kept old forecast or coalesced work: first=%+v second=%+v", first, second)
	}
}

func TestBatchCostEarlierSharedIdentityChargesFirstPrefixOwner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	units := []batch.Unit{{GoalID: "goal-a", State: batch.UnitJoined}, {GoalID: "goal-b", State: batch.UnitJoined}, {GoalID: "goal-c", State: batch.UnitJoined}}
	record := batch.Record{BaseTree: "base", TipTree: "tip", PrefixTrees: []string{"a", "b", "tip"}, Units: units}
	decision := func(_ string, selected []batch.Unit, _ string) (batch.PrefixDecision, error) {
		if len(selected) == 3 {
			return batch.PrefixDecision{Groups: []string{"tip-only"}}, nil
		}
		return batch.PrefixDecision{Groups: []string{"shared"}}, nil
	}
	run := func(_ string, selection testrun.CostSelection, cap uint64) (testrun.CostEvidence, error) {
		return testrun.CostEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{{
			RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
			GroupID: selection.Requirements[0], ExecutionIdentity: "identity-" + selection.Requirements[0],
			IdentityKnown: true, Status: "missing", DeclaredAllowanceMS: int64(cap) * 60000,
		}}}, nil
	}
	budget := func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchowner.BatchCostBudgetProjection, error) {
		attempts := uint64(1)
		if unit.GoalID == "goal-b" {
			attempts = 0
		}
		return batchowner.BatchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
			Limits: goal.Budget{ElapsedLimit: "4h", AttemptLimit: attempts, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}}}, nil
	}
	forecast, err := batchowner.ForecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal([]string{forecast.Requests[0].ChargeGoal, forecast.Requests[1].ChargeGoal, forecast.Requests[2].ChargeGoal},
		[]string{"goal-c", "goal-a", "goal-b"}) || forecast.Budgets[0].AttemptDemand != 1 ||
		forecast.Budgets[1].AttemptDemand != 0 || !forecast.Budgets[1].Fits || forecast.Budgets[2].AttemptDemand != 1 {
		t.Fatalf("tip then original-order prefix charge disagrees with receipt compositor: requests=%+v budgets=%+v", forecast.Requests, forecast.Budgets)
	}
}

func TestBatchCostBudgetDoesNotBorrowAnotherGoalsHeadroom(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	units := []batch.Unit{{GoalID: "goal-a", State: batch.UnitJoined}, {GoalID: "goal-b", State: batch.UnitJoined}}
	record := batch.Record{TipTree: "tip", Units: units}
	record.BaseTree, record.PrefixTrees = "base", []string{"prefix", "tip"}
	decision := func(_ string, _ []batch.Unit, _ string) (batch.PrefixDecision, error) {
		return batch.PrefixDecision{Groups: []string{"two-unknowns"}}, nil
	}
	run := func(_ string, selection testrun.CostSelection, cap uint64) (testrun.CostEvidence, error) {
		return testrun.CostEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{
			{RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID, GroupID: "u1", Status: "unknown", DeclaredAllowanceMS: int64(cap) * 60000},
			{RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID, GroupID: "u2", Status: "unknown", DeclaredAllowanceMS: int64(cap) * 60000},
		}}, nil
	}
	budget := func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchowner.BatchCostBudgetProjection, error) {
		limits := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}
		if unit.GoalID == "goal-b" {
			limits.AttemptLimit = 0
		}
		return batchowner.BatchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown, Limits: limits}}, nil
	}
	forecast, err := batchowner.ForecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if forecast.Requests[0].ReservationMinutes != 2 || forecast.UnknownMissing != 4 ||
		forecast.Budgets[0].Fits != true || forecast.Budgets[1].Fits || forecast.Budgets[1].Reason != "attempt-headroom" {
		t.Fatalf("per-request cap and isolated budget: %+v", forecast)
	}
	unknownBudget := func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchowner.BatchCostBudgetProjection, error) {
		if unit.GoalID == "goal-b" {
			return batchowner.BatchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetUnknown,
				Unknown: &dispatchcore.BudgetUnknownEvidence{Code: dispatchcore.BudgetUnknown,
					Record: "artifacts/agents/jobs/goal-b-unreadable.json", Reason: "retained job accounting is unreadable"}}}, nil
		}
		return batchowner.BatchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
			Limits: goal.Budget{ElapsedLimit: "4h", AttemptLimit: 100, ReservedJobMinutesLimit: 10000, ActiveJobLimit: 10}}}, nil
	}
	unknown, err := batchowner.ForecastBatchCostWith(root, record, nil, time.Now(), decision, run, unknownBudget)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Budgets[0].Fits != true || unknown.Budgets[0].AttemptDemand != 1 ||
		unknown.Budgets[1].Fits || unknown.Budgets[1].Status != string(dispatchcore.BudgetUnknown) ||
		unknown.Budgets[1].AttemptDemand != 1 || !strings.Contains(unknown.Budgets[1].Reason, "goal-b-unreadable.json") ||
		batchowner.ForecastCostRefusal(unknown) == nil {
		t.Fatalf("unknown charged-goal budget borrowed another member's ample headroom: %+v", unknown.Budgets)
	}
}

func TestBatchCostUnknownJoinKeepsFirstRecordAbsentAndClosesOnlyExistingAdmission(t *testing.T) {
	t.Parallel()
	for _, existing := range []bool{false, true} {
		name := "first-member"
		if existing {
			name = "prospective-member"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request, dependencies, admissionCalls := prepublicationJoinBed(t)
			request.GoalID = "goal-b"
			store := batch.NewStore(request.LandingRoot, nil)
			if existing {
				if err := store.Create(batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba99", State: batch.StateOpen,
					BaseTree: "base-tree", TipTree: "existing-tree", PrefixTrees: []string{"existing-tree"},
					Units: []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", State: batch.UnitJoined,
						Claim: batch.Claim{Machine: "seat-a", Lineage: "lineage-a", Epoch: 1, Revision: 2, AccountingRevision: 1}}}}); err != nil {
					t.Fatal(err)
				}
			}
			handedOver := false
			dependencies.Handover = func(batchowner.BatchJoinRequest, string, batch.Claim) error { handedOver = true; return nil }
			dependencies.PublishAdmission = func(batch.Store, string, batch.Unit, string, time.Time,
				func(string, string, string) (testpolicy.Plan, error), func() error, batch.JoinAdmissionRun) error {
				return fmt.Errorf("unknown budget reached publication")
			}
			dependencies.CostForecast = func(_ string, record batch.Record, incoming batch.Unit, _ time.Time,
				_ func(string, string, string) (testpolicy.Plan, error), _ func(string, string, []batch.Unit) ([]string, error)) (batch.Unit, batch.CostForecast, error) {
				incoming.SelectedGroups = []string{"c"}
				units := append(slices.Clone(record.Units), incoming)
				prefixes := append(slices.Clone(record.PrefixTrees), "candidate-tree")
				budgets := []batch.CostForecastBudget{}
				if existing {
					budgets = append(budgets, batch.CostForecastBudget{GoalID: "goal-a", Status: string(dispatchcore.BudgetKnown), Fits: true,
						AttemptsLeft: 100, ReservedMinutesLeft: 10000})
				}
				budgets = append(budgets, batch.CostForecastBudget{GoalID: incoming.GoalID, Status: string(dispatchcore.BudgetUnknown),
					Reason: "artifacts/agents/jobs/goal-b-unreadable.json: retained job accounting is unreadable"})
				return incoming, batch.CostForecast{SchemaVersion: 1, Currency: "snapshot-not-revalidated",
					Binding: batch.CostBinding(record, units, prefixes), Budgets: budgets}, nil
			}
			_, err := batchowner.ExecuteBatchJoin(request, dependencies)
			if err == nil || !strings.Contains(err.Error(), "BATCH_COST_HEADROOM_REFUSED") || handedOver || *admissionCalls != 0 {
				t.Fatalf("unknown budget crossed source ownership or native admission: err=%v handover=%t admission=%d", err, handedOver, *admissionCalls)
			}
			records, err := store.Records()
			if err != nil {
				t.Fatal(err)
			}
			if !existing {
				if len(records) != 0 {
					t.Fatalf("unknown first member created an empty batch: records=%+v", records)
				}
				return
			}
			if len(records) != 1 || len(records[0].Units) != 1 || records[0].Units[0].GoalID != "goal-a" ||
				records[0].ClosedReason != "budget-cost" {
				t.Fatalf("unknown prospective member moved custody or missed existing closure: records=%+v", records)
			}
		})
	}
}

func TestBatchCostElapsedFollowsLandingReadyAdmission(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("cap.min.proof.main.proof=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unit := batch.Unit{GoalID: "goal-ready", State: batch.UnitJoined}
	record := batch.Record{BaseTree: "base", TipTree: "tip", PrefixTrees: []string{"tip"}, Units: []batch.Unit{unit}}
	decision := func(_ string, _ []batch.Unit, _ string) (batch.PrefixDecision, error) {
		return batch.PrefixDecision{Groups: []string{"native"}}, nil
	}
	run := func(_ string, selection testrun.CostSelection, cap uint64) (testrun.CostEvidence, error) {
		return testrun.CostEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{{
			RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
			GroupID: "native", Status: "unknown", DeclaredAllowanceMS: int64(cap) * 60000,
		}}}, nil
	}
	projection := dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
		Limits:  goal.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1},
		Elapsed: 2 * time.Hour, ElapsedState: dispatchcore.ElapsedBreach}
	for _, test := range []struct {
		name         string
		landingReady bool
		fits         bool
	}{
		{name: "ordinary claim elapsed", fits: false},
		{name: "landing-ready elapsed suspended", landingReady: true, fits: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			budget := func(_ string, _ batch.Unit, _ *batch.Unit, _ time.Time) (batchowner.BatchCostBudgetProjection, error) {
				return batchowner.BatchCostBudgetProjection{Budget: projection, LandingClaim: test.landingReady}, nil
			}
			forecast, err := batchowner.ForecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
			if err != nil {
				t.Fatal(err)
			}
			if len(forecast.Budgets) != 1 || forecast.Budgets[0].Fits != test.fits ||
				(test.fits && forecast.Budgets[0].AttemptDemand != 1) {
				t.Fatalf("forecast disagreed with landing-ready admission: %+v", forecast.Budgets)
			}
		})
	}
}
