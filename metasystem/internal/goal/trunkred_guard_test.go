package goal

import "testing"

func TestGuardedIncidentClearRecoveryRequiresTheChecker(t *testing.T) {
	t.Parallel()
	for _, published := range []bool{false, true} {
		for _, batch := range []bool{false, true} {
			t.Run(map[bool]string{false: "unpublished", true: "published"}[published]+map[bool]string{false: "/single", true: "/batch"}[batch], func(t *testing.T) {
				e := localTrunkRedEndpoint(t)
				entry := testTrunkRedEntry("red:unit", "red:unit", "2026-09-17T01:00:00Z")
				before := publishTrunkRedFor(t, e, []TrunkRedEntry{entry}).Tip
				r := trunkRedVerbReqFor(e, "01J5X0000000000000000000B1", "mac-a")
				args := TrunkRedClearArgs{Entry: entry.ID, ExpectedEntry: entry, Attempt: "green", BaseCommit: before, Applicable: func(string) error { return nil }}
				req := trunkRedClearRequest(r, args)
				if batch {
					req = trunkRedClearBatchRequest(r, []TrunkRedClearArgs{args})
				}
				if published {
					result, err := Publish(e, req)
					if err != nil || result.Outcome != OutcomeConfirmed {
						t.Fatalf("initial publication: %+v %v", result, err)
					}
					saved, err := ReadEntry(e.Root, req.Opid)
					if err != nil {
						t.Fatal(err)
					}
					saved.Phase, saved.Outcome = PhasePushed, ""
					saved.Owner = OwnerIdentity{Pid: 999999999, PidStartedAt: 1}
					if err := writeEntry(e.Root, saved); err != nil {
						t.Fatal(err)
					}
				} else {
					createDeadTrunkRedEntry(t, e.Root, req)
				}
				if _, err := Recover(e); err != nil {
					t.Fatal(err)
				}
				saved, err := ReadEntry(e.Root, req.Opid)
				want := OutcomeRejected
				if published {
					want = OutcomeConfirmed
				}
				if err != nil || saved.Phase != PhaseTerminal || saved.Outcome != want || !published && acceptedTipForEndpoint(t, e) != before {
					t.Fatalf("guardless recovery: %+v %v", saved, err)
				}
			})
		}
	}
}
