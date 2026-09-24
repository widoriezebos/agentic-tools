package goal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readItemRequest(endpoint Endpoint, sequence int) VerbRequest {
	return verbReqFor(endpoint, fmt.Sprintf("01J5X%021d", sequence), "mac-a")
}

func readItemBed(t *testing.T, ids ...string) (Endpoint, Endpoint) {
	t.Helper()
	endpoint, peer := fakeGoalEndpointPair(t)
	for index, id := range ids {
		request := readItemRequest(endpoint, index+1)
		if result, err := Open(request, id, "Track independent read findings.", OriginMain, "Land the build."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	return endpoint, peer
}

type codeRefExpectation struct {
	ref string
	raw string
	err error
}

func strictCodeRefResolver(t *testing.T, endpoint Endpoint, expected ...codeRefExpectation) func(string, string) (string, error) {
	t.Helper()
	calls := make([][2]string, 0, len(expected))
	t.Cleanup(func() {
		if len(calls) != len(expected) {
			t.Errorf("code resolver consumed %d of %d expected calls: %v", len(calls), len(expected), calls)
		}
	})
	return func(root, ref string) (string, error) {
		t.Helper()
		if len(calls) >= len(expected) {
			t.Fatalf("unexpected or repeated code resolver call: root=%q ref=%q", root, ref)
		}
		want := expected[len(calls)]
		calls = append(calls, [2]string{root, ref})
		if root != endpoint.Root || ref != want.ref {
			t.Fatalf("code resolver call: root=%q ref=%q, want root=%q ref=%q", root, ref, endpoint.Root, want.ref)
		}
		return want.raw, want.err
	}
}

func TestReadItemsAddIsIdempotent(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "source")
	if result, err := AddReadItems(readItemRequest(endpoint, 10), "source", "read-a", []string{"Polish the error."}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first add: %+v %v", result, err)
	}
	result, err := AddReadItems(readItemRequest(endpoint, 11), "source", "read-a", []string{"Polish the error."})
	if err != nil || result.Outcome != OutcomeAbandoned || result.Detail != "all read items already tracked" {
		t.Fatalf("duplicate add: %+v %v", result, err)
	}
	tree, _ := acceptedTreeForEndpoint(t, endpoint)
	if got := tree.Live["source"].ReadItems; len(got) != 1 || got[0].ID != "read-a-1" {
		t.Fatalf("duplicate changed items: %+v", got)
	}
}

func TestReadItemIDsStayStableAcrossAddCalls(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "source")
	first, err := AddReadItems(readItemRequest(endpoint, 20), "source", "review", []string{"First.", "Second."})
	if err != nil || first.Outcome != OutcomeConfirmed {
		t.Fatalf("first add: %+v %v", first, err)
	}
	second, err := AddReadItems(readItemRequest(endpoint, 21), "source", "review", []string{"First.", "Third."})
	if err != nil || second.Outcome != OutcomeConfirmed {
		t.Fatalf("second add: %+v %v", second, err)
	}
	tree, _ := acceptedTreeForEndpoint(t, endpoint)
	items := tree.Live["source"].ReadItems
	if len(items) != 3 || items[0].ID != "review-1" || items[1].ID != "review-2" || items[2].ID != "review-3" {
		t.Fatalf("ids changed across calls: %+v", items)
	}
}

func TestReadItemsCloseNeedsExactlyOneWay(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "source")
	commit, moved, accepted := "HEAD", "target", "not a defect"
	for _, closure := range []ReadItemClosure{{}, {Fixed: &commit, Moved: &moved}, {Fixed: &commit, Accepted: &accepted}} {
		if _, err := closeReadItem(readItemRequest(endpoint, 30), "source", "read-1", closure, strictCodeRefResolver(t, endpoint)); err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Fatalf("closure %+v did not refuse exactly: %v", closure, err)
		}
	}
}

