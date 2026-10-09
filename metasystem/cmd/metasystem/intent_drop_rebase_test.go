package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// Git's replayed commit identities and inverse composition are the adapter
// contract; the public verb uses the real outcome publisher and read gate.
func TestWorkRebaseCarriesPublishedDropGitAdapter(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"published", "check-retry", "record-retry"} {
		t.Run(mode, func(t *testing.T) {
			redGate := mode == "check-retry"
			t.Parallel()
			b, owners, _ := rebaseIntentBed(t)
			b.lineage = b.goalFile(b.id).Claimed.Lineage
			repo := b.worktree
			git := func(args ...string) string { t.Helper(); return connectionGit(t, repo, args...) }
			write := func(path, body string) { t.Helper(); writeUnitCarryFile(t, filepath.Join(repo, path), body) }
			git("init", "-q", "-b", "main")
			git("config", "user.name", "fixture")
			git("config", "user.email", "fixture@example.invalid")
			git("config", "goal.human.Wido", "Wido <wido@example.invalid>")
			write("metasystem/memory/receipts.log", "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n")
			git("add", ".")
			git("commit", "-qm", "base")
			base := git("rev-parse", "HEAD")
			remote := filepath.Join(t.TempDir(), "origin.git")
			connectionGit(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
			git("remote", "add", "origin", remote)
			git("push", "-q", "origin", "HEAD:main")
			write("metasystem/unit.txt", "optional\n")
			git("add", ".")
			unit, err := branch.CommitStaged(branch.CommitRequest{Repo: repo, Remote: "origin", EndpointTip: base, GoalID: b.id, Unit: "U", Kind: branch.Unit, OpID: "build-U", CheckClaim: func() error { return nil }})
			if err != nil {
				t.Fatal(err)
			}
			inverse, err := branch.CommitStaged(branch.CommitRequest{Repo: repo, Remote: "origin", EndpointTip: base, GoalID: b.id, Unit: "U", Kind: branch.Drop, OpID: "drop-U", FrozenPatch: []byte{}, CheckClaim: func() error { return nil }, BeforeCommit: func(dir, _, _ string) error { connectionGit(t, dir, "revert", "--no-commit", unit); return nil }})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := branch.Push(branch.PushRequest{Repo: repo, Remote: "origin", EndpointTip: base, GoalID: b.id, OpID: "publish-drop", CheckClaim: func() error { return nil }}); err != nil {
				t.Fatal(err)
			}
			file := b.goalFile(b.id)
			before := goal.UnitDrop{Unit: "U", Operation: "drop-U", Loop: "unit-read", Subject: unit, Attempt: 1, Revision: file.Revision, Covered: []string{unit}, Findings: []string{"read:1"}, Commit: inverse, Tree: git("rev-parse", inverse+"^{tree}"), Proof: "original-drop-check", Decisions: strings.Repeat("a", 64), Requirements: strings.Repeat("b", 64), Actor: file.Claimed.Machine + "+" + file.Claimed.Lineage, Reason: "Remove optional work", At: file.Claimed.At}
			endpoint, err := owners.dependencies.endpoint(b.root())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := goal.RecordUnitDrop(goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage}, Ulid: "01ARZ3NDEKTSV4RRFFQ69G5FAB", Now: b.manager.Now()}, b.id, before, nil); err != nil {
				t.Fatal(err)
			}
			git("checkout", "-q", "main")
			pagePath := "metasystem/plans/goals/" + b.id + ".md"
			write(pagePath, string(goal.RenderFile(b.goalFile(b.id))))
			write("metasystem/main.txt", "main moved\n")
			git("add", ".")
			git("commit", "-qm", "published outcome and endpoint moved")
			main := git("rev-parse", "HEAD")
			git("push", "-q", "origin", "main")
			git("update-ref", goal.AcceptedRef, main)
			git("checkout", "-q", "goal/"+b.id)
			owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return main, nil }
			owners.connection.rebase = branch.Rebase
			checks := 0
			owners.connection.rebaseGate = func(dir string) (string, error) {
				checks++
				if data, err := os.ReadFile(filepath.Join(dir, "metasystem/main.txt")); err != nil || string(data) != "main moved\n" {
					t.Fatalf("check missed moved endpoint: %q %v", data, err)
				}
				if _, err := os.Stat(filepath.Join(dir, "metasystem/unit.txt")); !os.IsNotExist(err) {
					t.Fatalf("check saw optional work: %v", err)
				}
				if redGate && checks == 1 {
					return "rebased drop check failed", errors.New("check failed")
				}
				return "", nil
			}
			if redGate {
				code, result := b.runJSON(owners, "work", "rebase", b.id)
				if code == 0 || !strings.Contains(result.Summary, "rebased drop check failed") || !reflect.DeepEqual(b.goalFile(b.id).UnitDrops[0], before) || connectionGit(t, remote, "rev-parse", "refs/heads/goal/"+b.id) != inverse {
					t.Fatalf("red check published a drop: %d %+v", code, result)
				}
			}
			if mode == "record-retry" {
				resolve := owners.dependencies.endpoint
				lost := &lostDropOutcome{Repository: b.repo}
				owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
					e, err := resolve(root)
					e.Repository = lost
					return e, err
				}
				code, result := b.runJSON(owners, "work", "rebase", b.id)
				if code == 0 || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "goal sync --recover") {
					t.Fatalf("missing publication recovery: %d %+v", code, result)
				}
				if tip := connectionGit(t, remote, "rev-parse", "refs/heads/goal/"+b.id); tip == inverse {
					t.Fatal("outcome was recorded before the rebased branch was published")
				}
				if code, result := transferPublic(t, b, owners, result.Next.Argv[1:]...); code != 0 {
					t.Fatalf("publication remedy: %d %+v", code, result)
				}
			}
			code, result := b.runJSON(owners, "work", "rebase", b.id)
			if code != 0 {
				t.Fatalf("rebase: %d %+v", code, result)
			}
			tip := git("rev-parse", "HEAD")
			after := b.goalFile(b.id).UnitDrops[0]
			if after.Commit == inverse || after.Covered[0] == unit || after.Commit != tip || after.Tree != git("rev-parse", tip+"^{tree}") || after.Proof == before.Proof || after.Proof == "" {
				t.Fatalf("replayed drop wasn't carried: before=%+v after=%+v", before, after)
			}
			want := before
			want.Commit, want.Covered, want.Tree, want.Proof, want.Revision = after.Commit, after.Covered, after.Tree, after.Proof, after.Revision
			if !reflect.DeepEqual(want, after) {
				t.Fatalf("rebase changed drop authority or decisions: %+v", after)
			}
			if checks != 1+map[bool]int{false: 0, true: 1}[redGate] {
				t.Fatalf("checks=%d", checks)
			}
			if units := resultData(t, result)["needsReview"].([]any); len(units) != 0 {
				t.Fatalf("dropped U needs review: %+v", result)
			}
			revision := b.goalFile(b.id).Revision
			code, result = b.runJSON(owners, "work", "rebase", b.id)
			if code != 0 || b.goalFile(b.id).Revision != revision || git("rev-parse", "HEAD") != tip {
				t.Fatalf("repeat changed completed drop: %d %+v", code, result)
			}
			// Move the published goal page into this Git adapter's endpoint snapshot.
			git("checkout", "-q", "main")
			write(pagePath, string(goal.RenderFile(b.goalFile(b.id))))
			git("add", ".")
			git("commit", "-qm", "carried drop outcome")
			main = git("rev-parse", "HEAD")
			git("update-ref", goal.AcceptedRef, main)
			git("checkout", "-q", "goal/"+b.id)
			status, err := branch.InspectStatus(repo, main, tip, b.id)
			if err != nil || len(status.Units) != 1 || status.Prefix != 1 || status.Units[0].ReadState != "dropped" {
				t.Fatalf("rebased drop status: %+v %v", status, err)
			}
			read, err := branch.InspectBranchRead(repo, b.id, after.Commit)
			if err != nil || read.State != "dropped" || !read.Published || read.GateRunID != after.Proof {
				t.Fatalf("current branch drop read: %+v %v", read, err)
			}
			landing, err := branch.PrepareLanding(branch.LandRequest{Repo: repo, Remote: "origin", EndpointTip: main, BranchTip: tip, GoalID: b.id, Last: true, LandingReady: true, CandidateOnly: true, GoalPage: string(goal.RenderFile(b.goalFile(b.id))), ApprovedBy: "human:Wido", Seat: "seat", CheckClaim: func() error { return nil }})
			if err != nil {
				t.Fatalf("rebased drop landing refused: %v", err)
			}
			count, err := plain.UnitsOnMain(repo, landing.Candidate, b.id)
			if err != nil || count != 0 {
				t.Fatalf("dropped U counted as landed: %d %v", count, err)
			}

		})
	}
}

