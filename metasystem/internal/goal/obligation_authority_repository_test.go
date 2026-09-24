package goal

import (
	"os"
	"path/filepath"
	"testing"
)

func obligationAuthorityLocalEndpoint(t *testing.T, id string) Endpoint {
	t.Helper()
	endpoint, client := fakeGoalEndpoint(t)
	endpoint.Remote = SyncLocal
	openedAt := "2026-08-30T08:00:00Z"
	claimAt := "2026-08-30T08:05:00Z"
	rootRecord := &RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: SyncLocal, Revision: 1,
	}
	file := &GoalFile{
		Id: id, State: StateClaimed, Intent: "Govern validation.", Origin: OriginMain,
		NextStep: "Run it.", OpenedAt: openedAt, Revision: 2,
		Budget:  &Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2},
		Claimed: &ClaimRecord{Machine: "mac-a", Lineage: "lin-1", At: claimAt, Revision: 2},
		History: []HistoryLine{
			{At: openedAt, Opid: Opid("01ARZ3NDEKTSV4RRFFQ69G5FAA", "mac-a", "lin-1"), Verb: "open", Actor: "mac-a+lin-1", Targets: []string{id}, Keep: -1},
			{At: claimAt, Opid: Opid("01ARZ3NDEKTSV4RRFFQ69G5FAB", "mac-a", "lin-1"), Verb: "claim", Actor: "mac-a+lin-1", Targets: []string{id}, Keep: -1},
		},
	}
	files := vTree(rootRecord, []*GoalFile{file}, nil)
	seed := client.store.commits[client.store.canonical]
	seed.files = files
	client.store.commits[client.store.canonical] = seed
	for path, data := range files {
		materialized := filepath.Join(endpoint.Root, path)
		if err := os.MkdirAll(filepath.Dir(materialized), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(materialized, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return endpoint
}
