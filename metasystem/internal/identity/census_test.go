package identity

import (
	"os"
	"slices"
	"testing"
)

func TestProcessCensusMatchesKernelParents(t *testing.T) {
	t.Parallel()

	census, err := TakeProcessCensus()
	if err != nil {
		t.Fatal(err)
	}
	self := int64(os.Getpid())
	if !slices.Contains(census.Pids(), self) {
		t.Fatalf("process census does not contain this process %d", self)
	}
	parent, known := census.Parent(self)
	if want := int64(os.Getppid()); !known || parent != want {
		t.Fatalf("process census parent of this process = (%d, %v), want (%d, true)", parent, known, want)
	}
	if directParent, directKnown := ParentPid(self); parent != directParent || known != directKnown {
		t.Fatalf("process census parent of this process = (%d, %v), ParentPid = (%d, %v)", parent, known, directParent, directKnown)
	}
	rootParent, rootKnown := census.Parent(1)
	if directParent, directKnown := ParentPid(1); rootParent != directParent || rootKnown != directKnown {
		t.Fatalf("process census parent of pid 1 = (%d, %v), ParentPid = (%d, %v)", rootParent, rootKnown, directParent, directKnown)
	}
}
