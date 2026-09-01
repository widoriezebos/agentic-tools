package goal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type obligationAuthorityReader struct {
	snapshots map[int64]humanauthority.Snapshot
}

func (r *obligationAuthorityReader) Read(pid int64) (humanauthority.Snapshot, error) {
	snapshot, ok := r.snapshots[pid]
	if !ok {
		return humanauthority.Snapshot{}, os.ErrNotExist
	}
	return snapshot, nil
}

func (*obligationAuthorityReader) SessionLeader(int64) (int64, error) { return 10, nil }

func obligationAuthoritySnapshot(pid, parent int64, argv []string) humanauthority.Snapshot {
	return humanauthority.Snapshot{Exact: identity.Exact{Pid: pid, StartedAt: time.Unix(pid*10, 0), Argv: argv, ArgvKnown: true},
		Executable: "/fixture/" + argv[0], ExecutableKnown: true, ParentPID: parent, ParentKnown: true,
		TerminalID: "tty-obligation", TerminalKnown: true}
}

func enrollObligationHuman(t *testing.T, root string) *obligationAuthorityReader {
	t.Helper()
	directory := filepath.Join(root, "scripts", "agents", "adapters")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "codex.sh"), []byte("#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match codex-agent'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reader := &obligationAuthorityReader{snapshots: map[int64]humanauthority.Snapshot{
		10: obligationAuthoritySnapshot(10, 1, []string{"terminal"}),
		20: obligationAuthoritySnapshot(20, 10, []string{"shell"}),
		30: obligationAuthoritySnapshot(30, 20, []string{"human-command"}),
	}}
	if _, err := humanauthority.Enroll(root, 20, reader, time.Date(2026, 8, 30, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	return reader
}

func proveObligationHuman(t *testing.T, root string) humanauthority.Proof {
	t.Helper()
	reader := enrollObligationHuman(t, root)
	proof, err := humanauthority.Prove(root, 30, reader, time.Date(2026, 8, 30, 8, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return proof
}

func TestEnrolledAncestryWithRelayedFlagsLeavesNoLandedTemporaryMark(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "enrolled-precedence")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reader := enrollObligationHuman(t, root)
	now := time.Date(2026, 8, 30, 8, 1, 0, 0, time.UTC)
	proof, err := humanauthority.ProveOrTemporaryGoalAuthority(
		root, 30, reader, "Wido authorizes fallback obligation", "2026-09-06", now)
	if err != nil || !proof.ValidFor(root) {
		t.Fatalf("enrolled ancestry did not take precedence over valid relayed flags: proof=%+v err=%v", proof, err)
	}
	request := obligationAuthorityVerbReq(root, "01J5X00000000000000000Q541", "mac-a")
	request.Actor.Human = "Wido"
	request.Now = now
	if result, err := SetObligation(request, "enrolled-precedence", testGovernedObligation(ObligationDraft), &proof); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("enrolled set-obligation did not confirm: %+v %v", result, err)
	}
	projection, err := Project(obligationAuthorityEndpoint(root), false, now)
	if err != nil {
		t.Fatal(err)
	}
	file := projection.Tree.Live["enrolled-precedence"]
	if file.Obligation == nil {
		t.Fatal("enrolled set-obligation did not land its obligation")
	}
	record := file.Obligation
	history := file.History[len(file.History)-1]
	if record.AuthorityOutcome != "" || record.AuthorityReviewBy != "" || record.AuthorityRuling != "" || record.TemporaryHumanWord != "" ||
		history.AuthorityOutcome != "" || history.AuthorityReviewBy != "" || history.AuthorityRuling != "" || history.TemporaryHumanWord != "" {
		t.Fatalf("enrolled authority inherited a temporary marker: obligation=%+v history=%+v", record, history)
	}
}

func testGovernedObligation(state ObligationState) GovernedObligation {
	return GovernedObligation{State: state, Owner: "Wido",
		Effects: []GoverningEffect{EffectAuthorizeSpend, EffectRefuseWork, EffectResetWeight, EffectDischargeObligation},
		Assumptions: ObligationAssumptions{Recurrence: StandingSharedProcess, Platform: "fixture/os",
			ToolchainIdentity: "fixture-go", SurfaceDigest: "fixture-digest", MaxActiveJobs: 1,
			TimingEnvelopeSeconds: 60, ObservationSource: "run-terminal-record"},
		Triggers: HumanReviewTriggers{ValueJudgment: "unknown", Reversibility: "unknown", SevereHarm: "unknown",
			UnfamiliarApproach: "unknown", TestDiscrimination: "unknown", CorrelatedAssumptionRisk: "unknown",
			AuthorityScopeChange: "unknown", DestructiveReach: "unknown"}}
}

func obligationAuthorityLocalRoot(t *testing.T, id string) string {
	t.Helper()
	root := t.TempDir()
	obligationAuthorityGit(t, root, "init", "-q", "-b", "main")
	obligationAuthorityGit(t, root, "config", "user.name", "obligation-fixture")
	obligationAuthorityGit(t, root, "config", "user.email", "obligation-fixture@example.invalid")
	obligationAuthorityGit(t, root, "config", "goal.sync-remote", "local")
	openedAt := "2026-08-30T08:00:00Z"
	claimAt := "2026-08-30T08:05:00Z"
	rootRecord := &RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: SyncLocal, Revision: 1,
	}
	file := &GoalFile{
		Id: id, State: StateClaimed, Intent: "Govern validation.", Origin: OriginMain,
		NextStep: "Run it.", OpenedAt: openedAt, Revision: 2,
		Budget:  &Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2},
		Claimed: &ClaimRecord{Machine: "mac-a", Lineage: "lin-1", At: claimAt, Revision: 2},
		History: []HistoryLine{
			{At: openedAt, Opid: Opid("01ARZ3NDEKTSV4RRFFQ69G5FAA", "mac-a", "lin-1"), Verb: "open", Actor: "mac-a+lin-1", Targets: []string{id}, Keep: -1},
			{At: claimAt, Opid: Opid("01ARZ3NDEKTSV4RRFFQ69G5FAB", "mac-a", "lin-1"), Verb: "claim", Actor: "mac-a+lin-1", Targets: []string{id}, Keep: -1},
		},
	}
	for path, data := range map[string][]byte{
		filepath.Join(root, "plans", "goals", "backlog.md"): RenderRoot(rootRecord),
		filepath.Join(root, "plans", "goals", id+".md"):     RenderFile(file),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	obligationAuthorityGit(t, root, "add", "plans/goals")
	obligationAuthorityGit(t, root, "commit", "-qm", "obligation authority fixture")
	obligationAuthorityGit(t, root, "update-ref", LocalLedgerBranch, "HEAD")
	obligationAuthorityGit(t, root, "update-ref", AcceptedRef, "HEAD")
	return root
}

func obligationAuthorityGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=obligation-fixture", "-c", "user.email=obligation-fixture@example.invalid"}, args...)...)
	cmd.Env = environWithoutGitSteering()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func obligationAuthorityEndpoint(root string) Endpoint {
	return Endpoint{Root: root, Remote: SyncLocal, Branch: "refs/heads/main"}
}

func obligationAuthorityVerbReq(root, ulid, machine string) VerbRequest {
	req := verbReq(root, ulid, machine)
	req.Endpoint = obligationAuthorityEndpoint(root)
	return req
}

func TestOnlyHumanProofAndChosenPolicyCanActivateAnObligation(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "governed")
	proof := proveObligationHuman(t, root)
	human := obligationAuthorityVerbReq(root, "01J5X00000000000000000Q542", "mac-a")
	human.Actor.Human = "Wido"
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SetObligation(human, "governed", testGovernedObligation(ObligationEnforced), &proof); err == nil {
		t.Fatal("an empty policy slot activated an enforced obligation")
	}
	if _, err := SetObligation(human, "governed", testGovernedObligation(ObligationDraft), &proof); err != nil {
		t.Fatalf("DRAFT observation was not free to record: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	active := obligationAuthorityVerbReq(root, "01J5X00000000000000000Q543", "mac-a")
	active.Actor.Human = "Wido"
	if _, err := SetObligation(active, "governed", testGovernedObligation(ObligationEnforced), &proof); err != nil {
		t.Fatalf("Wido's one-word policy choice did not activate the human-authorized obligation: %v", err)
	}
	projection, err := Project(obligationAuthorityEndpoint(root), false, active.Now)
	if err != nil {
		t.Fatal(err)
	}
	record := projection.Tree.Live["governed"].Obligation
	if record == nil || record.State != ObligationEnforced || record.ReviewPolicy != "A" ||
		record.ReviewOutcome != "human-approved" || record.AuthorizedBy != "Wido" || len(record.AuthorizedEffects) != 4 {
		t.Fatalf("activated obligation lost its human authorization: %+v", record)
	}
}

func TestRecordedRelayCanOnlyReplaceSetObligationAncestry(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "temporary-governed")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	human := obligationAuthorityVerbReq(root, "01J5X00000000000000000Q552", "mac-a")
	human.Actor.Human = "Wido"
	if _, err := SetObligation(human, "temporary-governed", testGovernedObligation(ObligationDraft), nil); err == nil {
		t.Fatal("set-obligation accepted no authority proof")
	}
	temporaryProof := testTemporaryGoalProof(t, root, "Wido authorizes this obligation", "2026-09-06")
	if _, err := SetObligation(human, "temporary-governed", testGovernedObligation(ObligationDraft), &temporaryProof); err != nil {
		t.Fatalf("recorded relay did not substitute for set-obligation ancestry: %v", err)
	}
	if temporaryProof.ValidFor(root) {
		t.Fatal("temporary set-obligation authority became reusable enrolled ancestry")
	}
	projection, err := Project(obligationAuthorityEndpoint(root), false, human.Now)
	if err != nil {
		t.Fatal(err)
	}
	draftRecord := projection.Tree.Live["temporary-governed"].Obligation
	if draftRecord == nil || draftRecord.ReviewOutcome != "" || draftRecord.AuthorityOutcome != AuthorityOutcomeTemporaryHumanWord ||
		draftRecord.AuthorityReviewBy != "2026-09-06" || draftRecord.AuthorityRuling != TemporaryGoalAuthorityRuling ||
		draftRecord.TemporaryHumanWord != "Wido authorizes this obligation" {
		t.Fatalf("temporary DRAFT obligation was not enumerable from the landed record: %+v", draftRecord)
	}
	if rendered := string(RenderFile(projection.Tree.Live["temporary-governed"])); !strings.Contains(rendered, `authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Wido authorizes this obligation"`) {
		t.Fatalf("temporary DRAFT marker was missing from rendered goal record:\n%s", rendered)
	}
	lastHistory := projection.Tree.Live["temporary-governed"].History[len(projection.Tree.Live["temporary-governed"].History)-1]
	if lastHistory.Verb != "set-obligation" || lastHistory.AuthorityOutcome != AuthorityOutcomeTemporaryHumanWord ||
		lastHistory.AuthorityRuling != TemporaryGoalAuthorityRuling || lastHistory.TemporaryHumanWord != "Wido authorizes this obligation" {
		t.Fatalf("temporary set-obligation was not durable in append-only history: %+v", lastHistory)
	}

	activeRoot := obligationAuthorityLocalRoot(t, "temporary-active")
	if err := os.WriteFile(filepath.Join(activeRoot, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	active := obligationAuthorityVerbReq(activeRoot, "01J5X00000000000000000Q553", "mac-a")
	active.Actor.Human = "Wido"
	activeProof := testTemporaryGoalProof(t, activeRoot, "Wido authorizes active consequences", "2026-09-06")
	if _, err := SetObligation(active, "temporary-active", testGovernedObligation(ObligationEnforced), &activeProof); err != nil {
		t.Fatalf("recorded relay did not stamp an active obligation: %v", err)
	}
	projection, err = Project(obligationAuthorityEndpoint(activeRoot), false, active.Now)
	if err != nil {
		t.Fatal(err)
	}
	activeRecord := projection.Tree.Live["temporary-active"].Obligation
	if activeRecord == nil || activeRecord.ReviewOutcome != ReviewOutcomeRecordedRelay || activeRecord.AuthorizedBy != "recorded-relay" ||
		activeRecord.AuthorityOutcome != AuthorityOutcomeTemporaryHumanWord || activeRecord.AuthorityReviewBy != "2026-09-06" ||
		activeRecord.AuthorityRuling != TemporaryGoalAuthorityRuling || activeRecord.TemporaryHumanWord != "Wido authorizes active consequences" ||
		len(activeRecord.AuthorizedEffects) != len(activeRecord.Effects) {
		t.Fatalf("temporary active obligation lost its honest recorded-relay authority: %+v", activeRecord)
	}
	if decision := activeRecord.Decide(EffectAuthorizeSpend); !decision.Apply || !strings.Contains(decision.Reason, "human provenance not verified") {
		t.Fatalf("temporary consequence overstated its recorded relay: %+v", decision)
	}
}

func TestRelayedSetObligationIsBoundOncePerGoalPerRuling(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "one-relayed-obligation")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first := obligationAuthorityVerbReq(root, "01J5X00000000000000000Q554", "mac-a")
	first.Actor.Human = "Wido"
	firstWord := "Wido authorizes first obligation"
	firstProof := testTemporaryGoalProof(t, root, firstWord, "2026-09-06")
	if result, err := SetObligation(first, "one-relayed-obligation", testGovernedObligation(ObligationDraft), &firstProof); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first relayed set-obligation did not confirm: %+v %v", result, err)
	}

	second := obligationAuthorityVerbReq(root, "01J5X00000000000000000Q555", "mac-a")
	second.Actor.Human = "Wido"
	second.Now = first.Now.Add(time.Minute)
	secondProof := testTemporaryGoalProof(t, root, "Wido authorizes second obligation", "2026-09-06")
	result, err := SetObligation(second, "one-relayed-obligation", testGovernedObligation(ObligationDraft), &secondProof)
	want := `goal one-relayed-obligation already used relayed set-obligation authority on 2026-08-20T22:00:00Z with recorded word "Wido authorizes first obligation"; a further set-obligation needs freshly observed enrolled-terminal authority`
	if err != nil || result.Outcome != OutcomeRejected || result.Detail != want {
		t.Fatalf("second relayed set-obligation refusal mismatch: result=%+v err=%v", result, err)
	}
}

