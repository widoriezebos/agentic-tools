package goal

import (
	"os"
	"path/filepath"
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

func TestOnlyHumanProofAndChosenPolicyCanActivateAnObligation(t *testing.T) {
	_, root, _ := twoClones(t)
	seedLedger(t, root)
	claimRequest := verbReq(root, "01J5X00000000000000000Q541", "mac-a")
	if _, err := OpenClaim(claimRequest, "governed", "Govern validation", OriginMain, "Run it.", testBudget()); err != nil {
		t.Fatal(err)
	}
	proof := proveObligationHuman(t, root)
	human := verbReq(root, "01J5X00000000000000000Q542", "mac-a")
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
	active := verbReq(root, "01J5X00000000000000000Q543", "mac-a")
	active.Actor.Human = "Wido"
	if _, err := SetObligation(active, "governed", testGovernedObligation(ObligationEnforced), &proof); err != nil {
		t.Fatalf("Wido's one-word policy choice did not activate the human-authorized obligation: %v", err)
	}
	projection, err := Project(endpointFor(root), false, active.Now)
	if err != nil {
		t.Fatal(err)
	}
	record := projection.Tree.Live["governed"].Obligation
	if record == nil || record.State != ObligationEnforced || record.ReviewPolicy != "A" ||
		record.ReviewOutcome != "human-approved" || record.AuthorizedBy != "Wido" || len(record.AuthorizedEffects) != 4 {
		t.Fatalf("activated obligation lost its human authorization: %+v", record)
	}
}
