package dispatch

import "testing"

func grainFinding(id string, material bool) map[string]any {
	return map[string]any{"id": id, "material": material, "claim": id + " claim", "evidence": id + " evidence"}
}
func grainRow(id, grain, artifact string) map[string]any {
	return map[string]any{"findingId": id, "grain": grain, "artifact": artifact, "rigorClass": "bounded", "facts": map[string]any{}, "reopeningTrigger": "change"}
}
func requireGrain(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}
func TestFindingRegisterEncodeDefaultsUnsetGrain(t *testing.T) {
	decoded, err := decodeFindingRegister(encodeFindingRegister([]registerFinding{{FindingID: "F-1", Critic: "critic", RigorClass: "bounded", FactsDigest: digestJSON(nil), Status: "open", EvidenceDigest: digestJSON(nil), Multiplicity: 1}}))
	requireGrain(t, err == nil && len(decoded) == 1 && decoded[0].Grain == "invariant", "unset grain round trip = %#v, err %v", decoded, err)
}
func TestFoldCritiqueFindingsNormalizesGrainBySchemaVersion(t *testing.T) {
	findings := []any{grainFinding("F-1", true), grainFinding("F-2", true)}
	rows := []any{grainRow("F-1", "mechanical", "metasystem/a.go"), grainRow("F-2", "invariant", "metasystem/b.go")}
	v5, _, _, err := foldCritiqueFindingsVersioned(nil, "code-critic", "critic", findings, rows, 5, critiqueSubject{legacy: true}, 1)
	requireGrain(t, err == nil && len(v5) == 2 && v5[0].Grain == "mechanical" && v5[1].Grain == "invariant", "version 5 grains = %#v, err %v", v5, err)
	for _, version := range []int64{0, 4} {
		register, _, _, err := foldCritiqueFindingsVersioned(nil, "code-critic", "critic", findings, rows, version, critiqueSubject{legacy: true}, 1)
		requireGrain(t, err == nil && register[0].Grain == "invariant" && register[1].Grain == "invariant", "version %d trusted wire grain", version)
	}
	entry := map[string]any{"findingId": "old", "critic": "critic", "rigorClass": "bounded", "factsDigest": digestJSON(nil), "facts": nil, "artifact": "metasystem/a.go", "title": "old", "status": "open", "resolution": "", "decisionOpid": "", "evidence": "e", "evidenceDigest": digestJSON("e"), "multiplicity": 1}
	historical, err := decodeFindingRegister([]any{entry})
	encoded := encodeFindingRegister(historical)
	requireGrain(t, err == nil && historical[0].Grain == "invariant" && encoded[0].(map[string]any)["grain"] == "invariant", "historical register did not normalize losslessly: %#v, %v", encoded, err)
}
func TestFoldCritiqueFindingsNeverWeakensGrain(t *testing.T) {
	for _, grains := range [][2]string{{"invariant", "mechanical"}, {"mechanical", "invariant"}} {
		register, _, _, err := foldCritiqueFindingsVersioned(nil, "code-critic", "critic-r1", []any{grainFinding("F-1", true)}, []any{grainRow("F-1", grains[0], "metasystem/a.go")}, 5, critiqueSubject{legacy: true}, 1)
		requireGrain(t, err == nil, "round 1 fold: %v", err)
		register, _, _, err = foldCritiqueFindingsVersioned(register, "code-critic", "critic-r2", []any{grainFinding("F-1", true)}, []any{grainRow("F-1", grains[1], "metasystem/a.go")}, 5, critiqueSubject{legacy: true}, 2)
		requireGrain(t, err == nil && register[0].Grain == "invariant", "round 2 %q -> %q grain = %q, err %v", grains[0], grains[1], register[0].Grain, err)
	}
}
func TestMaterialByRoundCountsAdmittedDistinctFindings(t *testing.T) {
	subject := critiqueSubject{paths: map[string]bool{"metasystem/a.go": true, "metasystem/b.go": true}}
	register, demotions, admitted, err := foldCritiqueFindingsVersioned(nil, "code-critic", "critic", []any{grainFinding("F-1", true), grainFinding("F-2", true), grainFinding("F-3", true), grainFinding("F-1", true)}, []any{grainRow("F-1", "invariant", "metasystem/a.go"), grainRow("F-2", "invariant", "metasystem/b.go"), grainRow("F-3", "invariant", "metasystem/out.go")}, 5, subject, 1)
	requireGrain(t, err == nil && admitted == 2 && len(demotions) == 1, "round 1 admitted=%d demotions=%d err=%v", admitted, len(demotions), err)
	history, err := appendMaterialRound([]any{}, 1, admitted, false)
	if err != nil {
		t.Fatalf("round 1 appendMaterialRound: %v", err)
	}
	register, _, admitted, err = foldCritiqueFindingsVersioned(register, "code-critic", "critic-r2", []any{grainFinding("F-2", true), grainFinding("withdrawn", false)}, []any{grainRow("F-2", "invariant", "metasystem/b.go")}, 5, subject, 2)
	requireGrain(t, err == nil && admitted == 1 && len(register) == 2, "round 2 admitted=%d register=%d err=%v", admitted, len(register), err)
	history, err = appendMaterialRound(history, 2, admitted, false)
	if err != nil {
		t.Fatalf("round 2 appendMaterialRound: %v", err)
	}
	history, err = appendMaterialRound(history, 3, 0, true)
	requireGrain(t, err == nil && len(history) == 2 && history[0].(map[string]any)["material"] == int64(2) && history[1].(map[string]any)["material"] == int64(1), "materialByRound = %#v, err %v", history, err)
}
