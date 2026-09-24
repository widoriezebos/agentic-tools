package goal

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// migrateManifestFor binds the REAL schema's required headers to
// the bed's actual source digest.
func migrateManifestFor(digest string) string {
	return `# Queue amendments

Commentary between entries is ignored.

MIGRATION_EPOCH: 2026-08-20T00:00:00Z
REVIEWED_SOURCE_SHA256: ` + digest + `

### add-goal: new-work
- Intent: Fresh work the manifest queued while the ledger was frozen
- Origin: main
- Next: Start here.
- blockedBy: fix-docs

### amend-goal: fix-docs
- next: The amended next step.
`
}

// fakeLegacyMigrationEndpoint gives each migration its own committed legacy
// snapshot and real worktree files. The accepted pointer starts absent.
func fakeLegacyMigrationEndpoint(t *testing.T) (Endpoint, *fakeGoalRepository, MigrateOptions) {
	t.Helper()
	store := newFakeGoalStore()
	root := t.TempDir()
	plans := filepath.Join(root, "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	digest := sha256HexBytes([]byte(canonical))
	baseline, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "ledger": canonical, "sha256": digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte(migrateManifestFor(digest))
	worktree := map[string][]byte{
		"plans/goals.md":            []byte(canonical),
		"plans/goals-accepted.json": baseline,
		"manifest.md":               manifest,
		"stale-manifest.md":         []byte(migrateManifestFor(strings.Repeat("00", 32))),
		"metasystem.conf":           []byte("metasystem.runtimes=fake\n"),
	}
	for path, data := range worktree {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seed := store.commits[store.canonical]
	seed.files = copyFakeFiles(worktree)
	delete(seed.files, "metasystem.conf")
	store.commits[store.canonical] = seed
	client := store.client()
	endpoint := Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main", Repository: client}
	opts := MigrateOptions{SourceDigest: digest, ManifestPath: filepath.Join(root, "manifest.md"),
		Identity: "01J5XM00000000000000000000", SyncMode: SyncRemote}
	return endpoint, client, opts
}

type migrationStatusCall struct {
	root, path, porcelain string
	err                   error
}

type migrationStatusTranscript struct {
	t     *testing.T
	calls []migrationStatusCall
	next  int
}

func newMigrationStatusTranscript(t *testing.T, calls ...migrationStatusCall) *migrationStatusTranscript {
	t.Helper()
	transcript := &migrationStatusTranscript{t: t, calls: calls}
	t.Cleanup(func() {
		if transcript.next != len(transcript.calls) {
			t.Errorf("unused status declarations: consumed %d of %d", transcript.next, len(transcript.calls))
		}
	})
	return transcript
}

func (transcript *migrationStatusTranscript) status(root, path string) (string, error) {
	transcript.t.Helper()
	if transcript.next >= len(transcript.calls) {
		transcript.t.Fatalf("unexpected status call: root=%q path=%q", root, path)
	}
	want := transcript.calls[transcript.next]
	transcript.next++
	if root != want.root || path != want.path {
		transcript.t.Fatalf("status call %d: got root=%q path=%q, want root=%q path=%q", transcript.next, root, path, want.root, want.path)
	}
	return want.porcelain, want.err
}

func cleanMigrationStatusCalls(root, manifest string) []migrationStatusCall {
	paths := []string{"plans/goals.md", "plans/goals-accepted.json", "plans/goals"}
	if manifest != "" {
		paths = append(paths, manifest)
	}
	calls := make([]migrationStatusCall, len(paths))
	for i, path := range paths {
		calls[i] = migrationStatusCall{root: root, path: path, porcelain: "", err: nil}
	}
	return calls
}

func TestMigrateSynthesizesTheExpectedMap(t *testing.T) {
	t.Parallel()
	endpoint, _, opts := fakeLegacyMigrationEndpoint(t)
	status := newMigrationStatusTranscript(t, append(cleanMigrationStatusCalls(endpoint.Root, "manifest.md"), cleanMigrationStatusCalls(endpoint.Root, "manifest.md")...)...)

	res, err := migrateWithStatus(verbReqFor(endpoint, "01J5XM0000000000000000M000", "mac-a"), opts, status.status)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("migrate: %+v %v", res, err)
	}
	tree, err := loadTreeFor(endpoint, res.Tip)
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
	if tree.DonePaths["port-engine"] != recordsGoalsPrefix+"port-engine.md" {
		t.Fatalf("migration writes conclusions only to the records-owned archive: %s", tree.DonePaths["port-engine"])
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
	files, err := readCommitFiles(endpoint, res.Tip, "plans/goals.md", "plans/goals-accepted.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, present := files["plans/goals.md"]; present {
		t.Fatal("goals.md dies in the migration commit")
	}
	if _, present := files["plans/goals-accepted.json"]; present {
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
	if err := os.Remove(filepath.Join(endpoint.Root, "plans", "goals.md")); err != nil {
		t.Fatal(err)
	}
	res2, err := migrateWithStatus(verbReqFor(endpoint, "01J5XM0000000000000000M010", "mac-a"), opts, status.status)
	if err != nil || res2.Outcome != OutcomeConfirmed || res2.Detail != "idempotent" {
		t.Fatalf("the rerun classifies idempotent: %+v %v", res2, err)
	}
}

func TestMigrateRefusalsComeBeforeAnyMutation(t *testing.T) {
	t.Parallel()
	endpoint, client, good := fakeLegacyMigrationEndpoint(t)
	digest := good.SourceDigest
	canonicalTip := client.store.canonical
	original := copyFakeFiles(client.store.commits[canonicalTip].files)
	calls := append(cleanMigrationStatusCalls(endpoint.Root, "manifest.md"), cleanMigrationStatusCalls(endpoint.Root, "stale-manifest.md")...)
	calls = append(calls, cleanMigrationStatusCalls(endpoint.Root, "manifest.md")...)
	calls = append(calls, cleanMigrationStatusCalls(endpoint.Root, "")...)
	status := newMigrationStatusTranscript(t, calls...)

	// A caller digest disagreeing with the MANIFEST's bound literal
	// refuses at the binding gate; a manifest whose literal does not
	// match the worktree refuses at the source gate.
	badOpts := good
	badOpts.SourceDigest = strings.Repeat("00", 32)
	_, err := migrateWithStatus(verbReqFor(endpoint, "01J5XM0000000000000000M020", "mac-a"), badOpts, status.status)
	if err == nil || !strings.Contains(err.Error(), "the manifest is the authority") {
		t.Fatalf("the manifest binds the reviewed literal: %v", err)
	}
	staleOpts := good
	staleOpts.SourceDigest = strings.Repeat("00", 32)
	staleOpts.ManifestPath = filepath.Join(endpoint.Root, "stale-manifest.md")
	_, err = migrateWithStatus(verbReqFor(endpoint, "01J5XM0000000000000000M021", "mac-a"), staleOpts, status.status)
	if err == nil || !strings.Contains(err.Error(), "source digest mismatch") {
		t.Fatalf("the reviewed-source literal gates everything: %v", err)
	}
	// Nothing moved: the legacy ledger is untouched on the branch.
	if client.store.canonical != canonicalTip || !reflect.DeepEqual(client.store.commits[canonicalTip].files, original) {
		t.Fatal("a refused migration mutates nothing")
	}

	// Mode confusion: bare after manifest refuses by name.
	if res, err := migrateWithStatus(verbReqFor(endpoint, "01J5XM0000000000000000M030", "mac-a"), good, status.status); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("migrate: %+v %v", res, err)
	}
	// The confusion refuses BEFORE any journal write: a
	// doomed rerun never mints an entry.
	bare := MigrateOptions{SourceDigest: digest, Identity: good.Identity, SyncMode: SyncRemote}
	_, err = migrateWithStatus(verbReqFor(endpoint, "01J5XM0000000000000000M040", "mac-a"), bare, status.status)
	if err == nil || !strings.Contains(err.Error(), "confusion") {
		t.Fatalf("mode confusion refuses by name pre-journal: %v", err)
	}
}

func TestMigrateIsDeterministicUnderInjection(t *testing.T) {
	t.Parallel()
	// Two independent worlds have identical source bytes, manifest,
	// identity, actor, and timestamp.

	a, _, optsA := fakeLegacyMigrationEndpoint(t)
	b, _, optsB := fakeLegacyMigrationEndpoint(t)
	if optsA.SourceDigest != optsB.SourceDigest {
		t.Fatal("identical source bytes")
	}
	optsB.Identity = optsA.Identity
	statusA := newMigrationStatusTranscript(t, cleanMigrationStatusCalls(a.Root, "manifest.md")...)
	statusB := newMigrationStatusTranscript(t, cleanMigrationStatusCalls(b.Root, "manifest.md")...)

	resA, err := migrateWithStatus(verbReqFor(a, "01J5XM0000000000000000M050", "mac-a"), optsA, statusA.status)
	if err != nil || resA.Outcome != OutcomeConfirmed {
		t.Fatalf("A migrates: %+v %v", resA, err)
	}
	resB, err := migrateWithStatus(verbReqFor(b, "01J5XM0000000000000000M050", "mac-a"), optsB, statusB.status)
	if err != nil || resB.Outcome != OutcomeConfirmed {
		t.Fatalf("B migrates: %+v %v", resB, err)
	}
	filesA, err := readCommitGoals(a, resA.Tip)
	if err != nil {
		t.Fatal(err)
	}
	filesB, err := readCommitGoals(b, resB.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if len(filesA) != len(filesB) {
		t.Fatalf("same file set: %d vs %d", len(filesA), len(filesB))
	}
	for p, contentA := range filesA {
		contentB, present := filesB[p]
		if !present || !bytes.Equal(contentA, contentB) {
			t.Fatalf("byte-identical synthesis under injection: %s differs", p)
		}
	}
}

func TestTheCheckedInManifestParses(t *testing.T) {
	t.Parallel(
	// The PRODUCTION manifest must parse under the closed schema —
	// the review's F1: a toy fixture proved nothing about the file
	// the real cutover will consume. An ADOPTED repository ships no
	// migration manifest (the cutover artifact belongs to the
	// template repo alone), so absence skips; any other read error
	// still fails.
	)

	data, err := os.ReadFile(filepath.Join("..", "..", "records", "misc", "goals-migration-manifest.md"))
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("the checked-in manifest must parse: %v", err)
	}
	if m.Epoch != "2026-08-20T00:00:00Z" {
		t.Fatalf("the epoch header binds: %s", m.Epoch)
	}
	if m.ReviewedSHA256 != "266f3dc6a7c3c2cbb884349e54fca0c1f0f33db9b188a6d39ddd245f35e11a94" {
		t.Fatalf("the reviewed literal binds: %s", m.ReviewedSHA256)
	}
	adds, amends := 0, 0
	for _, e := range m.Entries {
		if e.Kind == "add-goal" {
			adds++
			if e.Intent == "" || e.Origin == "" || !e.HasNext {
				t.Fatalf("add-goal %s carries its required keys", e.Id)
			}
		} else {
			amends++
		}
	}
	if adds < 10 || amends < 5 {
		t.Fatalf("the real manifest's entries all parse: %d adds, %d amends", adds, amends)
	}
}

func TestMigrationCompleteRefusesAnUnparseableRootRecord(t *testing.T) {
	t.Parallel()
	endpoint, client, _ := fakeLegacyMigrationEndpoint(t)
	res, err := Publish(endpoint, PublishRequest{
		Opid: "op-torn-root-000000000000", Machine: "mac-a", Lineage: "l1",
		Intent: testIntentFor("migrate"), Message: "torn root",
		Mutate: func(tip string) ([]Change, error) {
			return []Change{{Path: goalsPrefix + "backlog.md", Content: []byte("not a root record at all\n")}}, nil
		},
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("seed torn root: %+v %v", res, err)
	}
	// A root record that EXISTS but does not parse is a broken world,
	// never "not migrated yet": reading it as absence would journal a
	// second migration on top of the wreck.
	rootFiles, err := readCommitFiles(endpoint, res.Tip, goalsPrefix+"backlog.md")
	if err != nil {
		t.Fatal(err)
	}
	if malformed, present := rootFiles[goalsPrefix+"backlog.md"]; !present || !bytes.Equal(malformed, []byte("not a root record at all\n")) {
		t.Fatalf("malformed committed root is present: %t %q", present, malformed)
	}
	if client.store.canonical != res.Tip {
		t.Fatal("the malformed root was not published")
	}
	if _, doneErr := migrationCompleteFor(endpoint, res.Tip, "01JIDENT", "bare", SyncRemote, ""); doneErr == nil ||
		!strings.Contains(doneErr.Error(), "does not parse") {
		t.Fatalf("a torn root refuses by name: %v", doneErr)
	}
}

func TestMigrationWorktreeStatusGitAdapterProvesPorcelainPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustGit(t, root, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(root, "plans", "goals.md")
	if err := os.WriteFile(legacyPath, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "add", "plans/goals.md")
	mustGit(t, root, "commit", "-qm", "tracked legacy ledger")
	if err := os.WriteFile(legacyPath, []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "untracked.md"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "manifest.md")
	if err := os.WriteFile(manifestPath, []byte("manifest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, rel := migrationCleanPaths(root, manifestPath)
	if rel != "manifest.md" || !reflect.DeepEqual(paths, []string{"plans/goals.md", "plans/goals-accepted.json", "plans/goals", "manifest.md"}) {
		t.Fatalf("in-root manifest path selection: %q %q", rel, paths)
	}
	for _, path := range []string{"plans/goals.md", "plans/goals", "manifest.md"} {
		porcelain, err := migrationGitStatus(root, path)
		if err != nil || strings.TrimSpace(porcelain) == "" {
			t.Fatalf("porcelain for changed path %s: %q %v", path, porcelain, err)
		}
	}
	external := filepath.Join(t.TempDir(), "external-manifest.md")
	externalPaths, externalRel := migrationCleanPaths(root, external)
	if externalRel != "" || !reflect.DeepEqual(externalPaths, []string{"plans/goals.md", "plans/goals-accepted.json", "plans/goals"}) {
		t.Fatalf("external manifest must be excluded: %q %q", externalRel, externalPaths)
	}
}
