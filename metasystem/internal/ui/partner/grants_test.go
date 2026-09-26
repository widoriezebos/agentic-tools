package partner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The examiner was not in the room, and cannot enter it — proved rather than
// asserted.
//
// Astra's F2 on g1-s53: the transcript lived under the state root, which on both
// layouts is inside the checkout — and inside the checkout is exactly what this
// Partner's own permission owner grants native reads of and what a critic
// receives as a read root. The master says the transcript is private sitting
// material in a protected server-local store outside the checkout, which
// examiners never read; the first sitting creates exactly the material an
// examiner must not read, and a transcript kept under the state root would have
// been readable by a seat from the first word.
//
// So the two grants are READ here, from the files that carry them, and the
// conversation's own path is put through the same decision machinery the
// Partner's permission owner puts a request through. A grant that widened to
// cover the account's home would fail this file rather than quietly start
// reading a human's sitting.
//
// Nothing here writes: the production path is computed from the real home and
// never opened, because a test that wrote into the account's own home would be
// doing the one thing this move is careful to prevent.

// The five-field envelope both grants are judged under. Only the read roots
// differ between them, which is the whole of what this file is about.
func grantOver(roots []string) acp.Envelope {
	return acp.Envelope{ReadRoots: roots, Network: "deny", Approvals: "deny", Tools: "read-only"}
}

// checkoutOf is this checkout, found from the package's own directory. The two
// grants are both expressed against it: the Partner's is the checkout it serves,
// and the critic's "." is the repository it is handed.
func checkoutOf(t *testing.T) string {
	t.Helper()
	here, err := os.Getwd()
	if err != nil {
		t.Fatalf("finding this package's directory: %v", err)
	}
	// internal/ui/partner -> the metasystem module root, resolved because the
	// grants are decided over canonical paths.
	resolved, err := filepath.EvalSymlinks(filepath.Join(here, "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving the checkout: %v", err)
	}
	root, err := filepath.Abs(resolved)
	if err != nil {
		t.Fatalf("finding the checkout: %v", err)
	}
	return root
}

// The production transcript, computed and never written.
func productionTranscript(t *testing.T, checkout string) string {
	t.Helper()
	home, err := Home()
	if err != nil {
		t.Fatalf("resolving the account's registry home: %v", err)
	}
	if !filepath.IsAbs(home) {
		t.Fatalf("the registry home is %q, which is not an absolute path", home)
	}
	return filepath.Join(Directory(home, checkout), fileName("Wido")+".jsonl")
}

// The conversation is outside the checkout it belongs to — not beside it, not
// under its state root, not under any directory of it.
func TestTheConversationLivesOutsideTheCheckout(t *testing.T) {
	t.Parallel()

	checkout := checkoutOf(t)
	path := productionTranscript(t, checkout)

	if strings.HasPrefix(path, checkout+string(filepath.Separator)) || path == checkout {
		t.Fatalf("the conversation is at %q, which is inside the checkout %q", path, checkout)
	}
	testutil.Expect(t, "that it is one human's own file", filepath.Base(path), "Wido.jsonl")
	testutil.Expect(t, "that it is under the registry home",
		strings.Contains(path, filepath.Join(".metasystem", "ui", Owner)), true)
	// And the state root is where it used to be, so the test is proving a move
	// rather than a coincidence.
	testutil.Expect(t, "that it is not under the old state root",
		strings.Contains(path, filepath.Join("artifacts", "agents", "ui")), false)
}

