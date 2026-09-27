package authority

import "testing"

// dispatch-fixtures.sh 3672-3697 (U6b port): a delegate-shaped caller of the
// internal critique mutations (register advance, read admission) is refused
// the holder-only write with the lease-holder diagnostic, for the named
// chain root as for none.
func TestDelegateCallerIsRefusedHolderOnlyCritiqueMutations(t *testing.T) {
	t.Parallel()
	for _, job := range []string{"flag-runtime", ""} {
		err := Authorize("holder-only", cls("DELEGATE", false, ""), job)
		if err == nil || err.Error() != "control-plane write requires the authenticated lease holder" {
			t.Fatalf("delegate holder-only write for %q: %v", job, err)
		}
	}
}
