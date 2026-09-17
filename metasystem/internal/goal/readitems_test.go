package goal

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func readItemRequest(root string, sequence int) VerbRequest {
	return verbReq(root, fmt.Sprintf("01J5X%021d", sequence), "mac-a")
}

func readItemBed(t *testing.T, ids ...string) string {
	t.Helper()
	_, root := oneClone(t)
	seedLedger(t, root)
	for index, id := range ids {
		request := readItemRequest(root, index+1)
		if result, err := Open(request, id, "Track independent read findings.", OriginMain, "Land the build."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	return root
}

func TestReadItemsAddIsIdempotent(t *testing.T) {
	root := readItemBed(t, "source")
	if result, err := AddReadItems(readItemRequest(root, 10), "source", "read-a", []string{"Polish the error."}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first add: %+v %v", result, err)
	}
	result, err := AddReadItems(readItemRequest(root, 11), "source", "read-a", []string{"Polish the error."})
	if err != nil || result.Outcome != OutcomeAbandoned || result.Detail != "all read items already tracked" {
		t.Fatalf("duplicate add: %+v %v", result, err)
	}
	tree, _ := acceptedTree(t, root, readItemRequest(root, 12).Now)
	if got := tree.Live["source"].ReadItems; len(got) != 1 || got[0].ID != "read-a-1" {
		t.Fatalf("duplicate changed items: %+v", got)
	}
}

func TestReadItemIDsStayStableAcrossAddCalls(t *testing.T) {
	root := readItemBed(t, "source")
	first, err := AddReadItems(readItemRequest(root, 20), "source", "review", []string{"First.", "Second."})
	if err != nil || first.Outcome != OutcomeConfirmed {
		t.Fatalf("first add: %+v %v", first, err)
	}
	second, err := AddReadItems(readItemRequest(root, 21), "source", "review", []string{"First.", "Third."})
	if err != nil || second.Outcome != OutcomeConfirmed {
		t.Fatalf("second add: %+v %v", second, err)
	}
	tree, _ := acceptedTree(t, root, readItemRequest(root, 22).Now)
	items := tree.Live["source"].ReadItems
	if len(items) != 3 || items[0].ID != "review-1" || items[1].ID != "review-2" || items[2].ID != "review-3" {
		t.Fatalf("ids changed across calls: %+v", items)
	}
}

func TestReadItemsCloseNeedsExactlyOneWay(t *testing.T) {
	root := readItemBed(t, "source")
	commit, moved, accepted := "HEAD", "target", "not a defect"
	for _, closure := range []ReadItemClosure{{}, {Fixed: &commit, Moved: &moved}, {Fixed: &commit, Accepted: &accepted}} {
		if _, err := CloseReadItem(readItemRequest(root, 30), "source", "read-1", closure); err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Fatalf("closure %+v did not refuse exactly: %v", closure, err)
		}
	}
}

func TestReadItemsFixedRefusesUnknownCommit(t *testing.T) {
	root := readItemBed(t, "source")
	unknown := strings.Repeat("f", 40)
	if _, err := CloseReadItem(readItemRequest(root, 40), "source", "read-1", ReadItemClosure{Fixed: &unknown}); err == nil || !strings.Contains(err.Error(), "does not resolve to a commit") {
		t.Fatalf("unknown fixed commit = %v", err)
	}
}

func TestReadItemsMoveIsAtomicAndRefusesDoneTarget(t *testing.T) {
	root := readItemBed(t, "source", "target", "done-target")
	added, err := AddReadItems(readItemRequest(root, 50), "source", "critic", []string{"Keep this visible."})
	if err != nil || added.Outcome != OutcomeConfirmed {
		t.Fatalf("add: %+v %v", added, err)
	}
	target := "target"
	moved, err := CloseReadItem(readItemRequest(root, 51), "source", "critic-1", ReadItemClosure{Moved: &target})
	if err != nil || moved.Outcome != OutcomeConfirmed {
		t.Fatalf("move: %+v %v", moved, err)
	}
	tree, err := loadTree(root, moved.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["source"].ReadItems[0].State != ReadItemMoved || tree.Live["source"].ReadItems[0].ClosingReference != "target" ||
		len(tree.Live["target"].ReadItems) != 1 || tree.Live["target"].ReadItems[0].State != ReadItemOpen || !strings.Contains(tree.Live["target"].ReadItems[0].Text, "moved from source") {
		t.Fatalf("atomic move missing one side: source=%+v target=%+v", tree.Live["source"].ReadItems, tree.Live["target"].ReadItems)
	}
	if result, doneErr := Done(readItemRequest(root, 52), "done-target", "No work remains."); doneErr != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("finish target: %+v %v", result, doneErr)
	}
	if result, addErr := AddReadItems(readItemRequest(root, 53), "source", "critic", []string{"A second item."}); addErr != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("second add: %+v %v", result, addErr)
	}
	doneTarget := "done-target"
	result, moveErr := CloseReadItem(readItemRequest(root, 54), "source", "critic-2", ReadItemClosure{Moved: &doneTarget})
	if moveErr != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not an open goal") {
		t.Fatalf("move to done target: %+v %v", result, moveErr)
	}
	tree, _ = acceptedTree(t, root, readItemRequest(root, 55).Now)
	var secondState string
	for _, item := range tree.Live["source"].ReadItems {
		if item.ID == "critic-2" {
			secondState = item.State
		}
	}
	if secondState != ReadItemOpen {
		t.Fatal("refused move partially closed its source")
	}
}