func TestReadItemsFixedRefusesUnknownCommit(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "source")
	if result, err := AddReadItems(readItemRequest(endpoint, 39), "source", "read", []string{"Fix this."}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("add: %+v %v", result, err)
	}
	unknown := strings.Repeat("f", 40)
	resolver := strictCodeRefResolver(t, endpoint,
		codeRefExpectation{ref: unknown, err: errors.New("unknown")},
		codeRefExpectation{ref: "", raw: "ignored\n"},
		codeRefExpectation{ref: "broken", err: errors.New("cannot resolve")},
		codeRefExpectation{ref: "HEAD", raw: "  " + strings.Repeat("a", 40) + "\n"},
	)
	for _, fixed := range []string{unknown, "  ", " broken "} {
		if _, err := closeReadItem(readItemRequest(endpoint, 40), "source", "read-1", ReadItemClosure{Fixed: &fixed}, resolver); err == nil || !strings.Contains(err.Error(), "does not resolve to a commit") {
			t.Fatalf("fixed %q refusal = %v", fixed, err)
		}
	}
	known := " HEAD "
	result, err := closeReadItem(readItemRequest(endpoint, 41), "source", "read-1", ReadItemClosure{Fixed: &known}, resolver)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("known fixed commit: %+v %v", result, err)
	}
	tree, _ := acceptedTreeForEndpoint(t, endpoint)
	if got := tree.Live["source"].ReadItems[0]; got.State != ReadItemFixed || got.ClosingReference != strings.Repeat("a", 40) {
		t.Fatalf("fixed commit was not normalized: %+v", got)
	}
}

func TestReadItemsMoveIsAtomicAndRefusesDoneTarget(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "source", "target", "done-target")
	added, err := AddReadItems(readItemRequest(endpoint, 50), "source", "critic", []string{"Keep this visible."})
	if err != nil || added.Outcome != OutcomeConfirmed {
		t.Fatalf("add: %+v %v", added, err)
	}
	target := "target"
	moved, err := closeReadItem(readItemRequest(endpoint, 51), "source", "critic-1", ReadItemClosure{Moved: &target}, strictCodeRefResolver(t, endpoint))
	if err != nil || moved.Outcome != OutcomeConfirmed {
		t.Fatalf("move: %+v %v", moved, err)
	}
	tree, err := loadTreeFor(endpoint, moved.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["source"].ReadItems[0].State != ReadItemMoved || tree.Live["source"].ReadItems[0].ClosingReference != "target" ||
		len(tree.Live["target"].ReadItems) != 1 || tree.Live["target"].ReadItems[0].State != ReadItemOpen || !strings.Contains(tree.Live["target"].ReadItems[0].Text, "moved from source") {
		t.Fatalf("atomic move missing one side: source=%+v target=%+v", tree.Live["source"].ReadItems, tree.Live["target"].ReadItems)
	}
	if result, doneErr := Done(readItemRequest(endpoint, 52), "done-target", "No work remains."); doneErr != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("finish target: %+v %v", result, doneErr)
	}
	if result, addErr := AddReadItems(readItemRequest(endpoint, 53), "source", "critic", []string{"A second item."}); addErr != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("second add: %+v %v", result, addErr)
	}
	doneTarget := "done-target"
	result, moveErr := closeReadItem(readItemRequest(endpoint, 54), "source", "critic-2", ReadItemClosure{Moved: &doneTarget}, strictCodeRefResolver(t, endpoint))
	if moveErr != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not an open goal") {
		t.Fatalf("move to done target: %+v %v", result, moveErr)
	}
	tree, _ = acceptedTreeForEndpoint(t, endpoint)
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
	t.Parallel()
	endpoint, _ := readItemBed(t, "source")
	blank := "  "
	if _, err := closeReadItem(readItemRequest(endpoint, 60), "source", "read-1", ReadItemClosure{Accepted: &blank}, strictCodeRefResolver(t, endpoint)); err == nil || !strings.Contains(err.Error(), "non-blank reason") {
		t.Fatalf("blank accepted reason = %v", err)
	}
}

