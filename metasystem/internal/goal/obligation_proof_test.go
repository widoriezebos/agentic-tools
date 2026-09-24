package goal

import (
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewObligationFixtureRoundTrip(t *testing.T) {
	t.Parallel()
	f := claimedGolden()
	f.ReviewObligations = []ReviewObligation{
		{Finding: "F-1", Chain: "critic", Artifact: "a.go", Test: "prove: a", Fixture: `group:section/a`, State: "open"},
		{Finding: "F-2", Chain: "critic", Artifact: "b.go", Test: "legacy", State: "open"},
	}
	rendered := RenderFile(f)
	parsed, problems := ParseFile(rendered)
	if len(problems) != 0 || string(RenderFile(parsed)) != string(rendered) || !strings.Contains(string(rendered), `test="legacy" state=open`) {
		t.Fatalf("fixture round trip changed bytes: problems=%v\n%s", problems, RenderFile(parsed))
	}
	_, problems = ParseFile([]byte(strings.Replace(string(rendered), " state=open", " unknown=x state=open", 1)))
	if !problemsContain(problems, `unknown key "unknown"`) {
		t.Fatalf("unknown key was accepted: %v", problems)
	}
}
func TestFixtureObligationLifecycle(t *testing.T) {
	t.Parallel()
	endpoint := obligationAuthorityLocalEndpoint(t, "review-proof")
	root := endpoint.Root
	req := verbReqFor(endpoint, "01J5X00000000000000000SP01", "mac-a")
	obligation := ReviewObligation{Finding: "F-1", Chain: "design-critic", Artifact: "a.go", Test: "prove: group:a", Fixture: "group:section/a"}
	if result, err := DeferFindings(req, "review-proof", []ReviewObligation{obligation}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("defer fixture obligation: %+v %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000SP02"
	obligation.Fixture = "group:section/b"
	if _, err := DeferFindings(req, "review-proof", []ReviewObligation{obligation}); err != nil {
		t.Fatal(err)
	}
	projected, _ := Project(req.Endpoint, false, req.Now)
	if got := projected.Tree.Live["review-proof"].ReviewObligations[0].Fixture; got != "group:section/a" {
		t.Fatalf("second defer rewrote fixture to %q", got)
	}
	req.Ulid = "01J5X00000000000000000SP03"
	if result, err := DischargeReviewObligation(req, "review-proof", "F-1", "design-critic", "mac-a", "bare"); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "requires --root") {
		t.Fatalf("bare citation discharged fixture obligation: %+v %v", result, err)
	}
	evidence := fixtureEvidence(t, root)
	req.Ulid = "01J5X00000000000000000SP04"
	if result, err := DischargeReviewObligation(req, "review-proof", "F-1", "design-critic", "mac-a", "", evidence); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("proved discharge: %+v %v", result, err)
	}
	projected, _ = Project(req.Endpoint, false, req.Now)
	got := projected.Tree.Live["review-proof"].ReviewObligations[0]
	wantTest := "proved: group:section/a chain=implementation-chain critic=critic-root result=run-passed"
	if got.State != "discharged" || got.Test != wantTest || got.Fixture != "group:section/a" || !strings.Contains(string(RenderFile(projected.Tree.Live["review-proof"])), `fixture="group:section/a"`) {
		t.Fatalf("proved obligation lost stored fixture or citation: %+v", got)
	}
}
func TestFixtureObligationEvidenceRefusals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	valid := fixtureEvidence(t, root)
	obligation := ReviewObligation{Finding: "F-1", Chain: "design-critic", Artifact: "a.go", Fixture: "group:section/a"}
	type mutate func(*DischargeEvidence, *ReviewObligation, map[string]any)
	cases := []struct {
		name, want string
		change     mutate
	}{
		{"missing_chain", "requires --implementation-chain", func(e *DischargeEvidence, _ *ReviewObligation, _ map[string]any) { e.ImplementationChain = "" }},
		{"missing_artifact", "requires --artifact", func(e *DischargeEvidence, _ *ReviewObligation, _ map[string]any) { e.Artifact = "" }},
		{"missing_result", "requires --result", func(e *DischargeEvidence, _ *ReviewObligation, _ map[string]any) { e.ResultRunID = "" }},
		{"unknown_result", "cannot use governed result", func(e *DischargeEvidence, _ *ReviewObligation, _ map[string]any) { e.ResultRunID = "run-unknown" }},
		{"near_miss_artifact", "mismatched --artifact", func(e *DischargeEvidence, _ *ReviewObligation, _ map[string]any) { e.Artifact = "prefix/a.go" }},
		{"invalid_fixture", "must name one test group", func(_ *DischargeEvidence, o *ReviewObligation, _ map[string]any) { o.Fixture = "group:section a" }},
		{"missing_group", "exactly one result group", func(_ *DischargeEvidence, o *ReviewObligation, _ map[string]any) { o.Fixture = "group:other" }},
		{"reused", "a reused group result", func(e *DischargeEvidence, _ *ReviewObligation, _ map[string]any) { e.ResultRunID = "run-reused" }},
		{"incomplete", "complete passed result group", func(e *DischargeEvidence, _ *ReviewObligation, _ map[string]any) { e.ResultRunID = "run-failed" }},
		{"unreadable_critic", "cannot read critic root", func(e *DischargeEvidence, _ *ReviewObligation, _ map[string]any) { e.CriticRoot = "absent-critic" }},
		{"malformed_critic", "cannot decode critic root", func(_ *DischargeEvidence, _ *ReviewObligation, c map[string]any) { c["malformed"] = func() {} }},
		{"wrong_job", "own code-critic record", func(_ *DischargeEvidence, _ *ReviewObligation, c map[string]any) { c["jobId"] = "other-critic" }},
		{"wrong_role", "code-critic record", func(_ *DischargeEvidence, _ *ReviewObligation, c map[string]any) { c["role"] = "design-critic" }},
		{"wrong_reviews", "review implementation chain", func(_ *DischargeEvidence, _ *ReviewObligation, c map[string]any) { c["reviews"] = "other-chain" }},
		{"open_chain", "closed chain", func(_ *DischargeEvidence, _ *ReviewObligation, c map[string]any) { c["chainClosed"] = false }},
		{"open_finding", "clean finding register", func(_ *DischargeEvidence, _ *ReviewObligation, c map[string]any) {
			c["findingRegister"] = []any{map[string]any{"findingId": "F", "critic": "c", "rigorClass": "bounded", "factsDigest": "d", "status": "open", "evidenceDigest": "d", "multiplicity": float64(1)}}
		}},
		{"open_finding_without_closure", "clean closure", func(_ *DischargeEvidence, _ *ReviewObligation, c map[string]any) { delete(c, "closure") }},
	}
	for _, tc := range cases {
		e, o, critic := valid, obligation, validCriticRecord()
		tc.change(&e, &o, critic)
		writeProofJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "critic-root.json"), critic)
		if err := proveFixtureObligation(e, o); err == nil || !strings.Contains(err.Error(), "discharge-review-obligation") || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s refusal = %v, want %q", tc.name, err, tc.want)
		}
	}
}
func fixtureEvidence(t *testing.T, root string) DischargeEvidence {
	digest := strings.Repeat("a", 64)
	identity := proofrun.BuildProofIdentityForContext(proofrun.ExecutionContext{ManifestDigest: digest, Configuration: digest, Platform: "fixture/os", Toolchain: digest}, "full", "fixture", nil, 1)
	for _, status := range []string{"passed", "reused", "failed"} {
		zero := 0
		group := proofrun.GroupResult{ID: "section/a", Kind: "unit", InputManifest: []string{"a.go"}, Status: status, CollectionComplete: status != "failed", NativeLaunched: status == "passed", NativeExitStatus: &zero, ReuseAttempt: "source", ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}
		if status != "reused" {
			group.ReuseAttempt = ""
		}
		attemptID := "attempt-" + status
		result := proofrun.TestResult{SchemaVersion: 1, AttemptID: attemptID, Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard, ProjectRoot: root, BaseCommit: "base", CandidateTree: strings.Repeat("b", 40), ContractDigest: digest, BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest, PlanDigest: digest, SelectedGroups: []string{group.ID}, Groups: []proofrun.GroupResult{group}, LaunchCounts: proofrun.LaunchCounts{CountsComplete: true}, Cost: proofrun.TestCost{DeclaredTargetMS: 1}}
		result.RecomputeDelivery()
		deadline, ended := "2026-09-01T11:00:00Z", "2026-09-01T10:01:00Z"
		owner := &proofrun.ReservationOwner{ControlRoot: root, RunID: "run-" + status, RunGeneration: 1, LaunchNonce: "nonce", GoalRevision: 1, ObligationRevision: 1, AttemptOrdinal: 1, Deadline: deadline}
		attempt := proofrun.Attempt{SchemaVersion: proofrun.AttemptSchemaVersion, AttemptID: attemptID, GoalID: "goal", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 60, ObservedMinutes: 1, StartedAt: "2026-09-01T10:00:00Z", Deadline: deadline, EndedAt: ended, ProofIdentity: identity, ControlRoot: root, ExecutionRoot: root, Launcher: proofrun.ProcessIdentity{Pid: 1, PidStartedAt: 1}, Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalSuccess, At: ended}, ReservationOwner: owner, TestResult: &result}
		path, _ := proofrun.AttemptPath(root, attemptID)
		writeProofJSON(t, path, attempt)
	}
	writeProofJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "critic-root.json"), validCriticRecord())
	return DischargeEvidence{Root: root, ImplementationChain: "implementation-chain", Artifact: "a.go", ResultRunID: "run-passed", CriticRoot: "critic-root"}
}
func mustFixtureProof(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
func validCriticRecord() map[string]any {
	return map[string]any{"jobId": "critic-root", "role": "code-critic", "reviews": []any{"implementation-chain"}, "chainClosed": true, "findingRegister": []any{}, "closure": map[string]any{"criticRoot": "critic-root", "round": float64(1), "subject": map[string]any{"kind": "live"}, "mechanism": "clean"}}
}
func writeProofJSON(t *testing.T, path string, value any) {
	mustFixtureProof(t, os.MkdirAll(filepath.Dir(path), 0o755))
	data, _ := json.Marshal(value)
	mustFixtureProof(t, os.WriteFile(path, data, 0o644))
}