func TestReadItemsAcceptedRefusesBlankReason(t *testing.T) {
	root := readItemBed(t, "source")
	blank := "  "
	if _, err := CloseReadItem(readItemRequest(root, 60), "source", "read-1", ReadItemClosure{Accepted: &blank}); err == nil || !strings.Contains(err.Error(), "non-blank reason") {
		t.Fatalf("blank accepted reason = %v", err)
	}
}

func TestReadItemsCloseRefusesClosedItem(t *testing.T) {
	root := readItemBed(t, "source")
	if result, err := AddReadItems(readItemRequest(root, 61), "source", "critic", []string{"One decision."}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("add: %+v %v", result, err)
	}
	reason := "intentional"
	if result, err := CloseReadItem(readItemRequest(root, 62), "source", "critic-1", ReadItemClosure{Accepted: &reason}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first close: %+v %v", result, err)
	}
	result, err := CloseReadItem(readItemRequest(root, 63), "source", "critic-1", ReadItemClosure{Accepted: &reason})
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "already accepted") {
		t.Fatalf("second close: %+v %v", result, err)
	}
}

func TestDoneRefusesOpenReadItemIDsAndPassesWhenClosed(t *testing.T) {
	root := readItemBed(t, "source")
	added, err := AddReadItems(readItemRequest(root, 70), "source", "read-z", []string{"Explain the fallback.", "Name the invariant."})
	if err != nil || added.Outcome != OutcomeConfirmed {
		t.Fatalf("add: %+v %v", added, err)
	}
	result, doneErr := Done(readItemRequest(root, 71), "source", "Built.")
	var openErr *DoneReadItemsOpenError
	_, tip := acceptedTree(t, root, readItemRequest(root, 72).Now)
	_, typedErr := doneRequest(readItemRequest(root, 73), "source", "Built.").Mutate(tip)
	if doneErr != nil || result.Outcome != OutcomeRejected || !errors.As(typedErr, &openErr) || !strings.Contains(result.Detail, "read-z-1, read-z-2") || !strings.Contains(result.Detail, "--fixed") || !strings.Contains(result.Detail, "--moved") || !strings.Contains(result.Detail, "--accepted") {
		t.Fatalf("open-item done refusal: result=%+v error=%v typed=%T %v", result, doneErr, typedErr, typedErr)
	}
	reason := "the behavior is intentional"
	for index, itemID := range []string{"read-z-1", "read-z-2"} {
		closed, closeErr := CloseReadItem(readItemRequest(root, 74+index), "source", itemID, ReadItemClosure{Accepted: &reason})
		if closeErr != nil || closed.Outcome != OutcomeConfirmed {
			t.Fatalf("close %s: %+v %v", itemID, closed, closeErr)
		}
	}
	result, doneErr = Done(readItemRequest(root, 76), "source", "Built.")
	if doneErr != nil || result.Outcome != OutcomeConfirmed || !strings.Contains(result.Detail, "read-z-1: the behavior is intentional") {
		t.Fatalf("done after closure: %+v %v", result, doneErr)
	}
}

func TestRetroReadItemsListsOpenAndFlagsConcludedDefect(t *testing.T) {
	live := vGoal("live-read", StateQueued)
	live.ReadItems = []ReadItem{{ID: "critic-1", Read: "critic", Text: "Follow up.", State: ReadItemOpen, AddedAt: "2026-09-17T10:00:00Z"}}
	doneBytes := string(RenderFile(vGoal("done-read", StateDone)))
	handWritten := "Open read items (fix unit critic): 1\n- ReadItem: id=critic-1 read=critic state=open addedAt=2026-09-17T10:00:00Z changedAt=- closingReference=\"\" text=\"Escaped conclusion.\"\n"
	doneBytes = strings.Replace(doneBytes, "- OpenedAt:", handWritten+"- OpenedAt:", 1)
	done, problems := ParseFile([]byte(withFreshIntegrity(doneBytes)))
	if len(problems) != 0 {
		t.Fatalf("hand-written concluded fixture did not parse: %v", problems)
	}
	lines := strings.Join(readItemRetroLines(&TreeGoals{Live: map[string]*GoalFile{"live-read": live}, Done: map[string]*GoalFile{"done-read": done}, Abandoned: map[string]*GoalFile{}}), "\n")
	for _, want := range []string{"goal=live-read state=queued open=1", "read=critic id=critic-1 text=\"Follow up.\"", "goal=done-read state=done open=0", "LEDGER DEFECT goal=done-read concluded with open read item"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("retro lines lack %q:\n%s", want, lines)
		}
	}
}
