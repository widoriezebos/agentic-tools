package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AssertHostAdmissionClean checks an isolated host-admission directory after
// the processes it admitted exited: every heavy lease marker is cleared,
// there are wantLeases of them (zero means at least one), and no managed
// proof process marker remains but the one allowed stale pid.
func AssertHostAdmissionClean(t testing.TB, directory string, wantLeases int, allowedStaleManagedPID ...int) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read isolated host admission: %v", err)
	}
	leases := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "managed-") {
			if len(allowedStaleManagedPID) != 1 || entry.Name() != fmt.Sprintf("managed-%d.json", allowedStaleManagedPID[0]) {
				t.Fatalf("unexpected managed proof process marker remained after exit: %s", entry.Name())
			}
			continue
		}
		if !strings.HasPrefix(entry.Name(), "lease-heavy-") {
			continue
		}
		leases++
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var marker struct {
			Cleared bool `json:"cleared"`
		}
		if json.Unmarshal(data, &marker) != nil || !marker.Cleared {
			t.Fatalf("resource custody remained dirty after exit: %s: %s", entry.Name(), data)
		}
	}
	if wantLeases > 0 && leases != wantLeases || wantLeases == 0 && leases == 0 {
		t.Fatalf("host resource leases = %d, want %d", leases, wantLeases)
	}
}
