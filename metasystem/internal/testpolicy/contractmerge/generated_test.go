package contractmerge

import (
	"errors"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestMergeGeneratedBySortedPathsIdentity(t *testing.T) {
	t.Parallel()
	base := mergeFixture()
	base.Generated = []testpolicy.Generated{{Paths: []string{"out/a", "out/b"}, Command: []string{"build", "base"}}}
	for _, side := range []string{"ours", "theirs"} {
		t.Run(side, func(t *testing.T) {
			t.Parallel()
			ours, theirs := cloneFixture(t, base), cloneFixture(t, base)
			changed, unchanged := &ours, &theirs
			if side == "theirs" {
				changed, unchanged = &theirs, &ours
			}
			changed.Generated[0].Command = []string{"build", "changed", "ordered"}
			changed.Generated[0].Then = []string{"finish", "ordered"}
			changed.Generated[0].Cwd = "src"
			unchanged.Generated[0].Paths = []string{"out/b", "out/a"}
			merged, err := Merge(base, ours, theirs)
			if err != nil || !reflect.DeepEqual(merged.Generated, changed.Generated) {
				t.Fatalf("one changed recipe: generated=%v err=%v", merged.Generated, err)
			}
		})
	}
}

func TestMergeGeneratedRefusesConcurrentRecipeChanges(t *testing.T) {
	t.Parallel()
	base := mergeFixture()
	base.Generated = []testpolicy.Generated{{Paths: []string{"out/a", "out/b"}, Command: []string{"build", "base"}}}
	for _, test := range []string{"same field", "independent fields", "delete change", "new identity"} {
		t.Run(test, func(t *testing.T) {
			t.Parallel()
			b := cloneFixture(t, base)
			ours, theirs := cloneFixture(t, b), cloneFixture(t, b)
			ours.Generated[0].Command = []string{"build", "ours"}
			theirs.Generated[0].Paths = []string{"out/b", "out/a"}
			switch test {
			case "same field", "new identity":
				theirs.Generated[0].Command = []string{"build", "theirs"}
			case "independent fields":
				theirs.Generated[0].Cwd = "src"
			case "delete change":
				theirs.Generated = nil
			}
			if test == "new identity" {
				b.Generated = nil
			}
			_, err := Merge(b, ours, theirs)
			var detail *Refusal
			if !errors.As(err, &detail) || detail.Field != "recipe" {
				t.Fatalf("concurrent recipe changes: err=%v; want recipe conflict", err)
			}
		})
	}
}

func TestMergeGeneratedKeepsIndependentSetsAndRemovals(t *testing.T) {
	t.Parallel()
	base := mergeFixture()
	base.Generated = []testpolicy.Generated{{Paths: []string{"old/**"}, Command: []string{"old"}}}
	ours, theirs := cloneFixture(t, base), cloneFixture(t, base)
	ours.Generated = []testpolicy.Generated{{Paths: []string{"ours/**"}, Command: []string{"ours"}}}
	theirs.Generated = append(theirs.Generated, testpolicy.Generated{Paths: []string{"theirs/**"}, Command: []string{"theirs"}})
	merged, err := Merge(base, ours, theirs)
	want := []testpolicy.Generated{ours.Generated[0], theirs.Generated[1]}
	if err != nil || !reflect.DeepEqual(merged.Generated, want) {
		t.Fatalf("independent sets and removal: generated=%v err=%v; want %v", merged.Generated, err, want)
	}
}
