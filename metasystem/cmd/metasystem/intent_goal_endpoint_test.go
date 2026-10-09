package main

import (
	"slices"
	"testing"
)

func TestGoalListMissingEndpointOffersInstallationCheck(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	owners := bed.owners()
	owners.dependencies.endpoint = nil
	code, result := bed.runJSON(owners, "goal", "list")
	if code != 1 || result.Outcome != intentFailed || result.Summary != "the goal list is unavailable: no endpoint reader is configured" || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "system", "check"}) {
		t.Fatalf("missing endpoint: exit %d %+v", code, result)
	}
	if bed.publications() != 0 {
		t.Fatal("an unavailable goal list changed the ledger")
	}
}
