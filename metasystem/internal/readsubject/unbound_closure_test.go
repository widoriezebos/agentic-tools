package readsubject

import (
	"encoding/json"
	"testing"
)

// TestClosureGateTakesAnAcceptedUnboundReturn: a closed round whose return
// names other work than its persisted subject is read only when the closure's
// accepted risks name that round's unbound-return finding; any other accepted
// finding, or another round's unbound-return finding, leaves it refused.
func TestClosureGateTakesAnAcceptedUnboundReturn(t *testing.T) {
	t.Parallel()
	unbound := func(t *testing.T, findingID string) *closureFixture {
		f := newClosureFixture(t)
		f.writeRound(1, "critic", f.subject, map[string]any{"jobId": "critic", "round": json.Number("1"), "reviewedTree": "tree-b"})
		entry := modernRegisterEntry("accepted-risk", "accepted-risk")
		entry["findingId"], entry["rigorClass"], entry["grain"], entry["fixture"] = findingID, "unproven", "invariant", ""
		entry["acceptedDigest"] = AcceptedFindingDigest(entry)
		f.root["findingRegister"] = []any{entry}
		f.root[closureField].(map[string]any)["acceptedRisks"] = []any{map[string]any{
			"findingId": findingID, "decisionOpid": "decision", "acceptedDigest": entry["acceptedDigest"],
		}}
		return f
	}

	// The identifier is the one registers already hold.
	if id := UnboundReturnFindingID("code-critic", "critic"); id != "synthetic-c60b2a2b8f381122f5da5367d3535dfa5438913e963365199110783fb77d73aa" {
		t.Fatalf("unbound-return finding identifier = %s", id)
	}

	t.Run("accepted", func(t *testing.T) {
		t.Parallel()
		id := UnboundReturnFindingID("code-critic", "critic")
		closure := unbound(t, id).requireValid()
		if len(closure.AcceptedRisks) != 1 || closure.AcceptedRisks[0].FindingID != id {
			t.Fatalf("unbound closure accepted risks = %v", closure.AcceptedRisks)
		}
	})
	for name, id := range map[string]string{
		"other finding": "F-1",
		"other round's": UnboundReturnFindingID("code-critic", "earlier"),
		"other role's":  UnboundReturnFindingID("warden", "critic"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			unbound(t, id).requireRefused()
		})
	}
}
