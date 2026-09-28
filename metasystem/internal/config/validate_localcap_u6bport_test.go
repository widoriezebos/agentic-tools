package config

import (
	"strings"
	"testing"
)

// delegate-caps AUTH-R2-008 (U6b port): the local override layer is attacked
// with the exact noncanonical cap key. A committed configuration that is
// itself valid still refuses when metasystem.conf.local carries
// cap.min.devin.swe-1.7, naming the local source and the canonical key.
func TestValidateRefusesANoncanonicalCapKeyInTheLocalOverride(t *testing.T) {
	t.Parallel()
	committed := "metasystem.runtimes=fake,devin\nevidence.root=@EVIDENCE@\nrole.default.runtime=fake\nrole.default.model.fake=fake-model\n"
	if problems := validateRepo(t, committed); hasProblem(problems, "cap key") {
		t.Fatalf("the committed layer alone already refuses a cap key: %v", problems)
	}
	problems := validateRepo(t, committed, "cap.min.devin.swe-1.7=250\n")
	want := "non-canonical cap key cap.min.devin.swe-1.7; use cap.min.devin.swe-1-7"
	var found string
	for _, problem := range problems {
		if strings.Contains(problem, want) {
			found = problem
		}
	}
	if found == "" {
		t.Fatalf("the local noncanonical cap key was accepted: %v", problems)
	}
	if !strings.Contains(found, "metasystem.conf.local:1:") {
		t.Fatalf("the refusal does not name the local source line: %q", found)
	}
}
