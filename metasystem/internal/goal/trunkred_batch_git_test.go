package goal

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// A real origin proves the publish adapter creates one main commit and carries
// the attempt and incident count in its actual commit message.
func TestTrunkRedResultOneCommitOnOriginGitAdapter(t *testing.T) {
	t.Parallel()
	for _, color := range []string{"green", "red"} {
		t.Run(color, func(t *testing.T) {
			t.Parallel()
			origin, root := oneClone(t)
			publishGoal(t, root, "trunk-result-seed", "fixture", nil)
			e := endpointFor(root)
			r := trunkRedVerbReqFor(e, "01J5X0000000000000000000G1", "mac-a")
			var result PublishResult
			var err error
			var before, message string
			var clears []TrunkRedClearArgs
			if color == "green" {
				data, readErr := os.ReadFile("testdata/trunk-red-7481259dd.json")
				if readErr != nil {
					t.Fatal(readErr)
				}
				var register trunkRedFile
				if err := json.Unmarshal(data, &register); err != nil {
					t.Fatal(err)
				}
				for i := range register.Entries {
					entry := &register.Entries[i]
					if entry.Closed != nil && entry.Closed.Attempt == "20261011T014956.528332000Z" {
						entry.Closed = nil
						clears = append(clears, TrunkRedClearArgs{Entry: entry.ID, ExpectedEntry: *entry, Attempt: "green-proof", BaseCommit: "proved-main", BaseTree: "proved-tree", Executed: true})
					}
				}
				seed := trunkRedVerbReqFor(e, "01J5X0000000000000000000G0", "mac-a")
				published, seedErr := Publish(e, PublishRequest{Opid: seed.opid(), Machine: seed.Actor.Machine, Lineage: seed.Actor.Lineage,
					Intent: Intent{Verb: "trunk-red-fixture"}, Message: "publish real incident shapes",
					Mutate: func(string) ([]Change, error) {
						return []Change{{Path: trunkRedPath, Content: renderTrunkRedState(register.Entries, register.Cadence, register.CadenceClaim)}}, nil
					},
					Validate: func(commit string) error { return validateCommitFor(e, commit) }})
				if seedErr != nil || published.Outcome != OutcomeConfirmed || len(clears) != 8 {
					t.Fatalf("real register fixture: %+v %v, clears=%d", published, seedErr, len(clears))
				}
				before = mustGit(t, origin, "rev-parse", "refs/heads/main")
				result, err = ClearTrunkReds(r, clears)
				message = "trunk-red clear green-proof: 8 incidents"
			} else {
				before = mustGit(t, origin, "rev-parse", "refs/heads/main")
				args := trunkRedRecordFixture("red:unit:TestOne", "batch", "red-proof", before, r.stamp())
				args.Groups = []TrunkRedRecordGroup{
					{Identity: "red:unit:TestOne", Group: "unit", Status: "failed", Failures: []TrunkRedFailure{{Report: "main.log", Classname: "unit", Name: "TestOne", Status: "failed"}}},
					{Identity: "red:unit:TestTwo", Group: "unit", Status: "failed", Failures: []TrunkRedFailure{{Report: "main.log", Classname: "unit", Name: "TestTwo", Status: "failed"}}},
					{Identity: "red:unit:TestThree", Group: "unit", Status: "failed", Failures: []TrunkRedFailure{{Report: "main.log", Classname: "unit", Name: "TestThree", Status: "failed"}}},
				}
				result, err = RecordTrunkRed(r, args)
				message = "trunk-red record red-proof: 3 incidents"
			}
			if err != nil || result.Outcome != OutcomeConfirmed {
				t.Fatalf("%s trunk result: %+v %v", color, result, err)
			}
			if got := mustGit(t, origin, "rev-list", "--count", before+"..refs/heads/main"); got != "1" {
				t.Fatalf("one %s trunk result wrote %s origin commits; want 1", color, got)
			}
			if got := mustGit(t, origin, "show", "-s", "--format=%s", "refs/heads/main"); got != message {
				t.Fatalf("origin commit message: %s, want %s", got, message)
			}
			if color == "green" {
				tip := mustGit(t, origin, "rev-parse", "refs/heads/main")
				count := mustGit(t, origin, "rev-list", "--count", "refs/heads/main")
				repeat := trunkRedVerbReqFor(e, "01J5X0000000000000000000G2", "mac-a")
				request := trunkRedClearBatchRequest(repeat, clears)
				changes, err := request.Mutate(tip)
				var already AlreadyApplied
				if !errors.As(err, &already) || len(changes) != 0 {
					t.Fatalf("same attempt under a second operation must be satisfied: changes=%+v err=%v", changes, err)
				}
				replayed, err := ClearTrunkReds(repeat, clears)
				if err != nil || replayed.Outcome != OutcomeConfirmed || replayed.Tip != tip {
					t.Fatalf("same attempt under a second operation: %+v %v", replayed, err)
				}
				if got := mustGit(t, origin, "rev-list", "--count", "refs/heads/main"); got != count {
					t.Fatalf("repeated green result wrote another origin commit: before=%s after=%s", count, got)
				}
				t.Logf("repeated green: second operation succeeded, origin commit count stayed %s", count)
			}
			t.Logf("%s: one origin commit, %s", color, message)
		})
	}
}
