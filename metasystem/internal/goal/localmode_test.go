package goal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// localBed writes the canonical legacy ledger into a fresh
// single-machine repository (no origin — local mode has none).
func localBed(t *testing.T) string {
	t.Helper()
	r := t.TempDir()
	mustGit(t, r, "init", "-q", "-b", "main")
	plans := filepath.Join(r, "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plans, "goals.md"), []byte(canonical), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "ledger": canonical, "sha256": sha256HexBytes([]byte(canonical)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plans, "goals-accepted.json"), baseline, 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, r, "add", "plans")
	mustGit(t, r, "commit", "-qm", "legacy ledger")
	return r
}

func localReq(root, ulid string) VerbRequest {
	req := verbReq(root, ulid, "mac-solo")
	req.Endpoint = Endpoint{Root: root, Remote: "local"}
	return req
}

func TestLocalMigrationBootstrapsItsBranch(t *testing.T) {
	t.Parallel()
	endpoint, _, seeded := fakeLegacyMigrationEndpoint(t)
	endpoint.Remote = "local"
	opts := MigrateOptions{SourceDigest: seeded.SourceDigest,
		Identity: "01J5XK00000000000000000000", SyncMode: SyncLocal}
	status := newMigrationStatusTranscript(t, append(
		cleanMigrationStatusCalls(endpoint.Root, ""), cleanMigrationStatusCalls(endpoint.Root, "")...)...)
	res, err := migrateWithStatus(verbReqFor(endpoint, "01J5X00000000000000000KM00", "mac-solo"), opts, status.status)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("local migrate: %+v %v", res, err)
	}
	if err := validateCommitFor(endpoint, res.Tip); err != nil {
		t.Fatalf("the bootstrapped ledger validates whole: %v", err)
	}
	tree, err := loadTreeFor(endpoint, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Root.SyncMode != SyncLocal {
		t.Fatalf("the root record commits the local mode: %+v", tree.Root)
	}
	// The rerun is idempotent on the same identity and mode.
	res2, err := migrateWithStatus(verbReqFor(endpoint, "01J5X00000000000000000KM10", "mac-solo"), opts, status.status)
	if err != nil || res2.Outcome != OutcomeConfirmed || res2.Detail != "idempotent" {
		t.Fatalf("the local rerun classifies idempotent: %+v %v", res2, err)
	}
	if res2.Tip != res.Tip {
		t.Fatalf("the idempotent rerun moved the tip: %s vs %s", short(res2.Tip), short(res.Tip))
	}
	// Ordinary verbs work against the local ledger.
	res3, err := Open(verbReqFor(endpoint, "01J5X00000000000000000KM20", "mac-solo"), "solo-work", "Single-machine work.", "main", "Go.")
	if err != nil || res3.Outcome != OutcomeConfirmed {
		t.Fatalf("open on the local ledger: %+v %v", res3, err)
	}
}

func TestLocalPublishCreatesBranchWithoutMovingCheckoutHEAD(t *testing.T) {
	t.Parallel()
	root := localBed(t)
	headBefore := strings.TrimSpace(mustGit(t, root, "rev-parse", "HEAD"))
	branchBefore := strings.TrimSpace(mustGit(t, root, "symbolic-ref", "HEAD"))
	opts := MigrateOptions{SourceDigest: sha256HexBytes([]byte(canonical)),
		Identity: "01J5XK00000000000000000000", SyncMode: SyncLocal}
	res, err := Migrate(localReq(root, "01J5X00000000000000000KM30"), opts)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("first local publication: %+v %v", res, err)
	}
	branchTip := strings.TrimSpace(mustGit(t, root, "rev-parse", "--verify", LocalLedgerBranch))
	if branchTip != res.Tip {
		t.Fatalf("local branch points to %s, migration returned %s", short(branchTip), short(res.Tip))
	}
	if got := strings.TrimSpace(mustGit(t, root, "rev-parse", "HEAD")); got != headBefore {
		t.Fatalf("checkout HEAD moved from %s to %s", short(headBefore), short(got))
	}
	if got := strings.TrimSpace(mustGit(t, root, "symbolic-ref", "HEAD")); got != branchBefore {
		t.Fatalf("checkout branch moved from %s to %s", branchBefore, got)
	}
}

