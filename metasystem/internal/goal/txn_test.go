package goal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mustGit runs git in dir or fails the test with the full output.
func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir,
		"-c", "user.name=t", "-c", "user.email=t@t",
		"-c", "protocol.file.allow=always"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// cloneBed builds the fixture spine with one required clone and an optional
// second clone for tests that exercise concurrent publishers.
func cloneBed(t *testing.T, second bool) (origin, a, b string) {
	t.Helper()
	origin = filepath.Join(t.TempDir(), "origin.git")
	mustGit(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", origin)
	seed := filepath.Join(t.TempDir(), "seed")
	mustGit(t, t.TempDir(), "clone", "-q", origin, seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, seed, "add", "README.md")
	mustGit(t, seed, "commit", "-qm", "seed")
	mustGit(t, seed, "push", "-q", "origin", "main")
	a = filepath.Join(t.TempDir(), "clone-a")
	mustGit(t, t.TempDir(), "clone", "-q", origin, a)
	if second {
		b = filepath.Join(t.TempDir(), "clone-b")
		mustGit(t, t.TempDir(), "clone", "-q", origin, b)
	}
	return origin, a, b
}

func oneClone(t *testing.T) (origin, clone string) {
	t.Helper()
	origin, clone, _ = cloneBed(t, false)
	return origin, clone
}

// twoClones builds two independent publishers against the same bare origin.
func twoClones(t *testing.T) (origin, a, b string) {
	t.Helper()
	return cloneBed(t, true)
}

func endpointFor(root string) Endpoint {
	return Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main"}
}

func goalChange(id, body string) Change {
	return Change{Path: "plans/goals/" + id + ".md", Content: []byte(body)}
}

func TestPublishLandsWithParentAndTrailer(t *testing.T) {
	origin, a, _ := twoClones(t)
	e := endpointFor(a)
	oldTip := mustGit(t, a, "rev-parse", "origin/main")
	headBefore := mustGit(t, a, "rev-parse", "HEAD")

	res, err := Publish(e, PublishRequest{
		Opid: "op-land", Machine: "mac-a", Lineage: "l1",
		Intent:  testIntentFor("open"),
		Message: "goal open fix-it",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{goalChange("fix-it", "# fix-it\nState: queued\n")}, nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the publish must confirm: %+v %v", res, err)
	}

	remoteTip := mustGit(t, origin, "rev-parse", "refs/heads/main")
	if remoteTip != res.Commit {
		t.Fatalf("the remote branch advances to the transaction commit: %s vs %s", remoteTip, res.Commit)
	}
	parent := mustGit(t, origin, "rev-parse", remoteTip+"^")
	if parent != oldTip {
		t.Fatalf("the commit's parent is exactly the captured tip (R3-02): %s vs %s", parent, oldTip)
	}
	body := mustGit(t, origin, "log", "-1", "--format=%B", remoteTip)
	if !strings.Contains(body, "Goal-Transaction: op-land") {
		t.Fatalf("the provenance trailer rides the commit: %q", body)
	}
	// The user's world never moved.
	if mustGit(t, a, "rev-parse", "HEAD") != headBefore {
		t.Fatal("HEAD must never move")
	}
	// The journal writes its machine-local record under artifacts/
	// (gitignored in the real repository); TRACKED state is what
	// must never move.
	if status := mustGit(t, a, "status", "--porcelain", "--untracked-files=no"); status != "" {
		t.Fatalf("the tracked worktree must stay untouched: %q", status)
	}
	// Journal terminal, temp refs cleaned, accepted advanced.
	entry, err := ReadEntry(a, "op-land")
	if err != nil || entry.Phase != PhaseTerminal || entry.Outcome != OutcomeConfirmed {
		t.Fatalf("the journal closes confirmed: %+v %v", entry, err)
	}
	if _, err := gitIn(a, "rev-parse", "--verify", "--quiet", fetchRefFor("op-land")); err == nil {
		t.Fatal("the fetch ref dies at terminal")
	}
	if _, err := gitIn(a, "rev-parse", "--verify", "--quiet", txnRefFor("op-land")); err == nil {
		t.Fatal("the txn ref dies at terminal")
	}
	accepted := mustGit(t, a, "rev-parse", AcceptedRef)
	if accepted != res.Commit {
		t.Fatalf("the accepted ref advances on confirmation: %s vs %s", accepted, res.Commit)
	}
}

func TestSameTargetRaceExactlyOneWins(t *testing.T) {
	_, a, b := twoClones(t)

	// A wins the target first.
	resA, err := Publish(endpointFor(a), PublishRequest{
		Opid: "op-winner", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("claim"), Message: "goal claim fix-it",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{goalChange("fix-it", "# fix-it\nState: claimed by mac-a\n")}, nil
		},
	})
	if err != nil || resA.Outcome != OutcomeConfirmed {
		t.Fatalf("A confirms: %+v %v", resA, err)
	}

	// B captured the OLD tip before A landed: the fixture seam
	// injects nothing — B's capture already sees A's commit, and its
	// mutate classifies the loss by reading the rebuilt world.
	resB, err := Publish(endpointFor(b), PublishRequest{
		Opid: "op-loser", Machine: "mac-b", Lineage: "l1",
		Intent: testIntentFor("claim"), Message: "goal claim fix-it",
		Mutate: func(tip string) ([]Change, error) {
			out, gErr := gitIn(b, "cat-file", "-p", tip+":plans/goals/fix-it.md")
			if gErr == nil && strings.Contains(out, "claimed by mac-a") {
				return nil, LostToCompetitor{Winner: "op-winner"}
			}
			return []Change{goalChange("fix-it", "# fix-it\nState: claimed by mac-b\n")}, nil
		},
	})
	if err != nil || resB.Outcome != OutcomeLost {
		t.Fatalf("B classifies the loss, reverting nothing: %+v %v", resB, err)
	}
	if !strings.Contains(resB.Detail, "op-winner") {
		t.Fatalf("the loser names the winner: %+v", resB)
	}
	entry, _ := ReadEntry(b, "op-loser")
	if entry.Outcome != OutcomeLost || !strings.Contains(entry.Evidence, "op-winner") {
		t.Fatalf("the journal carries the winner's opid: %+v", entry)
	}
}

func TestLeaseRefusalOnMidflightCompetitor(t *testing.T) {
	_, a, b := twoClones(t)
	e := endpointFor(a)

	// The fixture seam: between A's capture and A's push, B lands a
	// SAME-TARGET commit. A's CAS must refuse (exactly one winner);
	// the rebuild classifies the loss.
	injected := false
	res, err := Publish(e, PublishRequest{
		Opid: "op-cas", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("claim"), Message: "goal claim fix-it",
		Mutate: func(tip string) ([]Change, error) {
			out, gErr := gitIn(a, "cat-file", "-p", tip+":plans/goals/fix-it.md")
			if gErr == nil && strings.Contains(out, "mac-b") {
				return nil, LostToCompetitor{Winner: "op-b"}
			}
			return []Change{goalChange("fix-it", "# fix-it\nState: claimed by mac-a\n")}, nil
		},
		BeforePush: func(attempt int) error {
			if injected {
				return nil
			}
			injected = true
			resB, bErr := Publish(endpointFor(b), PublishRequest{
				Opid: "op-b", Machine: "mac-b", Lineage: "l1",
				Intent: testIntentFor("claim"), Message: "goal claim fix-it",
				Mutate: func(tip string) ([]Change, error) {
					return []Change{goalChange("fix-it", "# fix-it\nState: claimed by mac-b\n")}, nil
				},
			})
			if bErr != nil || resB.Outcome != OutcomeConfirmed {
				return fmt.Errorf("the competitor must land: %+v %v", resB, bErr)
			}
			return nil
		},
	})
	if err != nil || res.Outcome != OutcomeLost {
		t.Fatalf("the lease refuses and the rebuild classifies the loss: %+v %v", res, err)
	}
}

func TestBenignAdvancementRetriesWithinTheDeadline(t *testing.T) {
	_, a, b := twoClones(t)

	// The competitor's commit touches a DIFFERENT goal: lawful work,
	// not failure. A's first push refuses on the lease, the
	// rebuild carries the same change, the second push lands.
	injected := false
	attempts := 0
	res, err := Publish(endpointFor(a), PublishRequest{
		Opid: "op-benign", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open mine",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{goalChange("mine", "# mine\nState: queued\n")}, nil
		},
		BeforePush: func(attempt int) error {
			attempts = attempt
			if injected {
				return nil
			}
			injected = true
			resB, bErr := Publish(endpointFor(b), PublishRequest{
				Opid: "op-other", Machine: "mac-b", Lineage: "l1",
				Intent: testIntentFor("open"), Message: "goal open other",
				Mutate: func(tip string) ([]Change, error) {
					return []Change{goalChange("other", "# other\nState: queued\n")}, nil
				},
			})
			if bErr != nil || resB.Outcome != OutcomeConfirmed {
				return fmt.Errorf("the unrelated competitor must land: %+v %v", resB, bErr)
			}
			return nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("benign advancement retries and lands: %+v %v", res, err)
	}
	if attempts < 2 {
		t.Fatalf("the first lease must have refused: attempts=%d", attempts)
	}
	// Both goals live on the branch.
	tip := mustGit(t, a, "ls-remote", "origin", "refs/heads/main")
	_ = tip
	for _, id := range []string{"mine", "other"} {
		mustGit(t, a, "fetch", "-q", "origin", "main")
		if out := mustGit(t, a, "cat-file", "-p", "origin/main:plans/goals/"+id+".md"); out == "" {
			t.Fatalf("goal %s must be on the branch", id)
		}
	}
}

func TestPushedEntryBlocksTheWholeClone(t *testing.T) {
	_, a, _ := twoClones(t)
	// A stranded pushed entry (a crashed process's) blocks every new
	// mutation on this clone until classified — process-independent.
	if _, err := CreateEntry(a, "op-stuck", "mac-a", "l1", testIntentFor("claim")); err != nil {
		t.Fatal(err)
	}
	if err := MarkPushed(a, "op-stuck", "sometip", 1, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, err := Publish(endpointFor(a), PublishRequest{
		Opid: "op-next", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open x",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{goalChange("x", "# x\n")}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "op-stuck") {
		t.Fatalf("a pushed entry blocks own-clone mutations by name: %v", err)
	}
}

func TestValidationRefusalIsRejectedByName(t *testing.T) {
	_, a, _ := twoClones(t)
	res, err := Publish(endpointFor(a), PublishRequest{
		Opid: "op-invalid", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open bad",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{goalChange("bad", "not a goal file\n")}, nil
		},
		Validate: func(commit string) error {
			return fmt.Errorf("plans/goals/bad.md: State missing")
		},
	})
	if err != nil || res.Outcome != OutcomeRejected {
		t.Fatalf("a validation refusal is a definite rejection: %+v %v", res, err)
	}
	entry, _ := ReadEntry(a, "op-invalid")
	if entry.Outcome != OutcomeRejected || !strings.Contains(entry.Evidence, "State missing") {
		t.Fatalf("the rejection is journaled by name: %+v", entry)
	}
	// Nothing left the clone.
	if out := mustGit(t, a, "ls-remote", "origin", "refs/heads/main"); strings.Contains(out, "bad") {
		t.Fatal("a rejected transaction publishes nothing")
	}
}

func TestSingleMachineModeCASNeverMovesHead(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "solo")
	mustGit(t, t.TempDir(), "init", "-q", "-b", "main", repo)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("solo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", "README.md")
	mustGit(t, repo, "commit", "-qm", "seed")
	// The dedicated ledger branch — never the user's checked-out
	// branch (migration/adoption owns real bootstrap; the fixture
	// seeds it directly).
	mustGit(t, repo, "update-ref", LocalLedgerBranch, "HEAD")
	headBefore := mustGit(t, repo, "rev-parse", "HEAD")

	e := Endpoint{Root: repo, Remote: "local", Branch: "refs/heads/main"}
	res, err := Publish(e, PublishRequest{
		Opid: "op-solo", Machine: "mac-solo", Lineage: "l1",
		Intent: testIntentFor("open"), Message: "goal open here",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{goalChange("here", "# here\nState: queued\n")}, nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("local mode confirms by update-ref CAS: %+v %v", res, err)
	}
	ledger := mustGit(t, repo, "rev-parse", LocalLedgerBranch)
	if ledger != res.Commit {
		t.Fatalf("the ledger branch advances: %s vs %s", ledger, res.Commit)
	}
	if mustGit(t, repo, "rev-parse", "HEAD") != headBefore {
		t.Fatal("HEAD provably never moves in either mode")
	}
	if ok, err := TrailerPresent(e, ledger, "op-solo"); err != nil || !ok {
		t.Fatalf("the trailer postcondition resolves on the ledger branch: %v %v", ok, err)
	}
}

func TestPushFailureClassifierSeparatesLeaseFromTransport(t *testing.T) {
	for _, c := range []struct {
		output string
		want   CASOutcome
	}{
		{"! [rejected] main -> main (stale info)", CASRefused},
		{"! [rejected] abc..def main -> main (non-fast-forward)", CASRefused},
		{"hint: Updates were rejected... fetch first", CASRefused},
		{"fatal: unable to access 'https://x/': Could not resolve host", CASUnknown},
		{"ssh: connect to host x port 22: Connection timed out", CASUnknown},
	} {
		if got := classifyPushFailure(c.output); got != c.want {
			t.Fatalf("%q: got %s want %s", c.output, got, c.want)
		}
	}
}

func TestTransactionsAddressTheTreeUnderASubdirectoryRoot(t *testing.T) {
	// The REAL repository's shape: the goal root sits one level below
	// the git toplevel. Every rev:path read and raw index write must
	// address the tree under that prefix — the toplevel-rooted forms
	// pass every toplevel-rooted fixture and then refuse (or worse,
	// write beside the ledger) on the deployment layout itself.
	origin, a, _ := twoClones(t)
	_ = origin
	sub := filepath.Join(a, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	seedLedger(t, sub)
	res, err := Open(verbReq(sub, "01J5X00000000000000000SD00", "mac-a"), "deep-goal", "Prefixed world.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open under a subdirectory root: %+v %v", res, err)
	}
	// The write landed under the PREFIX at the toplevel view.
	out := mustGit(t, a, "ls-tree", "-r", "--name-only", res.Tip)
	if !strings.Contains(out, "nested/plans/goals/deep-goal.md") {
		t.Fatalf("the tree entry is not under the root's prefix:\n%s", out)
	}
	// The read side resolves the same world back.
	tree, err := loadTree(sub, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Live["deep-goal"] == nil {
		t.Fatalf("the prefixed ledger does not read back: %+v", tree.Live)
	}
}

func TestABrokenAcceptedRefRefusesMutations(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// The accepted ref EXISTS but points at a blob: identity and
	// descent cannot be gated, and skipping them because the ref
	// "did not resolve" is exactly the fail-open the gates forbid.
	oid, err := hashObject(a, []byte("not a commit"))
	if err != nil {
		t.Fatal(err)
	}
	mustGit(t, a, "update-ref", AcceptedRef, oid)
	_, openErr := Open(verbReq(a, "01J5X00000000000000000BA00", "mac-a"), "gated-out", "Blocked.", "main", "Go.")
	if openErr == nil || !strings.Contains(openErr.Error(), "does not resolve to a commit") {
		t.Fatalf("a broken accepted ref refuses the mutation by name: %v", openErr)
	}
}

func TestAMalformedAcceptedRefFileRefusesMutations(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// git WARNS and ignores a broken loose ref (exit 1, same as
	// absent): the file's existence is the only tell, and reading it
	// as pre-bootstrap would skip identity and descent.
	refFile := filepath.Join(a, ".git", "refs", "metasystem", "goals", "accepted")
	if err := os.MkdirAll(filepath.Dir(refFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refFile, []byte("garbagenothex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// End to end, remote mode already refuses — git's own fetch dies
	// enumerating the corrupt ref. The GATE's arm is what needs the
	// direct proof: absent and broken must answer differently.
	_, has, gateErr := acceptedTipForGates(a)
	if has || gateErr == nil || !strings.Contains(gateErr.Error(), "no valid ref") {
		t.Fatalf("a broken accepted-ref file refuses by name at the gate: %v (has=%v)", gateErr, has)
	}
}

func TestADanglingAcceptedRefSymlinkRefusesMutations(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	refFile := filepath.Join(a, ".git", "refs", "metasystem", "goals", "accepted")
	if err := os.MkdirAll(filepath.Dir(refFile), 0o755); err != nil {
		t.Fatal(err)
	}
	// The seed's own accepted ref steps aside first: the bed needs
	// the BROKEN shape at the path.
	_ = os.Remove(refFile)
	// A DANGLING symlink: git ignores it like an absent ref, a
	// following stat sees nothing — only Lstat tells the truth.
	if err := os.Symlink(filepath.Join(a, "no-such-target"), refFile); err != nil {
		t.Fatal(err)
	}
	_, has, gateErr := acceptedTipForGates(a)
	if has || gateErr == nil || !strings.Contains(gateErr.Error(), "no valid ref") {
		t.Fatalf("a dangling ref symlink refuses at the gate: %v (has=%v)", gateErr, has)
	}
}
