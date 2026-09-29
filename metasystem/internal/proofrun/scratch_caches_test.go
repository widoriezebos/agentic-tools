package proofrun

import (
	"testing"
)

// A v2 run records its resolved cache pair in the scratch record, where a
// proof child whose locator names the run's attempt reads it (disk-lifetimes
// A8, rule 3) instead of GOCACHE.
func TestScratchRecordCarriesTheRunsCachePair(t *testing.T) {
	t.Parallel()
	fixture := newScratchEnvFixture(t)
	request, run := prepareScratchEnvV2(t, fixture.control, withCaches(scratchEnvRequest(fixture.base, scratchEnvGroups()), "/machine/go-build", "/machine/staticcheck"))
	record, err := readScratchRecordFile(run.recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if record.GoCache != "/machine/go-build" || record.StaticcheckCache != "/machine/staticcheck" || request.ScratchEnvironment.GoCache != record.GoCache {
		t.Fatalf("scratch record caches = %q %q", record.GoCache, record.StaticcheckCache)
	}
}