func TestReadItemsCloseRefusesClosedItem(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "source")
	if result, err := AddReadItems(readItemRequest(endpoint, 61), "source", "critic", []string{"One decision."}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("add: %+v %v", result, err)
	}
	reason := "intentional"
	if result, err := closeReadItem(readItemRequest(endpoint, 62), "source", "critic-1", ReadItemClosure{Accepted: &reason}, strictCodeRefResolver(t, endpoint)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("first close: %+v %v", result, err)
	}
	result, err := CloseReadItem(readItemRequest(endpoint, 63), "source", "critic-1", ReadItemClosure{Accepted: &reason})
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "already accepted") {
		t.Fatalf("second close: %+v %v", result, err)
	}
}

func TestDoneRefusesOpenReadItemIDsAndPassesWhenClosed(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "source")
	added, err := AddReadItems(readItemRequest(endpoint, 70), "source", "read-z", []string{"Explain the fallback.", "Name the invariant."})
	if err != nil || added.Outcome != OutcomeConfirmed {
		t.Fatalf("add: %+v %v", added, err)
	}
	result, doneErr := Done(readItemRequest(endpoint, 71), "source", "Built.")
	var openErr *DoneReadItemsOpenError
	_, tip := acceptedTreeForEndpoint(t, endpoint)
	_, typedErr := doneRequest(readItemRequest(endpoint, 73), "source", "Built.").Mutate(tip)
	if doneErr != nil || result.Outcome != OutcomeRejected || !errors.As(typedErr, &openErr) || !strings.Contains(result.Detail, "read-z-1, read-z-2") || !strings.Contains(result.Detail, "--id source --item read-z-1 --fixed") || !strings.Contains(result.Detail, "--id source --item read-z-2 --moved") || !strings.Contains(result.Detail, "--id source --item read-z-2 --accepted") {
		t.Fatalf("open-item done refusal: result=%+v error=%v typed=%T %v", result, doneErr, typedErr, typedErr)
	}
	reason := "the behavior is intentional"
	for index, itemID := range []string{"read-z-1", "read-z-2"} {
		closed, closeErr := CloseReadItem(readItemRequest(endpoint, 74+index), "source", itemID, ReadItemClosure{Accepted: &reason})
		if closeErr != nil || closed.Outcome != OutcomeConfirmed {
			t.Fatalf("close %s: %+v %v", itemID, closed, closeErr)
		}
	}
	result, doneErr = Done(readItemRequest(endpoint, 76), "source", "Built.")
	if doneErr != nil || result.Outcome != OutcomeConfirmed || !strings.Contains(result.Detail, "read-z-1: the behavior is intentional") {
		t.Fatalf("done after closure: %+v %v", result, doneErr)
	}
}

type terminalLedgerSnapshot struct {
	accepted     string
	peerAccepted string
	canonical    string
	files        map[string][]byte
}

func snapshotTerminalLedger(t *testing.T, endpoint, peer Endpoint) terminalLedgerSnapshot {
	t.Helper()
	client := endpoint.Repository.(*fakeGoalRepository)
	peerClient := peer.Repository.(*fakeGoalRepository)
	client.store.mu.Lock()
	defer client.store.mu.Unlock()
	if client.store != peerClient.store {
		t.Fatal("endpoint pair does not share a canonical store")
	}
	return terminalLedgerSnapshot{
		accepted:     client.accepted,
		peerAccepted: peerClient.accepted,
		canonical:    client.store.canonical,
		files:        copyFakeFiles(client.store.commits[client.store.canonical].files),
	}
}

func assertTerminalReadItemRefusal(t *testing.T, endpoint, peer Endpoint, goalID, itemID string, before terminalLedgerSnapshot, result PublishResult, err error) {
	t.Helper()
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "goal "+goalID) || !strings.Contains(result.Detail, itemID) {
		t.Fatalf("terminal transition did not name its open item: result=%+v err=%v", result, err)
	}
	if after := snapshotTerminalLedger(t, endpoint, peer); !reflect.DeepEqual(after, before) {
		t.Fatalf("refused terminal transition changed ledger bytes or refs:\nbefore=%+v\nafter=%+v", before, after)
	}
}

