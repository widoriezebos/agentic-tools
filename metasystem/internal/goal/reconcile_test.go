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
