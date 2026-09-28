package goal

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The goal CLI shell bed's migration-recovery scenario, ported to the goal
// owners over the Git-free fake repository: the reviewed source digest is
// the file's sha256; the migration's synthesized claim carries the runner's
// lineage (F16); goals.md dies in the cutover; the read-side fetch reports the
// canonical tip and settles on already-current; the post-cutover rerun adopts
// the ledger's standing identity and classifies idempotent (F4); recovery is
// clean on a healthy journal.
func TestGoalCLILedgerMigrationRecovery(t *testing.T) {
	t.Parallel()
	endpoint, client, opts := fakeLegacyMigrationEndpoint(t)
	source, err := os.ReadFile(filepath.Join(endpoint.Root, "plans", "goals.md"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(source)
	if SourceDigestOf(source) != hex.EncodeToString(sum[:]) || opts.SourceDigest != hex.EncodeToString(sum[:]) {
		t.Fatalf("the source digest is not the file's sha256: %s", SourceDigestOf(source))
	}

	status := newMigrationStatusTranscript(t, append(cleanMigrationStatusCalls(endpoint.Root, "manifest.md"), cleanMigrationStatusCalls(endpoint.Root, "manifest.md")...)...)
	request := verbReqFor(endpoint, "01J5XM0000000000000000G000", "fixture-machine")
	request.Actor.Lineage = "fixture-lineage"
	migrated, err := migrateWithStatus(request, opts, status.status)
	if err != nil || migrated.Outcome != OutcomeConfirmed {
		t.Fatalf("goal migrate did not confirm: %+v %v", migrated, err)
	}
	files, err := readCommitFiles(endpoint, migrated.Tip, "plans/goals.md", livePath("ship-widget"))
	if err != nil {
		t.Fatal(err)
	}
	if _, present := files["plans/goals.md"]; present {
		t.Fatal("goals.md survived the cutover commit")
	}
	claim := ""
	for _, line := range strings.Split(string(files[livePath("ship-widget")]), "\n") {
		if strings.HasPrefix(line, "- Claimed: ") {
			claim = line
		}
	}
	if !strings.HasPrefix(claim, "- Claimed: machine=fixture-machine lineage=fixture-lineage at=") {
		t.Fatalf("the synthesized claim does not carry the runner's lineage (F16): %q", claim)
	}

	fetched, err := FetchAdvance(endpoint)
	if err != nil || fetched.Tip != client.store.canonical {
		t.Fatalf("goal fetch does not report the canonical tip %s: %+v %v", client.store.canonical, fetched, err)
	}
	again, err := FetchAdvance(endpoint)
	if err != nil || again.Detail != "already at the canonical tip" {
		t.Fatalf("the second fetch is not already-current: %+v %v", again, err)
	}

	// The post-cutover rerun names no identity: the CLI adopts the ledger's
	// standing one, and the migration classifies idempotent.
	if err := os.Remove(filepath.Join(endpoint.Root, "plans", "goals.md")); err != nil {
		t.Fatal(err)
	}
	standing := ExistingLedgerIdentityAtEndpoint(endpoint)
	if standing != opts.Identity {
		t.Fatalf("the standing identity is %q, want the migration's %q (F4)", standing, opts.Identity)
	}
	rerunOpts := opts
	rerunOpts.Identity = standing
	rerun := verbReqFor(endpoint, "01J5XM0000000000000000G010", "fixture-machine")
	rerun.Actor.Lineage = "fixture-lineage"
	again2, err := migrateWithStatus(rerun, rerunOpts, status.status)
	if err != nil || again2.Outcome != OutcomeConfirmed || again2.Detail != "idempotent" {
		t.Fatalf("the rerun did not classify idempotent: %+v %v", again2, err)
	}

	if _, err := Recover(endpoint); err != nil {
		t.Fatalf("goal recover refused a healthy journal: %v", err)
	}
}
