package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

func TestDispatchFollowUpReadClosesTerminalWork(t *testing.T) {
	repo := os.Getenv("METASYSTEM_FOLLOWUP_CLOSE_FIXTURE_REPO")
	if repo == "" {
		t.Skip("obligation A-13 is discharged by the engine bed rather than by this package test")
	}
	assertDispatchFollowUpCloseFixture(t, repo)
}

func assertDispatchFollowUpCloseFixture(t *testing.T, repo string) {
	t.Helper()
	prefix := os.Getenv("METASYSTEM_FOLLOWUP_CLOSE_FIXTURE_PREFIX")
	evidenceForm := os.Getenv("METASYSTEM_FOLLOWUP_CLOSE_EVIDENCE_FORM")
	refusalOutput := os.Getenv("METASYSTEM_FOLLOWUP_CLOSE_REFUSAL_OUTPUT")
	refusalCode, err := strconv.Atoi(os.Getenv("METASYSTEM_FOLLOWUP_CLOSE_REFUSAL_CODE"))
	if err != nil || prefix == "" || (evidenceForm != "root" && evidenceForm != "child") || refusalOutput == "" {
		t.Fatalf("incomplete follow-up close fixture coordinates: prefix=%q evidence=%q refusal=%q code=%v", prefix, evidenceForm, refusalOutput, err)
	}

	agents := filepath.Join(repo, "artifacts", "agents")
	jobs := filepath.Join(agents, "jobs")
	implementation := prefix + "-work"
	terminal := prefix + "-work-r3"
	critic := prefix + "-critic"
	criticTerminal := prefix + "-critic-r3"
	candidate := prefix + "-redundant"

	refusal, err := os.ReadFile(refusalOutput)
	if err != nil {
		t.Fatal(err)
	}
	wantRecovery := "close --job " + implementation + " --reconcile-evidence " + critic
	if refusalCode != 11 || !strings.Contains(string(refusal), "REDUNDANT_READ") || !strings.Contains(string(refusal), wantRecovery) {
		t.Fatalf("fresh equal read = exit %d output %q", refusalCode, refusal)
	}
	for _, path := range []string{filepath.Join(jobs, candidate+".json"), filepath.Join(agents, candidate)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("redundant read published candidate state %s", path)
		}
	}

	implementationRecord := readFollowUpCloseJSON(t, filepath.Join(jobs, implementation+".json"))
	if implementationRecord["independentCritiqueJobRef"] != critic || implementationRecord["chainClosed"] != true {
		t.Fatalf("implementation close did not stamp and close canonical critic root: %v", implementationRecord)
	}
	mirror, ok := implementationRecord["mirror"].(map[string]any)
	if !ok {
		t.Fatal("closed implementation has no durable mirror pointer")
	}
	manifest := readFollowUpCloseJSON(t, filepath.Join(mirror["path"].(string), "manifest.json"))
	files, ok := manifest["files"].(map[string]any)
	if !ok {
		t.Fatal("implementation mirror has no file inventory")
	}
	for _, relative := range []string{
		"jobs/" + implementation + ".json",
		"jobs/" + implementation + "-r2.json",
		"jobs/" + terminal + ".json",
		"rounds/1/diff.patch", "rounds/2/diff.patch", "rounds/3/diff.patch",
	} {
		if _, present := files[relative]; !present {
			t.Errorf("implementation mirror omitted %s", relative)
		}
	}

	criticRoot := readFollowUpCloseJSON(t, filepath.Join(jobs, critic+".json"))
	closure, present, err := dispatchcore.ReadClosure(criticRoot)
	if err != nil || !present || closure.CriticRoot != critic || closure.Round != 3 || closure.Subject.ReviewedMember != terminal {
		t.Fatalf("critic closure = %+v present=%v err=%v", closure, present, err)
	}
	if criticRoot["chainClosed"] != true {
		t.Fatal("critic root was not closed through the closure writer")
	}
	for _, member := range []string{critic, prefix + "-critic-r2", criticTerminal} {
		record := readFollowUpCloseJSON(t, filepath.Join(jobs, member+".json"))
		if record["reviews"] != implementation {
			t.Errorf("reconciliation changed immutable reviews on %s to %v", member, record["reviews"])
		}
	}

	review := readFollowUpCloseJSON(t, filepath.Join(agents, implementation, "rounds", "3", "review.json"))
	patch, err := os.ReadFile(filepath.Join(agents, implementation, "rounds", "3", "diff.patch"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(patch)
	if closure.Subject.Kind != dispatchcore.SubjectLive ||
		closure.Subject.ImplementerRoot != implementation ||
		closure.Subject.ReviewedProjectTree != review["reviewedTree"] ||
		closure.Subject.DiffDigest != hex.EncodeToString(digest[:]) {
		t.Fatalf("closure subject does not bind terminal work artifacts: %+v", closure.Subject)
	}
}

func readFollowUpCloseJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return value
}
