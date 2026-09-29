package enginebuild

import (
	"strings"
	"testing"
)

func TestDevelopmentStampNamesTheCommitAsDirty(t *testing.T) {
	t.Parallel()
	commit := strings.Repeat("b", 40)
	if got, want := DevelopmentStamp(commit), "dev-"+commit+"-dirty"; got != want {
		t.Fatalf("DevelopmentStamp = %q, want %q", got, want)
	}
}
