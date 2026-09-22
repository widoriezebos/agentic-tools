package act

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// testSession is a session's public reference, which is what the ledger
// records. A bearer never reaches this package at all.
const testSession = "sess-8Rk2Qp"

func sessionFor(t *testing.T, root string) Authority {
	t.Helper()
	proof, err := humanauthority.SignedInSessionProof(root, "Wido", testSession, "browser", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := SignedIn(root, "Wido", testSession, proof)
	if err != nil {
		t.Fatal(err)
	}
	return authority
}

// An approval from a browser a human signed into is the human's own approval:
// it names them, it records the session that carried it, and the ledger reads
// back clean afterwards.
func TestApproveUnderASignedInSessionWritesSessionAuthority(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	openGoal(t, root, "ui-session")
	authority := sessionFor(t, root)

	testutil.Expect(t, "who it acts as", authority.Human(), "Wido")
	testutil.Expect(t, "the lineage a browser act carries", SessionLineage, "browser-session")

	if err := authority.Approve("ui-session", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	file := readGoal(t, root, "ui-session")
	testutil.Expect(t, "the goal is approved", file.State, goal.StateApproved)
	testutil.Expect(t, "the approval's actor", file.Approved.By, "human:Wido")
	testutil.Expect(t, "the approval's authority", file.Approved.Authority, goal.ApprovalAuthoritySession)

	event := file.History[file.Approved.Revision-1]
	testutil.Expect(t, "the History outcome", event.AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)
	testutil.Expect(t, "the issuer", event.ChannelProvider, "browser")
	testutil.Expect(t, "the handle", event.ChannelUser, "Wido")
	testutil.Expect(t, "the session", event.ChannelRef, testSession)

	line := goal.RenderHistoryLine(event)
	want := " authorityOutcome=SIGNED_IN_SESSION channelProvider=browser channelUser=Wido channelRef=" + testSession
	if !strings.Contains(line, want) {
		t.Fatalf("the History line %q does not carry %q", line, want)
	}
	if _, problems := goal.ParseFile(goal.RenderFile(file)); len(problems) != 0 {
		t.Fatalf("the written goal does not read back clean: %v", problems)
	}
}

// Withdrawing an approval under a session says so on its own line. The class
// of a withdrawal is not recoverable from anywhere else once the approval
// record it removed is gone.
func TestWithdrawUnderASignedInSessionNamesTheSession(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	openGoal(t, root, "ui-session-undo")
	authority := sessionFor(t, root)
	if err := authority.Approve("ui-session-undo", box()); err != nil {
		t.Fatalf("approve: %v", err)
	}

	if err := authority.Withdraw("ui-session-undo", "the design is not settled"); err != nil {
		t.Fatalf("withdraw: %v", err)
	}

	file := readGoal(t, root, "ui-session-undo")
	testutil.Expect(t, "the goal is queued again", file.State, goal.StateQueued)
	testutil.Expect(t, "no approval stands", file.Approved == nil, true)
	last := file.History[len(file.History)-1]
	testutil.Expect(t, "the last verb", last.Verb, "unapprove")
	testutil.Expect(t, "the History outcome", last.AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)
	testutil.Expect(t, "the session", last.ChannelRef, testSession)
	if _, problems := goal.ParseFile(goal.RenderFile(file)); len(problems) != 0 {
		t.Fatalf("the written goal does not read back clean: %v", problems)
	}
}

// A re-rank under a session says so on every line it writes. The engine
// renumbers the band, so this act is never about one record, and each line it
// leaves names the hand that moved that goal.
func TestSetPriorityUnderASignedInSessionNamesTheSessionOnEveryLine(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	openGoal(t, root, "ui-rank-a")
	openGoal(t, root, "ui-rank-b")
	authority := sessionFor(t, root)

	if err := authority.SetPriority("ui-rank-b", 1, sequence(1)); err != nil {
		t.Fatalf("set-priority: %v", err)
	}

	moved := 0
	for _, id := range []string{"ui-rank-a", "ui-rank-b"} {
		file := readGoal(t, root, id)
		last := file.History[len(file.History)-1]
		if last.Verb != "set-priority" {
			continue
		}
		moved++
		testutil.Expect(t, id+" names the outcome", last.AuthorityOutcome, goal.AuthorityOutcomeSignedInSession)
		testutil.Expect(t, id+" names the issuer", last.ChannelProvider, "browser")
		testutil.Expect(t, id+" names the handle", last.ChannelUser, "Wido")
		testutil.Expect(t, id+" names the session", last.ChannelRef, testSession)
		if _, problems := goal.ParseFile(goal.RenderFile(file)); len(problems) != 0 {
			t.Fatalf("%s does not read back clean: %v", id, problems)
		}
	}
	if moved == 0 {
		t.Fatal("the re-rank wrote no set-priority line at all")
	}
}

// A session authority is built from a session proof and from nothing else,
// and it may not name a human or a session the proof was not minted for: the
// name it carries is the name the ledger records.
func TestASessionAuthorityMustMatchItsOwnProof(t *testing.T) {
	t.Parallel()
	root := ledger(t)
	elsewhere := t.TempDir()

	here, err := humanauthority.SignedInSessionProof(root, "Wido", testSession, "browser", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	there, err := humanauthority.SignedInSessionProof(elsewhere, "Wido", testSession, "browser", fixtureNow)
	if err != nil {
		t.Fatal(err)
	}

	// The lawful case first, so the refusals below are read as being about
	// what they name rather than about a constructor that refuses everything.
	lawful, err := SignedIn(root, "Wido", testSession, here)
	if err != nil || !lawful.Proven() {
		t.Fatalf("a session proof for this checkout was refused: %+v %v", lawful, err)
	}

	for name, built := range map[string]func() (Authority, error){
		"no human":        func() (Authority, error) { return SignedIn(root, "", testSession, here) },
		"no session":      func() (Authority, error) { return SignedIn(root, "Wido", "", here) },
		"another root":    func() (Authority, error) { return SignedIn(root, "Wido", testSession, there) },
		"no proof":        func() (Authority, error) { return SignedIn(root, "Wido", testSession, humanauthority.Proof{}) },
		"boot proof":      func() (Authority, error) { return SignedIn(root, "Wido", testSession, provenFor(t, root).proof) },
		"another human":   func() (Authority, error) { return SignedIn(root, "Somebody-Else", testSession, here) },
		"another session": func() (Authority, error) { return SignedIn(root, "Wido", "sess-NotThisOne", here) },
	} {
		authority, err := built()
		if err == nil {
			t.Fatalf("%s built an acting authority: %+v", name, authority)
		}
		if authority.Proven() {
			t.Fatalf("%s built a proven authority beside its error", name)
		}
	}
}
