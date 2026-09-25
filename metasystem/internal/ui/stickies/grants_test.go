package stickies

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

// The reason this package exists, proved rather than asserted.
//
// Astra's F1: the interface's other per-human state lives in the state root,
// which on both layouts is inside the checkout — and inside the checkout is
// exactly what the Project Partner's permission owner grants native reads of
// and what a critic receives as a read root. A note kept there would be
// readable by a seat from the moment it was written, so "no seat reads your
// stickies" would have been false from the first one.
//
// So the two grants are READ here, from the files that carry them, and the
// notepad's own path is put through the same decision machinery the Partner's
// permission owner puts a request through. A grant that widened to cover the
// account's home would fail this file rather than quietly start reading a
// human's reminders.
//
// Nothing here writes: the production path is computed from the real home and
// never opened, because a test that wrote into the account's own home would be
// doing the one thing this package is careful not to do.

// The five-field envelope both grants are judged under. Only the read roots
// differ between them, which is the whole of what this file is about.
func envelopeOver(roots []string) acp.Envelope {
	return acp.Envelope{ReadRoots: roots, Network: "deny", Approvals: "deny", Tools: "read-only"}
}

// repoRoot is this checkout, found from the package's own directory. The two
// grants are both expressed against it: the Partner's is the checkout it
// serves, and the critic's "." is the repository it is handed.
func repoRoot(t *testing.T) string {
	t.Helper()
	here, err := os.Getwd()
	if err != nil {
		t.Fatalf("finding this package's directory: %v", err)
	}
	// internal/ui/stickies -> the metasystem module root, resolved because the
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

// The production path, computed and never written.
func productionPath(t *testing.T, checkout string) string {
	t.Helper()
	home, err := Home()
	if err != nil {
		t.Fatalf("resolving the account's registry home: %v", err)
	}
	if !filepath.IsAbs(home) {
		t.Fatalf("the registry home is %q, which is not an absolute path", home)
	}
	return Path(home, checkout)
}

// The notepad is outside the checkout it belongs to — not beside it, not under
// its state root, not under any directory of it.
func TestTheNotepadLivesOutsideTheCheckout(t *testing.T) {
	t.Parallel()

	checkout := repoRoot(t)
	path := productionPath(t, checkout)

	if strings.HasPrefix(path, checkout+string(filepath.Separator)) || path == checkout {
		t.Fatalf("the notepad is at %q, which is inside the checkout %q", path, checkout)
	}
	testutil.Expect(t, "that it is the workspace's own file", filepath.Base(path), "stickies.json")
	testutil.Expect(t, "that it is under the registry home",
		strings.Contains(path, filepath.Join(".metasystem", "ui", "stickies")), true)
}

// The Project Partner's permission owner grants native reads inside the
// checkout and nowhere else. The grant is read from the owner itself, and the
// notepad is put through the same decision the owner makes.
func TestThePartnersPermissionOwnerDoesNotReachTheNotepad(t *testing.T) {
	t.Parallel()

	checkout := repoRoot(t)
	owner := filepath.Join(checkout, "internal", "ui", "partner", "permission.go")
	source, err := os.ReadFile(owner)
	testutil.Require(t, "reading the Partner's permission owner", err, nil)

	// The grant, as the owner writes it: one envelope, whose read roots are
	// the resolved checkout and nothing else. Reading it rather than repeating
	// it is what makes this test fail when the grant widens.
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
		[]acp.Effect{{Class: acp.EffectRead, Paths: []string{productionPath(t, checkout)}}},
		envelopeOver([]string{checkout}))

	testutil.Expect(t, "what the Partner's grant says about the notepad", verdict, acp.VerdictDeny)
	// The same grant does reach the checkout, so the test is proving where the
	// boundary is rather than that the machinery refuses everything.
	testutil.Expect(t, "what it says about a file of the checkout",
		acp.Decide([]acp.Effect{{Class: acp.EffectRead,
			Paths: []string{filepath.Join(checkout, "internal", "ui", "overview", "visit.go")}}},
			envelopeOver([]string{checkout})),
		acp.VerdictAllow)
}

// A critic receives the repository as a read root. The grant is read from the
// file that carries it, its "." expanded the way the dispatch owner expands
// it, and the notepad is put through the same decision.
func TestTheCriticsReadRootDoesNotReachTheNotepad(t *testing.T) {
	t.Parallel()

	checkout := repoRoot(t)
	// The permission files live in the metasystem installation; the repository
	// a critic is handed is the one above it where this kit is self-hosted,
	// so both are proved below.
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
	// notepad has to be outside of.
	above, err := filepath.Abs(filepath.Join(checkout, ".."))
	testutil.Require(t, "finding the repository above the installation", err, nil)
	roots := []string{checkout, above}

	verdict := acp.Decide(
		[]acp.Effect{{Class: acp.EffectRead, Paths: []string{productionPath(t, checkout)}}},
		envelopeOver(roots))

	testutil.Expect(t, "what the critic's read root says about the notepad", verdict, acp.VerdictDeny)
	testutil.Expect(t, "what it says about a record of the repository",
		acp.Decide([]acp.Effect{{Class: acp.EffectRead,
			Paths: []string{filepath.Join(checkout, "testing.json")}}}, envelopeOver(roots)),
		acp.VerdictAllow)
}

// The fixture seam is the registry's own, so a test that redirects the
// registry redirects this store with it. A test that redirected one and not
// the other would be a test writing into the real account's home.
func TestTheHomeFollowsTheRegistrysOwnFixtureSeam(t *testing.T) {
	// Not parallel: it sets an environment variable for its own process.
	redirected := t.TempDir()
	t.Setenv("METASYSTEM_SUPERVISION_REGISTRY_HOME", redirected)

	home, err := Home()

	testutil.Require(t, "resolving the redirected home", err, nil)
	testutil.Expect(t, "where the notepad goes", home, filepath.Join(redirected, ".metasystem"))
	testutil.Expect(t, "that the file is under it",
		strings.HasPrefix(Path(home, "/tmp/workspaces/example"), redirected+string(filepath.Separator)), true)
}

// Two checkouts are two notepads, and one checkout is one notepad however it
// is spelled. The key is the workspace's own name with a digest of its whole
// path after it, so it is readable and still cannot collide.
func TestTheDirectoryIsKeyedByWorkspace(t *testing.T) {
	t.Parallel()

	home := "/home/someone/.metasystem"
	one := Path(home, "/tmp/workspaces/example")
	two := Path(home, "/tmp/elsewhere/example")

	if one == two {
		t.Fatalf("two checkouts named example share one notepad at %q", one)
	}
	testutil.Expect(t, "that the workspace is recognisable in the path",
		strings.Contains(one, "example-"), true)
	testutil.Expect(t, "that one checkout is one notepad however it is spelled",
		Path(home, "/tmp/workspaces/../workspaces/example/"), one)
}
