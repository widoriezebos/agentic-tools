package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFMA_R2_FollowupCanonicalRelay(t *testing.T) {
	dir := t.TempDir()
	parent := writeJSONFile(t, dir, "legacy-parent.json", map[string]any{
		"role": "implementer", "mission": nil, "runtime": "fake", "reviews": nil,
		"workspaceRoot": dir, "baseSha": "base", "branch": "main", "machineId": nil,
		"permissions":    map[string]any{"requested": map[string]any{}},
		"requestedModel": "fake-source", "destructiveReach": "MECHANICAL",
	})
	capResolution := writeJSONFile(t, dir, "cap.json", map[string]any{
		"capMin": 30, "capDeadline": nil,
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	output := filepath.Join(dir, "child.json")
	err := BuildFollowRecord(BuildFollowRecordParams{
		Output: output, Parent: parent, Job: "legacy-r2", OperationID: "legacy-r2", Round: 2,
		ParentJob: "legacy", Fallbacks: "[]", ResumeMode: "fresh-context",
		CapResolution: capResolution, Model: "fake-model", AliasedFrom: "fake-source",
		DestructiveReach: HazardMechanical, LaunchMode: LaunchModeSharedCheckout,
		OutputStream: filepath.Join(dir, "child.jsonl"),
	})
	if err != nil {
		t.Fatal(err)
	}
	record := readJSONFile(t, output)
	if record["requestedModel"] != "fake-model" {
		t.Fatalf("requestedModel = %v, want canonical parameter fake-model", record["requestedModel"])
	}
	if record["aliasedFrom"] != "fake-source" || record["rosterAliasedFrom"] != nil {
		t.Fatalf("alias provenance = (%v, %v), want (fake-source, null)", record["aliasedFrom"], record["rosterAliasedFrom"])
	}
}

func TestBuildRecordAliasProvenance(t *testing.T) {
	root := sandbox(t)
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "fixture"}, {"config", "user.email", "fixture@example.invalid"}} {
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "tracked"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "tracked"}, {"commit", "-qm", "fixture"}} {
		if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	dir := t.TempDir()
	permissions := writeJSONFile(t, dir, "permissions.json", map[string]any{})
	capResolution := writeJSONFile(t, dir, "cap.json", map[string]any{
		"capMin": 30, "capDeadline": nil,
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	output := filepath.Join(dir, "record.json")
	err := BuildRecord(BuildRecordParams{
		Output: output, Job: "alias-record", Role: "implementer", Root: root,
		Runtime: "fake", Workspace: root, CapResolution: capResolution, Model: "fake-model",
		AliasedFrom: "fake-source", RosterAliasedFrom: "fake-roster-source",
		Permissions: permissions, Fallbacks: "[]", ReasoningEffort: "medium",
		DestructiveReach: HazardMechanical, LaunchMode: LaunchModeSharedCheckout,
		OutputStream: filepath.Join(dir, "record.jsonl"),
	})
	if err != nil {
		t.Fatal(err)
	}
	record := readJSONFile(t, output)
	if record["aliasedFrom"] != "fake-source" || record["rosterAliasedFrom"] != "fake-roster-source" {
		t.Fatalf("record alias provenance = (%v, %v)", record["aliasedFrom"], record["rosterAliasedFrom"])
	}
}
