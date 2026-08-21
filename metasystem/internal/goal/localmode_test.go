package goal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The F16 fold: the sync-mode identity holds at fetch and mutation
// (not only projection), local migration bootstraps its dedicated
// branch, and the cutover rerun stays idempotent after goals.md is
// gone — without a second identity.

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
	r := localBed(t)
	digest := sha256HexBytes([]byte(canonical))
	opts := MigrateOptions{
		SourceDigest: digest, Identity: "01J5XK00000000000000000000", SyncMode: SyncLocal,
	}
	res, err := Migrate(localReq(r, "01J5X00000000000000000KM00"), opts)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("local migrate: %+v %v", res, err)
	}
	// The dedicated branch was BORN by the publication, and HEAD
	// never moved off the user's checkout branch.
	branchTip := strings.TrimSpace(mustGit(t, r, "rev-parse", "--verify", LocalLedgerBranch))
	if branchTip != res.Tip {
		t.Fatalf("the local ledger branch carries the migration: %s vs %s", short(branchTip), short(res.Tip))
	}
	if err := ValidateCommit(r, branchTip); err != nil {
		t.Fatalf("the bootstrapped ledger validates whole: %v", err)
	}
	tree, err := loadTree(r, branchTip)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Root.SyncMode != SyncLocal {
		t.Fatalf("the root record commits the local mode: %+v", tree.Root)
	}
	// The rerun is idempotent on the same identity and mode.
	res2, err := Migrate(localReq(r, "01J5X00000000000000000KM10"), opts)
	if err != nil || res2.Outcome != OutcomeConfirmed || res2.Detail != "idempotent" {
		t.Fatalf("the local rerun classifies idempotent: %+v %v", res2, err)
	}
	// Ordinary verbs work against the born branch.
	res3, err := Open(localReq(r, "01J5X00000000000000000KM20"), "solo-work", "Single-machine work.", "main", "Go.")
	if err != nil || res3.Outcome != OutcomeConfirmed {
		t.Fatalf("open on the local ledger: %+v %v", res3, err)
	}
}

func TestSyncModeGateHoldsAtFetchAndMutation(t *testing.T) {
	_, a, _ := twoClones(t)
	digest := migrateBed(t, a)
	res, err := Migrate(verbReq(a, "01J5X00000000000000000SG00", "mac-a"), migrateOpts(t, a, digest))
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("migrate: %+v %v", res, err)
	}

	// A remote-committed ledger pointed at local config: copying the
	// canonical tip onto the local branch and flipping the config is
	// exactly the split brain the gate names — at FETCH.
	mustGit(t, a, "update-ref", LocalLedgerBranch, res.Tip)
	flipped := Endpoint{Root: a, Remote: "local"}
	if _, err := FetchAdvance(flipped); err == nil || !strings.Contains(err.Error(), "split brain") {
		t.Fatalf("the fetch refuses the local flip by name: %v", err)
	}
	// And at MUTATION: the publish gate refuses before anything moves.
	req := verbReq(a, "01J5X00000000000000000SG10", "mac-a")
	req.Endpoint = flipped
	openRes, err := Open(req, "smuggled", "Split-brain write.", "main", "Go.")
	if err == nil || !strings.Contains(err.Error(), "split brain") {
		t.Fatalf("the mutation refuses the local flip by name: %+v %v", openRes, err)
	}

	// The reverse arm: a local-committed ledger read by a remote
	// config refuses toward the promotion goal.
	local := localBed(t)
	localRes, err := Migrate(localReq(local, "01J5X00000000000000000SG20"), MigrateOptions{
		SourceDigest: sha256HexBytes([]byte(canonical)),
		Identity:     "01J5XK00000000000000000001", SyncMode: SyncLocal,
	})
	if err != nil || localRes.Outcome != OutcomeConfirmed {
		t.Fatalf("local migrate: %+v %v", localRes, err)
	}
	if err := SyncModeGate(Endpoint{Root: local, Remote: "origin", Branch: "refs/heads/main"}, localRes.Tip); err == nil ||
		!strings.Contains(err.Error(), "config flip") {
		t.Fatalf("the promotion arm refuses by name: %v", err)
	}
}

func TestMigrateRerunSurvivesTheCutoverCheckout(t *testing.T) {
	_, a, _ := twoClones(t)
	digest := migrateBed(t, a)
	opts := migrateOpts(t, a, digest)
	res, err := Migrate(verbReq(a, "01J5X00000000000000000RC00", "mac-a"), opts)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("migrate: %+v %v", res, err)
	}
	// The post-cutover checkout: HEAD advances to the migration
	// commit and goals.md is GONE from the worktree — the state
	// every clone reaches after its next pull.
	mustGit(t, a, "fetch", "-q", "origin")
	mustGit(t, a, "reset", "-q", "--hard", "origin/main")
	if _, statErr := os.Stat(filepath.Join(a, "plans", "goals.md")); !os.IsNotExist(statErr) {
		t.Fatalf("the cutover checkout carries no goals.md: %v", statErr)
	}
	// The standing identity is readable — the CLI's rerun path
	// adopts it instead of minting a second one (F4 residue).
	if got := ExistingLedgerIdentity(a); got != opts.Identity {
		t.Fatalf("the ledger's standing identity is adopted, never re-minted: %q", got)
	}
	// The rerun classifies idempotent with NOTHING to read in the
	// worktree.
	res2, err := Migrate(verbReq(a, "01J5X00000000000000000RC10", "mac-a"), opts)
	if err != nil || res2.Outcome != OutcomeConfirmed || res2.Detail != "idempotent" {
		t.Fatalf("the goals.md-less rerun classifies idempotent: %+v %v", res2, err)
	}
}