// This Partner's own permission owner grants native reads inside the checkout
// and nowhere else. The grant is read from the owner itself, and the
// conversation is put through the same decision the owner makes.
func TestThePartnersOwnGrantDoesNotReachTheConversation(t *testing.T) {
	t.Parallel()

	checkout := checkoutOf(t)
	owner := filepath.Join(checkout, "internal", "ui", "partner", "permission.go")
	source, err := os.ReadFile(owner)
	testutil.Require(t, "reading the Partner's permission owner", err, nil)

	// The grant, as the owner writes it: one envelope, whose read roots are the
	// resolved checkout and nothing else. Reading it rather than repeating it is
	// what makes this test fail when the grant widens.
	granted := regexp.MustCompile(
		`acp\.Envelope\{ReadRoots: \[\]string\{([A-Za-z]+)\}, Network: "deny", Approvals: "deny", Tools: "read-only"\}`).
		FindSubmatch(source)
	if granted == nil {
		t.Fatalf("%s no longer carries the one read envelope this test reads its grant from", owner)
	}
	testutil.Expect(t, "what the Partner's read roots are", string(granted[1]), "root")
	testutil.Expect(t, "that the root is the checkout, resolved",
		strings.Contains(string(source), "root := resolve(checkout)"), true)

	verdict := acp.Decide(
		[]acp.Effect{{Class: acp.EffectRead, Paths: []string{productionTranscript(t, checkout)}}},
		grantOver([]string{checkout}))

	testutil.Expect(t, "what the Partner's grant says about its own transcript", verdict, acp.VerdictDeny)
	// The same grant does reach the checkout, so the test is proving where the
	// boundary is rather than that the machinery refuses everything.
	testutil.Expect(t, "what it says about a file of the checkout",
		acp.Decide([]acp.Effect{{Class: acp.EffectRead,
			Paths: []string{filepath.Join(checkout, "internal", "ui", "partner", "conversation.go")}}},
			grantOver([]string{checkout})),
		acp.VerdictAllow)
}

// A critic receives the repository as a read root. The grant is read from the
// file that carries it, its "." expanded the way the dispatch owner expands it,
// and the conversation is put through the same decision.
func TestACriticsReadRootDoesNotReachTheConversation(t *testing.T) {
	t.Parallel()

	checkout := checkoutOf(t)
	// The permission files live in the metasystem installation; the repository a
	// critic is handed is the one above it where this kit is self-hosted, so both
	// are proved below.
	grant := filepath.Join(checkout, "scripts", "agents", "permissions", "critic.json")
	data, err := os.ReadFile(grant)
	testutil.Require(t, "reading the critic's grant", err, nil)
	var held struct {
		ReadRoots  []string `json:"readRoots"`
		WriteRoots []string `json:"writeRoots"`
		Tools      string   `json:"tools"`
	}
	testutil.Require(t, "parsing the critic's grant", json.Unmarshal(data, &held), nil)
	testutil.Expect(t, "what a critic may read", held.ReadRoots, []string{"."})
	testutil.Expect(t, "what a critic may write", len(held.WriteRoots), 0)

	// "." is the repository, which the dispatch owner resolves; the repository
	// above the installation is the wider of the two, so it is the one the
	// conversation has to be outside of.
	above, err := filepath.Abs(filepath.Join(checkout, ".."))
	testutil.Require(t, "finding the repository above the installation", err, nil)
	roots := []string{checkout, above}

	verdict := acp.Decide(
		[]acp.Effect{{Class: acp.EffectRead, Paths: []string{productionTranscript(t, checkout)}}},
		grantOver(roots))

	testutil.Expect(t, "what the critic's read root says about the conversation", verdict, acp.VerdictDeny)
	testutil.Expect(t, "what it says about a design of the repository",
		acp.Decide([]acp.Effect{{Class: acp.EffectRead,
			Paths: []string{filepath.Join(checkout, "testing.json")}}}, grantOver(roots)),
		acp.VerdictAllow)
}

// The conversation and the notepad are kept apart under one home, so neither can
// be read by asking for the other, and both are keyed by the workspace they
// belong to.
func TestTheConversationIsKeyedByWorkspaceBesideTheNotepad(t *testing.T) {
	t.Parallel()

	home := "/home/someone/.metasystem"
	one := Directory(home, "/tmp/workspaces/example")
	two := Directory(home, "/tmp/elsewhere/example")

	if one == two {
		t.Fatalf("two checkouts named example share one conversation directory at %q", one)
	}
	testutil.Expect(t, "that the workspace is recognisable in the path",
		strings.Contains(one, "example-"), true)
	testutil.Expect(t, "that one checkout is one directory however it is spelled",
		Directory(home, "/tmp/workspaces/../workspaces/example/"), one)
	testutil.Expect(t, "that it is this owner's own directory",
		strings.Contains(one, filepath.Join("ui", Owner)+string(filepath.Separator)), true)
	testutil.Expect(t, "and not the notepad's",
		strings.Contains(one, filepath.Join("ui", "stickies")), false)
}
