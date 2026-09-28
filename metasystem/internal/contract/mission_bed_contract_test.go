package contract

import (
	"os"
	"strings"
	"testing"
)

// Ported from scripts/agents/mission-fixtures.sh (contract-and-state): a
// dispatch-allow envelope survives sealing byte-exactly, and the sealed
// contract still validates. The repository facts come from the Git-free
// contract source.
func TestMissionBedDispatchAllowSurvivesSealAndStillValidates(t *testing.T) {
	t.Parallel()
	f := newContractSource(t, 2, false)
	const line = "envelope.dispatch-allow=fake:fake-model,codex:gpt-5.6-sol"
	authored, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(authored), "envelope.dependencies=jq") {
		t.Fatalf("fixture contract has no dependencies envelope to replace:\n%s", authored)
	}
	writeFileMode(t, f.path, strings.Replace(string(authored), "envelope.dependencies=jq", line, 1), 0o644)
	if _, err := f.seal(false); err != nil {
		t.Fatalf("seal a dispatch-allow contract: %v", err)
	}
	sealed, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sealed), "\n"+line+"\n") || !strings.Contains(string(sealed), "```mission-seal") {
		t.Fatalf("sealing altered the dispatch-allow envelope line:\n%s", sealed)
	}
	if _, _, err := contractValidateWithRepository(f.path, f.repository); err != nil {
		t.Fatalf("the sealed dispatch-allow contract no longer validates: %v", err)
	}
}
