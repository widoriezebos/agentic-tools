package enginebuild

import (
	"strings"
	"testing"
)

func TestStampCommitAcceptsOnlyClosedSourceLinkedForms(t *testing.T) {
	commit := strings.Repeat("a", 40)
	tests := []struct {
		name       string
		stamp      string
		wantCommit string
		wantDirty  bool
		wantOK     bool
	}{
		{name: "forty character commit", stamp: commit, wantCommit: commit, wantOK: true},
		{name: "development dirty wrapper", stamp: "dev-" + commit + "-dirty", wantCommit: commit, wantDirty: true, wantOK: true},
		{name: "uppercase hexadecimal", stamp: strings.Repeat("A", 40)},
		{name: "one character short", stamp: strings.Repeat("a", 39)},
		{name: "one character long", stamp: strings.Repeat("a", 41)},
		{name: "development dirty wraps non-commit", stamp: "dev-not-a-commit-dirty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commit, dirty, ok := StampCommit(test.stamp)
			if commit != test.wantCommit || dirty != test.wantDirty || ok != test.wantOK {
				t.Fatalf("StampCommit(%q) = (%q, %v, %v), want (%q, %v, %v)", test.stamp, commit, dirty, ok, test.wantCommit, test.wantDirty, test.wantOK)
			}
		})
	}
}
