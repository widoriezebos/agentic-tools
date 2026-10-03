package dispatch

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestBriefDraftsHeldFreezesTheSeatsUntrackedFile: a brief cites a file the
// seat wrote and never committed, by its path from the installation. The
// goal worktree's tree lacks it; the seat's checkout holds it. The draft is
// found there, frozen into the dispatcher's checkout, and admitted; a cited
// path no checkout holds is not frozen and still refuses.
func TestBriefDraftsHeldFreezesTheSeatsUntrackedFile(t *testing.T) {
	t.Parallel()
	seat := newNestedSeat(t)
	writeSeatFile(t, filepath.Join(seat.install, "plans", "trace.md"), "the seat's trace\n")
	text := "Working Mode: implement\n\nFact trace: `plans/trace.md` in the seat's checkout.\nAlso `plans/absent.md`.\n"
	drafts, err := BriefDraftsHeld([]byte(text), seat.worktreeInstall, seat.install, seat.worktreeInstall)
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 1 || drafts[0].Path != "plans/trace.md" || string(drafts[0].Content) != "the seat's trace\n" {
		t.Fatalf("drafts = %+v, want the seat's plans/trace.md only", drafts)
	}
	copyPath, digest := writeFrozenCopy(t, seat.worktreeInstall, "artifacts/agents/frozen/trace.md", string(drafts[0].Content))
	err = seat.admitAtGoalWorktree(t, text+"\n"+FrozenInputLine("plans/trace.md", digest, copyPath)+"\n")
	wantMissingPaths(t, "a draft no checkout holds", err, "plans/absent.md")
	if err := seat.admitAtGoalWorktree(t, strings.Replace(text, "Also `plans/absent.md`.\n", "", 1)+"\n"+FrozenInputLine("plans/trace.md", digest, copyPath)+"\n"); err != nil {
		t.Fatalf("the frozen seat file was refused: %v", err)
	}
}