func TestWorkDropStatusChecksCurrentBranch(t *testing.T) {
	t.Parallel()
	f := newDropFixture(t)
	f.connect()
	if code, result := f.review(t); code != 1 {
		t.Fatalf("prepare: %d %+v", code, result)
	}
	f.bed.head = f.v
	path := filepath.Join(f.retained(t).Rounds[1].Directory, "stop-dispositions.md")
	if code, result := f.review(t, "--dispositions", path); code != 0 {
		t.Fatalf("drop: %d %+v", code, result)
	}
	for _, state := range []string{"dropped", "drop pending", "unreadable"} {
		f.owners.work.inspectRead = func(_, _, commit string) (branch.BranchReadResult, error) {
			if commit != f.inverse {
				t.Fatalf("status read original instead of inverse: %s", commit)
			}
			if state == "unreadable" {
				return branch.BranchReadResult{}, errors.New("branch unavailable")
			}
			return branch.BranchReadResult{State: state, Published: state == "dropped"}, nil
		}
		code, result := transferPublic(t, f.bed, f.owners, "work", "status", f.bed.id, "--work", "stopped")
		if code != 0 {
			t.Fatalf("status: %d %+v", code, result)
		}
		if state == "dropped" {
			if !strings.Contains(result.Summary, "is dropped;") {
				t.Fatalf("published branch hidden: %+v", result)
			}
		} else if strings.Contains(result.Summary, "is dropped;") || !strings.Contains(result.Summary, "drop pending") || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "work rebase "+f.bed.id) {
			t.Fatalf("closed run hid current branch: %+v", result)
		}
	}
}
