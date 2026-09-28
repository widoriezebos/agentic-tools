package delegation_test

import (
	"strings"
	"testing"
)

// The held reap entry is brain-fenced like the public reap: the retired
// internal_reap_held reached reap_jobs, whose first act is the brain fence,
// so `__reap-held` on a fenced checkout records BRAIN_REFUSED and exits 2.
func TestReapHeldCallbackIsBrainFenced(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.corruptBrain()
	result := b.run("__reap-held")
	if result.ExitCode != 2 || !strings.Contains(string(result.Stdout), `"outcome":"BRAIN_REFUSED"`) ||
		!strings.Contains(string(result.Outcome), `"outcome":"BRAIN_REFUSED"`) {
		t.Fatalf("__reap-held on a fenced checkout: exit %d stdout %q outcome %q stderr %q; dispatch.sh refused it with BRAIN_REFUSED exit 2",
			result.ExitCode, result.Stdout, result.Outcome, b.stderr.String())
	}
}
