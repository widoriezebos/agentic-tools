package branch_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func unitTree(t *testing.T, f *branchFixture, commit string) string {
	t.Helper()
	return git(t, f.root, "rev-parse", commit+"^{tree}")
}

func readerRecord(t *testing.T, f *branchFixture, commit string) string {
	t.Helper()
	digest, err := branch.UnitDigest(f.root, commit)
	if err != nil {
		t.Fatal(err)
	}
	path := "metasystem/records/misc/goal-a-u1-read.md"
	write(t, f.root, path, "Read commit "+commit+" with unit digest "+digest+" and found it clean.\n")
	return path
}

func writeJSONFixture(t *testing.T, root, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, path, string(append(data, '\n')))
}

func TestReadAdoptionRefusalsLeaveCheckoutUntouched(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, wantCode string
		remoteRead     bool
		claimMoves     bool
	}{
		{name: "staged change conflicts with remote", wantCode: branch.StaleCode, remoteRead: true},
		{name: "claim moves during preparation", wantCode: branch.NotHolderCode, claimMoves: true},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newBranchFixture(t)
			unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
			if _, err := branch.Push(pushRequest(f, "first-push")); err != nil {
				t.Fatal(err)
			}

			other := cloneBranchFixture(t, f)
			if _, err := branch.Push(pushRequest(other, "other-adopt")); err != nil {
				t.Fatal(err)
			}
			if test.remoteRead {
				record := readerRecord(t, other, unit)
				if _, _, err := branch.CommitRead(branch.CommitReadRequest{
					Repo: other.root, Remote: "origin", EndpointTip: other.base, GoalID: "goal-a", Unit: "u1", OpID: "other-read",
					CheckClaim: claimAllowed, ReaderRecord: record, GateRunID: "remote-fast", GateTree: unitTree(t, other, unit),
				}); err != nil {
					t.Fatal(err)
				}
			} else {
				commitUnit(t, other, "u2", "metasystem/other.go", "two")
			}
			if _, err := branch.Push(pushRequest(other, "other-push")); err != nil {
				t.Fatal(err)
			}
			remote := remoteGoalTip(t, f)

			record := readerRecord(t, f, unit)
			if test.remoteRead {
				write(t, f.root, record, "Independent read of commit "+unit+" with unit digest "+mustUnitDigest(t, f.root, unit)+".\n")
			}
			before := snapshotCheckout(t, f.root)
			checkClaim := claimAllowed
			if test.claimMoves {
				checks := 0
				checkClaim = func() error {
					checks++
					if checks > 1 {
						return errors.New("claim moved")
					}
					return nil
				}
			}
			_, _, err := branch.CommitRead(branch.CommitReadRequest{
				Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "local-read",
				CheckClaim: checkClaim, ReaderRecord: record, GateRunID: "local-fast", GateTree: unitTree(t, f, unit),
			})
			var refusal *branch.OpError
			if !errors.As(err, &refusal) || refusal.Code != test.wantCode {
				t.Fatalf("read adoption refusal = %v", err)
			}
			attestation := filepath.Join(f.root, "metasystem", "records", "reads", "goal-a", unit+".json")
			if _, statErr := os.Stat(attestation); !os.IsNotExist(statErr) {
				t.Fatalf("refused read left attestation: %v", statErr)
			}
			requireCheckoutUnchanged(t, f.root, before)

			if err := os.Remove(filepath.Join(f.root, filepath.FromSlash(record))); err != nil {
				t.Fatal(err)
			}
			stage(t, f, "metasystem/next.go", "next")
			next, err := branch.CommitStaged(branch.CommitRequest{
				Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "next", OpID: "next-unit",
				Kind: branch.Unit, CheckClaim: claimAllowed,
			})
			if err != nil || git(t, f.root, "rev-parse", next+"^") != remote {
				t.Fatalf("unit commit after refused read = %s, %v", next, err)
			}
		})
	}
}

func TestReadAdoptionKeepsUntrackedScratchFile(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	if _, err := branch.Push(pushRequest(f, "scratch-first")); err != nil {
		t.Fatal(err)
	}
	other := cloneBranchFixture(t, f)
	if _, err := branch.Push(pushRequest(other, "scratch-adopt")); err != nil {
		t.Fatal(err)
	}
	commitUnit(t, other, "u2", "metasystem/other.go", "two")
	if _, err := branch.Push(pushRequest(other, "scratch-remote")); err != nil {
		t.Fatal(err)
	}
	remote := remoteGoalTip(t, f)
	record := readerRecord(t, f, unit)
	write(t, f.root, "metasystem/scratch-notes.txt", "operator scratch")
	tip, _, err := branch.CommitRead(branch.CommitReadRequest{
		Repo: f.root, Remote: "origin", EndpointTip: f.base, GoalID: "goal-a", Unit: "u1", OpID: "scratch-read",
		CheckClaim: claimAllowed, ReaderRecord: record, GateRunID: "scratch-fast", GateTree: unitTree(t, f, unit),
	})
	if err != nil || git(t, f.root, "rev-parse", tip+"^") != remote {
		t.Fatalf("read adoption=%s err=%v", tip, err)
	}
	if got := git(t, f.root, "status", "--porcelain=v1", "--untracked-files=all"); got != "?? metasystem/scratch-notes.txt" {
		t.Fatalf("scratch status=%q", got)
	}
}

func mustUnitDigest(t *testing.T, root, commit string) string {
	t.Helper()
	digest, err := branch.UnitDigest(root, commit)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
