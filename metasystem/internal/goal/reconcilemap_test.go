package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// editFile rewrites one materialized goal file through a transform.
func editFile(t *testing.T, root, rel string, transform func(*GoalFile)) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	f, problems := ParseFile(data)
	if len(problems) > 0 {
		t.Fatalf("fixture parse: %v", problems)
	}
	transform(f)
	if err := os.WriteFile(abs, RenderFile(f), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandEditsMapToTheSmallestVerbSet(t *testing.T) {
	a, tip := reconcileBed(t)
	// One file: a state change (queued → parked) AND a field change
	// (next step) — the pinned precedence maps the state verb first,
	// then ONE edit.
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.State = StateParked
		f.Parked = &ParkRecord{Because: "waiting on review"}
		f.NextStep = "Amended next."
	})
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := MapDeltas(a, tip, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("state verb + one edit: %+v", rows)
	}
	kinds := map[string]bool{}
	for _, row := range rows {
		kinds[row.Verb] = true
		if row.Id != "editable" {
			t.Fatalf("one goal, its own rows: %+v", row)
		}
	}
	if !kinds["park"] || !kinds["edit"] {
		t.Fatalf("the decomposition names park and edit: %+v", rows)
	}
}

func TestGeneratedFieldTamperRefusesByFileAndField(t *testing.T) {
	a, tip := reconcileBed(t)
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.Revision = 99
	})
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = MapDeltas(a, tip, snap)
	if err == nil || !strings.Contains(err.Error(), "editable.md") || !strings.Contains(err.Error(), "Revision") {
		t.Fatalf("a tampered generated field refuses by file and field: %v", err)
	}
}

