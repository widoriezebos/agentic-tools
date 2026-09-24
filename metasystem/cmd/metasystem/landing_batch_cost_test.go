package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestBatchCostPrefixGenericBudgetRefusalUsesTypedReturn(t *testing.T) {
	t.Parallel()
	root, tree := batchPrefixReceiptTestRoot(t)
	fake := filepath.Join(root, "budget-refusal-engine")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' 'metasystem test run: BUDGET_REFUSED reservedJobMinutesLimit used=1000 limit=1000' >&2\nexit %d\n", proofrun.ExitAdmissionRefused)
	if err := testexec.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	dependencies := batchTestExecutionDependencies(t, root, tree, fake)
	record := batch.Record{Units: []batch.Unit{{GoalID: "goal-a", Claim: batch.Claim{Revision: 7, AccountingRevision: 5}}}}
	_, err := executeBatchPrefixReceiptWithDependencies(root, "batch", record, "goal-a", tree, batch.PrefixDecision{Groups: []string{"same"}}, dependencies)
	var budget *batch.PrefixBudgetRefusal
	if !errors.As(err, &budget) || !strings.Contains(budget.Error(), "BUDGET_REFUSED") {
		t.Fatalf("generic engine budget refusal was not typed for member return: %T %v", err, err)
	}
}

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
	file := &goal.GoalFile{Id: "goal-ready", State: goal.StateClaimed, Revision: 2,
		Budget: &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1},
		Claimed: &goal.ClaimRecord{At: claimed, Revision: 2, AccountingRevision: 2,
			HandedOver: goal.HandedOver{FromMachine: claim.Machine, FromLineage: claim.Lineage, FromEpoch: claim.Epoch, Batch: record.BatchID}},
		History: []goal.HistoryLine{{At: now.Add(-3 * time.Hour).Format(time.RFC3339), Verb: "open"}, {At: claimed, Verb: "claim"}},
		Landing: &goal.LandingRecord{At: now.Add(-time.Minute).Format(time.RFC3339)},
	}
	projection := goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{unit.GoalID: file}}}
	if err := authorizeBatchMemberInProjection(root, now, record, unit, projection); err != nil {
		t.Fatalf("landing-ready claim lost elapsed suspension: %v", err)
	}
	file.Landing = nil
	var budget *batch.PrefixBudgetRefusal
	if err := authorizeBatchMemberInProjection(root, now, record, unit, projection); !errors.As(err, &budget) {
		t.Fatalf("ordinary elapsed claim was not refused: %v", err)
	}
	file.Landing = &goal.LandingRecord{At: now.Add(-time.Minute).Format(time.RFC3339)}
	file.StopFence = &goal.StopFence{StopID: "stop-goal-ready-r2"}
	var fenced *batch.PrefixFencedRefusal
	if err := authorizeBatchMemberInProjection(root, now, record, unit, projection); !errors.As(err, &fenced) {
		t.Fatalf("landing-ready elapsed suspension escaped a live fence: %v", err)
	}
}