func TestSyncModeGateHoldsAtFetchAndMutation(t *testing.T) {
	t.Parallel()
	endpoint, repo, opts := fakeLegacyMigrationEndpoint(t)
	status := newMigrationStatusTranscript(t, cleanMigrationStatusCalls(endpoint.Root, "manifest.md")...)
	res, err := migrateWithStatus(verbReqFor(endpoint, "01J5X00000000000000000SG00", "mac-a"), opts, status.status)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("migrate: %+v %v", res, err)
	}

	// A local configuration reading the same remote-committed tip is
	// the split brain the gate names at fetch and mutation.
	flipped := endpoint
	flipped.Remote = "local"
	if _, err := FetchAdvance(flipped); err == nil || !strings.Contains(err.Error(), "split brain") {
		t.Fatalf("the fetch refuses the local flip by name: %v", err)
	}
	req := verbReqFor(flipped, "01J5X00000000000000000SG10", "mac-a")
	openRes, err := Open(req, "smuggled", "Split-brain write.", "main", "Go.")
	if err == nil || !strings.Contains(err.Error(), "split brain") {
		t.Fatalf("the mutation refuses the local flip by name: %+v %v", openRes, err)
	}
	if repo.store.canonical != res.Tip || repo.accepted != res.Tip {
		t.Fatalf("a refused flip moved the canonical or accepted tip: canonical=%s accepted=%s", short(repo.store.canonical), short(repo.accepted))
	}

	// A separate local-committed ledger refuses a remote config flip.
	local, _, seeded := fakeLegacyMigrationEndpoint(t)
	local.Remote = "local"
	localOpts := MigrateOptions{SourceDigest: seeded.SourceDigest,
		Identity: "01J5XK00000000000000000001", SyncMode: SyncLocal,
	}
	localStatus := newMigrationStatusTranscript(t, cleanMigrationStatusCalls(local.Root, "")...)
	localRes, err := migrateWithStatus(verbReqFor(local, "01J5X00000000000000000SG20", "mac-solo"), localOpts, localStatus.status)
	if err != nil || localRes.Outcome != OutcomeConfirmed {
		t.Fatalf("local migrate: %+v %v", localRes, err)
	}
	remote := local
	remote.Remote = "origin"
	if err := SyncModeGate(remote, localRes.Tip); err == nil ||
		!strings.Contains(err.Error(), "config flip") {
		t.Fatalf("the promotion arm refuses by name: %v", err)
	}
}

func TestMigrateRerunSurvivesTheCutoverCheckout(t *testing.T) {
	t.Parallel()
	endpoint, repo, opts := fakeLegacyMigrationEndpoint(t)
	status := newMigrationStatusTranscript(t, append(
		cleanMigrationStatusCalls(endpoint.Root, "manifest.md"), cleanMigrationStatusCalls(endpoint.Root, "manifest.md")...)...)
	res, err := migrateWithStatus(verbReqFor(endpoint, "01J5X00000000000000000RC00", "mac-a"), opts, status.status)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("migrate: %+v %v", res, err)
	}
	// Materialize the exact migrated snapshot as ordinary checkout files.
	files, err := repo.Files(res.Tip, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		checkoutPath := filepath.Join(endpoint.Root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(checkoutPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(checkoutPath, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"plans/goals.md", "plans/goals-accepted.json"} {
		if err := os.Remove(filepath.Join(endpoint.Root, filepath.FromSlash(path))); err != nil {
			t.Fatal(err)
		}
	}
	if _, statErr := os.Stat(filepath.Join(endpoint.Root, "plans", "goals.md")); !os.IsNotExist(statErr) {
		t.Fatalf("the cutover checkout carries no goals.md: %v", statErr)
	}
	// The standing identity is readable — the CLI's rerun path
	// adopts it instead of minting a second one (F4 residue).
	if got := existingLedgerIdentityFor(endpoint); got != opts.Identity {
		t.Fatalf("the ledger's standing identity is adopted, never re-minted: %q", got)
	}
	// The rerun classifies idempotent with NOTHING to read in the
	// worktree.
	res2, err := migrateWithStatus(verbReqFor(endpoint, "01J5X00000000000000000RC10", "mac-a"), opts, status.status)
	if err != nil || res2.Outcome != OutcomeConfirmed || res2.Detail != "idempotent" {
		t.Fatalf("the goals.md-less rerun classifies idempotent: %+v %v", res2, err)
	}
	if res2.Tip != res.Tip {
		t.Fatalf("the rerun minted a second tip: %s vs %s", short(res2.Tip), short(res.Tip))
	}
}
