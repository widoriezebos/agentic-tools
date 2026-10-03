package readsubject

import "testing"

// TestLandableRegister: a register whose entries are withdrawn, ruled
// out-of-scope (bounded) or accepted as a risk by a person's recorded act
// yields the landing's read and names its accepted risks; it is still not a
// clean read. Open, disputed, deferred, refuted, accepted (fix required),
// out-of-scope severe or unproven, and an accepted risk with no recorded act
// are not landable. An accepted risk covers only the finding content it was
// accepted over (F4; content only, Wido 2026-10-03): an acceptance of content
// that changed since is not landable; one that predates binding (no digest)
// covers the finding as it stands.
func TestLandableRegister(t *testing.T) {
	t.Parallel()
	entry := func(id, status, resolution, class, opid string) map[string]any {
		value := modernRegisterEntry(status, resolution)
		value["findingId"], value["rigorClass"], value["decisionOpid"] = id, class, opid
		return value
	}
	accepted := func(id, class, opid string) map[string]any {
		value := entry(id, "accepted-risk", "accepted-risk", class, opid)
		value["grain"], value["fixture"] = "invariant", ""
		value["acceptedDigest"] = AcceptedFindingDigest(value)
		return value
	}
	risk := accepted("F-2", "unproven", "op-2")
	landable, risks, err := LandableRegister([]any{entry("F-0", "resolved", "withdrawn", "bounded", ""), entry("F-1", "resolved", "out-of-scope", "bounded", ""), risk})
	if err != nil || !landable || len(risks) != 1 || risks[0] != (AcceptedRisk{FindingID: "F-2", DecisionOpID: "op-2", AcceptedDigest: risk["acceptedDigest"].(string)}) {
		t.Fatalf("withdrawn, out-of-scope and a person's risk = %v %v %v", landable, risks, err)
	}
	if clean, err := CleanRegister([]any{risk}); err != nil || clean {
		t.Fatalf("an accepted risk became a clean read: %v %v", clean, err)
	}
	if landable, risks, err := LandableRegister([]any{}); err != nil || !landable || risks != nil {
		t.Fatalf("an empty register = %v %v %v", landable, risks, err)
	}
	// Fields outside the finding's content (the reporting critic, how often
	// it was reported, its grain) do not change what was accepted.
	recounted := accepted("F-3", "severe", "op-3")
	recounted["critic"], recounted["multiplicity"], recounted["grain"] = "another-round", 3, "mechanical"
	if landable, risks, err := LandableRegister([]any{recounted}); err != nil || !landable || len(risks) != 1 {
		t.Fatalf("a re-reported finding of the same content = %v %v %v", landable, risks, err)
	}
	// An acceptance recorded before content binding has no digest: it covers
	// the finding as it stands (R-142-m1e), and a closure written then, which
	// names no digest either, still holds.
	predating := entry("F-4", "accepted-risk", "accepted-risk", "severe", "op-4")
	landable, risks, err = LandableRegister([]any{predating})
	if err != nil || !landable || len(risks) != 1 || risks[0] != (AcceptedRisk{FindingID: "F-4", DecisionOpID: "op-4", AcceptedDigest: AcceptedFindingDigest(predating)}) {
		t.Fatalf("an acceptance that predates content binding = %v %v %v", landable, risks, err)
	}
	if !RecordedAcceptedRisksHold([]AcceptedRisk{{FindingID: "F-4", DecisionOpID: "op-4"}}, risks) {
		t.Fatal("a closure recorded before content binding no longer holds")
	}
	if RecordedAcceptedRisksHold([]AcceptedRisk{{FindingID: "F-4", DecisionOpID: "op-4", AcceptedDigest: "other"}}, risks) ||
		RecordedAcceptedRisksHold([]AcceptedRisk{{FindingID: "F-4", DecisionOpID: "other-op"}}, risks) {
		t.Fatal("a closure of other content or another decision holds")
	}
	changedFacts := accepted("F", "severe", "op")
	changedFacts["factsDigest"] = "changed-facts"
	changedEvidence := accepted("F", "severe", "op")
	changedEvidence["evidenceDigest"] = "changed-evidence"
	for name, value := range map[string]map[string]any{
		"open":                     entry("F", "open", "", "bounded", ""),
		"disputed":                 entry("F", "disputed", "", "bounded", ""),
		"deferred":                 entry("F", "deferred", "deferred", "bounded", "op"),
		"refuted":                  entry("F", "resolved", "refuted", "bounded", ""),
		"accepted":                 entry("F", "resolved", "accepted", "bounded", ""),
		"out-of-scope severe":      entry("F", "resolved", "out-of-scope", "severe", ""),
		"out-of-scope unproven":    entry("F", "resolved", "out-of-scope", "unproven", ""),
		"risk without an act":      accepted("F", "severe", ""),
		"risk of changed facts":    changedFacts,
		"risk of changed evidence": changedEvidence,
	} {
		if landable, risks, err := LandableRegister([]any{risk, value}); err != nil || landable || risks != nil {
			t.Fatalf("%s beside an accepted risk = %v %v %v", name, landable, risks, err)
		}
	}
	misplaced := entry("F", "open", "", "severe", "")
	misplaced["grain"], misplaced["fixture"], misplaced["decisionOpid"], misplaced["acceptedDigest"] = "invariant", "", "", "digest"
	if _, _, err := LandableRegister([]any{misplaced}); err == nil {
		t.Fatal("an accepted digest on an open finding was read")
	}
}
