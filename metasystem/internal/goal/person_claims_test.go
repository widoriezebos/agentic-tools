package goal

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func personalRequest(t *testing.T, endpoint Endpoint, ulid string) VerbRequest {
	t.Helper()
	r := verbReqFor(endpoint, ulid, "mac-a")
	r.Actor.Human = "Wido"
	r.Authority = testHumanAuthority(t, endpoint.Root, r.Now)
	r.ClaimEpoch = 0
	return r
}

func TestPersonClaimReclaimKeepsEpisode(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	r := verbReqFor(endpoint, "01J5X00000000000000000PR00", "mac-a")
	result, err := openClaimForTest(t, r, "reclaimed", "Continue the same accounting episode.", OriginMain, "Build it.", testBudget())
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("initial claim: %+v %v", result, err)
	}
	tree, _ := loadTreeFor(endpoint, result.Tip)
	episode := tree.Live["reclaimed"].Claimed.EpisodeRevision
	r.Ulid, r.Now = "01J5X00000000000000000PR10", r.Now.Add(time.Minute)
	if result, err = Release(r, "reclaimed"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("release: %+v %v", result, err)
	}
	person := personalRequest(t, endpoint, "01J5X00000000000000000PR20")
	person.Now = r.Now.Add(time.Minute)
	if result, err = Claim(person, "reclaimed"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("personal re-claim: %+v %v", result, err)
	}
	tree, _ = loadTreeFor(endpoint, result.Tip)
	f := tree.Live["reclaimed"]
	if !f.PersonalReservation() || f.StopCapability.ClaimEpoch != 0 || f.Claimed.EpisodeRevision != episode || f.Claimed.IdleSeconds != 60 {
		t.Fatalf("person origin was confused with the continued accounting episode: %+v", f)
	}
}

func TestPersonClaimProofAndStorageProvenance(t *testing.T) {
	t.Parallel()
	endpoint, _ := fakeGoalEndpoint(t)
	r := verbReqFor(endpoint, "01J5X00000000000000000PC00", "mac-a")
	if result, err := Open(r, "reserved", "Reserve before approval.", OriginMain, "Wait for approval."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	original, _ := loadTreeFor(endpoint, acceptedTipForEndpoint(t, endpoint))
	originalBudget := original.Live["reserved"].Budget
	person := personalRequest(t, endpoint, "01J5X00000000000000000PC10")
	for _, forgery := range []string{"name", "serialized proof", "grant"} {
		t.Run(forgery, func(t *testing.T) {
			t.Parallel()
			r := person
			switch forgery {
			case "name":
				r.Authority = nil
			case "serialized proof":
				data, _ := json.Marshal(person.Authority)
				p := &humanauthority.Proof{}
				_ = json.Unmarshal(data, p)
				r.Authority = p
			case "grant":
				p := *person.Authority
				p.Helm = &humanauthority.HelmGrant{Grant: "grant"}
				r.Authority = &p
			}
			if _, err := Claim(r, "reserved"); err == nil {
				t.Fatal("unproved person overrode claim admission")
			}
		})
	}
	result, err := Claim(person, "reserved")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("reservation: %+v %v", result, err)
	}
	tree, err := loadTreeFor(endpoint, result.Tip)
	if err != nil {
		t.Fatal(err)
	}
	f := tree.Live["reserved"]
	if f.Approved != nil || !reflect.DeepEqual(f.Budget, originalBudget) || f.StopCapability.ClaimEpoch != 0 || !f.PersonalReservation() {
		t.Fatalf("reservation invented execution authority: %+v", f)
	}
	t.Run("same pair renewal preserves provenance", func(t *testing.T) {
		t.Parallel()
		copy, problems := ParseFile(RenderFile(f))
		if len(problems) > 0 {
			t.Fatal(problems)
		}
		copy.StopCapability.ClaimEpoch = 7
		copy.Revision++
		copy.History = append(copy.History, HistoryLine{At: person.stamp(), Opid: Opid("01J5X00000000000000000PC20", copy.Claimed.Machine, copy.Claimed.Lineage), Verb: "handover", Actor: copy.Claimed.Machine + "+" + copy.Claimed.Lineage, Targets: []string{copy.Id}, Keep: -1})
		if _, problems := ParseFile(RenderFile(copy)); len(problems) != 0 || !copy.PersonalReservation() {
			t.Fatalf("a binding-preserving renewal lost personal provenance: %v", problems)
		}
		copy.History[len(copy.History)-1].Opid = Opid("01J5X00000000000000000PC20", "another-machine", "another-lineage")
		if _, problems := ParseFile(RenderFile(copy)); len(problems) == 0 {
			t.Fatal("a foreign handover fabricated personal provenance")
		}
	})
	for _, mutation := range []string{"erase origin", "forge actor", "forge pair", "negative epoch", "bare by"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			copy, problems := ParseFile(RenderFile(f))
			if len(problems) > 0 {
				t.Fatal(problems)
			}
			switch mutation {
			case "erase origin":
				copy.History[copy.Claimed.EpisodeRevision-1].AuthorityOutcome = ""
			case "forge actor":
				copy.History[copy.Claimed.EpisodeRevision-1].Actor = "human:another"
			case "forge pair":
				copy.Claimed.Lineage = "another"
			case "negative epoch":
				copy.StopCapability.ClaimEpoch = -1
			case "bare by":
				copy.History[copy.Claimed.EpisodeRevision-1].Verb = "edit"
			}
			_, problems = ParseFile(RenderFile(copy))
			if len(problems) == 0 {
				t.Fatalf("forged reservation loaded: %s", mutation)
			}
		})
	}
}

