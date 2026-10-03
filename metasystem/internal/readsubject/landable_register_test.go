package readsubject

import "testing"

// TestLandableRegister: a register whose entries are withdrawn, ruled
// out-of-scope (bounded) or accepted as a risk by a person's recorded act
// yields the landing's read and names its accepted risks; it is still not a
// clean read. Open, disputed, deferred, refuted, accepted (fix required),
// out-of-scope severe or unproven, and an accepted risk with no recorded act
// are not landable.
func TestLandableRegister(t *testing.T) {
	entry := func(id, status, resolution, class, opid string) map[string]any {
		value := modernRegisterEntry(status, resolution)
		value["findingId"], value["rigorClass"], value["decisionOpid"] = id, class, opid
		return value
	}
	risk := entry("F-2", "accepted-risk", "accepted-risk", "unproven", "op-2")
	landable, risks, err := LandableRegister([]any{entry("F-0", "resolved", "withdrawn", "bounded", ""), entry("F-1", "resolved", "out-of-scope", "bounded", ""), risk})
	if err != nil || !landable || len(risks) != 1 || risks[0] != (AcceptedRisk{FindingID: "F-2", DecisionOpID: "op-2"}) {
		t.Fatalf("withdrawn, out-of-scope and a person's risk = %v %v %v", landable, risks, err)
	}
	if clean, err := CleanRegister([]any{risk}); err != nil || clean {
		t.Fatalf("an accepted risk became a clean read: %v %v", clean, err)
	}
	if landable, risks, err := LandableRegister([]any{}); err != nil || !landable || risks != nil {
		t.Fatalf("an empty register = %v %v %v", landable, risks, err)
	}
	for name, value := range map[string]map[string]any{
		"open":                  entry("F", "open", "", "bounded", ""),
		"disputed":              entry("F", "disputed", "", "bounded", ""),
		"deferred":              entry("F", "deferred", "deferred", "bounded", "op"),
		"refuted":               entry("F", "resolved", "refuted", "bounded", ""),
		"accepted":              entry("F", "resolved", "accepted", "bounded", ""),
		"out-of-scope severe":   entry("F", "resolved", "out-of-scope", "severe", ""),
		"out-of-scope unproven": entry("F", "resolved", "out-of-scope", "unproven", ""),
		"risk without an act":   entry("F", "accepted-risk", "accepted-risk", "severe", ""),
	} {
		if landable, risks, err := LandableRegister([]any{risk, value}); err != nil || landable || risks != nil {
			t.Fatalf("%s beside an accepted risk = %v %v %v", name, landable, risks, err)
		}
	}
}
