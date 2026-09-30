package main

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
)

// The production cadence re-arms a lane engine behind the fetched trunk.
func TestProductionCadenceOwnerRearms(t *testing.T) {
	t.Parallel()
	if owner := productionCadenceOwner(batchowner.BatchOwnerLease{}); owner.Rearm == nil || owner.Prepare == nil {
		t.Fatal("the production cadence owner has no re-arm")
	}
}
