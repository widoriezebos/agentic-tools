package act

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

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

// A session authority is built from a session proof and from nothing else: a
// name without one, or a proof bound to another checkout, authorizes nothing.
func TestASessionAuthorityNeedsItsOwnProofForThisCheckout(t *testing.T) {
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

	for name, built := range map[string]func() (Authority, error){
		"no human":      func() (Authority, error) { return SignedIn(root, "", testSession, here) },
		"no session":    func() (Authority, error) { return SignedIn(root, "Wido", "", here) },
		"another root":  func() (Authority, error) { return SignedIn(root, "Wido", testSession, there) },
		"no proof":      func() (Authority, error) { return SignedIn(root, "Wido", testSession, humanauthority.Proof{}) },
		"boot proof":    func() (Authority, error) { return SignedIn(root, "Wido", testSession, provenFor(t, root).proof) },
		"other session": func() (Authority, error) { return SignedIn(root, "Wido", testSession, here) },
	} {
		authority, err := built()
		if name == "other session" {
			// The one lawful case, which proves the refusals above are about
			// the proof rather than about the constructor refusing everything.
			if err != nil || !authority.Proven() {
				t.Fatalf("a session proof for this checkout was refused: %+v %v", authority, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s built an acting authority: %+v", name, authority)
		}
	}
}
