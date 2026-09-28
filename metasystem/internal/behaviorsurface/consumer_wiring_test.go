package behaviorsurface

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The landing consumer's own wiring (the commit boundary's proof scope,
// unbound bytes, --no-renames removals and critical symlinks) is tested
// where it lives, internal/landing/landpath; the retired contract-off
// branch's proof-built policy engine has no successor.

func TestEveryEffectiveDeliverySkipIsPolicyOwned(t *testing.T) {
	policy := mustPolicy(t)
	const witnessEngineGate = "witness-engine-gate"
	if !policy.SkipAllowed(WitnessScope, witnessEngineGate) {
		t.Fatalf("the witnessed engine-gate omission is absent from policy: %q", witnessEngineGate)
	}
	goGate, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "go-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	consult := `--scope WITNESS --family witness-engine-gate`
	canonicalInvocation := `go run -p="$gate_workers" ./cmd/metasystem behavior-surface skip-allowed`
	if !strings.Contains(string(goGate), canonicalInvocation) || !strings.Contains(string(goGate), consult) {
		t.Fatalf("the witness fast path does not consult its declared skip family: %q", witnessEngineGate)
	}
	for _, family := range policy.DeliveryContractSkips {
		if !policy.SkipAllowed(DeliveryScope, family) {
			t.Errorf("declared family was not accepted: %q", family)
		}
	}
	if policy.SkipAllowed(DeliveryScope, "not-declared") {
		t.Fatal("undeclared validation family was accepted")
	}
}
