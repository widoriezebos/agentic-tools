package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migrateManifest = `# Queue amendments

Commentary between entries is ignored.

### add-goal: new-work
- Intent: Fresh work the manifest queued while the ledger was frozen
- Origin: main
- Next: Start here.
- blockedBy: fix-docs

### amend-goal: fix-docs
- next: The amended next step.
`

// migrateBed writes the canonical legacy ledger into a clone.
func migrateBed(t *testing.T, root string) string {
	t.Helper()
	plans := filepath.Join(root, "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plans, "goals.md"), []byte(canonical), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plans, "goals-accepted.json"), []byte(`{"baseline":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "add", "plans")
	mustGit(t, root, "commit", "-qm", "legacy ledger")
	mustGit(t, root, "push", "-q", "origin", "main")
	return sha256HexBytes([]byte(canonical))
}

func migrateOpts(t *testing.T, root, digest string) MigrateOptions {
	t.Helper()
	manifestPath := filepath.Join(t.TempDir(), "manifest.md")
	if err := os.WriteFile(manifestPath, []byte(migrateManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return MigrateOptions{
		SourceDigest: digest, ManifestPath: manifestPath,
		Identity: "01J5XM00000000000000000000", SyncMode: SyncRemote,
	}
}

func TestMigrateSynthesizesTheExpectedMap(t *testing.T) {
	_, a, _ := twoClones(t)
	digest := migrateBed(t, a)

	res, err := Migrate(verbReq(a, "01J5XM0000000000000000M000", "mac-a"), migrateOpts(t, a, digest))
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("migrate: %+v %v", res, err)
	}
	tree, err := loadTree(a, res.Tip)
	if err != nil {
		t.Fatal(err)
	}

	// The expected map, verbatim: Current → claimed by this pair;
	// Queued → queued with the amendment applied; Parked → parked
	// with its reason; Done → archived with its conclusion; the
	// manifest's add queued behind its blocker.
	current := tree.Live["ship-widget"]
	if current == nil || current.State != StateClaimed || current.Claimed == nil || current.Claimed.Machine != "mac-a" {
		t.Fatalf("the legacy Current is this machine's claim: %+v", current)
	}
	queued := tree.Live["fix-docs"]
	if queued == nil || queued.State != StateQueued || queued.NextStep != "The amended next step." {
		t.Fatalf("the queued goal carries the manifest amendment: %+v", queued)
	}
	parked := tree.Live["perf-pass"]
	if parked == nil || parked.State != StateParked || !strings.Contains(parked.Parked.Because, "vendor") {
		t.Fatalf("the parked goal keeps its reason: %+v", parked)
	}
	done := tree.Done["port-engine"]
	if done == nil || done.Conclude != "Landed and gated on both hosts." {
		t.Fatalf("the archive keeps the conclusion: %+v", done)
	}
	added := tree.Live["new-work"]
	if added == nil || added.State != StateQueued || len(added.Blocked) != 1 || added.Blocked[0] != "fix-docs" {
		t.Fatalf("the manifest's add is queued behind its blocker: %+v", added)
	}
	// The root record binds the migration facts.
	if tree.Root.Identity != "01J5XM00000000000000000000" || tree.Root.MigrationMode != "manifest" ||
		tree.Root.SyncMode != SyncRemote || tree.Root.ManifestDigest == "" {
		t.Fatalf("the root record binds identity, mode, sync mode, and the manifest digest: %+v", tree.Root)
	}
	// The clean-path set: the legacy files are gone from the tip.
	if _, err := gitIn(a, "cat-file", "-p", res.Tip+":plans/goals.md"); err == nil {
		t.Fatal("goals.md dies in the migration commit")
	}
	if _, err := gitIn(a, "cat-file", "-p", res.Tip+":plans/goals-accepted.json"); err == nil {
		t.Fatal("goals-accepted.json dies in the migration commit")
	}
	// ONE commit, ONE opid: every synthesized file carries the same
	// migrate opid.
	wantOpid := Opid("01J5XM0000000000000000M000", "mac-a", "lin-1")
	for _, f := range []*GoalFile{current, queued, parked, added} {
		if f.History[0].Opid != wantOpid {
			t.Fatalf("one opid across the footprint: %s has %s", f.Id, f.History[0].Opid)
		}
	}

	// The rerun is idempotent, keyed on the root record + mode.
	res2, err := Migrate(verbReq(a, "01J5XM0000000000000000M010", "mac-a"), migrateOpts(t, a, digest))
	if err != nil || res2.Outcome != OutcomeConfirmed || res2.Detail != "idempotent" {
		t.Fatalf("the rerun classifies idempotent: %+v %v", res2, err)
	}
}

func TestMigrateRefusalsComeBeforeAnyMutation(t *testing.T) {
	_, a, _ := twoClones(t)
	digest := migrateBed(t, a)

	// A source-digest mismatch refuses before anything happens.
	badOpts := migrateOpts(t, a, strings.Repeat("00", 32))
	_, err := Migrate(verbReq(a, "01J5XM0000000000000000M020", "mac-a"), badOpts)
	if err == nil || !strings.Contains(err.Error(), "source digest mismatch") {
		t.Fatalf("the reviewed-source literal gates everything: %v", err)
	}
	// Nothing moved: the legacy ledger is untouched on the branch.
	if _, catErr := gitIn(a, "cat-file", "-p", "origin/main:plans/goals.md"); catErr != nil {
		t.Fatal("a refused migration mutates nothing")
	}

	// Mode confusion: bare after manifest refuses by name.
	good := migrateOpts(t, a, digest)
	if res, err := Migrate(verbReq(a, "01J5XM0000000000000000M030", "mac-a"), good); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("migrate: %+v %v", res, err)
	}
	bare := MigrateOptions{SourceDigest: digest, Identity: good.Identity, SyncMode: SyncRemote}
	res, err := Migrate(verbReq(a, "01J5XM0000000000000000M040", "mac-a"), bare)
	if err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "confusion") {
		t.Fatalf("mode confusion rejects by name: %+v %v", res, err)
	}
}

func TestMigrateIsDeterministicUnderInjection(t *testing.T) {
	// Two INDEPENDENT worlds, identical inputs (source bytes,
	// manifest, identity, actor, timestamp): byte-identical ledger
	// trees (R10's determinism leg).
	_, a, _ := twoClones(t)
	_, b, _ := twoClones(t)
	digestA := migrateBed(t, a)
	digestB := migrateBed(t, b)
	if digestA != digestB {
		t.Fatal("identical source bytes")
	}
	optsA := migrateOpts(t, a, digestA)
	optsB := migrateOpts(t, b, digestB)
	optsB.Identity = optsA.Identity

	resA, err := Migrate(verbReq(a, "01J5XM0000000000000000M050", "mac-a"), optsA)
	if err != nil || resA.Outcome != OutcomeConfirmed {
		t.Fatalf("A migrates: %+v %v", resA, err)
	}
	resB, err := Migrate(verbReq(b, "01J5XM0000000000000000M050", "mac-a"), optsB)
	if err != nil || resB.Outcome != OutcomeConfirmed {
		t.Fatalf("B migrates: %+v %v", resB, err)
	}
	filesA, err := ReadCommitGoals(a, resA.Tip)
	if err != nil {
		t.Fatal(err)
	}
	filesB, err := ReadCommitGoals(b, resB.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if len(filesA) != len(filesB) {
		t.Fatalf("same file set: %d vs %d", len(filesA), len(filesB))
	}
	for p, contentA := range filesA {
		if string(contentA) != string(filesB[p]) {
			t.Fatalf("byte-identical synthesis under injection: %s differs", p)
		}
	}
}
