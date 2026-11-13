package launch

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"testing"
)

func TestCheckEnvironmentDigest(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		env  []string
		base string
		want string
	}{
		{"sorted", []string{"Z=last", "A=first"}, "parent", "A=first\x00Z=last\x00LANDING_ONLY=\x00LANDING_PROOF_BASE=parent"},
		{"runtime inputs replaced", []string{"LANDING_ONLY=replay", "Z=last", "METASYSTEM_CHECK_SELECTION=path", CheckEnvironmentVariable + "=digest", "LANDING_PROOF_BASE=old", "A=first"}, "parent", "A=first\x00Z=last\x00LANDING_ONLY=\x00LANDING_PROOF_BASE=parent"},
		{"empty base", []string{"A=first"}, "", "A=first\x00LANDING_ONLY=\x00LANDING_PROOF_BASE="},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			frozen := slices.Clone(row.env)
			want := fmt.Sprintf("%x", sha256.Sum256([]byte(row.want)))
			if got := CheckEnvironmentDigest(row.env, row.base); got != want {
				t.Fatalf("digest=%s want=%s", got, want)
			}
			if !slices.Equal(row.env, frozen) {
				t.Fatalf("digest changed frozen environment: %v", row.env)
			}
		})
	}
}
