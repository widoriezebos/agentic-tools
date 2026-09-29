package steward

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// Round B2-4, F-2: an armed checkout missing from disk (an unmounted
// volume) is unreadable to the union, never silently left out; only the
// armed registry decides that a checkout is gone.
func TestAnArmedCheckoutMissingFromDiskIsUnreadableToTheUnion(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	missing := filepath.Join(bed.root, "Volumes", "Unmounted", "clone", "metasystem")
	pass := bed.pass(diskstore.ModeReport, []string{bed.inst, missing}, false)
	pass.UserHome = filepath.Join(bed.root, "user")
	env, err := EvidenceEnv(context.Background(), bed.inst, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, unreadable := range env.Exclusions(false).Unreadable {
		if unreadable == missing {
			return
		}
	}
	t.Fatalf("the missing armed checkout %s is unreadable to the union: %+v", missing, env.Exclusions(false).Unreadable)
}
