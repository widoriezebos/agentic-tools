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

func proveObligationHuman(t *testing.T, root string) humanauthority.Proof {
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
	proof, err := humanauthority.Prove(root, 30, reader, time.Date(2026, 8, 30, 8, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return proof
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

func TestTemporaryHumanWordCanOnlyReplaceSetObligationAncestry(t *testing.T) {
	root := obligationAuthorityLocalRoot(t, "temporary-governed")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	human := obligationAuthorityVerbReq(root, "01J5X00000000000000000Q552", "mac-a")
	human.Actor.Human = "Wido"
	if _, err := SetObligation(human, "temporary-governed", testGovernedObligation(ObligationDraft), nil); err == nil {
		t.Fatal("set-obligation accepted no authority proof")
	}
	temporaryProof, err := humanauthority.TemporaryProof(root, "Wido authorizes this obligation", "2026-09-06", human.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetObligation(human, "temporary-governed", testGovernedObligation(ObligationDraft), &temporaryProof); err != nil {
		t.Fatalf("temporary human word did not substitute for set-obligation ancestry: %v", err)
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
		draftRecord.AuthorityReviewBy != "2026-09-06" {
		t.Fatalf("temporary DRAFT obligation was not enumerable from the landed record: %+v", draftRecord)
	}
	if rendered := string(RenderFile(projection.Tree.Live["temporary-governed"])); !strings.Contains(rendered, "authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06") {
		t.Fatalf("temporary DRAFT marker was missing from rendered goal record:\n%s", rendered)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	active := obligationAuthorityVerbReq(root, "01J5X00000000000000000Q553", "mac-a")
	active.Actor.Human = "Wido"
	activeProof, err := humanauthority.TemporaryProof(root, "Wido authorizes active consequences", "2026-09-06", active.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SetObligation(active, "temporary-governed", testGovernedObligation(ObligationEnforced), &activeProof); err != nil {
		t.Fatalf("temporary human word did not stamp an active obligation: %v", err)
	}
	projection, err = Project(obligationAuthorityEndpoint(root), false, active.Now)
	if err != nil {
		t.Fatal(err)
	}
	activeRecord := projection.Tree.Live["temporary-governed"].Obligation
	if activeRecord == nil || activeRecord.ReviewOutcome != ReviewOutcomeHumanApproved ||
		activeRecord.AuthorityOutcome != AuthorityOutcomeTemporaryHumanWord || activeRecord.AuthorityReviewBy != "2026-09-06" ||
		len(activeRecord.AuthorizedEffects) != len(activeRecord.Effects) {
		t.Fatalf("temporary active obligation lost approval or provenance: %+v", activeRecord)
	}
}
