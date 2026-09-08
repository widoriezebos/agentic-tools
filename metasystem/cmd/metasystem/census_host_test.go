package main

import "testing"

func TestProcFindAncestorAllHostsIsExplicitAndMutuallyExclusive(t *testing.T) {
	if code := runCensusFindAncestor([]string{"--repo", t.TempDir(), "--pid", "1", "--all-hosts", "--runtime", "devin"}); code != 2 {
		t.Fatalf("combined selectors exit = %d, want 2", code)
	}
}