func TestBatchCostPortableJoinRefusesOverBudgetBeforeHandoverAndStatusShowsSnapshot(t *testing.T) {
	t.Parallel()
	t.Run("before-handover", func(t *testing.T) {
		request, dependencies, admissionCalls := prepublicationJoinBed(t)
		request.GoalID, request.ChainID, request.At = "goal-b", "chain-b", time.Now().UTC()
		if err := os.WriteFile(filepath.Join(request.LandingRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		fixture := newPortableFileProof(t)
		fixture.root = costCanonicalRoot(t, fixture.root)
		tree, files := fixture.snapshot()
		run := fixture.request(tree, files, fixture.plan(fixture.loadedContract(files), "app/a.txt"))
		prepared := fixture.prepared(run, proofrun.NewTestResult(run))
		source := newProofAdmissionRepositoryFixture(t, request.At, false).goalFile(t, "standing-validation")
		writeGoal := func(root, id string, limit uint64) *goal.GoalFile {
			t.Helper()
			file := *source
			budget := *source.Budget
			budget.AttemptLimit = limit
			file.Id, file.Budget = id, &budget
			claimed := *source.Claimed
			claimed.AccountingRevision = 2
			file.Claimed = &claimed
			approved := *source.Approved
			approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, budget, file.Risk)
			file.Approved = &approved
			if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "plans", "goals", id+".md"), goal.RenderFile(&file), 0o644); err != nil {
				t.Fatal(err)
			}
			return &file
		}
		_ = writeGoal(request.LandingRoot, "goal-a", 4)
		goalB := writeGoal(request.SeatRoot, "goal-b", 1)
		beforeB, err := os.ReadFile(filepath.Join(request.SeatRoot, "plans", "goals", "goal-b.md"))
		if err != nil {
			t.Fatal(err)
		}
		claimA := batch.Claim{Machine: source.Claimed.Machine, Lineage: source.Claimed.Lineage, Epoch: 1, Revision: source.Claimed.Revision, AccountingRevision: 2}
		const batchID = "01j5x00000000000000000ba99"
		old := batch.Record{Schema: 1, BatchID: batchID, State: batch.StateOpen, BaseTree: tree, TipTree: tree,
			PrefixTrees: []string{tree}, Units: []batch.Unit{{GoalID: "goal-a", Chain: "chain-a", State: batch.UnitJoined,
				Claim: claimA, SelectedGroups: []string{"app-a"}}}, SelectedGroups: []string{"app-a"}}
		if err := batch.NewStore(request.LandingRoot, nil).Create(old); err != nil {
			t.Fatal(err)
		}
		dependencies.binding = func(root, id string, _ time.Time) (dispatchcore.GoalBinding, error) {
			if root != request.SeatRoot || id != "goal-b" {
				return dispatchcore.GoalBinding{}, fmt.Errorf("unexpected binding %s %s", root, id)
			}
			return dispatchcore.GoalBinding{Revision: goalB.Claimed.Revision, Machine: goalB.Claimed.Machine, Lineage: goalB.Claimed.Lineage,
				File: goalB, Capability: goal.StopCapability{ClaimEpoch: 1}}, nil
		}
		dependencies.base = func(string) (string, error) { return tree, nil }
		dependencies.assemble = func(_ string, base string, units []batch.Unit) ([]string, error) {
			if base != tree {
				return nil, fmt.Errorf("unexpected base %s", base)
			}
			return slices.Repeat([]string{tree}, len(units)), nil
		}
		dependencies.plan = func(string, string, string) (testpolicy.Plan, error) { return run.Plan, nil }
		dependencies.costForecast = func(root string, record batch.Record, incoming batch.Unit, at time.Time,
			_ func(string, string, string) (testpolicy.Plan, error), _ func(string, string, []batch.Unit) ([]string, error)) (batch.Unit, batch.CostForecast, error) {
			incoming.SelectedGroups = []string{"app-a"}
			incoming.Admission = &batch.JoinAdmission{Tree: tree, Status: "pending"}
			candidate := record
			candidate.Units = append(slices.Clone(record.Units), incoming)
			candidate.PrefixTrees, candidate.TipTree = []string{tree, tree}, tree
			cost, err := forecastBatchCostWith(root, candidate, &incoming, at,
				func(string, []batch.Unit, string) (batch.PrefixDecision, error) {
					return batch.PrefixDecision{Groups: []string{"app-a"}}, nil
				},
				func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
					engine, check := fixture.engineIO(tree)
					value, err := forecastTestingSelectionPrepared(selection, cap, testingSelectionRequest{Root: fixture.root, GoalID: "portable",
						Tree: tree, Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery,
						BatchPrefixReceipt: !selection.Admission, BatchAdmission: selection.Admission, BatchRequirements: selection.Requirements},
						prepared, forecastTestingDependencies{workspace: gittree.Workspace{Dir: fixture.root}, candidateIO: engine,
							openCandidate: fixture.open(tree, files), now: func() time.Time { return at }})
					check()
					return value, err
				},
				func(_ string, unit batch.Unit, _ *batch.Unit, at time.Time) (batchCostBudgetProjection, error) {
					budgetRoot := request.LandingRoot
					if unit.GoalID == "goal-b" {
						budgetRoot = request.SeatRoot
					}
					data, err := os.ReadFile(filepath.Join(budgetRoot, "plans", "goals", unit.GoalID+".md"))
					if err != nil {
						return batchCostBudgetProjection{}, err
					}
					file, problems := goal.ParseFile(data)
					if len(problems) != 0 {
						return batchCostBudgetProjection{}, fmt.Errorf("goal %s: %v", unit.GoalID, problems)
					}
					return batchCostBudgetProjection{Budget: dispatchcore.ProjectBudget(budgetRoot, file, at)}, nil
				})
			if err != nil {
				return batch.Unit{}, batch.CostForecast{}, err
			}
			if refused := forecastCostRefusal(cost); refused == nil {
				t.Fatalf("insufficient forecast fitted: %+v", cost.Budgets)
			}
			return incoming, cost, nil
		}
		fatal := func(name string) error {
			t.Errorf("%s crossed pre-handover refusal", name)
			return fmt.Errorf("%s crossed refusal", name)
		}
		dependencies.handover = func(batchJoinRequest, string, batch.Claim) error { return fatal("handover") }
		dependencies.admissionRun = func(string, string, batch.Unit) (batch.JoinAdmission, error) {
			return batch.JoinAdmission{}, fatal("admission")
		}
		dependencies.publishAdmission = func(batch.Store, string, batch.Unit, string, time.Time, func(string, string, string) (testpolicy.Plan, error), func() error, batch.JoinAdmissionRun) error {
			return fatal("publication")
		}
		dependencies.publishForecast = func(batch.Store, string, batch.Unit, string, time.Time, func(string, string, string) (testpolicy.Plan, error), func() error, batch.JoinAdmissionRun, batch.CostForecast) error {
			return fatal("forecast publication")
		}
		dependencies.ensure = func(string) error { return nil }
		_, err = executeBatchJoin(request, dependencies)
		if err == nil || !strings.Contains(err.Error(), "BATCH_COST_HEADROOM_REFUSED") || *admissionCalls != 0 {
			t.Fatalf("pre-handover refusal=%v admission calls=%d", err, *admissionCalls)
		}
		after, err := batch.NewStore(request.LandingRoot, nil).Load(batchID)
		if err != nil || after.ClosedReason != "budget-cost" || len(after.Units) != 1 || after.Units[0].GoalID != "goal-a" {
			t.Fatalf("existing member cost closure: %+v err=%v", after, err)
		}
		afterB, err := os.ReadFile(filepath.Join(request.SeatRoot, "plans", "goals", "goal-b.md"))
		if err != nil || !reflect.DeepEqual(afterB, beforeB) {
			t.Fatalf("B claim changed: err=%v", err)
		}
	})
	t.Run("after-handover", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		repository := newProofAdmissionRepositoryFixture(t, now, false)
		repository.root = costCanonicalRoot(t, repository.root)
		repository.top = costCanonicalRoot(t, repository.top)
		root, id := repository.root, "standing-validation"
		file := repository.amend(t, id, func(file *goal.GoalFile) {
			file.Claimed.AccountingRevision = 2
			file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
			file.Budget.AttemptLimit = 1
			file.Budget.ActiveJobLimit = 2
			file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		})
		path := filepath.Join(root, "plans", "goals", id+".md")
		if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
			t.Fatal(err)
		}
		claim := batch.Claim{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage, Epoch: 1, Revision: file.Claimed.Revision,
			AccountingRevision: file.Claimed.AccountingRevision}
		const batchID = "01j5x00000000000000000ba48"
		fixture := newPortableFileProof(t)
		fixture.root = costCanonicalRoot(t, fixture.root)
		tree, files := fixture.snapshot()
		unit := batch.Unit{GoalID: id, Chain: "cost-join", State: batch.UnitJoining, Claim: claim,
			Admission: &batch.JoinAdmission{Tree: tree, Status: "handed-over"}}
		record := batch.Record{Schema: 1, BatchID: batchID, State: batch.StateOpen, BaseTree: tree, TipTree: tree,
			PrefixTrees: []string{tree}, Units: []batch.Unit{unit}}
		proofRequest := fixture.request(tree, files, fixture.plan(fixture.loadedContract(files), "app/a.txt"))
		proofRequest.CandidateEngineBuildIdentity = fixture.engineIdentity(tree, proofRequest.Environment)
		proofResult, _ := fixture.execute(proofRequest, true)
		compute := func(candidate batch.Record) (batch.CostForecast, error) {
			return forecastBatchCostWith(root, candidate, nil, now,
				func(string, []batch.Unit, string) (batch.PrefixDecision, error) {
					return batch.PrefixDecision{Groups: []string{"app-a"}}, nil
				},
				func(_ string, selected costSelection, cap uint64) (costSelectionEvidence, error) {
					engine, check := fixture.engineIO(tree)
					value, err := forecastTestingSelectionPrepared(selected, cap, testingSelectionRequest{Root: fixture.root, GoalID: "portable", Tree: tree,
						Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, BatchPrefixReceipt: true, BatchRequirements: []string{"app-a"}},
						fixture.prepared(proofRequest, proofResult), forecastTestingDependencies{workspace: gittree.Workspace{Dir: fixture.root},
							candidateIO: engine, openCandidate: fixture.open(tree, files), now: func() time.Time { return now }})
					check()
					return value, err
				},
				func(root string, _ batch.Unit, _ *batch.Unit, at time.Time) (batchCostBudgetProjection, error) {
					return batchCostBudgetProjection{Budget: dispatchcore.ProjectBudget(root, file, at)}, nil
				})
		}
		fit, err := compute(record)
		if err != nil || forecastCostRefusal(fit) != nil {
			t.Fatalf("fitting forecast=%+v err=%v", fit, err)
		}
		record.CostForecast = &fit
		store := batch.NewStore(root, nil).WithReassembly(
			func(string, []batch.Unit) ([]string, error) { return nil, fmt.Errorf("unexpected reassembly") },
			func(string, string) error { return fmt.Errorf("unexpected delete") },
			func(string, string, string, string, []batch.Unit) (string, error) {
				return "", fmt.Errorf("unexpected rebuild")
			})
		if err := store.Create(record); err != nil {
			t.Fatal(err)
		}
		historical := record
		const historicalID = "01j5x00000000000000000ba49"
		historical.BatchID, historical.State = historicalID, batch.StateSealed
		historical.Units = []batch.Unit{unit}
		historical.Units[0].State = batch.UnitJoined
		historical.Seal = map[string]batch.Claim{id: claim}
		historicalFit, err := compute(historical)
		if err != nil || forecastCostRefusal(historicalFit) != nil {
			t.Fatalf("historical forecast=%+v err=%v", historicalFit, err)
		}
		historical.CostForecast = &historicalFit
		if err := store.Create(historical); err != nil {
			t.Fatal(err)
		}
		seat := t.TempDir()
		readStatus := func() batchStatusView {
			t.Helper()
			facts := &batchRawFacts{root: root}
			var output strings.Builder
			code := runBatchStatusWithOutput([]string{"--root", seat, "--landing-root", root, "--batch", historicalID},
				facts.source(), repository.reads().ResolveEndpoint, &output)
			facts.assertConsumed(t, true)
			var decoded batchStatusOutput
			if code != 0 || json.Unmarshal([]byte(output.String()), &decoded) != nil || len(decoded.Batches) != 1 {
				t.Fatalf("status code=%d json=%s", code, output.String())
			}
			return decoded.Batches[0]
		}
		before := readStatus()
		if err := reserveCostFixtureSpend(root, id, claim, 1, repository.reads()); err != nil {
			t.Fatal(err)
		}
		attemptsBefore := costAttemptBytes(t, root)
		announceProofFixtureHolder(t, root)
		var actual error
		err = batch.ResumeJoinAdmission(store, batchID, id, "landing", now, func(_ string, joined batch.Unit) (batch.JoinAdmission, error) {
			if joined.Admission == nil || joined.Admission.Tree != tree {
				t.Fatalf("lost handed-over admission: %+v", joined)
			}
			_, _, _, actual = admitCandidateProofLaunchWithRepository(t, repository, proofLaunchAdmission{
				ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"), GoalID: id,
				CandidateTree: tree, CapMin: "1", ScopeClass: "selected", CommandClass: "testing",
				ExpectedGoalRevision: claim.Revision, ExpectedAccountingRevision: claim.AccountingRevision,
				Now: now,
			})
			if actual == nil {
				t.Fatal("competing spend passed final admission")
			}
			return batch.JoinAdmission{}, joinAdmissionRefusal(actual.Error())
		})
		var refusal *batch.PrefixBudgetRefusal
		if !errors.As(err, &refusal) || actual == nil || !strings.Contains(actual.Error(), "BUDGET_REFUSED") {
			t.Fatalf("real final budget refusal: returned=%v actual=%v", err, actual)
		}
		if !reflect.DeepEqual(costAttemptBytes(t, root), attemptsBefore) {
			t.Fatal("final admission reserved a new attempt")
		}
		returned, err := store.Load(batchID)
		if err != nil || returned.State != batch.StateDissolved || len(returned.Units) != 1 ||
			returned.Units[0].State != batch.UnitReturnPending || returned.Units[0].Outcome != batch.UnitWithdrawnBudget {
			t.Fatalf("post-handover return=%+v err=%v", returned, err)
		}
		configPath := filepath.Join(root, "metasystem.conf")
		configuration, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configPath, append(configuration, []byte("cap.min.proof.main.proof=1\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		after := readStatus()
		if after.CostForecast == nil || after.CostForecast.ObservedAt != historicalFit.ObservedAt || after.CostForecast.Currency != "snapshot-not-revalidated" ||
			after.CostSnapshotStale || len(after.LiveHeadroom) != 1 || after.LiveHeadroom[0].AttemptsLeft >= before.LiveHeadroom[0].AttemptsLeft {
			t.Fatalf("historical snapshot/live headroom before=%+v after=%+v", before, after)
		}
	})
}

