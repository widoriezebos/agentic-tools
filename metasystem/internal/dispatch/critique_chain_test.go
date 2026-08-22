package dispatch

import (
	"encoding/json"
	"testing"
)

// The critique chain helpers, driven directly: the lineage walk's
// verdicts and the highest-round member selection.

func chainState(records map[string]map[string]any) critiqueState {
	return critiqueState{agents: "unused", records: records}
}

func TestChainRootVerdicts(t *testing.T) {
	state := chainState(map[string]map[string]any{
		"root-a":  {"jobId": "root-a"},
		"child-b": {"jobId": "child-b", "parentJob": "root-a"},
		"grand-c": {"jobId": "grand-c", "parentJob": "child-b"},
		"cyc-x":   {"jobId": "cyc-x", "parentJob": "cyc-y"},
		"cyc-y":   {"jobId": "cyc-y", "parentJob": "cyc-x"},
		"dangler": {"jobId": "dangler", "parentJob": "not-loaded"},
		"badtype": {"jobId": "badtype", "parentJob": 42},
	})
	if got := state.chainRoot("grand-c"); got != "root-a" {
		t.Fatalf("three-deep walk: %q", got)
	}
	if got := state.chainRoot("root-a"); got != "root-a" {
		t.Fatalf("a root is its own root: %q", got)
	}
	if got := state.chainRoot("cyc-x"); got != "" {
		t.Fatalf("a cycle must resolve to nothing: %q", got)
	}
	if got := state.chainRoot("dangler"); got != "" {
		t.Fatalf("a walk leaving the table must resolve to nothing: %q", got)
	}
	if got := state.chainRoot("badtype"); got != "" {
		t.Fatalf("a malformed parent must resolve to nothing: %q", got)
	}
	if got := state.chainRoot("absent"); got != "" {
		t.Fatalf("an unloaded job must resolve to nothing: %q", got)
	}
}

func TestLatestMemberSelection(t *testing.T) {
	state := chainState(map[string]map[string]any{
		"root-a":  {"jobId": "root-a", "round": json.Number("1")},
		"child-b": {"jobId": "child-b", "parentJob": "root-a", "round": json.Number("3")},
		"child-c": {"jobId": "child-c", "parentJob": "root-a", "round": json.Number("2")},
		"noround": {"jobId": "noround", "parentJob": "root-a"},
		"other":   {"jobId": "other", "round": json.Number("9")},
	})
	best := state.latestMember("root-a")
	if best == nil || best["jobId"] != "child-b" {
		t.Fatalf("highest round not selected: %v", best)
	}
	// A chain whose members all lack integer rounds has no latest.
	empty := chainState(map[string]map[string]any{
		"r": {"jobId": "r"},
	})
	if got := empty.latestMember("r"); got != nil {
		t.Fatalf("roundless chain returned a member: %v", got)
	}
	// A chain that does not exist has no latest.
	if got := state.latestMember("nope"); got != nil {
		t.Fatalf("absent chain returned a member: %v", got)
	}
}
