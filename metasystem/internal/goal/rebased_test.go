package goal

import (
	"slices"
	"strings"
	"testing"
)

func TestRecordRebaseWritesOneHistoryLineAndReplays(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, tiered("under-review", 1))
	r := verbReqFor(endpoint, reviewUlid(9, 4), "mac-a")
	r.Now = reviewNow
	reason := "rebased 111111111111 onto main 222222222222; reviews carried: unit-a; needs review: unit-b"
	result, err := RecordRebase(r, "under-review", reason)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("record rebase: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["under-review"]
	line := f.History[len(f.History)-1]
	if line.Verb != "rebase" || !slices.Equal(line.Targets, []string{f.Id}) || line.Reason != reason || line.Actor != "mac-a+lin-1" {
		t.Fatalf("history: %+v", line)
	}
	if _, problems := ParseFile(RenderFile(f)); len(problems) != 0 {
		t.Fatalf("history does not read back: %v", problems)
	}
	replay, err := RecordRebase(r, f.Id, reason)
	if err != nil || replay.Tip != result.Tip {
		t.Fatalf("replay wrote again: %+v %v", replay, err)
	}
}

func TestRecordRebaseRefusesAnotherSeatAndInvalidReason(t *testing.T) {
	t.Parallel()
	endpoint := reviewBed(t, tiered("under-review", 1))
	r := verbReqFor(endpoint, reviewUlid(9, 5), "mac-b")
	r.Now = reviewNow
	result, err := RecordRebase(r, "under-review", "on main 222222222222")
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not this seat's claim") {
		t.Fatalf("another seat: %+v %v", result, err)
	}
	for _, reason := range []string{"", "  ", "two\nlines", "two\rlines", "on main\n"} {
		if _, err := RecordRebase(r, "under-review", reason); err == nil {
			t.Fatalf("accepted reason %q", reason)
		}
	}
	r.Actor.Human = "Wido"
	if _, err := RecordRebase(r, "under-review", "on main 222222222222"); err == nil {
		t.Fatal("a human wrote the holder's rebase line")
	}
}