func TestPruneRetainsRelayedUseForAReopenedGoalIdentifier(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result, err := Open(verbReq(root, "01J5X00000000000000000Q560", "mac-a"), "relay-survives-prune", "Keep relay use durable.", OriginMain, "Exercise it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	if result, err := Claim(verbReq(root, "01J5X00000000000000000Q561", "mac-a"), "relay-survives-prune", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", result, err)
	}
	first := verbReq(root, "01J5X00000000000000000Q562", "mac-a")
	first.Actor.Human = "Wido"
	firstProof := testTemporaryGoalProof(t, root, "Wido authorizes retained obligation", "2026-09-06")
	if result, err := SetObligation(first, "relay-survives-prune", testGovernedObligation(ObligationDraft), &firstProof); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first relayed set-obligation: %+v %v", result, err)
	}
	if result, err := Done(verbReq(root, "01J5X00000000000000000Q563", "mac-a"), "relay-survives-prune", "Archive it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("done: %+v %v", result, err)
	}
	if result, err := Prune(verbReq(root, "01J5X00000000000000000Q564", "mac-a"), 0); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("prune: %+v %v", result, err)
	}
	if result, err := Open(verbReq(root, "01J5X00000000000000000Q565", "mac-a"), "relay-survives-prune", "Reuse the stable identifier.", OriginMain, "Try another relay."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("reopen identifier: %+v %v", result, err)
	}
	if result, err := Claim(verbReq(root, "01J5X00000000000000000Q566", "mac-a"), "relay-survives-prune", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("reclaim identifier: %+v %v", result, err)
	}
	second := verbReq(root, "01J5X00000000000000000Q567", "mac-a")
	second.Actor.Human = "Wido"
	second.Now = first.Now.Add(time.Minute)
	secondProof := testTemporaryGoalProof(t, root, "Wido authorizes reset obligation", "2026-09-06")
	result, err := SetObligation(second, "relay-survives-prune", testGovernedObligation(ObligationDraft), &secondProof)
	want := `goal relay-survives-prune already used relayed set-obligation authority on 2026-08-20T22:00:00Z with recorded word "Wido authorizes retained obligation"; a further set-obligation needs freshly observed enrolled-terminal authority`
	if err != nil || result.Outcome != OutcomeRejected || result.Detail != want {
		t.Fatalf("prune reset the per-goal relay bound: result=%+v err=%v", result, err)
	}
}