func reserveCostFixtureSpend(root, goalID string, claim batch.Claim, minutes uint64, reads dispatchcore.ProofAdmissionReads) error {
	canonical, err := canonicalProofRoot(root)
	if err != nil {
		return err
	}
	root = canonical
	goalLock, err := goalrevision.Acquire(root, goalID, claim.Revision, "cost-forecast-fixture")
	if err != nil {
		return err
	}
	defer goalLock.Release()
	proofLock, err := proofrun.AcquireMutation(root)
	if err != nil {
		return err
	}
	defer proofLock.Release()
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		return err
	}
	identity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "fixture", "cost-spend", []string{"spend"}, 1)
	if err != nil {
		return err
	}
	request := proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root, ConfPath: filepath.Join(root, "metasystem.conf"),
		GoalID: goalID, GoalRevision: claim.Revision, AccountingRevision: claim.AccountingRevision,
		CandidateGoalID: goalID, CandidateRevision: claim.AccountingRevision, ReservedMinutes: minutes,
		Identity: identity, Launcher: launcher, Now: time.Now().UTC()}
	_, decision, err := proofrun.ReserveLocked(privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(request, "0")))
	if err != nil {
		return err
	}
	if decision.Disposition != proofrun.DispositionExecuted {
		return fmt.Errorf("cost fixture reservation was not admitted: %+v", decision)
	}
	data, err := os.ReadFile(filepath.Join(root, "plans", "goals", goalID+".md"))
	if err != nil {
		return err
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		return fmt.Errorf("cost fixture goal parse: %v", problems)
	}
	projection := dispatchcore.ProjectBudget(root, file, time.Now().UTC())
	if projection.Status != dispatchcore.BudgetKnown {
		return fmt.Errorf("cost fixture reservation did not project: %+v", projection.Unknown)
	}
	endpoint, err := reads.ResolveEndpoint(root)
	if err != nil {
		return err
	}
	live, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		return err
	}
	if live.Tree == nil || live.Tree.Live[goalID] == nil {
		return fmt.Errorf("cost fixture accepted ledger lost %s", goalID)
	}
	accepted := dispatchcore.ProjectBudget(root, live.Tree.Live[goalID], time.Now().UTC())
	if accepted.Status != dispatchcore.BudgetKnown {
		return fmt.Errorf("cost fixture accepted projection unknown: %+v (accepted rev=%d accounting=%d, local rev=%d accounting=%d)",
			accepted.Unknown, live.Tree.Live[goalID].Claimed.Revision, live.Tree.Live[goalID].Claimed.AccountingRevision,
			file.Claimed.Revision, file.Claimed.AccountingRevision)
	}
	return nil
}

