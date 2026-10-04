package steward

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Health names the state root and the installation apart; the context role
// finds its holder and registers the holder's session under the installation,
// and leaves the state root without context run state.
func TestHealthContextRoleSeparatedRootsReadsTheInstallation(t *testing.T) {
	t.Parallel()
	state, installation := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeContextHolder(t, installation, "devin", "session")
	probe := healthProbe{123: {exact: identity.Exact{Pid: 123, StartedAt: time.Unix(456, 0)}, state: identity.Alive}}
	roles, _ := evaluateHealthRoles(state, installation, time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), probe, true)
	var role RoleVerdict
	for _, candidate := range roles {
		if candidate.Role == RoleContext {
			role = candidate
		}
	}
	if role.Status != HealthAlive || role.Reason != "unknown (per-invocation usage only)" {
		t.Fatalf("context role over separated roots = %+v", role)
	}
	if _, err := os.Stat(filepath.Join(installation, "artifacts", "agents", "context", "sessions.jsonl")); err != nil {
		t.Fatalf("the holder's session is not registered under the installation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "artifacts", "agents", "context")); !os.IsNotExist(err) {
		t.Fatalf("the state root gained context run state: %v", err)
	}
}
