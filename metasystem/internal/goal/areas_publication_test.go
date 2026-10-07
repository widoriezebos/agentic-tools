package goal

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// areaRaceRepository pauses only the first publication of each client. Both
// claims reach the real compare-and-swap from the same fetched ledger.
type areaRaceRepository struct {
	Repository
	once    sync.Once
	ready   chan<- struct{}
	proceed <-chan struct{}
}

func TestClaimArcAreasSkipsAlreadyOwnedBinding(t *testing.T) {
	t.Parallel()
	endpoint, repo := fakeGoalEndpoint(t)
	for index, id := range []string{"arc-owned", "arc-new"} {
		req := verbReqFor(endpoint, fmt.Sprintf("01J5X00000000000000000E%03d", index), "mac-a")
		req.Actor.Human = "Wido"
		if result, err := Open(req, id, "Complete the arc.", OriginHuman, "Build it."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", result, err)
		}
		approveGoalForTest(t, req, id, testBudget())
	}
	old := AreaSnapshot{Known: true, Source: "owned-design@" + strings.Repeat("a", 64), Areas: []string{"owned/**"}}
	req := verbReqFor(endpoint, "01J5X00000000000000000E010", "mac-a")
	req.ClaimAreaReaders.Design = func(string, string) AreaSnapshot { return old }
	if result, err := Claim(req, "arc-owned"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim first member: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, repo.store.canonical)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"arc-owned", "arc-new"} {
		tree.Live[id].Arc = "shared-arc"
		repo.store.commits[repo.store.canonical].files[livePath(id)] = RenderFile(tree.Live[id])
	}
	req.Ulid = "01J5X00000000000000000E011"
	ownedReads := 0
	req.ClaimAreaReaders.Design = func(id, _ string) AreaSnapshot {
		if id == "arc-owned" {
			ownedReads++
			return AreaSnapshot{Known: true, Source: "owned-design@" + strings.Repeat("b", 64), Areas: old.Areas}
		}
		return AreaSnapshot{Known: true, Source: "new-design@" + strings.Repeat("c", 64), Areas: []string{"new/**"}}
	}
	result, err := ClaimArc(req, "arc-owned")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("arc completion after owned design edit: %+v %v", result, err)
	}
	tree, err = loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["arc-owned"].Claimed.Source != old.Source || tree.Live["arc-new"].Claimed == nil || tree.Live["arc-new"].Claimed.Source != "new-design@"+strings.Repeat("c", 64) || ownedReads != 0 {
		t.Fatalf("arc rebound an owned member or missed the new claim: owned=%+v new=%+v ownedReads=%d", tree.Live["arc-owned"].Claimed, tree.Live["arc-new"].Claimed, ownedReads)
	}
}

func TestClaimAreasBindingRereadBound(t *testing.T) {
	t.Parallel()
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("deadline=%v", deadline), func(t *testing.T) {
			t.Parallel()
			endpoint, repo := fakeGoalEndpoint(t)
			req := verbReqFor(endpoint, "01J5X00000000000000000F000", "mac-a")
			req.Actor.Human = "Wido"
			if result, err := Open(req, "unstable", "Bind stable design bytes.", OriginHuman, "Build it."); err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("open: %+v %v", result, err)
			}
			approveGoalForTest(t, req, "unstable", testBudget())
			req.Actor.Human, req.Ulid = "", "01J5X00000000000000000F001"
			reads := 0
			req.ClaimAreaReaders.Design = func(string, string) AreaSnapshot {
				reads++
				if reads > 6 {
					t.Fatal("design rereading exceeded three attempts")
				}
				return AreaSnapshot{Known: true, Source: fmt.Sprintf("changing-design@%064x", reads), Areas: []string{"future/**"}}
			}
			request := claimRequest(req, "unstable", nil)
			clock := req.Now
			request.Deadline = time.Second
			request.now = func() time.Time {
				if deadline && reads > 0 {
					return clock.Add(request.Deadline)
				}
				return clock
			}
			before := repo.store.canonical
			result, err := Publish(endpoint, request)
			wantReads := 6
			if deadline {
				wantReads = 2
			}
			if err != nil || result.Outcome != OutcomeRejected || reads != wantReads || repo.store.canonical != before || !strings.Contains(result.Detail, "unstable") || !strings.Contains(result.Detail, "changing-design@") || !strings.Contains(result.Detail, "retry once") {
				t.Fatalf("unstable binding did not refuse within its bound: %+v err=%v reads=%d want=%d", result, err, reads, wantReads)
			}
			entry, err := ReadEntry(endpoint.Root, req.opid())
			if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeRejected {
				t.Fatalf("refusal left the journal open: %+v %v", entry, err)
			}
		})
	}
}

