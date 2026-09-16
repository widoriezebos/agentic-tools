package dispatch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestRoundTwoCloseFixtureReachesRegister(t *testing.T) {
	finding := []any{map[string]any{"id": "fixture-finding", "material": true, "claim": "title", "evidence": "evidence"}}
	rigor := []any{map[string]any{"findingId": "fixture-finding", "artifact": "metasystem/internal/x.go", "rigorClass": "bounded", "facts": nil, "grain": "mechanical", "fixture": "go test ./fixture"}}
	v5, demotions, _, err := foldCritiqueFindingsVersioned(nil, "design-critic", "critic-round", finding, rigor, 5, critiqueSubject{legacy: true}, 2)
	check(t, err == nil && len(v5) == 1 && v5[0].Fixture == "go test ./fixture" && v5[0].Grain == "mechanical", "version 5 fold did not retain the mechanical fixture: finding=%+v demotions=%+v err=%v", v5, demotions, err)
	v4, _, _, err := foldCritiqueFindingsVersioned(nil, "design-critic", "critic-round", finding, rigor, 4, critiqueSubject{legacy: true}, 2)
	check(t, err == nil && len(v4) == 1 && v4[0].Fixture == "" && v4[0].Grain == "invariant", "version 4 fold gained version 5 fields: finding=%+v err=%v", v4, err)
	encoded := encodeFindingRegister(v5)
	clean, cleanErr := readsubject.CleanRegister(encoded)
	check(t, cleanErr == nil && !clean, "fixture-bearing register was not accepted as canonical and open: clean=%v err=%v", clean, cleanErr)
	historical := encoded[0].(map[string]any)
	delete(historical, "grain")
	delete(historical, "fixture")
	decoded, err := decodeFindingRegister([]any{historical})
	check(t, err == nil && decoded[0].Fixture == "" && decoded[0].Grain == "invariant", "historical thirteen-field register did not decode: finding=%+v err=%v", decoded, err)
}
func TestRoundTwoCloseCleanIgnoresAccounting(t *testing.T) {
	repo, root, _ := writeCloseRoot(t, "design-critic", 2, nil, nil, 99, 0)
	outcome, err := CritiqueRegisterClose(repo, root)
	check(t, err == nil && outcome == "closed", "clean folded round two did not close: outcome=%q err=%v", outcome, err)
}
func TestRoundTwoCloseMechanicalFallingUsesOwnFixtures(t *testing.T) {
	findings := []registerFinding{
		closeFinding("finding-a", "mechanical", "go test ./a", "title a", critiqueModel.Bounded),
		closeFinding("finding-b", "mechanical", "go test ./b -run TestB", "title b", critiqueModel.Bounded),
	}
	repo, root, path := writeCloseRoot(t, "design-critic", 2, findings, materialHistory(3, 2), 2, 2)
	var got []goal.ReviewObligation
	outcome, err := critiqueRegisterClose(repo, root, captureObligations(&got))
	check(t, err == nil && outcome == "deferred", "falling mechanical round did not defer: outcome=%q err=%v", outcome, err)
	check(t, len(got) == 2 && got[0].Test == "prove: go test ./a" && got[1].Test == "prove: go test ./b -run TestB", "obligations did not name each finding's own fixture: %+v", got)
	rootRecord, _ := readObject(path)
	register, err := decodeFindingRegister(rootRecord[findingRegisterField])
	check(t, err == nil && register[0].Status == "deferred" && register[1].Status == "deferred", "deferred findings were not recorded: register=%+v err=%v", register, err)
}
func TestRoundTwoCloseHumanRaiseTable(t *testing.T) {
	mechanical := closeFinding("mechanical", "mechanical", "go test ./m", "mechanical title", critiqueModel.Bounded)
	cases := []struct {
		name     string
		findings []registerFinding
		history  []any
		wantIDs  []string
	}{
		{"empty_fixture", []registerFinding{closeFinding("empty-fixture", "mechanical", "", "title", critiqueModel.Bounded)}, materialHistory(2, 1), []string{"empty-fixture"}},
		{"invariant", []registerFinding{closeFinding("invariant", "invariant", "go test ./i", "invariant title", critiqueModel.Bounded), mechanical}, materialHistory(3, 2), []string{"invariant"}},
		{"severe", []registerFinding{closeFinding("severe", "mechanical", "go test ./s", "severe title", critiqueModel.Severe)}, materialHistory(2, 1), []string{"severe"}},
		{"unproven", []registerFinding{closeFinding("unproven", "mechanical", "go test ./u", "unproven title", critiqueModel.Unproven)}, materialHistory(2, 1), []string{"unproven"}},
		{"multiple_causes", []registerFinding{closeFinding("severe", "mechanical", "go test ./s", "severe title", critiqueModel.Severe), closeFinding("invariant", "invariant", "go test ./i", "invariant title", critiqueModel.Bounded)}, materialHistory(3, 2), []string{"severe", "invariant"}},
		{"not_falling", []registerFinding{mechanical, closeFinding("mechanical-two", "mechanical", "go test ./m2", "title two", critiqueModel.Bounded)}, materialHistory(2, 2), []string{"mechanical", "mechanical-two"}},
		{"missing_trajectory", []registerFinding{mechanical}, nil, []string{"mechanical"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, root, path := writeCloseRoot(t, "design-critic", 2, tc.findings, tc.history, 2, 2)
			before, readErr := os.ReadFile(path)
			check(t, readErr == nil, "cannot read root before close: %v", readErr)
			var obligations []goal.ReviewObligation
			_, err := critiqueRegisterClose(repo, root, captureObligations(&obligations))
			assertHumanRaise(t, err, tc.wantIDs...)
			after, readErr := os.ReadFile(path)
			check(t, readErr == nil && string(after) == string(before), "round-two refusal changed the root record or its limit: readErr=%v", readErr)
		})
	}
}
func TestRoundTwoCloseTableIsDesignRoundTwoOnly(t *testing.T) {
	for _, role := range []string{"code-critic", "warden"} {
		t.Run(role+"_uses_titles_only_after_exhaustion", func(t *testing.T) {
			findings := []registerFinding{closeFinding("one", "mechanical", "fixture one", "title one", critiqueModel.Bounded), closeFinding("two", "mechanical", "fixture two", "title two", critiqueModel.Bounded)}
			repo, root, path := writeCloseRoot(t, role, 2, findings, materialHistory(3, 2), 2, 1)
			var got []goal.ReviewObligation
			outcome, err := critiqueRegisterClose(repo, root, captureObligations(&got))
			check(t, err != nil && strings.Contains(err.Error(), "dispatch the next round") && len(got) == 0, "%s deferred before exhaustion: outcome=%q obligations=%+v err=%v", role, outcome, got, err)
			record, _ := readObject(path)
			record[criticRoundsConsumedField] = int64(2)
			check(t, writeRecord(path, record) == nil, "cannot exhaust generic critic budget")
			outcome, err = critiqueRegisterClose(repo, root, captureObligations(&got))
			check(t, err == nil && outcome == "deferred" && len(got) == 2 && got[0].Test == "prove: title one" && got[1].Test == "prove: title two", "%s did not retain generic exhausted deferral: outcome=%q obligations=%+v err=%v", role, outcome, got, err)
		})
	}
	t.Run("design_round_one_dispatches_next_round", func(t *testing.T) {
		findings := []registerFinding{closeFinding("round-one", "mechanical", "fixture text", "title text", critiqueModel.Bounded)}
		repo, root, _ := writeCloseRoot(t, "design-critic", 1, findings, materialHistory(1, 0), 2, 1)
		outcome, err := CritiqueRegisterClose(repo, root)
		check(t, outcome == "closed" && err != nil && strings.Contains(err.Error(), "dispatch the next round"), "design round one did not retain generic budget behavior: outcome=%q err=%v", outcome, err)
	})
}
func TestRoundTwoCloseGenericOrderPrecedesFoldMetadata(t *testing.T) {
	resolved := closeFinding("resolved", "mechanical", "fixture", "title", critiqueModel.Bounded)
	resolved.Status, resolved.Resolution = "resolved", "withdrawn"
	for name, findings := range map[string][]registerFinding{"blocker": {closeFinding("severe", "invariant", "", "title", critiqueModel.Severe)}, "clean": {resolved}} {
		t.Run(name, func(t *testing.T) {
			repo, root, path := writeCloseRoot(t, "code-critic", 1, findings, nil, 2, 1)
			record, _ := readObject(path)
			delete(record, findingRegisterRoundField)
			check(t, writeRecord(path, record) == nil, "cannot remove folded metadata")
			outcome, err := CritiqueRegisterClose(repo, root)
			ok := name == "blocker" && err != nil && strings.Contains(err.Error(), "blocks close") || name == "clean" && err == nil && outcome == "closed"
			check(t, ok, "generic %s did not precede folded metadata: outcome=%q err=%v", name, outcome, err)
		})
	}
}
func closeFinding(id, grain, fixture, title string, class critiqueModel.RigorClass) registerFinding {
	return registerFinding{FindingID: id, Critic: "critic-round", RigorClass: class, Grain: grain, Fixture: fixture,
		FactsDigest: digestJSON(nil), Artifact: "x.go", Title: title, Status: "open", Evidence: "evidence",
		EvidenceDigest: digestJSON("evidence"), Multiplicity: 1}
}
func materialHistory(roundOne, roundTwo int64) []any {
	return []any{map[string]any{"round": int64(1), "material": roundOne}, map[string]any{"round": int64(2), "material": roundTwo}}
}
func writeCloseRoot(t *testing.T, role string, round int64, findings []registerFinding, history []any, limit, consumed int64) (string, string, string) {
	t.Helper()
	repo, root := t.TempDir(), "critic-root"
	path := filepath.Join(repo, "artifacts", "agents", "jobs", root+".json")
	check(t, os.MkdirAll(filepath.Dir(path), 0o755) == nil, "cannot create test job directory")
	record := map[string]any{"jobId": root, "role": role, "findingRegister": encodeFindingRegister(findings), "findingRegisterRound": round,
		"reviewRoundLimit": limit, "criticRoundsConsumed": consumed, "goalId": "goal-1", "machineId": "machine-1", "mainId": "main-1", "claimEpoch": int64(1)}
	if history != nil {
		record[materialByRoundField] = history
	}
	check(t, writeRecord(path, record) == nil, "cannot write test root")
	return repo, root, path
}
func captureObligations(got *[]goal.ReviewObligation) deferReviewObligationsFunc {
	return func(_, _, _, _, _ string, _ int64, obligations []goal.ReviewObligation) (string, error) {
		*got = append([]goal.ReviewObligation(nil), obligations...)
		return "decision-opid", nil
	}
}
func assertHumanRaise(t *testing.T, err error, findingIDs ...string) {
	t.Helper()
	var opErr *OpError
	check(t, errors.As(err, &opErr), "close did not return the cap-exhausted refusal: %T %v", err, err)
	check(t, opErr.Code == CritiqueCapExhaustedExitCode && strings.Contains(err.Error(), CritiqueCapExhaustedReason), "close refusal carried code=%d reason=%q: %+v", opErr.Code, opErr.Reason, opErr)
	message := err.Error()
	for _, want := range append(findingIDs, "goal accept-risk --finding <id> --chain <root> --by <human> --why", "goal edit") {
		check(t, strings.Contains(message, want), "human refusal did not name %q: %v", want, err)
	}
}
func check(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}
func TestRoundTwoCloseRefusesUnrecordedDeferral(t *testing.T) {
	repo := revisionBindingBed(t, 2)
	path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic-root.json")
	check(t, os.MkdirAll(filepath.Dir(path), 0o755) == nil, "cannot create test job directory")
	finding := closeFinding("finding-a", "mechanical", "go test ./a", "title a", critiqueModel.Bounded)
	record := map[string]any{"jobId": "critic-root", "role": "design-critic", "findingRegister": encodeFindingRegister([]registerFinding{finding}), "findingRegisterRound": int64(2),
		"reviewRoundLimit": int64(2), "criticRoundsConsumed": int64(2), materialByRoundField: materialHistory(2, 1), "goalId": "bounded", "machineId": "bed-m1", "mainId": "successor-main", "claimEpoch": int64(7)}
	check(t, writeRecord(path, record) == nil, "cannot write test root")
	outcome, err := CritiqueRegisterClose(repo, "critic-root")
	check(t, err != nil && strings.Contains(err.Error(), "goal bounded defer-findings ended rejected: goal bounded defer-findings requires its owning pair"), "a deferral the goal refused closed anyway: outcome=%q err=%v", outcome, err)
	written, _ := readObject(path)
	register, decodeErr := decodeFindingRegister(written[findingRegisterField])
	check(t, decodeErr == nil && register[0].Status == "open", "a refused deferral changed the register: %+v %v", register, decodeErr)
	record["mainId"] = "coordinator"
	check(t, writeRecord(path, record) == nil, "cannot rewrite test root")
	outcome, err = CritiqueRegisterClose(repo, "critic-root")
	check(t, err == nil && outcome == "deferred", "the owning pair's deferral did not close: outcome=%q err=%v", outcome, err)
}