func TestPersonClaimArcWarnsAndForeignTakeoverKeepsScope(t *testing.T) {
	t.Parallel()
	endpoint, repo := fakeGoalEndpoint(t)
	for i, id := range []string{"arc-a", "arc-b", "arc-park", "elsewhere", "blocker"} {
		r := verbReqFor(endpoint, []string{"01J5X00000000000000000PA00", "01J5X00000000000000000PA10", "01J5X00000000000000000PA20", "01J5X00000000000000000PA30", "01J5X00000000000000000PA40"}[i], "mac-a")
		if result, err := Open(r, id, "Preserve explicit selection.", OriginMain, "Work it."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, result, err)
		}
	}
	tree, _ := loadTreeFor(endpoint, repo.store.canonical)
	for _, id := range []string{"arc-a", "arc-b", "arc-park"} {
		tree.Live[id].Arc = "chosen"
	}
	tree.Live["arc-a"].Pinned = "mac-b"
	tree.Live["arc-a"].Blocked = []string{"blocker"}
	tree.Live["arc-park"].State = StateParked
	tree.Live["arc-park"].Parked = &ParkRecord{By: "human:Wido", At: tree.Live["arc-park"].OpenedAt, Because: "explicit pause"}
	for _, f := range tree.Live {
		repo.store.commits[repo.store.canonical].files[livePath(f.Id)] = RenderFile(f)
	}
	r := personalRequest(t, endpoint, "01J5X00000000000000000PA50")
	snapshot := AreaSnapshot{Known: true, Source: "design@" + strings.Repeat("a", 64), Areas: []string{"shared/**"}}
	r.ClaimAreaReaders.Design = func(string, string) AreaSnapshot { return snapshot }
	result, err := ClaimArc(r, "arc-a")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("personal arc: %+v %v", result, err)
	}
	tree, _ = loadTreeFor(endpoint, result.Tip)
	if tree.Live["arc-park"].State != StateParked || tree.Live["elsewhere"].Claimed != nil || tree.Live["arc-a"].Pinned != "mac-b" || len(tree.Live["arc-a"].Blocked) != 1 || !strings.Contains(strings.Join(tree.Live["arc-a"].Claimed.Warnings, ";"), "unfinished goal blocker") {
		t.Fatalf("arc silently changed selection or sequencing: %+v", tree.Live)
	}
	// A person's takeover follows only the displaced pair within this arc.
	r.Ulid = "01J5X00000000000000000PA60"
	r.Actor.Machine = "mac-c"
	result, err = StealWithReason(r, "arc-a", "Move this pair.")
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("takeover: %+v %v", result, err)
	}
	tree, _ = loadTreeFor(endpoint, result.Tip)
	for _, id := range []string{"arc-a", "arc-b"} {
		if tree.Live[id].Claimed.Machine != "mac-c" || !tree.Live[id].PersonalReservation() {
			t.Fatalf("takeover lost scope or provenance: %+v", tree.Live[id])
		}
	}
	if tree.Live["elsewhere"].Claimed != nil || tree.Live["arc-park"].State != StateParked {
		t.Fatal("takeover crossed the explicit arc")
	}
	before := RenderFile(tree.Live["arc-a"])
	r.Ulid = "01J5X00000000000000000PA70"
	r.Actor.Machine = "mac-a"
	if result, err := ClaimArc(r, "arc-a"); err != nil || result.Outcome != OutcomeLost {
		t.Fatalf("foreign arc claim was not atomic: %+v %v", result, err)
	}
	after, _ := loadTreeFor(endpoint, repo.store.canonical)
	if !reflect.DeepEqual(before, RenderFile(after.Live["arc-a"])) {
		t.Fatal("foreign arc refusal partially moved ownership")
	}
}
