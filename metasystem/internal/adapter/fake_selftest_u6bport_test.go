package adapter

import (
	"path/filepath"
	"testing"
)

// Port of dispatch-fixtures.sh adapter-selftest (lines 5696-5703): the fake
// selftest's pass record claims resume identity and both denied-envelope
// probes as behaviorally proven, and never lists network as merely
// constructed.
func TestU6bPortFakeSelftestRecordProvesResumeAndTheDeniedProbes(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "fake-selftest.json")
	if err := WriteFakeSelftestRecord(path, "fake-selftest-1"); err != nil {
		t.Fatal(err)
	}
	record := readJSONFile(t, path)
	proven := map[string]bool{}
	for _, value := range record["provenBehaviorally"].([]any) {
		proven[value.(string)] = true
	}
	for _, want := range []string{"resume-identity", "denied-write", "denied-network"} {
		if !proven[want] {
			t.Fatalf("provenBehaviorally lacks %q: %v", want, record["provenBehaviorally"])
		}
	}
	for _, value := range record["constructedOnly"].([]any) {
		if value == "network" {
			t.Fatalf("network stayed constructed-only: %v", record["constructedOnly"])
		}
	}
}
