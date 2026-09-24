package goal

import (
	"testing"
)

// fakeServingFixture binds immutable committed files to one Store without a
// checkout. The same rendered records feed the Git-backed serving bed.
func fakeServingFixture(t *testing.T, machine string, files map[string]*GoalFile) (*Store, *fakeGoalRepository, Endpoint) {
	t.Helper()
	root := t.TempDir()
	commits := newFakeGoalStore()
	seed := commits.commits[commits.canonical]
	seed.files = copyFakeFiles(servingFixtureFiles(machine, files))
	commits.commits[commits.canonical] = seed
	client := commits.client()
	client.accepted = commits.canonical
	endpoint := Endpoint{Root: root, Remote: "local", Branch: LocalLedgerBranch, Repository: client}
	store := &Store{Root: root, projectionDeps: projectionDependencies{source: &projectionSource{endpoint: endpoint, machine: machine}}}
	return store, client, endpoint
}

func servingFixtureFiles(machine string, files map[string]*GoalFile) map[string][]byte {
	committed := map[string][]byte{}
	committed[goalsPrefix+"backlog.md"] = RenderRoot(&RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1",
		SyncMode: SyncLocal, Revision: 1,
	})
	history := []HistoryLine{{
		At: "2026-08-23T00:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-00000000",
		Verb: "open", Actor: machine + "+coordinator", Targets: []string{"any"}, Keep: -1,
	}}
	for id, f := range files {
		f.History = append([]HistoryLine(nil), history...)
		if f.State == StateApproved && f.Budget != nil {
			f.Revision = 2
			event := HistoryLine{
				At: "2026-08-23T00:01:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-human-00000002",
				Verb: "approve", Actor: "human:wido", Targets: []string{id}, Keep: -1,
			}
			f.History = append(f.History, event)
			digest := ApprovalDigest(f.Intent, f.Tier, *f.Budget)
			if f.Tier == 0 {
				digest = legacyApprovalDigest(f.Intent, *f.Budget)
			}
			f.Approved = &ApprovalRecord{
				By: event.Actor, At: event.At, Revision: 2, Opid: event.Opid,
				Authority: ApprovalAuthorityProven, Digest: digest,
			}
		}
		if f.Claimed != nil && f.Claimed.Revision == 2 {
			f.History = append(f.History, HistoryLine{
				At: f.Claimed.At, Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-bed-00000001",
				Verb: "claim", Actor: f.Claimed.Machine + "+" + f.Claimed.Lineage, Targets: []string{id}, Keep: -1,
			})
		}
		committed[goalsPrefix+id+".md"] = RenderFile(f)
	}
	return committed
}

func TestServingProjectionConvertedClaimCarriesOnlyIdentityAndIntent(t *testing.T) {
	t.Parallel()
	store, _, _ := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{
		"ship-it": {
			Id: "ship-it", State: "claimed", Intent: "Ship the whole thing", Origin: "main",
			NextStep: "Land it in pieces.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		},
	})
	id, intent, ok := store.ServingProjection()
	if !ok || id != "ship-it" || intent != "Ship the whole thing" {
		t.Fatalf("this machine's claim did not serve its identity and intent: %q %q %v", id, intent, ok)
	}
}

func TestServingProjectionForeignClaimServesNothing(t *testing.T) {
	t.Parallel()
	store, _, _ := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{
		"theirs": {
			Id: "theirs", State: "claimed", Intent: "Someone else's", Origin: "main",
			NextStep: "Work elsewhere.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
			Claimed: &ClaimRecord{Machine: "bed-m2", Lineage: "coordinator", At: "2026-08-23T01:00:00Z"},
		},
	})
	if _, _, ok := store.ServingProjection(); ok {
		t.Fatal("a foreign claim must serve nothing here")
	}
}

func TestServingProjectionAlwaysServesLiveClaimInsteadOfFencedClaim(t *testing.T) {
	t.Parallel()
	fenced := breachStoppedGoalForTest("fenced-first", "bed-m1")
	fenced.Priority, fenced.Sequence = 1, 1
	live := &GoalFile{
		Id: "live-second", State: StateClaimed, Intent: "Carry the live work", Origin: OriginMain,
		NextStep: "Continue it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 2,
		Priority: 1, Sequence: 2,
		Claimed: &ClaimRecord{
			Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-23T01:01:00Z",
			Revision: 2, AccountingRevision: 2,
		},
	}
	store, _, _ := fakeServingFixture(t, "bed-m1", map[string]*GoalFile{
		fenced.Id: fenced,
		live.Id:   live,
	})
	for call := 1; call <= 40; call++ {
		id, intent, ok := store.ServingProjection()
		if !ok || id != live.Id || intent != live.Intent {
			t.Fatalf("call %d served fenced or missing work: id=%q intent=%q ok=%t", call, id, intent, ok)
		}
	}
}
