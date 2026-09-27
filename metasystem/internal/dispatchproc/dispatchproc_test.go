package dispatchproc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// TestClaimAuthorizationSpendsTheDelegateCapabilityNotTheMarker proves the
// internal marker carries no authority by itself: the claim needs the bearer
// word the delegate boundary minted, a preflight validates without spending
// it, and the authoritative claim spends it exactly once.
func TestClaimAuthorizationSpendsTheDelegateCapabilityNotTheMarker(t *testing.T) {
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", "")
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	capability, err := dispatch.MintDelegateClaimCapability(root, dispatch.DispatchModeFresh)
	if err != nil {
		t.Fatal(err)
	}
	binding := dispatch.DelegateClaimCapabilityBinding{JobID: "j1", DispatchMode: dispatch.DispatchModeFresh, AdapterVerb: "dispatch"}
	if ClaimAuthorized(root, ClaimSurface{Capability: capability}, binding, true) {
		t.Fatal("a capability outside the delegate boundary authorized a claim")
	}
	if ClaimAuthorized(root, ClaimSurface{DelegateInternal: true}, binding, true) {
		t.Fatal("the internal marker alone authorized a claim")
	}
	internal := ClaimSurface{DelegateInternal: true, Capability: capability}
	for range 2 {
		if !ClaimAuthorized(root, internal, binding, true) {
			t.Fatal("the preflight refused a fresh capability (or a preflight spent it)")
		}
	}
	if !ClaimAuthorized(root, internal, binding, false) {
		t.Fatal("the authoritative claim refused a fresh capability")
	}
	if ClaimAuthorized(root, internal, binding, false) {
		t.Fatal("a spent capability authorized a second claim")
	}
}