func TestEveryTerminalGoalTransitionRefusesOpenReadItems(t *testing.T) {
	t.Parallel()
	t.Run("split", func(t *testing.T) {
		t.Parallel()
		endpoint, peer := readItemBed(t, "split-parent")
		if result, err := AddReadItems(readItemRequest(endpoint, 100), "split-parent", "critic", []string{"Keep the parent live."}); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("add: %+v %v", result, err)
		}
		members := testMembers("split-parent")
		before := snapshotTerminalLedger(t, endpoint, peer)
		result, err := Split(readItemRequest(endpoint, 101), "split-parent", members, mainRatification("split-parent", members), nil)
		assertTerminalReadItemRefusal(t, endpoint, peer, "split-parent", "critic-1", before, result, err)
		reason := "addressed before decomposition"
		if closed, closeErr := CloseReadItem(readItemRequest(endpoint, 102), "split-parent", "critic-1", ReadItemClosure{Accepted: &reason}); closeErr != nil || closed.Outcome != OutcomeConfirmed {
			t.Fatalf("close: %+v %v", closed, closeErr)
		}
		if split, splitErr := Split(readItemRequest(endpoint, 103), "split-parent", members, mainRatification("split-parent", members), nil); splitErr != nil || split.Outcome != OutcomeConfirmed {
			t.Fatalf("split after close: %+v %v", split, splitErr)
		}
	})

	t.Run("abandon --also member", func(t *testing.T) {
		t.Parallel()
		endpoint, peer := readItemBed(t, "abandon-parent", "abandon-child")
		configureAbandonFloorTest(t, strings.Repeat("a", 40))
		recordAbandonFloorTestForEndpoint(t, endpoint, "01J5X00000000000000000RT00")
		blocked := []string{"abandon-parent"}
		if result, err := Edit(readItemRequest(endpoint, 110), "abandon-child", EditFields{Blocked: &blocked}); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("block child: %+v %v", result, err)
		}
		if result, err := AddReadItems(readItemRequest(endpoint, 111), "abandon-child", "critic", []string{"Do not strand this child item."}); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("add: %+v %v", result, err)
		}
		request := readItemRequest(endpoint, 112)
		request.Actor.Human = "Wido"
		spec := AbandonSpec{Because: "the work is obsolete", Also: []string{"abandon-child"}}
		before := snapshotTerminalLedger(t, endpoint, peer)
		result, err := Abandon(request, "abandon-parent", spec, goalHumanProof(t, endpoint.Root, request.Now))
		assertTerminalReadItemRefusal(t, endpoint, peer, "abandon-child", "critic-1", before, result, err)
		reason := "accepted before abandonment"
		if closed, closeErr := CloseReadItem(readItemRequest(endpoint, 113), "abandon-child", "critic-1", ReadItemClosure{Accepted: &reason}); closeErr != nil || closed.Outcome != OutcomeConfirmed {
			t.Fatalf("close: %+v %v", closed, closeErr)
		}
		request = readItemRequest(endpoint, 114)
		request.Actor.Human = "Wido"
		if abandoned, abandonErr := Abandon(request, "abandon-parent", spec, goalHumanProof(t, endpoint.Root, request.Now)); abandonErr != nil || abandoned.Outcome != OutcomeConfirmed {
			t.Fatalf("abandon after close: %+v %v", abandoned, abandonErr)
		}
	})

	t.Run("reconcile to done", func(t *testing.T) {
		t.Parallel()
		file := vGoal("reconcile-done", StateQueued)
		file.ReadItems = []ReadItem{{ID: "critic-1", Read: "critic", Text: "Close before reconciling.", State: ReadItemOpen, AddedAt: "2026-09-17T10:00:00Z"}}
		endpoint, peer := fakeGoalEndpointPair(t)
		publishGoalFixturesForEndpoint(t, endpoint, file)
		publishHandDone := func(sequence int) (PublishResult, error) {
			request := readItemRequest(endpoint, sequence)
			request.Actor.Human = "Wido"
			return Publish(endpoint, PublishRequest{
				Opid: request.opid(), Machine: request.Actor.Machine, Lineage: request.Actor.Lineage,
				Intent: Intent{Verb: "reconcile", Targets: []string{"reconcile-done"}}, Message: "goal reconcile (Wido)",
				Mutate: func(tip string) ([]Change, error) {
					files, err := endpoint.Repository.Files(tip, goalsPrefix, recordsGoalsPrefix)
					if err != nil {
						return nil, err
					}
					candidate := copyFakeFiles(files)
					current, problems := ParseFile(candidate[livePath("reconcile-done")])
					if len(problems) != 0 {
						return nil, fmt.Errorf("candidate source: %v", problems)
					}
					current.State = StateDone
					current.Conclude = "Hand-concluded."
					candidate[livePath("reconcile-done")] = RenderFile(current)
					candidateTree, problems := ParseTreeFiles(candidate)
					if len(problems) != 0 {
						return nil, fmt.Errorf("candidate edit: %v", problems)
					}
					handEdited := candidateTree.Live["reconcile-done"]
					if handEdited == nil || handEdited.State != StateDone {
						return nil, fmt.Errorf("candidate has no hand-concluded goal")
					}
					tree, err := loadTreeFor(endpoint, tip)
					if err != nil {
						return nil, err
					}
					return applyRow(tree, request, MappedVerb{Verb: "done", Id: "reconcile-done", BaseState: StateQueued, Conclude: handEdited.Conclude}, newReplaySession())
				}, Validate: func(commit string) error { return validateCommitFor(endpoint, commit) },
			})
		}
		before := snapshotTerminalLedger(t, endpoint, peer)
		result, err := publishHandDone(119)
		assertTerminalReadItemRefusal(t, endpoint, peer, "reconcile-done", "critic-1", before, result, err)
		reason := "accepted before hand conclusion"
		closed, closeErr := CloseReadItem(readItemRequest(endpoint, 120), "reconcile-done", "critic-1", ReadItemClosure{Accepted: &reason})
		if closeErr != nil || closed.Outcome != OutcomeConfirmed {
			t.Fatalf("close: %+v %v", closed, closeErr)
		}
		if reconciled, reconcileErr := publishHandDone(121); reconcileErr != nil || reconciled.Outcome != OutcomeConfirmed {
			t.Fatalf("reconcile after close: %+v %v", reconciled, reconcileErr)
		}
	})

	t.Run("done on parked goal", func(t *testing.T) {
		t.Parallel()
		endpoint, peer := readItemBed(t, "parked-done")
		if result, err := AddReadItems(readItemRequest(endpoint, 130), "parked-done", "critic", []string{"Close before done."}); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("add: %+v %v", result, err)
		}
		if result, err := Park(readItemRequest(endpoint, 131), "parked-done", "waiting for review"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("park: %+v %v", result, err)
		}
		request := readItemRequest(endpoint, 132)
		request.Actor.Human = "Wido"
		before := snapshotTerminalLedger(t, endpoint, peer)
		result, err := Done(request, "parked-done", "Done after review.")
		assertTerminalReadItemRefusal(t, endpoint, peer, "parked-done", "critic-1", before, result, err)
		reason := "accepted before conclusion"
		closeRequest := readItemRequest(endpoint, 133)
		closeRequest.Actor.Human = "Wido"
		if closed, closeErr := CloseReadItem(closeRequest, "parked-done", "critic-1", ReadItemClosure{Accepted: &reason}); closeErr != nil || closed.Outcome != OutcomeConfirmed {
			t.Fatalf("close: %+v %v", closed, closeErr)
		}
		request = readItemRequest(endpoint, 134)
		request.Actor.Human = "Wido"
		if done, doneErr := Done(request, "parked-done", "Done after review."); doneErr != nil || done.Outcome != OutcomeConfirmed {
			t.Fatalf("done after close: %+v %v", done, doneErr)
		}
	})
}