// Reserve through the real attempt owner using an identity from an earlier
// native command/JUnit result. No result or observation record is fabricated.
func reserveCostNewerGroupObservation(t *testing.T, root, groupID string) proofrun.Attempt {
	t.Helper()
	controlRoot, err := canonicalProofRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	attempts, err := proofrun.ReadAttempts(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	var source proofrun.Attempt
	identity := ""
	for _, attempt := range attempts {
		if attempt.Terminal == nil || attempt.Terminal.Result != proofrun.TerminalSuccess || attempt.TestResult == nil {
			continue
		}
		for _, group := range attempt.TestResult.Groups {
			if group.ID == groupID && group.Status == "passed" && group.NativeLaunched && group.ExecutionIdentity != "" {
				source, identity = attempt, group.ExecutionIdentity
			}
		}
	}
	if identity == "" {
		t.Fatalf("no retained native command/JUnit success for %s", groupID)
	}
	goalLock, err := goalrevision.Acquire(controlRoot, source.GoalID, source.GoalRevision, "cost-newer-observation-fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer goalLock.Release()
	proofLock, err := proofrun.AcquireMutation(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer proofLock.Release()
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	request := proofrun.AdmissionRequest{ControlRoot: controlRoot, ExecutionRoot: source.ExecutionRoot,
		ConfPath: filepath.Join(controlRoot, "metasystem.conf"), GoalID: source.GoalID,
		GoalRevision: source.GoalRevision, AccountingRevision: source.AccountingRevision,
		BudgetEpoch: source.BudgetEpoch, CandidateGoalID: source.CandidateGoalID,
		CandidateRevision: source.CandidateRevision, CandidateBudgetEpoch: source.CandidateBudgetEpoch,
		CandidateTree: source.CandidateTree, ReservedMinutes: 1, Identity: source.ProofIdentity,
		Launcher: launcher, Now: time.Now().UTC(), ComponentIdentities: map[string]string{groupID: identity},
		SharedComponents: true, ForceAttempt: true, ForceGroups: true}
	live, decision, err := proofrun.ReserveLocked(privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(request, "0")))
	if err != nil || decision.Disposition != proofrun.DispositionExecuted || live.TestOwned[groupID] != identity {
		t.Fatalf("newer retained producer reservation: attempt=%+v decision=%+v err=%v", live, decision, err)
	}
	return live
}

func TestBatchCostStatusLabelsStoredSnapshotHistoricalAndStale(t *testing.T) {
	t.Parallel()
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ba01", State: batch.StateSealed,
		TipTree: "tip", Units: []batch.Unit{{GoalID: "goal-a", Chain: "a", State: batch.UnitJoined}},
	}
	record.BaseTree, record.PrefixTrees = "base", []string{"tip"}
	forecast := batch.CostForecast{SchemaVersion: 1, ObservedAt: time.Unix(5, 0).UTC().Format(time.RFC3339Nano),
		Currency: "snapshot-not-revalidated", Binding: batch.CostBinding(record, record.Units, record.PrefixTrees)}
	record.CostForecast = &forecast
	settings := config.BatchLanding{Root: t.TempDir(), MaxWait: time.Minute}
	view := batchRecordStatus(record, settings)
	if view.CostForecast == nil || view.CostForecast.Currency != "snapshot-not-revalidated" || view.CostSnapshotStale {
		t.Fatalf("exact stored snapshot was presented incorrectly: %+v", view)
	}
	record.BaseTree = "moved"
	view = batchRecordStatus(record, settings)
	if view.CostForecast == nil || !view.CostSnapshotStale {
		t.Fatalf("moved base was presented as current: %+v", view)
	}
}

func TestBatchCostPortableEvidenceForecastIsReadOnlyAndWarmsFromRealCommand(t *testing.T) {
	t.Parallel()
	fixture := newPortableFileProof(t)
	fixture.root = costCanonicalRoot(t, fixture.root)
	repository := newProofAdmissionRepositoryFixture(t, time.Now().UTC(), false)
	goalFile := repository.goalFile(t, "standing-validation")
	goalFile.Id = "portable"
	goalFile.Claimed.AccountingRevision = 2
	for i := range goalFile.History {
		goalFile.History[i].Targets = []string{"portable"}
	}
	goalBytes := goal.RenderFile(goalFile)
	if err := os.MkdirAll(filepath.Join(fixture.root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "plans", "goals", "portable.md"), goalBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	repository.seed(map[string][]byte{"metasystem/plans/goals/portable.md": goalBytes})
	tree, files := fixture.snapshot()
	contract := fixture.loadedContract(files)
	run := fixture.request(tree, files, fixture.plan(contract, "app/a.txt"))
	run.CandidateEngineBuildIdentity = fixture.engineIdentity(tree, run.Environment)
	selection := costSelection{ID: "tip:portable", Kind: "tip", Tree: tree, GoalID: "portable", Requirements: []string{"app-a"}}
	selected := testingSelectionRequest{Root: fixture.root, GoalID: "portable", Tree: tree, Mode: testpolicy.ModeAuto,
		Purpose: testpolicy.PurposeDelivery, BatchPrefixReceipt: true, BatchRequirements: []string{"app-a"}}
	forecast := func(result proofrun.TestResult) costSelectionEvidence {
		t.Helper()
		beforeBytes := costAttemptBytes(t, fixture.root)
		beforeNative := fixture.counts()
		engine, check := fixture.engineIO(tree)
		view, err := forecastTestingSelectionPrepared(selection, 2, selected, fixture.prepared(run, result), forecastTestingDependencies{
			workspace: gittree.Workspace{Dir: fixture.root}, candidateIO: engine, openCandidate: fixture.open(tree, files), now: time.Now,
		})
		check()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(costAttemptBytes(t, fixture.root), beforeBytes) || !reflect.DeepEqual(fixture.counts(), beforeNative) {
			t.Fatal("forecast changed retained attempts or launched native work")
		}
		return view
	}
	cold := forecast(proofrun.NewTestResult(run))
	if len(cold.Groups) != 1 || cold.Groups[0].Status != "unknown" || cold.Groups[0].IdentityKnown {
		t.Fatalf("cold forecast invented evidence: %+v", cold)
	}
	result, _ := fixture.execute(run, true)
	if group := portableGroups(result)["app-a"]; group.Status != "passed" || !group.NativeLaunched {
		t.Fatalf("warm source lacks real command/JUnit result: %+v", group)
	}
	requirePortableCounts(t, fixture.counts(), map[string]int{"a": 1})
	warm := forecast(result)
	if len(warm.Groups) != 1 || warm.Groups[0].Status != "reusable" || !warm.Groups[0].IdentityKnown {
		t.Fatalf("warm forecast missed retained result: %+v", warm)
	}
	claim := batch.Claim{Machine: goalFile.Claimed.Machine, Lineage: goalFile.Claimed.Lineage,
		Epoch: 1, Revision: goalFile.Claimed.Revision, AccountingRevision: goalFile.Claimed.AccountingRevision}
	const batchID = "01j5x00000000000000000ba47"
	record := batch.Record{Schema: 1, BatchID: batchID, State: batch.StateSealed, BaseTree: tree, TipTree: tree,
		PrefixTrees: []string{tree}, SelectedGroups: []string{"app-a"},
		Units: []batch.Unit{{GoalID: "portable", Chain: "portable-cost", Claim: claim, State: batch.UnitJoined,
			ChangedPaths: []string{"app/a.txt"}, SelectedGroups: []string{"app-a"}}},
		Seal: map[string]batch.Claim{"portable": claim}}
	budget := func(root string, unit batch.Unit, _ *batch.Unit, at time.Time) (batchCostBudgetProjection, error) {
		data, err := os.ReadFile(filepath.Join(root, "plans", "goals", unit.GoalID+".md"))
		if err != nil {
			return batchCostBudgetProjection{}, err
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			return batchCostBudgetProjection{}, fmt.Errorf("goal budget facts: %v", problems)
		}
		return batchCostBudgetProjection{Budget: dispatchcore.ProjectBudget(root, file, at), LandingClaim: file.IsLandingClaim()}, nil
	}
	stored, err := forecastBatchCostWith(fixture.root, record, nil, time.Now().UTC(),
		func(string, []batch.Unit, string) (batch.PrefixDecision, error) {
			return batch.PrefixDecision{Groups: []string{"app-a"}}, nil
		},
		func(string, costSelection, uint64) (costSelectionEvidence, error) { return forecast(result), nil }, budget)
	if err != nil || len(stored.Groups) != 1 || stored.Groups[0].Status != "reusable" {
		t.Fatalf("stored cost forecast: %+v err=%v", stored, err)
	}
	record.CostForecast = &stored
	if err := batch.NewStore(fixture.root, nil).Create(record); err != nil {
		t.Fatal(err)
	}
	seat := t.TempDir()
	readStatus := func(stage string) batchStatusView {
		t.Helper()
		beforeBytes := costAttemptBytes(t, fixture.root)
		beforeNative := fixture.counts()
		facts := &batchRawFacts{root: fixture.root}
		var output strings.Builder
		code := runBatchStatusWithOutput([]string{"--root", seat, "--landing-root", fixture.root, "--batch", batchID},
			facts.source(), func(root string) (goal.Endpoint, error) {
				if root != fixture.root {
					return goal.Endpoint{}, fmt.Errorf("status root %s", root)
				}
				return repository.reads().ResolveEndpoint(repository.root)
			}, &output)
		facts.assertConsumed(t, true)
		var decoded batchStatusOutput
		if code != 0 || json.Unmarshal([]byte(output.String()), &decoded) != nil || len(decoded.Batches) != 1 {
			t.Fatalf("%s public status code=%d json=%s", stage, code, output.String())
		}
		view := decoded.Batches[0]
		if view.CostForecast == nil || view.CostForecast.ObservedAt != stored.ObservedAt ||
			view.CostForecast.Currency != "snapshot-not-revalidated" || view.CostSnapshotStale ||
			len(view.LiveHeadroom) != 1 || view.LiveHeadroom[0].GoalID != "portable" {
			t.Fatalf("%s historical snapshot/live headroom: %+v", stage, view)
		}
		if !reflect.DeepEqual(costAttemptBytes(t, fixture.root), beforeBytes) || !reflect.DeepEqual(fixture.counts(), beforeNative) {
			t.Fatalf("%s status changed proof attempts or native work", stage)
		}
		return view
	}
	beforeStatus := readStatus("older-green")
	liveAttempt := reserveCostNewerGroupObservation(t, fixture.root, "app-a")
	live := forecast(result)
	if len(live.Groups) != 1 || live.Groups[0].Status != "live-wait" || live.Groups[0].Reason != "newer-live-producer" {
		t.Fatalf("newer live producer: %+v", live)
	}
	liveStatus := readStatus("newer-live")
	if liveStatus.LiveHeadroom[0].AttemptsLeft >= beforeStatus.LiveHeadroom[0].AttemptsLeft {
		t.Fatalf("live reservation did not lower headroom: before=%+v live=%+v", beforeStatus.LiveHeadroom, liveStatus.LiveHeadroom)
	}
	if _, err := proofrun.FinalizeAttempt(liveAttempt.ControlRoot, liveAttempt.AttemptID, proofrun.TerminalFailed, 1,
		"newer red observation", nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	red := forecast(result)
	if len(red.Groups) != 1 || red.Groups[0].Status != "missing" || red.Groups[0].Reason != "newer-red-observation" {
		t.Fatalf("newer red observation: %+v", red)
	}
	redStatus := readStatus("newer-red")
	if redStatus.LiveHeadroom[0].AttemptsLeft != liveStatus.LiveHeadroom[0].AttemptsLeft {
		t.Fatalf("red finalization restored reservation: live=%+v red=%+v", liveStatus.LiveHeadroom, redStatus.LiveHeadroom)
	}
}

func costAttemptBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "artifacts", "agents", "proof-runs", "attempts"))
	if os.IsNotExist(err) {
		return map[string]string{}
	}
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string, len(entries))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "proof-runs", "attempts", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(data)
	}
	return files
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
	selection := costSelection{ID: "tip:fresh", Kind: "tip", Tree: tree, GoalID: "portable", Requirements: []string{"app-a"},
		FreshEpisode: run.FreshnessEpisode, FreshExpiresAt: run.FreshnessExpiresAt}
	selected := testingSelectionRequest{Root: fixture.root, GoalID: "portable", Tree: tree, Mode: testpolicy.ModeAuto,
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
			forecast, forecastErr := forecastTestingSelectionPrepared(selection, 2, selected, prepared, forecastTestingDependencies{
				workspace: projection, candidateIO: engine, openCandidate: fixture.open(tree, files),
				now: func() time.Time { return boundary.at },
			})
			checkEngine()
			checkProjection()
			verifyEngine, checkVerifyEngine := fixture.engineIO(tree)
			verifyProjection := gittree.Workspace{Dir: fixture.root}
			checkVerifyProjection := func() {}
			if boundary.reusable {
				verifyProjection, checkVerifyProjection = fixture.projection(tree)
			}
			verified, verifyErr := verifyRetainedTestingPrepared(selected, prepared, retainedTestingVerification{
				clock: func() time.Time { return boundary.at }, revalidate: proofrun.RevalidateRetainedGroupExecutionIdentities,
				workspace: verifyProjection, candidateIO: verifyEngine, openCandidate: fixture.open(tree, files),
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
			const expired = "freshness episode has expired; renew the proof decision"
			if verifyErr == nil || verifyErr.Error() != expired {
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
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		out := costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
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
	budget := func(_ string, _ batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown, Limits: goal.Budget{
			ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}}}, nil
	}
	forecast, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
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
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		return costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{{
			RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
			GroupID: "shared", ExecutionIdentity: "same-identity", IdentityKnown: true, Status: "missing",
			FreshEpisode: selection.FreshEpisode, DeclaredAllowanceMS: int64(cap) * 60000,
		}}}, nil
	}
	budget := func(_ string, _ batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
			Limits: goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}}}, nil
	}
	first, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
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
	second, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
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
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		return costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{{
			RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID,
			GroupID: selection.Requirements[0], ExecutionIdentity: "identity-" + selection.Requirements[0],
			IdentityKnown: true, Status: "missing", DeclaredAllowanceMS: int64(cap) * 60000,
		}}}, nil
	}
	budget := func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		attempts := uint64(1)
		if unit.GoalID == "goal-b" {
			attempts = 0
		}
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
			Limits: goal.Budget{ElapsedLimit: "4h", AttemptLimit: attempts, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1}}}, nil
	}
	forecast, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
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
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		return costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
			Tree: selection.Tree, ChargeGoal: selection.GoalID}, Groups: []batch.CostForecastGroup{
			{RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID, GroupID: "u1", Status: "unknown", DeclaredAllowanceMS: int64(cap) * 60000},
			{RequestID: selection.ID, Tree: selection.Tree, ChargeGoal: selection.GoalID, GroupID: "u2", Status: "unknown", DeclaredAllowanceMS: int64(cap) * 60000},
		}}, nil
	}
	budget := func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		limits := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 1000, ActiveJobLimit: 2}
		if unit.GoalID == "goal-b" {
			limits.AttemptLimit = 0
		}
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown, Limits: limits}}, nil
	}
	forecast, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
	if err != nil {
		t.Fatal(err)
	}
	if forecast.Requests[0].ReservationMinutes != 2 || forecast.UnknownMissing != 4 ||
		forecast.Budgets[0].Fits != true || forecast.Budgets[1].Fits || forecast.Budgets[1].Reason != "attempt-headroom" {
		t.Fatalf("per-request cap and isolated budget: %+v", forecast)
	}
	unknownBudget := func(_ string, unit batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
		if unit.GoalID == "goal-b" {
			return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetUnknown,
				Unknown: &dispatchcore.BudgetUnknownEvidence{Code: dispatchcore.BudgetUnknown,
					Record: "artifacts/agents/jobs/goal-b-unreadable.json", Reason: "retained job accounting is unreadable"}}}, nil
		}
		return batchCostBudgetProjection{Budget: dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown,
			Limits: goal.Budget{ElapsedLimit: "4h", AttemptLimit: 100, ReservedJobMinutesLimit: 10000, ActiveJobLimit: 10}}}, nil
	}
	unknown, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, unknownBudget)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Budgets[0].Fits != true || unknown.Budgets[0].AttemptDemand != 1 ||
		unknown.Budgets[1].Fits || unknown.Budgets[1].Status != string(dispatchcore.BudgetUnknown) ||
		unknown.Budgets[1].AttemptDemand != 1 || !strings.Contains(unknown.Budgets[1].Reason, "goal-b-unreadable.json") ||
		forecastCostRefusal(unknown) == nil {
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
			handedOver, ensured := false, 0
			dependencies.handover = func(batchJoinRequest, string, batch.Claim) error { handedOver = true; return nil }
			dependencies.ensure = func(string) error { ensured++; return nil }
			dependencies.publishAdmission = func(batch.Store, string, batch.Unit, string, time.Time,
				func(string, string, string) (testpolicy.Plan, error), func() error, batch.JoinAdmissionRun) error {
				return fmt.Errorf("unknown budget reached publication")
			}
			dependencies.costForecast = func(_ string, record batch.Record, incoming batch.Unit, _ time.Time,
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
			_, err := executeBatchJoin(request, dependencies)
			if err == nil || !strings.Contains(err.Error(), "BATCH_COST_HEADROOM_REFUSED") || handedOver || *admissionCalls != 0 {
				t.Fatalf("unknown budget crossed source ownership or native admission: err=%v handover=%t admission=%d", err, handedOver, *admissionCalls)
			}
			records, err := store.Records()
			if err != nil {
				t.Fatal(err)
			}
			if !existing {
				if len(records) != 0 || ensured != 0 {
					t.Fatalf("unknown first member created an empty batch: records=%+v ensured=%d", records, ensured)
				}
				return
			}
			if len(records) != 1 || len(records[0].Units) != 1 || records[0].Units[0].GoalID != "goal-a" ||
				records[0].ClosedReason != "budget-cost" || ensured != 1 {
				t.Fatalf("unknown prospective member moved custody or missed existing closure: records=%+v ensured=%d", records, ensured)
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
	run := func(_ string, selection costSelection, cap uint64) (costSelectionEvidence, error) {
		return costSelectionEvidence{Request: batch.CostForecastRequest{ID: selection.ID, Kind: selection.Kind,
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
			budget := func(_ string, _ batch.Unit, _ *batch.Unit, _ time.Time) (batchCostBudgetProjection, error) {
				return batchCostBudgetProjection{Budget: projection, LandingClaim: test.landingReady}, nil
			}
			forecast, err := forecastBatchCostWith(root, record, nil, time.Now(), decision, run, budget)
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

func costCanonicalRoot(t *testing.T, root string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}
