package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// materialize writes a commit's ledger files into the worktree and
// records the base, the way a refresh leaves a checkout.
func materialize(t *testing.T, root, commit string) {
	t.Helper()
	files, err := ReadCommitGoals(root, commit)
	if err != nil {
		t.Fatal(err)
	}
	for p, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := RecordMaterialized(root, commit); err != nil {
		t.Fatal(err)
	}
}

func reconcileBed(t *testing.T) (string, string) {
	t.Helper()
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	res, err := Open(verbReq(a, "01J5X00000000000000000R000", "mac-a"), "editable", "Original intent.", "main", "Original next.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	materialize(t, a, res.Tip)
	return a, res.Tip
}

func TestBaseIsPersistedNotHead(t *testing.T) {
	a, tip := reconcileBed(t)
	// HEAD moves mid-session (an ordinary code commit): the base
	// stays the recorded materialized commit.
	if err := os.WriteFile(filepath.Join(a, "unrelated.txt"), []byte("code\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, a, "add", "unrelated.txt")
	mustGit(t, a, "commit", "-qm", "unrelated code")
	base, err := BaseTip(a)
	if err != nil {
		t.Fatal(err)
	}
	if base != tip {
		t.Fatalf("the base is the RECORDED materialized commit, never HEAD: %s vs %s", short(base), short(tip))
	}
	// The anchor ref keeps the base reachable.
	if out := mustGit(t, a, "rev-parse", baseAnchorRef); strings.TrimSpace(out) != tip {
		t.Fatalf("the anchor ref pins the base: %s", out)
	}
}

func TestCaptureIsStableAndDiffNamesTheDeltas(t *testing.T) {
	a, tip := reconcileBed(t)
	// Hand edits: change one file, add one, remove none.
	editablePath := filepath.Join(a, "plans", "goals", "editable.md")
	edited, err := os.ReadFile(editablePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(editablePath, []byte(strings.Replace(string(edited), "Original intent.", "Hand-edited intent.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	newFile := vGoal("hand-made", StateQueued)
	if err := os.WriteFile(filepath.Join(a, "plans", "goals", "hand-made.md"), RenderFile(newFile), 0o644); err != nil {
		t.Fatal(err)
	}

	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	// An editor save AFTER capture never changes the snapshot.
	if err := os.WriteFile(editablePath, []byte("torn mid-session"), 0o644); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(snap.Files[goalsPrefix+"editable.md"]), "torn") {
		t.Fatal("the snapshot is immune to post-capture saves")
	}

	deltas, err := DiffAgainstBase(a, tip, snap)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, d := range deltas {
		kinds[d.Path] = d.Kind
	}
	if kinds[goalsPrefix+"editable.md"] != "changed" || kinds[goalsPrefix+"hand-made.md"] != "added" {
		t.Fatalf("the diff names the deltas: %v", kinds)
	}
	if len(deltas) != 2 {
		t.Fatalf("nothing else differs: %v", deltas)
	}
}

func TestRefreshPreservesPostCaptureEdits(t *testing.T) {
	a, _ := reconcileBed(t)
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	// A publish lands (any new tip works for the refresh contract).
	res, err := Open(verbReq(a, "01J5X00000000000000000R010", "mac-a"), "published", "New goal.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// The user edits ONE file after capture, before the refresh.
	editablePath := filepath.Join(a, "plans", "goals", "editable.md")
	if err := os.WriteFile(editablePath, []byte("post-capture edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skipped, err := Refresh(a, res.Tip, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 1 || skipped[0] != goalsPrefix+"editable.md" {
		t.Fatalf("the post-capture edit is preserved and NAMED: %v", skipped)
	}
	if data, _ := os.ReadFile(editablePath); string(data) != "post-capture edit\n" {
		t.Fatal("the user's bytes survive the refresh")
	}
	// The published file materialized; the base record is clean.
	if _, err := os.Stat(filepath.Join(a, "plans", "goals", "published.md")); err != nil {
		t.Fatal("the published tree materialized")
	}
	rec, exists, err := ReadBase(a)
	if err != nil || !exists || rec.RefreshDue {
		t.Fatalf("the base record completes clean: %+v %v %v", rec, exists, err)
	}
}

func TestRefreshOnlyCompletesADiedRefresh(t *testing.T) {
	a, _ := reconcileBed(t)
	res, err := Open(verbReq(a, "01J5X00000000000000000R020", "mac-a"), "crashed", "Died mid-refresh.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// The crash shape: publication landed, the base says refreshDue
	// with the DURABLY captured snapshot, no files were written.
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteBase(a, BaseRecord{Commit: res.Tip, WrittenAt: "2026-08-21T00:00:00Z", RefreshDue: true, Snapshot: snap.Files}); err != nil {
		t.Fatal(err)
	}
	// An ordinary session refuses while the refresh is pending.
	if _, err := BaseTip(a); err == nil || !strings.Contains(err.Error(), "--refresh-only") {
		t.Fatalf("a pending refresh blocks ordinary reconcile by name: %v", err)
	}
	skipped, err := RefreshOnly(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Fatalf("nothing was user-edited; nothing skips: %v", skipped)
	}
	if _, err := os.Stat(filepath.Join(a, "plans", "goals", "crashed.md")); err != nil {
		t.Fatal("the died refresh completed from the durable record")
	}
	rec, _, _ := ReadBase(a)
	if rec.RefreshDue {
		t.Fatal("the completed refresh clears the pending flag")
	}
	// Idempotent: a second completion finds nothing to do.
	if _, err := RefreshOnly(a); err == nil {
		t.Fatal("a clean base has no pending refresh to complete")
	}
	// A pending record WITHOUT its snapshot refuses by name (the
	// pre-durable shape completes by hand, never by guessing).
	if err := WriteBase(a, BaseRecord{Commit: res.Tip, WrittenAt: "2026-08-21T00:00:00Z", RefreshDue: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshOnly(a); err == nil || !strings.Contains(err.Error(), "no snapshot") {
		t.Fatalf("a snapshotless pending record refuses by name: %v", err)
	}
}

func TestRefreshOnlyResolvesTheCrashedPublishWindow(t *testing.T) {
	a, tip := reconcileBed(t)
	// The hand edit stands in the worktree; the crash fell INSIDE the
	// publish window (Publishing=true, Commit still the BASE).
	editablePath := filepath.Join(a, "plans", "goals", "editable.md")
	edited, err := os.ReadFile(editablePath)
	if err != nil {
		t.Fatal(err)
	}
	handBytes := []byte(strings.Replace(string(edited), "Original intent.", "Hand-edited intent.", 1))
	if err := os.WriteFile(editablePath, handBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}

	// Arm one: the publish NEVER landed (the opid is on no branch).
	ghost := Opid("01J5X00000000000000000GH00", "mac-a", "lin-1")
	if err := WriteBase(a, BaseRecord{Commit: tip, WrittenAt: "2026-08-21T09:00:00Z",
		RefreshDue: true, Publishing: true, Opid: ghost, Snapshot: snap.Files}); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshOnly(a); err == nil || !strings.Contains(err.Error(), "never published") {
		t.Fatalf("an unlanded publish resolves by name (R2-1): %v", err)
	}
	if data, _ := os.ReadFile(editablePath); string(data) != string(handBytes) {
		t.Fatal("the hand edit is UNTOUCHED when the publish never landed — completing from the base would have erased it")
	}
	if rec, _, _ := ReadBase(a); rec.RefreshDue {
		t.Fatal("the resolved window clears the pending flag")
	}

	// Arm two: the publish LANDED — proven with the RECONCILE's OWN
	// opid, not a bystander's (round 3 finding 4 called the old
	// unrelated-trailer shape unproven). A real reconcile publishes
	// the hand edit; the crash is reconstructed as the publishing-
	// shaped record with that reconcile's opid, and the completion
	// must materialize the tree CARRYING the hand edit.
	req := verbReq(a, "01J5X00000000000000000GH10", "mac-a")
	req.Actor.Human = "wido"
	recRes, err := Reconcile(req)
	if err != nil || recRes.Publish.Outcome != OutcomeConfirmed {
		t.Fatalf("the real reconcile lands the hand edit: %+v %v", recRes.Publish, err)
	}
	reconcileOpid := Opid("01J5X00000000000000000GH10", "mac-a", "lin-1")
	if err := WriteBase(a, BaseRecord{Commit: tip, WrittenAt: "2026-08-21T09:00:00Z",
		RefreshDue: true, Publishing: true, Opid: reconcileOpid, Snapshot: snap.Files}); err != nil {
		t.Fatal(err)
	}
	// The crash window's worktree truth: the publish left, the
	// refresh never ran, so the file still carries the CAPTURED hand
	// bytes — that is what the completion lawfully overwrites.
	if err := os.WriteFile(editablePath, handBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshOnly(a); err != nil {
		t.Fatalf("a landed publish completes from its own trailer: %v", err)
	}
	materialized, err := os.ReadFile(editablePath)
	if err != nil {
		t.Fatal("the completion materialized the reconciled file")
	}
	if !strings.Contains(string(materialized), "Hand-edited intent.") {
		t.Fatalf("the completion carries the HAND EDIT the crashed reconcile published: %s", materialized)
	}
	if rec, _, _ := ReadBase(a); rec.RefreshDue {
		t.Fatal("the completed refresh clears the pending flag")
	}

	// A file DELETED after capture is a hand act the completion
	// preserves: named, never resurrected.
	if err := WriteBase(a, BaseRecord{Commit: tip, WrittenAt: "2026-08-21T09:30:00Z",
		RefreshDue: true, Publishing: true, Opid: reconcileOpid, Snapshot: snap.Files}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(editablePath); err != nil {
		t.Fatal(err)
	}
	skipped, err := RefreshOnly(a)
	if err != nil {
		t.Fatalf("the deletion-preserving completion still completes: %v", err)
	}
	if _, statErr := os.Stat(editablePath); !os.IsNotExist(statErr) {
		t.Fatal("a file deleted after capture stays deleted")
	}
	named := false
	for _, p := range skipped {
		if strings.Contains(p, "editable") {
			named = true
		}
	}
	if !named {
		t.Fatalf("the preserved deletion is NAMED: %v", skipped)
	}
}

func TestRefreshPreservesAPostCaptureCreation(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	res, err := Open(verbReq(a, "01J5X00000000000000000RC00", "mac-a"), "fresh-row", "Row.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// The published tree carries fresh-row.md; the capture does NOT
	// (the file appeared locally after capture, bytes of its own).
	p := goalsPrefix + "fresh-row.md"
	abs := filepath.Join(a, filepath.FromSlash(p))
	mine := []byte("# fresh-row\n\nmine, written after capture\n")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, mine, 0o644); err != nil {
		t.Fatal(err)
	}
	skipped, err := Refresh(a, res.Tip, &Snapshot{Files: map[string][]byte{}})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	named := false
	for _, s := range skipped {
		if s == p {
			named = true
		}
	}
	if !named {
		t.Fatalf("the post-capture creation is preserved and NAMED: %v", skipped)
	}
	got, _ := os.ReadFile(abs)
	if string(got) != string(mine) {
		t.Fatalf("the post-capture creation was overwritten: %s", got)
	}
}

func TestRefreshNeverFollowsAPostCaptureSymlink(t *testing.T) {
	a, _ := reconcileBed(t)
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Open(verbReq(a, "01J5X00000000000000000SM00", "mac-a"), "sym-bait", "New goal.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	// Post-capture, the captured file is REPLACED by a symlink to an
	// external copy carrying the same captured bytes: reading through
	// it shows "unchanged", writing through it would mutate the
	// outside file. Identity decides, not content.
	editablePath := filepath.Join(a, "plans", "goals", "editable.md")
	outside := filepath.Join(t.TempDir(), "outside-copy.md")
	if err := os.WriteFile(outside, snap.Files[goalsPrefix+"editable.md"], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(editablePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, editablePath); err != nil {
		t.Fatal(err)
	}
	outsideBefore, _ := os.ReadFile(outside)
	skipped, err := Refresh(a, res.Tip, snap)
	if err != nil {
		t.Fatal(err)
	}
	named := false
	for _, s := range skipped {
		if s == goalsPrefix+"editable.md" {
			named = true
		}
	}
	if !named {
		t.Fatalf("the identity change is preserved and NAMED: %v", skipped)
	}
	outsideAfter, _ := os.ReadFile(outside)
	if string(outsideBefore) != string(outsideAfter) {
		t.Fatal("the refresh wrote THROUGH the symlink to the outside file")
	}
}

func TestVerbsAreImmuneToGitEnvironmentSteering(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	// A hostile-or-accidental environment points at ANOTHER
	// repository and injects a replacement remote URL. The
	// transaction must operate on its --root and its configured
	// remote regardless.
	decoy := filepath.Join(t.TempDir(), "decoy")
	mustGit(t, t.TempDir(), "init", "-q", "-b", "main", decoy)
	t.Setenv("GIT_DIR", filepath.Join(decoy, ".git"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "steered://wrong")
	res, err := Open(verbReq(a, "01J5X00000000000000000EV00", "mac-a"), "steered-not", "Immune.", "main", "Go.")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the steered environment must not move the transaction: %+v %v", res, err)
	}
	// The verification reads through the package's own scrubbed git
	// runner — the TEST process still carries the steering env, and a
	// raw git here would answer for the decoy.
	out, catErr := gitIn(a, "cat-file", "-p", "origin/main:./plans/goals/steered-not.md")
	if catErr != nil || !strings.Contains(out, "Immune.") {
		t.Fatalf("the publish landed on the REAL origin: %v %s", catErr, out)
	}
}