func TestRetroReadItemsListsOpenAndFlagsConcludedDefect(t *testing.T) {
	t.Parallel()
	live := vGoal("live-read", StateQueued)
	live.ReadItems = []ReadItem{{ID: "critic-1", Read: "critic", Text: "Follow up.", State: ReadItemOpen, AddedAt: "2026-09-17T10:00:00Z"}}
	doneBytes := string(RenderFile(vGoal("done-read", StateDone)))
	handWritten := "Open read items (fix unit critic): 1\n- ReadItem: id=critic-1 read=critic state=open addedAt=2026-09-17T10:00:00Z changedAt=- closingReference=\"\" text=\"Escaped conclusion.\"\n"
	doneBytes = strings.Replace(doneBytes, "- OpenedAt:", handWritten+"- OpenedAt:", 1)
	_, problems := ParseFile([]byte(withFreshIntegrity(doneBytes)))
	if len(problems) != 0 {
		t.Fatalf("hand-written concluded fixture did not parse: %v", problems)
	}
	endpoint, client := fakeGoalEndpoint(t, live)
	client.store.mu.Lock()
	seed := client.store.commits[client.store.canonical]
	seed.files[donePath("done-read")] = []byte(withFreshIntegrity(doneBytes))
	client.store.commits[client.store.canonical] = seed
	client.store.mu.Unlock()
	linesForRetro, present, err := readItemsForRetro(endpoint)
	if err != nil || !present {
		t.Fatalf("retro accepted tree: present=%t err=%v", present, err)
	}
	lines := strings.Join(linesForRetro, "\n")
	for _, want := range []string{"goal=live-read state=queued open=1", "read=critic id=critic-1 text=\"Follow up.\"", "goal=done-read state=done open=0", "LEDGER DEFECT goal=done-read concluded with open read item"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("retro lines lack %q:\n%s", want, lines)
		}
	}
	client.accepted = ""
	if absent, present, err := readItemsForRetro(endpoint); err != nil || present || absent != nil {
		t.Fatalf("absent accepted tree: lines=%v present=%t err=%v", absent, present, err)
	}
	client.brokenAccepted = errors.New("accepted ref unreadable")
	if _, present, err := readItemsForRetro(endpoint); present || err != client.brokenAccepted {
		t.Fatalf("broken accepted tree: present=%t err=%v", present, err)
	}
	client.brokenAccepted = nil
	client.accepted = client.store.canonical
	client.store.mu.Lock()
	seed = client.store.commits[client.store.canonical]
	seed.files[livePath("live-read")] = []byte("invalid goal file")
	client.store.commits[client.store.canonical] = seed
	client.store.mu.Unlock()
	if _, present, err := readItemsForRetro(endpoint); !present || err == nil {
		t.Fatalf("unreadable present tree: present=%t err=%v", present, err)
	}
}

func TestReadItemCodeCommitResolverUsesGitCommitObjects(t *testing.T) {
	root := t.TempDir()
	mustGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "sample.txt"), []byte("commit object\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "add", "sample.txt")
	mustGit(t, root, "commit", "-qm", "sample")
	want := mustGit(t, root, "rev-parse", "HEAD")
	if raw, err := resolveReadItemCodeCommit(root, "HEAD"); err != nil || strings.TrimSpace(raw) != want {
		t.Fatalf("commit resolution: raw=%q err=%v want=%q", raw, err, want)
	}
	for _, ref := range []string{"does-not-exist", mustGit(t, root, "rev-parse", "HEAD:sample.txt")} {
		if raw, err := resolveReadItemCodeCommit(root, ref); err == nil {
			t.Fatalf("non-commit %q resolved as %q", ref, raw)
		}
	}
}