func (r *areaRaceRepository) Publish(parent, commit string) (CASOutcome, error) {
	r.once.Do(func() { r.ready <- struct{}{}; <-r.proceed })
	return r.Repository.Publish(parent, commit)
}

func TestClaimAreasConcurrentPublication(t *testing.T) {
	t.Parallel()
	first, second := fakeGoalEndpointPair(t)
	for index, id := range []string{"area-one", "area-two"} {
		req := verbReqFor(first, fmt.Sprintf("01J5X00000000000000000A%03d", index), "mac-a")
		req.Actor.Human = "Wido"
		if result, err := Open(req, id, "Sequence future files.", OriginHuman, "Build it."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open: %+v %v", result, err)
		}
		req.Ulid = fmt.Sprintf("01J5X00000000000000000B%03d", index)
		approveGoalForTest(t, req, id, testBudget())
	}
	client := first.Repository.(*fakeGoalRepository)
	client.store.mu.Lock()
	seed := client.store.commits[client.store.canonical]
	for _, id := range []string{"area-one", "area-two"} {
		seed.files["plans/designs/"+id+".md"] = []byte(fmt.Sprintf("# Areas\n\n- Kind: design\n- Id: %s-design\n- Status: accepted\n- Goals: %s\n- Areas: future/*.go\n", id, id))
	}
	client.store.commits[client.store.canonical] = seed
	client.store.mu.Unlock()
	ready := make(chan struct{}, 2)
	proceed := make(chan struct{})
	first.Repository = &areaRaceRepository{Repository: first.Repository, ready: ready, proceed: proceed}
	second.Repository = &areaRaceRepository{Repository: second.Repository, ready: ready, proceed: proceed}
	type answer struct {
		result PublishResult
		err    error
	}
	results := make(chan answer, 2)
	for index, endpoint := range []Endpoint{first, second} {
		go func() {
			req := verbReqFor(endpoint, fmt.Sprintf("01J5X00000000000000000C%03d", index), []string{"mac-a", "mac-b"}[index])
			result, err := Claim(req, []string{"area-one", "area-two"}[index])
			results <- answer{result, err}
		}()
	}
	<-ready
	<-ready
	close(proceed)
	confirmed, rejected := 0, 0
	for range 2 {
		a := <-results
		if a.err != nil {
			t.Fatal(a.err)
		}
		switch a.result.Outcome {
		case OutcomeConfirmed:
			confirmed++
		case OutcomeRejected:
			rejected++
			if !strings.Contains(a.result.Detail, "literal prefixes") {
				t.Fatalf("wrong refusal: %+v", a.result)
			}
		default:
			t.Fatalf("claim: %+v", a.result)
		}
	}
	if confirmed != 1 || rejected != 1 {
		t.Fatalf("competing claims: confirmed=%d rejected=%d", confirmed, rejected)
	}
}

func TestClaimAreasBindingReread(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	req := verbReqFor(endpoint, "01J5X00000000000000000D000", "mac-a")
	req.Actor.Human = "Wido"
	if result, err := Open(req, "binding", "Keep the accepted design binding.", OriginHuman, "Build it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000D001"
	approveGoalForTest(t, req, "binding", testBudget())
	reads := 0
	req.Actor.Human = ""
	req.Ulid = "01J5X00000000000000000D002"
	req.ClaimAreaReaders.Design = func(id, tip string) AreaSnapshot {
		reads++
		source := "old@" + strings.Repeat("a", 64)
		area := "old/**"
		if reads > 1 {
			source = "new@" + strings.Repeat("b", 64)
			area = "new/**"
		}
		return AreaSnapshot{Known: true, Source: source, Areas: []string{area}}
	}
	result, err := Claim(req, "binding")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil || tree.Live["binding"].Claimed.Source != "new@"+strings.Repeat("b", 64) || reads < 4 {
		t.Fatalf("stale binding: tree=%+v reads=%d err=%v", tree.Live["binding"].Claimed, reads, err)
	}
}
