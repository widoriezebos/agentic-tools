package dispatch

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func goalFreeCriticReads(t *testing.T) goalAdmissionReads {
	t.Helper()
	return goalAdmissionReads{ResolveEndpoint: func(root string) (goal.Endpoint, error) {
		t.Fatalf("goal-free critic read a goal endpoint at %s", root)
		return goal.Endpoint{}, nil
	}}
}

func buildCriticLimitWithReads(t *testing.T, p BuildRecordParams, reads goalAdmissionReads) int64 {
	t.Helper()
	p.Workspace = t.TempDir()
	designPath := ""
	if p.Role == "design-critic" {
		designPath = p.Design
	}
	if err := buildRecordWithReads(p, recordFacts(t, p.Workspace, 1, designPath), reads); err != nil {
		t.Fatal(err)
	}
	limit, _ := numInt(readJSONFile(t, p.Output)[reviewRoundLimitField])
	return limit
}