func TestHandCreatedFileMapsToOpen(t *testing.T) {
	a, tip := reconcileBed(t)
	created := &GoalFile{
		Id: "hand-opened", State: StateQueued,
		Intent: "Opened in an editor.", Origin: "human", NextStep: "Start.",
	}
	if err := os.WriteFile(filepath.Join(a, "plans", "goals", "hand-opened.md"), RenderFile(created), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := MapDeltas(a, tip, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Verb != "open" || rows[0].Id != "hand-opened" {
		t.Fatalf("a hand-created file maps to open: %+v", rows)
	}
	if *rows[0].Fields.Intent != "Opened in an editor." {
		t.Fatalf("the human fields ride the open: %+v", rows[0].Fields)
	}
}

func TestHandDeletionIsUnmappable(t *testing.T) {
	a, tip := reconcileBed(t)
	if err := os.Remove(filepath.Join(a, "plans", "goals", "editable.md")); err != nil {
		t.Fatal(err)
	}
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = MapDeltas(a, tip, snap)
	if err == nil || !strings.Contains(err.Error(), "done and prune are verbs") {
		t.Fatalf("hand deletion refuses toward the verbs: %v", err)
	}
}

func TestWhitespaceOnlyChangeNamesTheClosedSurface(t *testing.T) {
	a, tip := reconcileBed(t)
	abs := filepath.Join(a, "plans", "goals", "editable.md")
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = MapDeltas(a, tip, snap)
	if err == nil || !strings.Contains(err.Error(), "surface is closed") {
		t.Fatalf("bytes without a field refuse by name: %v", err)
	}
}

func TestFullArcHandParkMapsToOneCascade(t *testing.T) {
	_, a, _ := twoClones(t)
	seedLedger(t, a)
	for i, id := range []string{"harc-one", "harc-two"} {
		ulid := []string{"01J5X00000000000000000H000", "01J5X00000000000000000H010"}[i]
		if res, err := Open(verbReq(a, ulid, "mac-a"), id, "Arc "+id, "main", "Go."); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("open %s: %+v %v", id, res, err)
		}
		arcUlid := []string{"01J5X00000000000000000H020", "01J5X00000000000000000H030"}[i]
		if res, err := SetArc(verbReq(a, arcUlid, "mac-a"), id, "hand-arc"); err != nil || res.Outcome != OutcomeConfirmed {
			t.Fatalf("set-arc %s: %+v %v", id, res, err)
		}
	}
	p, err := Project(endpointFor(a), true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	materialize(t, a, p.Tip)

	// The full-arc park: identical deltas across BOTH live members.
	for _, id := range []string{"harc-one", "harc-two"} {
		editFile(t, a, goalsPrefix+id+".md", func(f *GoalFile) {
			f.State = StateParked
			f.Parked = &ParkRecord{By: "human:wido", At: "2026-08-21T00:00:00Z", Because: "the whole track pauses"}
		})
	}
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := MapDeltas(a, p.Tip, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Verb != "park" || len(rows[0].ArcIds) != 2 {
		t.Fatalf("identical parks across the whole arc map to ONE cascade: %+v", rows)
	}

	// The partial-arc park refuses: revert one member.
	editFile(t, a, goalsPrefix+"harc-two.md", func(f *GoalFile) {
		f.State = StateQueued
		f.Parked = nil
	})
	snap2, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = MapDeltas(a, p.Tip, snap2)
	if err == nil || !strings.Contains(err.Error(), "all-or-none") {
		t.Fatalf("a partial-arc hand-park refuses: %v", err)
	}
}

func TestHandGrammarRefusalArms(t *testing.T) {
	a, tip := reconcileBed(t)

	// Root-record hand edits have no grammar.
	rootPath := filepath.Join(a, "plans", "goals", "backlog.md")
	data, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rootPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MapDeltas(a, tip, snap); err == nil || !strings.Contains(err.Error(), "declare-free and prune are verbs") {
		t.Fatalf("the root record refuses toward its verbs: %v", err)
	}
	if err := os.WriteFile(rootPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// OpenedAt is generated; Claimed is generated; Origin is off the
	// surface; Concluded belongs to done — each refuses by field. (An
	// arc change MAPS — TestHandArcMoveMapsToItsVerbs owns that leg.)
	for _, leg := range []struct {
		name      string
		transform func(*GoalFile)
		fragment  string
	}{
		{"openedat", func(f *GoalFile) { f.OpenedAt = "1999-01-01T00:00:00Z" }, "OpenedAt"},
		{"claimed", func(f *GoalFile) {
			f.Claimed = &ClaimRecord{Machine: "mac-x", Lineage: "l9", At: "2026-08-21T00:00:00Z"}
		}, "Claimed"},
		{"origin", func(f *GoalFile) { f.Origin = "smuggled-origin" }, "not on the closed edit surface"},
		{"conclude", func(f *GoalFile) { f.Conclude = "edited in place" }, "Concluded belongs to done"},
	} {
		editFile(t, a, goalsPrefix+"editable.md", leg.transform)
		snap, err := CaptureSnapshot(a)
		if err != nil {
			t.Fatal(err)
		}
		_, err = MapDeltas(a, tip, snap)
		if err == nil || !strings.Contains(err.Error(), leg.fragment) {
			t.Fatalf("%s: refusal names the field: %v", leg.name, err)
		}
		// Restore for the next leg.
		published, _ := ReadCommitGoals(a, tip)
		if err := os.WriteFile(filepath.Join(a, "plans", "goals", "editable.md"), published[goalsPrefix+"editable.md"], 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// A hand-done maps; a hand-done without its conclusion refuses.
	// Raw bytes here: the fixture helper's own strict parse would
	// refuse the invalid intermediate before the mapper could.
	published, _ := ReadCommitGoals(a, tip)
	raw := strings.Replace(string(published[goalsPrefix+"editable.md"]), "- State: queued", "- State: done", 1)
	if err := os.WriteFile(filepath.Join(a, "plans", "goals", "editable.md"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	snap3, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MapDeltas(a, tip, snap3); err == nil || !strings.Contains(err.Error(), "Concluded") {
		t.Fatalf("a hand-done needs its conclusion: %v", err)
	}
	withConclude := strings.Replace(raw, "- State: done", "- State: done\n- Concluded: Hand-concluded.", 1)
	if err := os.WriteFile(filepath.Join(a, "plans", "goals", "editable.md"), []byte(withConclude), 0o644); err != nil {
		t.Fatal(err)
	}
	snap4, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := MapDeltas(a, tip, snap4)
	if err != nil || len(rows) != 1 || rows[0].Verb != "done" || rows[0].Conclude != "Hand-concluded." {
		t.Fatalf("a hand-done with its conclusion maps: %+v %v", rows, err)
	}
}

func TestUnparkAndLenientDisplacedRoundTrip(t *testing.T) {
	a, _ := reconcileBed(t)
	// Park through the verb so the base carries a parked state, then
	// re-materialize and hand-unpark.
	res, err := Park(verbReq(a, "01J5X00000000000000000H100", "mac-a"), "editable", "pausing")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	materialize(t, a, res.Tip)
	editFile(t, a, goalsPrefix+"editable.md", func(f *GoalFile) {
		f.State = StateQueued
		f.Parked = nil
	})
	snap, err := CaptureSnapshot(a)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := MapDeltas(a, res.Tip, snap)
	if err != nil || len(rows) != 1 || rows[0].Verb != "unpark" {
		t.Fatalf("a hand-unpark maps: %+v %v", rows, err)
	}
	// handLenient's displaced extraction survives a rebuild.
	line := handLenient([]byte("- Parked: by= at= displaced=mac-b+l1@2026-08-21T00:00:00Z because=held\n"))
	if !strings.Contains(string(line), "displaced=mac-b+l1@2026-08-21T00:00:00Z") ||
		!strings.Contains(string(line), "because=held") {
		t.Fatalf("the lenient rebuild keeps displaced and because: %s", line)
	}
}
